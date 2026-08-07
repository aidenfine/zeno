package aof

import (
	"os"
	"path/filepath"
	"testing"
	"zeno/src/resp"
)

func tempAofPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.aof")
}

func TestNewAof(t *testing.T) {
	tc := map[string]struct {
		path      string
		expectErr bool
	}{
		"creates file at valid path": {
			path:      tempAofPath(t),
			expectErr: false,
		},
		"fails on invalid path": {
			path:      "/nonexistent/dir/test.aof",
			expectErr: true,
		},
	}

	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			aof, err := NewAof(test.path)
			if test.expectErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer aof.Close()

			if aof.File == nil {
				t.Fatal("expected File to be set")
			}
			if aof.Rd == nil {
				t.Fatal("expected Rd to be set")
			}

			if _, err := os.Stat(test.path); os.IsNotExist(err) {
				t.Fatal("expected aof file to be created on disk")
			}
		})
	}
}

func TestClose(t *testing.T) {
	path := tempAofPath(t)
	aof, err := NewAof(path)
	if err != nil {
		t.Fatalf("unexpected error creating aof: %v", err)
	}

	if err := aof.Close(); err != nil {
		t.Fatalf("unexpected error closing aof: %v", err)
	}

	// writing after close should fail
	err = aof.Write(resp.Value{Type: "bulk", Bulk: "test"})
	if err == nil {
		t.Fatal("expected error writing to closed aof")
	}
}

func TestWrite(t *testing.T) {
	tc := map[string]struct {
		value          resp.Value
		expectedOutput string
	}{
		"write bulk value": {
			value:          resp.Value{Type: "bulk", Bulk: "hello"},
			expectedOutput: "$5\r\nhello\r\n",
		},
		"write array with bulk elements": {
			value: resp.Value{
				Type: "array",
				Array: []resp.Value{
					{Type: "bulk", Bulk: "SET"},
					{Type: "bulk", Bulk: "key"},
					{Type: "bulk", Bulk: "value"},
				},
			},
			expectedOutput: "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n",
		},
		"write string value": {
			value:          resp.Value{Type: "string", Str: "OK"},
			expectedOutput: "+OK\r\n",
		},
	}

	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			path := tempAofPath(t)
			aof, err := NewAof(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer aof.Close()

			if err := aof.Write(test.value); err != nil {
				t.Fatalf("unexpected error writing: %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("unexpected error reading file: %v", err)
			}
			if string(data) != test.expectedOutput {
				t.Fatalf("unexpected file content:\ngot:  %q\nwant: %q", string(data), test.expectedOutput)
			}
		})
	}
}

func TestWriteMultiple(t *testing.T) {
	path := tempAofPath(t)
	aof, err := NewAof(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer aof.Close()

	values := []resp.Value{
		{Type: "bulk", Bulk: "first"},
		{Type: "bulk", Bulk: "second"},
	}
	for _, v := range values {
		if err := aof.Write(v); err != nil {
			t.Fatalf("unexpected error writing: %v", err)
		}
	}

	expected := "$5\r\nfirst\r\n$6\r\nsecond\r\n"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected error reading file: %v", err)
	}
	if string(data) != expected {
		t.Fatalf("unexpected file content:\ngot:  %q\nwant: %q", string(data), expected)
	}
}

func TestRead(t *testing.T) {
	path := tempAofPath(t)

	// write a known RESP array to the file directly
	content := "*2\r\n$3\r\nSET\r\n$3\r\nfoo\r\n"
	if err := os.WriteFile(path, []byte(content), 0666); err != nil {
		t.Fatalf("unexpected error writing test file: %v", err)
	}

	aof, err := NewAof(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer aof.Close()

	var got []resp.Value
	err = aof.Read(func(value resp.Value) {
		got = append(got, value)
	})
	if err != nil {
		t.Fatalf("unexpected error reading: %v", err)
	}

	if len(got) == 0 {
		t.Fatal("expected at least one value from callback")
	}

	if got[0].Type != "array" {
		t.Fatalf("unexpected type: got %s expected array", got[0].Type)
	}
	if len(got[0].Array) != 2 {
		t.Fatalf("unexpected array length: got %d expected 2", len(got[0].Array))
	}
	if got[0].Array[0].Bulk != "SET" {
		t.Fatalf("unexpected first element: got %s expected SET", got[0].Array[0].Bulk)
	}
	if got[0].Array[1].Bulk != "foo" {
		t.Fatalf("unexpected second element: got %s expected foo", got[0].Array[1].Bulk)
	}
}

func TestReadEmptyFile(t *testing.T) {
	path := tempAofPath(t)
	aof, err := NewAof(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer aof.Close()

	called := false
	err = aof.Read(func(value resp.Value) {
		called = true
	})

	if called {
		t.Fatal("callback should not be called for empty file")
	}
}

func TestWriteThenRead(t *testing.T) {
	path := tempAofPath(t)
	aof, err := NewAof(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	written := resp.Value{
		Type: "array",
		Array: []resp.Value{
			{Type: "bulk", Bulk: "SET"},
			{Type: "bulk", Bulk: "mykey"},
			{Type: "bulk", Bulk: "myvalue"},
		},
	}
	if err := aof.Write(written); err != nil {
		t.Fatalf("unexpected error writing: %v", err)
	}
	aof.Close()

	// reopen and read
	aof2, err := NewAof(path)
	if err != nil {
		t.Fatalf("unexpected error reopening: %v", err)
	}
	defer aof2.Close()

	var got []resp.Value
	err = aof2.Read(func(value resp.Value) {
		got = append(got, value)
	})
	if err != nil {
		t.Fatalf("unexpected error reading: %v", err)
	}

	if len(got) == 0 {
		t.Fatal("expected at least one value from read")
	}
	if got[0].Type != "array" {
		t.Fatalf("unexpected type: got %s expected array", got[0].Type)
	}
	if len(got[0].Array) != 3 {
		t.Fatalf("unexpected array length: got %d expected 3", len(got[0].Array))
	}
	if got[0].Array[0].Bulk != "SET" {
		t.Fatalf("unexpected command: got %s expected SET", got[0].Array[0].Bulk)
	}
	if got[0].Array[1].Bulk != "mykey" {
		t.Fatalf("unexpected key: got %s expected mykey", got[0].Array[1].Bulk)
	}
	if got[0].Array[2].Bulk != "myvalue" {
		t.Fatalf("unexpected value: got %s expected myvalue", got[0].Array[2].Bulk)
	}
}
