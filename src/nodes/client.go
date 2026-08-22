package nodes

import (
	"context"
	"log/slog"
	"time"
	"zeno/pb"
	"zeno/src/resp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// peerClient handles outbound gRPC calls to a single peer node.
// It knows *how* to talk to a peer, not *whether* we should — that
// policy lives in the cluster layer (see cluster.go).
type peerClient struct {
	addr string
}

func newPeerClient(addr string) *peerClient {
	return &peerClient{addr: addr}
}

func (p *peerClient) dial() (*grpc.ClientConn, pb.NodeServiceClient, error) {
	conn, err := grpc.NewClient(p.addr+":6380", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to node client", "error", err)
		return nil, nil, err
	}
	return conn, pb.NewNodeServiceClient(conn), nil
}

func (p *peerClient) heartbeat() (*pb.NodeHeartbeatResponse, error) {
	conn, client, err := p.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

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

func (p *peerClient) forwardCommand(command string, args []resp.Value) (*pb.ForwardCommandResponse, error) {
	conn, client, err := p.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

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
