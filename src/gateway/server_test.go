package gateway

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"
	"zeno/pb"
	"zeno/src/config"
	"zeno/src/controlplane"
	"zeno/src/node"
	"zeno/src/utils"

	"google.golang.org/grpc"
)

func TestGatewayRoutesToIndependentNode(t *testing.T) {
	nodeListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	pb.RegisterNodeServiceServer(server, &node.NodeServer{})
	go server.Serve(nodeListener)
	defer server.Stop()
	cfg := config.Default()
	addr := nodeListener.Addr().String()
	cluster, err := controlplane.New(config.ClusterConfig{Leader: addr, Nodes: []config.NodeConfig{{IP: addr, State: "read-write"}}}, cfg.RPC)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, listener, cluster, utils.NewQueue[utils.Message]()) }()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	// Pipeline commands in one write to exercise reuse of the buffered RESP reader.
	_, err = conn.Write([]byte("*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$4\r\nzeno\r\n*2\r\n$3\r\nGET\r\n$4\r\nname\r\n*1\r\n$7\r\nUNKNOWN\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	for _, want := range []string{"+OK\r\n", "$4\r\n", "zeno\r\n", "-ERR invalid command\r\n"} {
		got, err := reader.ReadString('\n')
		if err != nil || got != want {
			t.Fatalf("response = %q, %v; want %q", got, err, want)
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("gateway failed to stop")
	}
}
