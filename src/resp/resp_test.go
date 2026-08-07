package resp

import (
	"strings"
	"testing"
)

func TestReadBulk(t *testing.T) {
	tc := map[string]struct {
		input        string
		expectedType string
		expectedBulk string
	}{
		"simple bulk string": {
			input:        "$5\r\nhello\r\n",
			expectedType: "bulk",
			expectedBulk: "hello",
		},
		"empty bulk string": {
			input:        "$0\r\n\r\n",
			expectedType: "bulk",
			expectedBulk: "",
		},
		"bulk with spaces": {
			input:        "$11\r\nhello world\r\n",
			expectedType: "bulk",
			expectedBulk: "hello world",
		},
	}

	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			r := NewResp(strings.NewReader(test.input))
			got, err := r.Read()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Type != test.expectedType {
				t.Fatalf("unexpected type: got %s expected %s", got.Type, test.expectedType)
			}
			if got.Bulk != test.expectedBulk {
				t.Fatalf("unexpected bulk: got %q expected %q", got.Bulk, test.expectedBulk)
			}
		})
	}
}

func TestReadArray(t *testing.T) {
	tc := map[string]struct {
		input         string
		expectedLen   int
		expectedBulks []string
	}{
		"two element array": {
			input:         "*2\r\n$3\r\nSET\r\n$3\r\nfoo\r\n",
			expectedLen:   2,
			expectedBulks: []string{"SET", "foo"},
		},
		"three element array": {
			input:         "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n",
			expectedLen:   3,
			expectedBulks: []string{"SET", "key", "value"},
		},
		"single element array": {
			input:         "*1\r\n$4\r\nPING\r\n",
			expectedLen:   1,
			expectedBulks: []string{"PING"},
		},
		"empty array": {
			input:       "*0\r\n",
			expectedLen: 0,
		},
	}

	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			r := NewResp(strings.NewReader(test.input))
			got, err := r.Read()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Type != "array" {
				t.Fatalf("unexpected type: got %s expected array", got.Type)
			}
			if len(got.Array) != test.expectedLen {
				t.Fatalf("unexpected array length: got %d expected %d", len(got.Array), test.expectedLen)
			}
			for i, expected := range test.expectedBulks {
				if got.Array[i].Bulk != expected {
					t.Fatalf("unexpected element %d: got %q expected %q", i, got.Array[i].Bulk, expected)
				}
			}
		})
	}
}

func TestMarshalBulk(t *testing.T) {
	tc := map[string]struct {
		value    Value
		expected string
	}{
		"simple bulk": {
			value:    Value{Type: "bulk", Bulk: "hello"},
			expected: "$5\r\nhello\r\n",
		},
		"empty bulk": {
			value:    Value{Type: "bulk", Bulk: ""},
			expected: "$0\r\n\r\n",
		},
	}

	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			got := string(test.value.Marshal())
			if got != test.expected {
				t.Fatalf("unexpected marshal:\ngot:  %q\nwant: %q", got, test.expected)
			}
		})
	}
}

func TestMarshalString(t *testing.T) {
	v := Value{Type: "string", Str: "OK"}
	got := string(v.Marshal())
	expected := "+OK\r\n"
	if got != expected {
		t.Fatalf("unexpected marshal:\ngot:  %q\nwant: %q", got, expected)
	}
}

func TestMarshalError(t *testing.T) {
	v := Value{Type: "error", Str: "ERR unknown command"}
	got := string(v.Marshal())
	expected := "-ERR unknown command\r\n"
	if got != expected {
		t.Fatalf("unexpected marshal:\ngot:  %q\nwant: %q", got, expected)
	}
}

func TestMarshalNull(t *testing.T) {
	v := Value{Type: "null"}
	got := string(v.Marshal())
	expected := "$-1\r\n"
	if got != expected {
		t.Fatalf("unexpected marshal:\ngot:  %q\nwant: %q", got, expected)
	}
}

func TestMarshalArray(t *testing.T) {
	tc := map[string]struct {
		value    Value
		expected string
	}{
		"array of bulk strings": {
			value: Value{
				Type: "array",
				Array: []Value{
					{Type: "bulk", Bulk: "SET"},
					{Type: "bulk", Bulk: "key"},
					{Type: "bulk", Bulk: "value"},
				},
			},
			expected: "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n",
		},
		"empty array": {
			value:    Value{Type: "array", Array: []Value{}},
			expected: "*0\r\n",
		},
	}

	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			got := string(test.value.Marshal())
			if got != test.expected {
				t.Fatalf("unexpected marshal:\ngot:  %q\nwant: %q", got, test.expected)
			}
		})
	}
}

func TestMarshalUnknownType(t *testing.T) {
	v := Value{Type: "unknown"}
	got := v.Marshal()
	if len(got) != 0 {
		t.Fatalf("expected empty bytes for unknown type, got %q", string(got))
	}
}

func TestRoundTrip(t *testing.T) {
	original := Value{
		Type: "array",
		Array: []Value{
			{Type: "bulk", Bulk: "HSET"},
			{Type: "bulk", Bulk: "users"},
			{Type: "bulk", Bulk: "user1"},
		},
	}

	marshaled := original.Marshal()
	r := NewResp(strings.NewReader(string(marshaled)))
	got, err := r.Read()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Type != original.Type {
		t.Fatalf("type mismatch: got %s expected %s", got.Type, original.Type)
	}
	if len(got.Array) != len(original.Array) {
		t.Fatalf("array length mismatch: got %d expected %d", len(got.Array), len(original.Array))
	}
	for i := range original.Array {
		if got.Array[i].Bulk != original.Array[i].Bulk {
			t.Fatalf("element %d mismatch: got %q expected %q", i, got.Array[i].Bulk, original.Array[i].Bulk)
		}
	}
}
