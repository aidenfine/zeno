package main

import (
	"context"
	"flag"
	"github.com/aidenfine/flog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"zeno/pb"
	"zeno/src/logger"
	"zeno/src/node"

	"google.golang.org/grpc"
)

// This executable deliberately has no dependency on the coordinator or gateway.
func main() {
	listen := flag.String("listen", ":6380", "internal node RPC listen address")
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "zeno-node"
	}
	name := flag.String("name", hostname, "node name used as the log service name (defaults to hostname)")
	flag.Parse()
	logger.Init(*name)
	defer logger.Log.Close()
	l, err := net.Listen("tcp", *listen)
	if err != nil {
		logger.Log.Error("node listen failed", flog.Any("error", err))
		os.Exit(1)
	}
	server := grpc.NewServer()
	pb.RegisterNodeServiceServer(server, &node.NodeServer{})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); server.Stop() }()
	logger.Log.Info("storage node listening", flog.Any("address", l.Addr()))
	if err := server.Serve(l); err != nil {
		logger.Log.Error("node failed", flog.Any("error", err))
		os.Exit(1)
	}
}
