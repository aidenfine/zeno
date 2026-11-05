// Package logger provides the shared server logger.
package logger

import (
	"time"

	"github.com/aidenfine/flog"
)

// Log writes timestamped server logs with structured fields to stderr.
// Pass fields with flog constructors such as flog.String or flog.Int. INFO excludes per-command debug logs.
var Log = newLogger("zeno")

// Init sets the service name for all server packages. Call once during startup,
// before starting any goroutines that log.
func Init(name string) {
	next := newLogger(name)
	Log.Close()
	Log = next
}

func newLogger(name string) *flog.Logger {
	log, err := flog.New(false, name, flog.Config{
		LogLevel:     "INFO",
		ShowTime:     true,
		TimeEncoding: time.RFC3339,
		Encoding:     "console",
	})
	if err != nil {
		panic(err)
	}
	return log
}
