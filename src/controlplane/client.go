package controlplane

import (
	"context"
	"log/slog"
	"net"
	"strconv"
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
	addr    string
	timeout time.Duration
	conn    *grpc.ClientConn
	client  pb.NodeServiceClient
}

func newPeerClient(addr string, port int, timeout time.Duration) (*peerClient, error) {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, strconv.Itoa(port))
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &peerClient{addr: addr, timeout: timeout, conn: conn, client: pb.NewNodeServiceClient(conn)}, nil
}

func (p *peerClient) heartbeat() (*pb.NodeHeartbeatResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	resp, err := p.client.Heartbeat(ctx, &pb.NodeHeartbeatRequest{
		Ping: "PING",
	})
	if err != nil {
		slog.Error("failed to send heartbeat to node", "error", err)
		return nil, err
	}
	return resp, nil
}

func (p *peerClient) forwardCommand(command string, args []resp.Value) (*pb.ForwardCommandResponse, error) {
	strArgs := make([]string, len(args))
	for i, a := range args {
		strArgs[i] = a.Bulk
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	resp, err := p.client.ForwardCommand(ctx, &pb.ForwardCommandRequest{
		Command: command,
		Args:    strArgs,
	})
	if err != nil {
		slog.Error("failed to send message to node", "error", err)
		return nil, err
	}
	return resp, nil
}
