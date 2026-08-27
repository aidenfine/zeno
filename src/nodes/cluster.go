package nodes

import (
	"errors"
	"log/slog"
	"net"
	"sync"
	"zeno/src/resp"

	"zeno/pb"
)

// Nodes holds cluster membership and coordination policy: who the leader
// is, which peers exist, and whether *this* process should be acting as
// coordinator. Outbound RPC mechanics live in peerClient (client.go);
// inbound RPC handling lives in NodeServer (node_server.go).
type Nodes struct {
	leader        string   // ip
	nodes         []string // list of node ips (excludes the leader)
	ipToContainer map[string]string
}

// hard coded for now but set a1 to leader
func MakeNodes() (*Nodes, error) {
	nodes := &Nodes{
		leader: "10.10.1.10",
		nodes:  []string{"10.10.1.11", "10.10.2.10", "10.10.2.11"},
		ipToContainer: map[string]string{
			"10.10.1.10": "zeno-a1",
			"10.10.1.11": "zeno-a2",
			"10.10.2.10": "zeno-b1",
			"10.10.2.11": "zeno-b2",
		},
	}

	return nodes, nil
}

// IsLeader reports whether this process is the leader, by checking whether
// the leader ip is bound to one of our local interfaces. Only the leader
// should drive heartbeats/coordination — otherwise every node pings every
// other node and heartbeats multiply by the cluster size.
func (n *Nodes) IsLeader() bool {
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

// RestartNodes takes a list of node ips (strings) and will wait for these nodes to come back to life.
// If node B goes down RestartNode is responsible for making sure node B is up to date when node B comes
// back up
// func (n *Nodes) RestartNodes(nodes []string) []string {
// 	deadNodes := []string{}

// 	for i := range len(nodes) {
// 		containerName, exists := n.ipToContainer[nodes[i]]
// 		if !exists {
// 			slog.Error("ip does not exist in container", "ip", nodes[i])
// 			continue
// 		}

// 		cmd := exec.Command("docker", "restart", containerName)
// 		_, err := cmd.Output()
// 		if err != nil {
// 			slog.Error("failed to restart container", "container", containerName, "error", err)
// 			deadNodes = append(deadNodes, nodes[i])
// 		}
// 	}
// 	return deadNodes

// }

func (n *Nodes) SendHeartbeat() ([]string, error) {
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

func (n *Nodes) SendCommand(command string, args []resp.Value) (*pb.ForwardCommandResponse, error) {
	// send to leader.
	result, err := newPeerClient(n.leader).forwardCommand(command, args)
	if err != nil {
		return nil, err
	}

	// fan out to other nodes
	for _, node := range n.nodes {
		go func(node string) {
			newPeerClient(node).forwardCommand(command, args)
		}(node)
	}

	return result, nil
}

func (n *Nodes) printNodes() {
	for _, v := range n.nodes {
		slog.Info("node", "address", v)
	}
}
