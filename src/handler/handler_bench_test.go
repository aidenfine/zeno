package handler

import (
	"strconv"
	"strings"
	"testing"
	"zeno/src/resp"
)

// Sink vars keep the compiler from optimizing away the benchmarked work.
var (
	resultSink resp.Value
	arraySink  []resp.Value
	bytesSink  []byte
)

// keyspace bounds how many distinct keys the read/write benchmarks touch.
// Bounding it keeps the maps from growing without limit while still
// exercising realistic hashing and lookups.
const keyspace = 10_000

// makeKeys precomputes keys outside the timed loop so we measure the map
// operation itself, not strconv.Itoa.
func makeKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = "key:" + strconv.Itoa(i)
	}
	return keys
}

func BenchmarkSet(b *testing.B) {
	SETs = make(map[string]string, keyspace)
	keys := makeKeys(keyspace)
	args := []resp.Value{{Bulk: ""}, {Bulk: "benchmark-value-payload"}}

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		args[0].Bulk = keys[i%keyspace]
		resultSink = set(args)
		i++
	}
}

// BenchmarkSetParallel measures SET under concurrency, exercising the write
// lock on SETsMu — the hot path when many connections write at once.
func BenchmarkSetParallel(b *testing.B) {
	SETs = make(map[string]string, keyspace)
	keys := makeKeys(keyspace)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		args := []resp.Value{{Bulk: ""}, {Bulk: "benchmark-value-payload"}}
		for pb.Next() {
			args[0].Bulk = keys[i%keyspace]
			resultSink = set(args)
			i++
		}
	})
}

func BenchmarkGet(b *testing.B) {
	keys := makeKeys(keyspace)
	SETs = make(map[string]string, keyspace)
	for _, k := range keys {
		SETs[k] = "benchmark-value-payload"
	}
	args := []resp.Value{{Bulk: ""}}

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		args[0].Bulk = keys[i%keyspace]
		resultSink = get(args)
		i++
	}
}

// BenchmarkGetParallel measures GET under concurrency (shared read lock).
func BenchmarkGetParallel(b *testing.B) {
	keys := makeKeys(keyspace)
	SETs = make(map[string]string, keyspace)
	for _, k := range keys {
		SETs[k] = "benchmark-value-payload"
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		args := []resp.Value{{Bulk: ""}}
		for pb.Next() {
			args[0].Bulk = keys[i%keyspace]
			resultSink = get(args)
			i++
		}
	})
}

func BenchmarkHSet(b *testing.B) {
	HSETs = make(map[string]map[string]string)
	keys := makeKeys(keyspace)
	args := []resp.Value{{Bulk: "users"}, {Bulk: ""}, {Bulk: "benchmark-value-payload"}}

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		args[1].Bulk = keys[i%keyspace]
		resultSink = hset(args)
		i++
	}
}

func BenchmarkHGet(b *testing.B) {
	keys := makeKeys(keyspace)
	HSETs = map[string]map[string]string{"users": make(map[string]string, keyspace)}
	for _, k := range keys {
		HSETs["users"][k] = "benchmark-value-payload"
	}
	args := []resp.Value{{Bulk: "users"}, {Bulk: ""}}

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		args[1].Bulk = keys[i%keyspace]
		resultSink = hget(args)
		i++
	}
}

// BenchmarkHGetAll materializes an entire hash into a RESP array. Cost grows
// with the number of fields, so we fix a representative field count.
func BenchmarkHGetAll(b *testing.B) {
	const fields = 100
	keys := makeKeys(fields)
	HSETs = map[string]map[string]string{"users": make(map[string]string, fields)}
	for _, k := range keys {
		HSETs["users"][k] = "benchmark-value-payload"
	}
	args := []resp.Value{{Bulk: "users"}}

	b.ReportAllocs()
	for b.Loop() {
		resultSink = hgetall(args)
		arraySink = resultSink.Array
	}
}

// BenchmarkCommandRoundTrip measures the full per-command CPU cost the server
// pays for one request: parse the RESP array, dispatch to the handler, and
// marshal the reply. It deliberately excludes the network and the gRPC
// leader-forwarding hop (see controlplane.SendCommand) so it isolates command
// processing. Use the load generator in ./bench for end-to-end numbers.
func BenchmarkCommandRoundTrip(b *testing.B) {
	SETs = map[string]string{"foo": "bar"}
	inputs := map[string]string{
		"SET": "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n",
		"GET": "*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n",
	}

	for name, input := range inputs {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				value, err := resp.NewResp(strings.NewReader(input)).Read()
				if err != nil {
					b.Fatalf("parse: %v", err)
				}
				command := strings.ToUpper(value.Array[0].Bulk)
				args := value.Array[1:]
				handler, ok := Handlers[command]
				if !ok {
					b.Fatalf("no handler for %q", command)
				}
				resultSink = handler(args)
				bytesSink = resultSink.Marshal()
			}
		})
	}
}
