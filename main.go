package main

import (
	"fmt"
	"net"
	"strings"
	"zeno/src/aof"
	"zeno/src/handler"
	"zeno/src/resp"
	"zeno/src/writer"
)

// main in-mem db goes like this
// Client -> TCP Request -> RESP deserialze -> commands hander -> RESP serialze -> Response

func main() {
	// Setup tcp server
	fmt.Println("Running on port: 6379")
	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		fmt.Println(err)
		return
	}
	aof, err := aof.NewAof("database.aof")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer aof.Close()
	aof.Read(func(value resp.Value) {
		command := strings.ToUpper(value.Array[0].Bulk)
		args := value.Array[1:]
		hanlderFunc, ok := handler.Handlers[command]
		if !ok {
			fmt.Println("Invalid Command: ", command)
			return
		}
		hanlderFunc(args)
	})
	conn, err := l.Accept()
	if err != nil {
		fmt.Println(err)
		return
	}
	// defer close to close later
	defer conn.Close()
	// loop to wait and receive commands
	for {
		response := resp.NewResp(conn)
		value, err := response.Read()
		fmt.Println(value)
		if err != nil {
			fmt.Println(err)
			return
		}
		if value.Type != "array" {
			fmt.Println("Invalid Request Expected an Array")
			continue
		}
		if len(value.Array) == 0 {
			fmt.Println("Invalid Requested expected array len greater than 0")
		}
		command := strings.ToUpper(value.Array[0].Bulk)
		args := value.Array[1:]

		writer := writer.NewWriter(conn)
		handlerResponse, ok := handler.Handlers[command]
		if !ok {
			fmt.Println("Invalid Command: ", command)
			writer.Write(resp.Value{Type: "string", Str: ""})
			continue

		}
		if command == "SET" || command == "HSET" {
			aof.Write(value)
		}

		result := handlerResponse(args)
		writer.Write(result)
		// write out once finished
		// writer.Write({typ: "string", str: "OK"})

	}
}
