package writer

import (
	"bytes"
	"errors"
	"testing"
	"zeno/src/resp"
)

type errWriter struct{}

func (e *errWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestNewWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if w == nil {
		t.Fatal("expected non-nil Writer")
	}
}

func TestWrite(t *testing.T) {
	tc := map[string]struct {
		value    resp.Value
		expected string
	}{
		"bulk string": {
			value:    resp.Value{Type: "bulk", Bulk: "hello"},
			expected: "$5\r\nhello\r\n",
		},
		"simple string": {
			value:    resp.Value{Type: "string", Str: "OK"},
			expected: "+OK\r\n",
		},
		"error": {
			value:    resp.Value{Type: "error", Str: "ERR bad command"},
			expected: "-ERR bad command\r\n",
		},
		"null": {
			value:    resp.Value{Type: "null"},
			expected: "$-1\r\n",
		},
		"array of bulk strings": {
			value: resp.Value{
				Type: "array",
				Array: []resp.Value{
					{Type: "bulk", Bulk: "SET"},
					{Type: "bulk", Bulk: "key"},
					{Type: "bulk", Bulk: "val"},
				},
			},
			expected: "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$3\r\nval\r\n",
		},
		"empty array": {
			value:    resp.Value{Type: "array", Array: []resp.Value{}},
			expected: "*0\r\n",
		},
	}

	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(&buf)

			err := w.Write(test.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if buf.String() != test.expected {
				t.Fatalf("unexpected output:\ngot:  %q\nwant: %q", buf.String(), test.expected)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	w := NewWriter(&errWriter{})
	err := w.Write(resp.Value{Type: "string", Str: "OK"})
	if err == nil {
		t.Fatal("expected error but got nil")
	}
}

func TestWriteMultiple(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	values := []resp.Value{
		{Type: "string", Str: "OK"},
		{Type: "bulk", Bulk: "data"},
	}
	for _, v := range values {
		if err := w.Write(v); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	expected := "+OK\r\n$4\r\ndata\r\n"
	if buf.String() != expected {
		t.Fatalf("unexpected output:\ngot:  %q\nwant: %q", buf.String(), expected)
	}
}
