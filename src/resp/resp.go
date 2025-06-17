package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

func NewResp(rd io.Reader) *Resp {
	return &Resp{reader: bufio.NewReader(rd)}
}
func (r *Resp) Read() (Value, error) {
	_type, err := r.reader.ReadByte()
	if err != nil {
		return Value{}, err
	}
	// add bool to this switch statement
	switch _type {
	case ARRAY:
		return r.readArray()
	case BULK:
		return r.readBulk()
	case BOOLEAN:
		b, err := r.readBoolean()
		if err != nil {
			return Value{Type: "bool", Boolean: b}, err
		}
		return Value{}, nil
	default:
		fmt.Printf("Unknown type %v", string(_type))
		return Value{}, nil
	}
}

// reads lines until it reaches and \r ( end line )
func (r *Resp) readLine() (line []byte, n int, err error) {
	for {
		b, err := r.reader.ReadByte()
		if err != nil {
			return nil, 0, err
		}
		n += 1
		line = append(line, b)
		if len(line) >= 2 && line[len(line)-2] == '\r' {
			break
		}
	}
	return line[:len(line)-2], n, nil
}

// reads values inside an array
// for example message will come in this format *2\r\n$5\r\nhello\r\n$5\r\nworld\r\n
// Iterates through each item and calls `Read()` to read the correct type according to the char in the beginning of the line
func (r *Resp) readArray() (Value, error) {
	v := Value{}
	v.Type = "array"

	// get len of array
	length, _, err := r.readInteger()
	if err != nil {
		return v, err
	}
	// for each line parse and read val
	v.Array = make([]Value, length)
	for i := range length {
		val, err := r.Read()
		if err != nil {
			return v, err
		}
		// parse value add to array
		v.Array[i] = val
	}
	return v, nil
}

func (r *Resp) readBulk() (Value, error) {
	v := Value{}
	v.Type = "bulk"

	length, _, err := r.readInteger()
	if err != nil {
		return v, err
	}

	bulk := make([]byte, length)
	_, err = io.ReadFull(r.reader, bulk)
	if err != nil {
		return v, err
	}
	_, _, err = r.readLine()
	if err != nil {
		return v, err
	}

	v.Bulk = string(bulk)
	return v, nil
}

func (r *Resp) readInteger() (x int, n int, err error) {
	line, n, err := r.readLine()
	if err != nil {
		return 0, 0, err
	}
	i64, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		return 0, n, err
	}
	return int(i64), n, nil
}
func (r *Resp) readBoolean() (b bool, err error) {
	line, _, err := r.readLine()
	if err != nil {
		return false, err
	}
	boolean, err := strconv.ParseBool(string(line))
	if err != nil {
		return false, err
	}
	return boolean, err
}

// Marshal funcs

func (v Value) Marshal() []byte {
	switch v.Type {
	case "array":
		return v.marshalArray()
	case "bulk":
		return v.marshalBulk()
	case "string":
		return v.marshalString()
	case "null":
		return v.marshalNull()
	case "error":
		return v.marshalError()
	default:
		return []byte{}
	}
}

func (v Value) marshalString() []byte {
	var bytes []byte
	bytes = append(bytes, STRING)
	bytes = append(bytes, []byte(v.Str)...)
	bytes = append(bytes, '\r', '\n')
	return bytes
}

func (v Value) marshalBulk() []byte {
	var bytes []byte
	bytes = append(bytes, BULK)
	bytes = append(bytes, strconv.Itoa(len(v.Bulk))...)
	bytes = append(bytes, '\r', '\n')
	bytes = append(bytes, []byte(v.Bulk)...)
	bytes = append(bytes, '\r', '\n')

	return bytes
}
func (v Value) marshalArray() []byte {
	len := len(v.Array)
	var bytes []byte
	bytes = append(bytes, ARRAY)
	bytes = append(bytes, strconv.Itoa(len)...)
	bytes = append(bytes, '\r', '\n')
	for i := range len {
		bytes = append(bytes, v.Array[i].Marshal()...)
	}
	return bytes
}

func (v Value) marshalError() []byte {
	var bytes []byte
	bytes = append(bytes, ERROR)
	bytes = append(bytes, []byte(v.Str)...)
	bytes = append(bytes, '\r', '\n')

	return bytes
}

func (v Value) marshalNull() []byte {
	return []byte("$-1\r\n")
}
