package nodes

import (
	"context"
	"fmt"
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
		return nil, err
	}
	defer conn.Close()

	client := pb.NewNodeServiceClient(conn)

	strArgs := make([]string, len(args))
	for i, a := range args {
		strArgs[i] = a.Bulk
	}
	resp, err := client.ForwardCommand(context.Background(), &pb.ForwardCommandRequest{
		Command: command,
		Args:    strArgs,
	})
	return resp, err

}

func (n *Nodes) printNodes() {
	for _, v := range n.nodes {
		fmt.Println("Node %s", v)
	}
}
