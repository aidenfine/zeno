// Package logger provides the shared server logger.
package logger

import "github.com/aidenfine/flog"

// Log writes timestamped server logs to stderr. INFO excludes per-command debug logs.
var Log = newLogger("zeno")

// Init sets the service name for all server packages. Call once during startup,
// before starting any goroutines that log.
func Init(name string) {
	next := newLogger(name)
	Log.Close()
	Log = next
}

func newLogger(name string) *flog.Logger {
	log, err := flog.New(false, name, flog.Config{LogLevel: "INFO", ShowTime: true})
	if err != nil {
		panic(err)
	}
	return log
}
