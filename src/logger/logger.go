package logger

import (
	"bytes"
	"io"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

type Logger struct {
	w  io.Writer
	b  bytes.Buffer
	mu *sync.RWMutex
	re *lipgloss.Renderer

	level   int64
	message string
}
