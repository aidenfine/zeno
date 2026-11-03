package controlplane

import (
	"google.golang.org/grpc"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"zeno/pb"
	"zeno/src/config"
	"zeno/src/node"
	"zeno/src/resp"
)

type countingListener struct {
	net.Listener
	accepted atomic.Int64
}

func (l *countingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil {
		l.accepted.Add(1)
	}
	return conn, err
}

func TestConcurrentCommandsAndHeartbeatsReuseConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	counted := &countingListener{Listener: listener}
	server := grpc.NewServer()
	pb.RegisterNodeServiceServer(server, &node.NodeServer{})
	go server.Serve(counted)
	defer server.Stop()
	cfg := config.Default()
	addr := listener.Addr().String()
	cluster, err := New(config.ClusterConfig{Leader: addr, Nodes: []config.NodeConfig{{IP: addr, State: "read-write"}}}, cfg.RPC)
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Close()
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				result, err := cluster.SendCommand("PING", []resp.Value{})
				if err != nil || result.Result != "PONG" {
					t.Errorf("PING: %v, %v", result, err)
				}
			} else {
				result, err := cluster.peer(addr).heartbeat()
				if err != nil || result.Response != "OK" {
					t.Errorf("heartbeat: %v, %v", result, err)
				}
			}
		}()
	}
	wg.Wait()
	if got := counted.accepted.Load(); got != 1 {
		t.Fatalf("opened %d TCP connections for concurrent RPCs; want 1", got)
	}
	if cluster.peer(addr).timeout != 2*time.Second {
		t.Fatal("configured timeout lost")
	}
}
