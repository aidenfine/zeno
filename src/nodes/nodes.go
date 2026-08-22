package nodes

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
	"zeno/pb"
	"zeno/src/resp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// should we use a struct to keep track of current leader, node ips,
//
//
// What should nodes.go support
// electing new leader
// leader should take the validated command
// leader should fan out the command to the other nodes
//
//
//

type Nodes struct {
	leader string   // ip
	nodes  []string // list of ips

}

var nodes = []string{
	"b1",
	"b2",
	"a1",
}

// hard coded for now but set a1 to leader
func MakeNodes() (*Nodes, error) {
	nodes := &Nodes{
		leader: "10.10.1.10",
		nodes:  []string{"10.10.1.11", "10.10.2.10", "10.10.2.11"},
	}

	return nodes, nil
}

func (n *Nodes) SendHeartbeat() ([]string, error) {
	// check leader node first
	// TODO: handle leader failure later, do we vote a new leader?
	result, err := n.sendHeartbeatToNode(n.leader)
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
			resp, err := n.sendHeartbeatToNode(node)
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

func (n *Nodes) sendHeartbeatToNode(node string) (*pb.NodeHeartbeatResponse, error) {
	conn, err := grpc.NewClient(node+":6380", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to node client", "error", err)
		return nil, err
	}
	defer conn.Close()

	client := pb.NewNodeServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := client.Heartbeat(ctx, &pb.NodeHeartbeatRequest{
		Ping: "PING",
	})
	if err != nil {
		slog.Error("failed to send heartbeat to node", "error", err)
		return nil, err
	}
	return resp, nil

}

func (n *Nodes) SendCommand(command string, args []resp.Value) (*pb.ForwardCommandResponse, error) {
	// send to leader.
	result, err := n.sendToNode(command, args, n.leader)
	if err != nil {
		return nil, err
	}

	// fan out to other nodes
	for _, node := range n.nodes {
		go func(node string) {
			n.sendToNode(command, args, node)
		}(node)
	}

	return result, nil

}
func (n *Nodes) sendToNode(command string, args []resp.Value, node string) (*pb.ForwardCommandResponse, error) {

	conn, err := grpc.NewClient(node+":6380", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to node client", "error", err)
		return nil, err
	}
	defer conn.Close()

	client := pb.NewNodeServiceClient(conn)

	strArgs := make([]string, len(args))
	for i, a := range args {
		strArgs[i] = a.Bulk
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := client.ForwardCommand(ctx, &pb.ForwardCommandRequest{
		Command: command,
		Args:    strArgs,
	})
	if err != nil {
		slog.Error("failed to send message to node", "error", err)
		return nil, err
	}
	return resp, nil

}

func (n *Nodes) printNodes() {
	for _, v := range n.nodes {
		slog.Info("node", "address", v)
	}
}
