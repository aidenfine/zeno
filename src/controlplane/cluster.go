package controlplane

import (
	"errors"
	"log/slog"
	"net"
	"sort"
	"sync"
	"zeno/src/resp"
	"zeno/src/utils"

	"zeno/pb"
)

// Cluster holds cluster membership and coordination policy: who the leader
// is, which peers exist, and whether *this* process should be acting as
// coordinator. This is the control-plane view of the cluster. Outbound RPC
// mechanics live in peerClient (client.go); the data-plane server that
// applies commands lives in the node package (node/server.go).

const (
	NoReadOrWriteState = 0
	ReadState          = 1
	WriteState         = 2
	ReadAndWriteState  = 3
	DownState          = 4
)

type Cluster struct {
	leader        string   // ip
	nodes         []string // list of node ips (excludes the leader)
	ipToContainer map[string]string
	nodeState     map[string]int // ip: node state (0 - no read or write, 1 - read, 2 - write, 3 - read and write (normal state), 4 - node is down)
	rrCounter     uint64         // round-robin cursor for read routing
	mu            sync.Mutex
}

// hard coded for now but set a1 to leader
func New() (*Cluster, error) {
	nodes := &Cluster{
		leader: "10.10.1.10",
		nodes:  []string{"10.10.1.11", "10.10.2.10", "10.10.2.11"},
		ipToContainer: map[string]string{
			"10.10.1.10": "zeno-a1",
			"10.10.1.11": "zeno-a2",
			"10.10.2.10": "zeno-b1",
			"10.10.2.11": "zeno-b2",
		},
		nodeState: map[string]int{
			"10.10.1.10": ReadAndWriteState,
			"10.10.1.11": ReadAndWriteState,
			"10.10.2.10": WriteState,
			"10.10.2.11": ReadState,
		},
		mu: sync.Mutex{},
	}

	return nodes, nil
}

// IsLeader reports whether this process is the leader, by checking whether
// the leader ip is bound to one of our local interfaces. Only the leader
// should drive heartbeats/coordination — otherwise every node pings every
// other node and heartbeats multiply by the cluster size.
func (n *Cluster) IsLeader() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		slog.Error("failed to read local interface addresses", "error", err)
		return false
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.String() == n.leader {
			return true
		}
	}
	return false
}

// ReconcileHealth diffs this round's heartbeat failures against the last
// known state and returns the nodes that just recovered (were down, now
// responding). Newly failed nodes are recorded as down. This is what turns a
// stateless per-round heartbeat into up/down *transition* detection.
func (n *Cluster) ReconcileHealth(failedNodes []string) (recovered []string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	failedSet := make(map[string]bool, len(failedNodes))
	for _, ip := range failedNodes {
		failedSet[ip] = true
		if n.nodeState[ip] != DownState {
			slog.Warn("node went down", "node", ip)
			n.nodeState[ip] = DownState
		}
	}

	for ip := range n.nodeState {
		if !failedSet[ip] { // was down, responded this round
			slog.Info("node recovered", "node", ip)
			recovered = append(recovered, ip)
			n.nodeState[ip] = ReadAndWriteState
		}
	}
	return recovered
}

// RestartNodes takes a list of node ips (strings) and will wait for these nodes to come back to life.
// If node B goes down RestartNode is responsible for making sure node B is up to date when node B comes
// back up
func (n *Cluster) RestartNode(node string, queuedMessages *utils.Queue[utils.Message]) error {
	// make sure node is in readonly
	if n.nodeState[node] != 3 {
		// send command to update node state to other
		slog.Info("node not in correct state", "nodeState", n.nodeState[node])
	}

	slog.Info("Starting restart node process", "queue_len", queuedMessages.QueueLength())
	for !queuedMessages.IsEmpty() {
		item, _ := queuedMessages.Dequeue()

		res, err := newPeerClient(node).forwardCommand(item.Command, item.Args)
		if err != nil {
			slog.Error("failed to get response", "error", err)
			return err
		}
		slog.Info("res from forwardcommand RestartNode()", "res", res)
	}

	// TODO: we still need to sync from the main items, how do we ensure that the node stays up to date now.
	// example if node cannot be written to by the main args, since its being sync'd do we need to queue the live feed again?
	return nil

}

func (n *Cluster) SendHeartbeat() ([]string, error) {
	// check leader node first
	// TODO: handle leader failure later, do we vote a new leader?
	result, err := newPeerClient(n.leader).heartbeat()
	if err != nil {
		return nil, err
	}
	if result.Response != "OK" {
		slog.Info("leader node responded with failure", "error", result.Response)
		return nil, errors.New("leader failed health check")
	}

	var (
		mu          sync.Mutex
		wg          sync.WaitGroup
		failedNodes = []string{}
	)
	for _, node := range n.nodes {
		wg.Add(1)
		go func(node string) {
			defer wg.Done()
			resp, err := newPeerClient(node).heartbeat()
			slog.Info("heartbeat resp", "resp", resp)
			if err != nil || resp.Response != "OK" {
				mu.Lock()
				failedNodes = append(failedNodes, node)
				mu.Unlock()
			}
		}(node)
	}
	wg.Wait()
	return failedNodes, nil
}

// writeCommands classifies which verbs mutate state. Everything else is
// treated as a read and can be served by any read-capable replica.
var writeCommands = map[string]bool{
	"SET":  true,
	"HSET": true,
}

func isWrite(command string) bool {
	return writeCommands[command]
}

// SendCommand is the single client-facing entry point: the client talks to
// the control plane and we decide where the command goes. Writes go to the
// leader (source of truth) and replicate to healthy followers; reads are
// served by any healthy read-capable replica.
func (n *Cluster) SendCommand(command string, args []resp.Value) (*pb.ForwardCommandResponse, error) {
	if isWrite(command) {
		return n.routeWrite(command, args)
	}
	return n.routeRead(command, args)
}

// routeWrite applies the write on the leader, then asynchronously replicates
// it to every follower that is currently write-capable. The leader's result
// is what the client sees; replication is fire-and-forget.
func (n *Cluster) routeWrite(command string, args []resp.Value) (*pb.ForwardCommandResponse, error) {
	result, err := newPeerClient(n.leader).forwardCommand(command, args)
	if err != nil {
		return nil, err
	}

	for _, node := range n.nodes {
		// skip leader node.
		if node == n.leader {
			continue
		}
		n.mu.Lock()
		state := n.nodeState[node]
		n.mu.Unlock()

		if state != ReadAndWriteState && state != WriteState {
			slog.Info("skipping replication due to state", "node", n.ipToContainer[node], "state", state)
			continue
		}

		go func(node string) {
			if _, err := newPeerClient(node).forwardCommand(command, args); err != nil {
				slog.Error("replication failed", "node", node, "error", err)
			}
		}(node)
	}

	return result, nil
}

// routeRead picks one healthy read-capable node and serves the read from it.
func (n *Cluster) routeRead(command string, args []resp.Value) (*pb.ForwardCommandResponse, error) {
	target := n.pickReadNode()
	if target == "" {
		return nil, errors.New("no read-capable node available")
	}
	return newPeerClient(target).forwardCommand(command, args)
}

// pickReadNode round-robins across all read-capable nodes (leader included).
// Sorting gives a stable order so the round-robin counter is meaningful even
// though nodeState iteration order is random.
func (n *Cluster) pickReadNode() string {
	n.mu.Lock()
	defer n.mu.Unlock()

	candidates := make([]string, 0, len(n.nodeState))
	for ip, state := range n.nodeState {
		if state == ReadAndWriteState || state == ReadState {
			candidates = append(candidates, ip)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)

	target := candidates[n.rrCounter%uint64(len(candidates))]
	n.rrCounter++
	return target
}

func (n *Cluster) printNodes() {
	for _, v := range n.nodes {
		slog.Info("node", "address", v)
	}
}
