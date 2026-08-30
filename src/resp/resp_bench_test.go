package resp

import (
	"strconv"
	"strings"
	"testing"
)

// Sink vars keep the compiler from optimizing away the benchmarked work.
var (
	valueSink Value
	bytesSink []byte
)

// buildArray produces a RESP array of n bulk strings, e.g. an HGETALL-style
// reply or a wide command. Used to measure how parse/marshal cost scales.
func buildArray(n int) string {
	var sb strings.Builder
	sb.WriteString("*" + strconv.Itoa(n) + "\r\n")
	for i := range n {
		el := "el:" + strconv.Itoa(i)
		sb.WriteString("$" + strconv.Itoa(len(el)) + "\r\n" + el + "\r\n")
	}
	return sb.String()
}

func BenchmarkReadSetCommand(b *testing.B) {
	input := "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	b.ReportAllocs()
	for b.Loop() {
		valueSink, _ = NewResp(strings.NewReader(input)).Read()
	}
}

func BenchmarkReadGetCommand(b *testing.B) {
	input := "*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n"
	b.ReportAllocs()
	for b.Loop() {
		valueSink, _ = NewResp(strings.NewReader(input)).Read()
	}
}

// BenchmarkReadLargeArray exercises the parser on a 1000-element array to
// surface per-element allocation cost.
func BenchmarkReadLargeArray(b *testing.B) {
	input := buildArray(1000)
	b.ReportAllocs()
	for b.Loop() {
		valueSink, _ = NewResp(strings.NewReader(input)).Read()
	}
}

func BenchmarkMarshalString(b *testing.B) {
	v := Value{Type: "string", Str: "OK"}
	b.ReportAllocs()
	for b.Loop() {
		bytesSink = v.Marshal()
	}
}

func BenchmarkMarshalBulk(b *testing.B) {
	v := Value{Type: "bulk", Bulk: "benchmark-value-payload"}
	b.ReportAllocs()
	for b.Loop() {
		bytesSink = v.Marshal()
	}
}

// BenchmarkMarshalArray measures serializing a 100-element array reply, the
// shape HGETALL returns.
func BenchmarkMarshalArray(b *testing.B) {
	const n = 100
	arr := make([]Value, n)
	for i := range arr {
		arr[i] = Value{Type: "bulk", Bulk: "el:" + strconv.Itoa(i)}
	}
	v := Value{Type: "array", Array: arr}

	b.ReportAllocs()
	for b.Loop() {
		bytesSink = v.Marshal()
	}
}
