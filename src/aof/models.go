package aof

import (
	"bufio"
	"os"
	"sync"
)

type Aof struct {
	File *os.File
	Rd   *bufio.Reader
	Mu   sync.Mutex
}
