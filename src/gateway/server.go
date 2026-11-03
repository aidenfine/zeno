// Package gateway exposes the client protocol without owning local storage.
package gateway

import (
	"context"
	"net"
	"strings"
	"zeno/src/controlplane"
	"zeno/src/resp"
	"zeno/src/utils"
	"zeno/src/writer"
)

func Serve(ctx context.Context, listener net.Listener, cluster *controlplane.Cluster, queue *utils.Queue[utils.Message]) error {
	stop := context.AfterFunc(ctx, func() { listener.Close() })
	defer stop()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go handleConnection(ctx, conn, cluster, queue)
	}
}

func handleConnection(ctx context.Context, conn net.Conn, cluster *controlplane.Cluster, queue *utils.Queue[utils.Message]) {
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	reader := resp.NewResp(conn)
	w := writer.NewWriter(conn)
	for {
		value, err := reader.Read()
		if err != nil {
			return
		}
		if value.Type != "array" || len(value.Array) == 0 {
			if w.Write(resp.Value{Type: "error", Str: "ERR expected a nonempty command array"}) != nil {
				return
			}
			continue
		}
		command := strings.ToUpper(value.Array[0].Bulk)
		args := value.Array[1:]
		result, err := cluster.SendCommand(command, args)
		if err != nil {
			if w.Write(resp.Value{Type: "error", Str: "ERR " + err.Error()}) != nil {
				return
			}
			continue
		}
		if !result.Success {
			if w.Write(resp.Value{Type: "error", Str: "ERR " + result.Error}) != nil {
				return
			}
			continue
		}
		if (command == "SET" || command == "HSET") && result.Type != "error" {
			queue.Enqueue(*utils.NewMessage(command, args))
		}
		if w.Write(resp.Value{Type: result.Type, Str: result.Result, Bulk: result.Result}) != nil {
			return
		}
	}
}
