package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"zeno/pb"
	"zeno/src/node"

	"google.golang.org/grpc"
)

// This executable deliberately has no dependency on the coordinator or gateway.
func main() {
	listen := flag.String("listen", ":6380", "internal node RPC listen address")
	flag.Parse()
	l, err := net.Listen("tcp", *listen)
	if err != nil {
		slog.Error("node listen failed", "error", err)
		os.Exit(1)
	}
	server := grpc.NewServer()
	pb.RegisterNodeServiceServer(server, &node.NodeServer{})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); server.Stop() }()
	slog.Info("storage node listening", "address", l.Addr())
	if err := server.Serve(l); err != nil {
		slog.Error("node failed", "error", err)
		os.Exit(1)
	}
}
