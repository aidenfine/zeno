package nodes

import (
	"context"
	"log/slog"
	"zeno/pb"
	"zeno/src/handler"
	"zeno/src/resp"
)

type NodeServer struct {
	pb.UnimplementedNodeServiceServer
}

func (s *NodeServer) Heartbeat(ctx context.Context, req *pb.NodeHeartbeatRequest) (*pb.NodeHeartbeatResponse, error) {
	slog.Info("received heartbeat", "ping", req.Ping)
	return &pb.NodeHeartbeatResponse{
		Response: "OK",
	}, nil
}

func (s *NodeServer) ForwardCommand(ctx context.Context, req *pb.ForwardCommandRequest) (*pb.ForwardCommandResponse, error) {
	handlerFunc, ok := handler.Handlers[req.Command]
	if !ok {
		return &pb.ForwardCommandResponse{Success: false, Error: "invalid command"}, nil
	}

	args := make([]resp.Value, len(req.Args))
	for i, a := range req.Args {
		args[i] = resp.Value{Type: "bulk", Bulk: a}
	}

	res := handlerFunc(args)
	result := res.Bulk
	if result == "" {
		result = res.Str
	}
	return &pb.ForwardCommandResponse{Success: true, Result: result, Type: res.Type}, nil
}
