package handler

import (
	"fmt"
	"sync"
	"zeno/src/resp"
)

// our command handlers
var Handlers = map[string]func([]resp.Value) resp.Value{
	"PING":    ping,
	"SET":     set,
	"GET":     get,
	"HSET":    hset,
	"HGET":    hget,
	"HGETALL": hgetall,
}
var SETs = map[string]string{}
var SETsMu = sync.RWMutex{} // handles concurrent requests

var HSETs = map[string]map[string]string{}
var HSETsMu = sync.RWMutex{}

func ping(args []resp.Value) resp.Value {
	if len(args) == 0 {
		return resp.Value{Type: "string", Str: "PONG"}
	}
	return resp.Value{Type: "string", Str: args[0].Bulk}
}
func set(args []resp.Value) resp.Value {
	fmt.Println(args, "args")
	if len(args) != 2 {
		return resp.Value{Type: "error", Str: "Invalid Arguments for SET"}
	}
	// set the values to the args
	key := args[0].Bulk
	value := args[1].Bulk
	SETsMu.Lock() // prevents access while writing
	SETs[key] = value
	SETsMu.Unlock() // finished writing we can allow acess to write again
	return resp.Value{Type: "string", Str: "OK"}
}

func get(args []resp.Value) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: "error", Str: "Invalid Arguments for GET"}
	}
	key := args[0].Bulk
	SETsMu.RLock()
	value, ok := SETs[key]
	SETsMu.RUnlock()
	if !ok {
		return resp.Value{Type: "null"}
	}
	return resp.Value{Type: "bulk", Bulk: value}
}

func hset(args []resp.Value) resp.Value {
	if len(args) != 3 {
		return resp.Value{Type: "error", Str: "Invalid Arguments for HSET command"}
	}
	hash := args[0].Bulk
	key := args[1].Bulk
	value := args[2].Bulk
	HSETsMu.Lock()
	if _, ok := HSETs[hash]; !ok {
		HSETs[hash] = map[string]string{}
	}
	HSETs[hash][key] = value
	HSETsMu.Unlock()
	return resp.Value{Type: "string", Str: "OK"}
}
func hget(args []resp.Value) resp.Value {
	if len(args) != 2 {
		return resp.Value{Type: "erorr", Str: "Invalid Arguments for HGET command"}
	}
	hash := args[0].Bulk
	key := args[1].Bulk
	HSETsMu.Lock()
	value, ok := HSETs[hash][key]
	HSETsMu.Unlock()
	if !ok {
		return resp.Value{Type: "null"}
	}
	return resp.Value{Type: "bulk", Bulk: value}
}

func hgetall(args []resp.Value) resp.Value {
	if len(args) != 1 {
		return resp.Value{Type: "error", Str: "Invalid Arguments for HGETALL command"}
	}
	hash := args[0].Bulk

	HSETsMu.Lock()
	defer HSETsMu.Unlock()

	values, ok := HSETs[hash]
	if !ok {
		return resp.Value{Type: "array", Array: []resp.Value{}}
	}

	var result []resp.Value
	for k, v := range values {
		result = append(result, resp.Value{Type: "bulk", Bulk: k})
		result = append(result, resp.Value{Type: "bulk", Bulk: v})
	}

	return resp.Value{Type: "array", Array: result}
}
