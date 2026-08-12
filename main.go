package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"zeno/pb"
	"zeno/src/aof"
	"zeno/src/nodes"
	"zeno/src/resp"
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

	fmt.Printf("gRPC: received task #%s - %s\n", task.Id, task.Title)
	return &pb.SendTaskResponse{Success: true}, nil
}

// main in-mem db goes like this
// Client -> TCP Request -> RESP deserialze -> commands hander -> RESP serialze -> Response

func main() {
	n, err := nodes.MakeNodes()
	if err != nil {
		panic("Failed to make nodes")
	}
	go func() {
		lis, err := net.Listen("tcp", ":6380")
		if err != nil {
			fmt.Println("gRPC listen error:", err)
			return
		}
		grpcServer := grpc.NewServer()
		pb.RegisterTaskServiceServer(grpcServer, &taskServer{})
		pb.RegisterNodeServiceServer(grpcServer, &nodes.NodeServer{})
		reflection.Register(grpcServer)
		fmt.Println("gRPC server running on port: 6380")
		if err := grpcServer.Serve(lis); err != nil {
			fmt.Println("gRPC serve error:", err)
		}
	}()

	fmt.Println("Running on port: 6379")
	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		fmt.Println(err)
		return
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
			fmt.Println(err)
			continue
		}
		go handleConnection(conn, aof, n)
	}
}

func handleConnection(conn net.Conn, _ *aof.Aof, n *nodes.Nodes) {
	defer conn.Close()
	for {
		response := resp.NewResp(conn)
		value, err := response.Read()
		if err != nil {
			return
		}
		if value.Type != "array" {
			fmt.Println("Invalid Request Expected an Array")
			continue
		}
		if len(value.Array) == 0 {
			fmt.Println("Invalid Requested expected array len greater than 0")
			continue
		}
		command := strings.ToUpper(value.Array[0].Bulk)
		args := value.Array[1:]
		w := writer.NewWriter(conn)

		result, err := n.SendToLeader(command, args)
		if err != nil {
			w.Write(resp.Value{Type: "string", Str: "ERR " + err.Error()})
			continue
		}
		w.Write(resp.Value{Type: result.Type, Str: result.Result, Bulk: result.Result})
	}
}
