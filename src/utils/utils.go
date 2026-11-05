package utils

import (
	"time"
	"zeno/src/logger"

	"github.com/aidenfine/flog"
)

// TODO: rework this function to maybe use genrics?
// Info this function will run at a delay of currTime + (interval * 2) due to the first interation being skipped.
func RunOnInterval(interval time.Duration, stopChan <-chan struct{}, fn func() ([]string, error), onResult func([]string)) {
	go func() {
		now := time.Now().UTC()
		next := now.Truncate(interval).Add(interval)
		time.Sleep(time.Until(next))

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				failedNodes, err := fn()
				if err != nil {
					logger.Log.Error("failed to check heartbeat", flog.String("error", err.Error()))
					continue
				}
				onResult(failedNodes)
			case <-stopChan:
				logger.Log.Info("Stopping interval runner")
				return
			}
		}
	}()
}
