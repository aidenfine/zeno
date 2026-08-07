package handler

import (
	"fmt"
	"testing"
	"zeno/src/resp"
)

// Test ping()
func TestPing(t *testing.T) {
	tc := map[string]struct {
		args           []resp.Value
		expectedType   string
		expectedString string
	}{
		"OK ping": {
			args:           []resp.Value{},
			expectedType:   "string",
			expectedString: "PONG",
		},
		"more than one arg": {
			args: []resp.Value{
				{Bulk: "foo"},
				{Bulk: "123"},
			},
			expectedType:   "string",
			expectedString: "foo",
		},
	}
	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {

			got := ping(test.args)

			if got.Type != test.expectedType {
				t.Fatalf("unexpected type: got %s expected %s", got.Type, test.expectedType)
			}
			if got.Str != test.expectedString {
				t.Fatalf("unexpected string: got %s expected %s", got.Str, test.expectedString)
			}

		})
	}
}

// string: {args: []resp.Value, expectedType: string, expectedString: string}
// Tests the set() function
func TestSet(t *testing.T) {
	tc := map[string]struct {
		args           []resp.Value
		expectedType   string
		expectedString string
	}{
		"OK set, `set foo bar`": {
			args: []resp.Value{
				{Bulk: "foo"},
				{Bulk: "bar"},
			},
			expectedType:   "string",
			expectedString: "OK",
		},
		"error set, `set foo`": {
			args: []resp.Value{
				{Bulk: "foo"},
			},
			expectedType:   "error",
			expectedString: "Invalid Arguments for SET",
		},
	}

	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			SETs = make(map[string]string)

			got := set(test.args)

			if got.Type != test.expectedType {
				t.Fatalf("unexpected type: got %s expected %s", got.Type, test.expectedType)
			}
			if got.Str != test.expectedString {
				t.Fatalf("unexpected string: got %s expected %s", got.Str, test.expectedString)
			}

			if len(test.args) >= 2 {
				if SETs[test.args[0].Bulk] != test.args[1].Bulk {
					t.Fatalf("unexpected value: got %s expected %s", SETs[test.args[0].Bulk], test.args[1].Bulk)
				}
			}

		})
	}
}

func TestHset(t *testing.T) {
	tc := map[string]struct {
		args           []resp.Value
		expectedType   string
		expectedString string
	}{
		"OK hset, `hset users user1 joe`": {
			args: []resp.Value{
				{Bulk: "users"},
				{Bulk: "user1"},
				{Bulk: "joe"},
			},
			expectedType:   "string",
			expectedString: "OK",
		},
		"error hset, too few args `hset users`": {
			args: []resp.Value{
				{Bulk: "users"},
			},
			expectedType:   "error",
			expectedString: "Invalid Arguments for HSET command",
		},
		"error hset, too many args `hset users user1 joe extra`": {
			args: []resp.Value{
				{Bulk: "users"},
				{Bulk: "user1"},
				{Bulk: "joe"},
				{Bulk: "extra"},
			},
			expectedType:   "error",
			expectedString: "Invalid Arguments for HSET command",
		},
	}

	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			HSETs = make(map[string]map[string]string)

			got := hset(test.args)

			if got.Type != test.expectedType {
				t.Fatalf("unexpected type: got %s expected %s", got.Type, test.expectedType)
			}
			if got.Str != test.expectedString {
				t.Fatalf("unexpected string: got %s expected %s", got.Str, test.expectedString)
			}

			if len(test.args) == 3 {
				if HSETs[test.args[0].Bulk][test.args[1].Bulk] != test.args[2].Bulk {
					t.Fatalf("unexpected value: got %s expected %s", HSETs[test.args[0].Bulk][test.args[1].Bulk], test.args[2].Bulk)
				}
			}

		})
	}
}

func TestHGet(t *testing.T) {
	tc := map[string]struct {
		args         []resp.Value
		testMap      map[string]map[string]string
		expectedType string
		expectedBulk string
	}{
		"OK hget, `hget words foo` should return bar": {
			args: []resp.Value{
				{Bulk: "words"},
				{Bulk: "foo"},
			},
			testMap: map[string]map[string]string{
				"words": {
					"foo": "bar",
				},
			},
			expectedType: "bulk",
			expectedBulk: "bar",
		},
		"error hget, `hget words foo bar` should return error": {
			args: []resp.Value{
				{Bulk: "words"},
				{Bulk: "foo"},
				{Bulk: "bar"},
			},
			testMap: map[string]map[string]string{
				"words": {
					"foo": "bar",
				},
			},
			expectedType: "error",
			expectedBulk: "Invalid Arguments for HGET command",
		},
	}
	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			HSETs = test.testMap

			got := hget(test.args)
			fmt.Println(got)

			if got.Type != test.expectedType {
				t.Fatalf("unexpected type: got %s expected %s", got.Type, test.expectedType)
			}

			// handle the str/bulk difference in response
			// kind of weird but oh well.
			if got.Type == "error" {
				if got.Str != test.expectedBulk {
					t.Fatalf("unexpected bulk: got %s expected %s", got.Str, test.expectedBulk)
				}
			} else {
				if got.Bulk != test.expectedBulk {
					t.Fatalf("unexpected bulk: got %s expected %s", got.Bulk, test.expectedBulk)
				}
			}

			if len(test.args) == 2 {
				if HSETs[test.args[0].Bulk][test.args[1].Bulk] != test.expectedBulk {
					t.Fatalf("unexpected value: got %s expected %s", HSETs[test.args[0].Bulk][test.args[1].Bulk], test.expectedBulk)
				}
			}

		})
	}

}

func TestHGetAll(t *testing.T) {
	tc := map[string]struct {
		args          []resp.Value
		testMap       map[string]map[string]string
		expectedType  string
		expectedError string
		expectedPairs map[string]string
	}{
		"OK hgetall, single entry": {
			args: []resp.Value{
				{Bulk: "users"},
			},
			testMap: map[string]map[string]string{
				"users": {
					"user1": "joe",
				},
			},
			expectedType:  "array",
			expectedPairs: map[string]string{"user1": "joe"},
		},
		"OK hgetall, multiple entries": {
			args: []resp.Value{
				{Bulk: "users"},
			},
			testMap: map[string]map[string]string{
				"users": {
					"user1": "joe",
					"user2": "jane",
				},
			},
			expectedType:  "array",
			expectedPairs: map[string]string{"user1": "joe", "user2": "jane"},
		},
		"OK hgetall, nonexistent hash returns empty array": {
			args: []resp.Value{
				{Bulk: "nonexistent"},
			},
			testMap:       map[string]map[string]string{},
			expectedType:  "array",
			expectedPairs: map[string]string{},
		},
		"error hgetall, too many args": {
			args: []resp.Value{
				{Bulk: "users"},
				{Bulk: "extra"},
			},
			testMap:       map[string]map[string]string{},
			expectedType:  "error",
			expectedError: "Invalid Arguments for HGETALL command",
		},
		"error hgetall, no args": {
			args:          []resp.Value{},
			testMap:       map[string]map[string]string{},
			expectedType:  "error",
			expectedError: "Invalid Arguments for HGETALL command",
		},
	}

	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			HSETs = test.testMap

			got := hgetall(test.args)

			if got.Type != test.expectedType {
				t.Fatalf("unexpected type: got %s expected %s", got.Type, test.expectedType)
			}

			if got.Type == "error" {
				if got.Str != test.expectedError {
					t.Fatalf("unexpected error: got %s expected %s", got.Str, test.expectedError)
				}
				return
			}

			expectedLen := len(test.expectedPairs) * 2
			if len(got.Array) != expectedLen {
				t.Fatalf("unexpected array length: got %d expected %d", len(got.Array), expectedLen)
			}

			for i := 0; i < len(got.Array); i += 2 {
				key := got.Array[i].Bulk
				value := got.Array[i+1].Bulk
				expected, ok := test.expectedPairs[key]
				if !ok {
					t.Fatalf("unexpected key in result: %s", key)
				}
				if value != expected {
					t.Fatalf("unexpected value for key %s: got %s expected %s", key, value, expected)
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	tc := map[string]struct {
		args         []resp.Value
		testMap      map[string]string
		expectedType string
		expectedBulk string
	}{
		"OK get, `get foo` return bar": {
			args: []resp.Value{
				{Bulk: "foo"},
			},
			testMap: map[string]string{
				"foo": "bar",
			},
			expectedType: "bulk",
			expectedBulk: "bar",
		},
		"OK more than one value in map": {
			args: []resp.Value{
				{Bulk: "foo"},
			},
			testMap: map[string]string{
				"test":         "123",
				"dasndadn":     "31231231233",
				"dadajd2d2":    "ufwfjaw",
				"wnfqjf29013j": "dasj31239dwkadjawdkjawdkna",
				"foo":          "bar",
			},
			expectedType: "bulk",
			expectedBulk: "bar",
		},
		"error get, `get foo bar`": {
			args: []resp.Value{
				{Bulk: "foo"},
				{Bulk: "bar"},
			},
			testMap: map[string]string{
				"foo": "bar",
			},
			expectedType: "error",
			expectedBulk: "Invalid Arguments for GET",
		},
	}

	for tcName, test := range tc {
		t.Run(tcName, func(t *testing.T) {
			SETs = test.testMap

			got := get(test.args)

			if got.Type != test.expectedType {
				t.Fatalf("unexpected type: got %s expected %s", got.Type, test.expectedType)
			}

			// handle the str/bulk difference in response
			// kind of weird but oh well.
			if got.Type == "error" {
				if got.Str != test.expectedBulk {
					t.Fatalf("unexpected bulk: got %s expected %s", got.Str, test.expectedBulk)
				}
			} else {
				if got.Bulk != test.expectedBulk {
					t.Fatalf("unexpected bulk: got %s expected %s", got.Bulk, test.expectedBulk)
				}
			}

			if len(test.args) >= 2 {
				if SETs[test.args[0].Bulk] != test.args[1].Bulk {
					t.Fatalf("unexpected value: got %s expected %s", SETs[test.args[0].Bulk], test.args[1].Bulk)
				}
			}

		})
	}
}
