package aof

import (
	"bufio"
	"io"
	"os"
	"time"
	"zeno/src/resp"
)

func NewAof(path string) (*Aof, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}
	aof := &Aof{
		File: f,
		Rd:   bufio.NewReader(f),
	}
	go func() {
		for {
			aof.Mu.Lock()
			aof.File.Sync()
			aof.Mu.Unlock()
			time.Sleep(time.Second)
		}

	}()
	return aof, nil
}
func (aof *Aof) Close() error {
	aof.Mu.Lock()
	defer aof.Mu.Unlock()
	return aof.File.Close()
}
func (aof *Aof) Write(value resp.Value) error {
	aof.Mu.Lock()
	defer aof.Mu.Unlock()
	_, err := aof.File.Write(value.Marshal())
	if err != nil {
		return err
	}
	return nil
}

func (aof *Aof) Read(callback func(value resp.Value)) error {
	aof.Mu.Lock()
	defer aof.Mu.Unlock()
	response := resp.NewResp(aof.File)
	for {
		value, err := response.Read()
		if err == nil {
			callback(value)
		}
		if err == io.EOF {
			break
		}
		return err
	}
	return nil
}
