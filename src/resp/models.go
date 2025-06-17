package resp

import "bufio"

const (
	STRING  = '+'
	ERROR   = '-'
	INTEGER = ':'
	BULK    = '$'
	ARRAY   = '*'
	BOOLEAN = '#'
)

type Value struct {
	Type    string
	Str     string
	Num     int
	Bulk    string
	Array   []Value
	Boolean bool
}

type Resp struct {
	reader *bufio.Reader
}
