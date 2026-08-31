package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
	"zeno/pb"
	"zeno/src/aof"
	"zeno/src/controlplane"
	"zeno/src/node"
	"zeno/src/resp"
	"zeno/src/utils"
	"zeno/src/writer"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// gRPC taskServer
type taskServer struct {
	pb.UnimplementedTaskServiceServer
	mu      sync.Mutex
	tasks   []*pb.Task
	counter int
}

func (s *taskServer) SendTask(ctx context.Context, req *pb.SendTaskRequest) (*pb.SendTaskResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task := req.GetTask()
	s.counter++
	task.Id = fmt.Sprintf("%d", s.counter)
	s.tasks = append(s.tasks, task)

	slog.Info("gRPC: received task", "id", task.Id, "title", task.Title)
	return &pb.SendTaskResponse{Success: true}, nil
}

// main in-mem db goes like this
// Client -> TCP Request -> RESP deserialze -> commands hander -> RESP serialze -> Response

func main() {
	n, err := controlplane.New()
	messageQueue := utils.NewQueue[utils.Message]()
	if err != nil {
		panic("Failed to make nodes")
	}

	// grpc stuff
	go func() {
		lis, err := net.Listen("tcp", ":6380")
		if err != nil {
			slog.Error("gRPC listen error", "error", err)
			return
		}
		grpcServer := grpc.NewServer()
		pb.RegisterTaskServiceServer(grpcServer, &taskServer{})
		pb.RegisterNodeServiceServer(grpcServer, &node.NodeServer{})
		reflection.Register(grpcServer)
		slog.Info("gRPC server running", "port", 6380)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC serve error", "error", err)
		}
	}()

	slog.Info("Running", "port", 6379)
	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		slog.Error("listen error", "error", err)
		return
	}

	// Only the leader drives heartbeats. Without this guard every node
	// coordinates, so heartbeats multiply by the cluster size.
	heartbeatStopChan := make(chan struct{})
	if n.IsLeader() {
		utils.RunOnInterval(30*time.Second, heartbeatStopChan, n.SendHeartbeat, func(failedNodes []string) {
			if len(failedNodes) > 0 {
				slog.Warn("nodes failed heartbeat", "nodes", failedNodes)
			}
			// Diff this round against known state; any node that was down and
			// is now responding gets caught up from the replication queue.
			for _, node := range n.ReconcileHealth(failedNodes) {
				if err := n.RestartNode(node, messageQueue); err != nil {
					slog.Error("failed to resync recovered node", "node", node, "error", err)
				}
			}
		})
	}

	// TODO: remove aof support?
	aof, err := aof.NewAof("database.aof")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// defer aof.Close()
	// aof.Read(func(value resp.Value) {
	// 	command := strings.ToUpper(value.Array[0].Bulk)
	// 	args := value.Array[1:]
	// 	hanlderFunc, ok := handler.Handlers[command]
	// 	if !ok {
	// 		fmt.Println("Invalid Command: ", command)
	// 		return
	// 	}
	// 	hanlderFunc(args)
	// })
	for {
		conn, err := l.Accept()
		if err != nil {
			slog.Error("accept error", "error", err)
			continue
		}
		go handleConnection(conn, aof, n, messageQueue)
	}
}

func handleConnection(conn net.Conn, _ *aof.Aof, n *controlplane.Cluster, q *utils.Queue[utils.Message]) {
	defer conn.Close()
	for {
		response := resp.NewResp(conn)
		value, err := response.Read()
		if err != nil {
			return
		}
		if value.Type != "array" {
			slog.Warn("invalid request, expected an array")
			continue
		}
		if len(value.Array) == 0 {
			slog.Warn("invalid request, expected array len greater than 0")
			continue
		}
		command := strings.ToUpper(value.Array[0].Bulk)
		args := value.Array[1:]
		w := writer.NewWriter(conn)

		result, err := n.SendCommand(command, args)
		if err != nil {
			w.Write(resp.Value{Type: "string", Str: "ERR " + err.Error()})
			continue
		}

		// Add to queue only if node is leader and command is a SET.
		if n.IsLeader() && command == "SET" {
			m := utils.NewMessage(command, args)
			q.Enqueue(*m)
			slog.Info("queue length", "length", q.QueueLength())
		}

		w.Write(resp.Value{Type: result.Type, Str: result.Result, Bulk: result.Result})
	}
}
