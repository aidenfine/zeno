package main

import (
	"context"
	"flag"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	"zeno/src/config"
	"zeno/src/controlplane"
	"zeno/src/gateway"
	"zeno/src/logger"
	"zeno/src/utils"

	"github.com/aidenfine/flog"
)

func main() {
	path := flag.String("config", "zeno.yaml", "control-plane configuration file")
	name := flag.String("name", "zeno-controlplane", "service name used in logs")
	flag.Parse()
	logger.Init(*name)
	defer logger.Log.Close()
	if err := run(*path); err != nil {
		logger.Log.Error("control plane failed", flog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(path string) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	cluster, err := controlplane.New(cfg.Cluster, cfg.RPC)
	if err != nil {
		return err
	}
	defer cluster.Close()
	l, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(cfg.Server.RESPPort)))
	if err != nil {
		return err
	}
	defer l.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	queue := utils.NewQueue[utils.Message]()
	go func() {
		ticker := time.NewTicker(cfg.Heartbeat.Interval.Duration)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				failed, err := cluster.SendHeartbeat()
				if err != nil {
					logger.Log.Error("heartbeat failed", flog.String("error", err.Error()))
					continue
				}
				for _, addr := range cluster.ReconcileHealth(failed) {
					if err := cluster.RestartNode(addr, queue); err != nil {
						logger.Log.Error("node recovery failed", flog.String("node", addr), flog.String("error", err.Error()))
					}
				}
			}
		}
	}()
	logger.Log.Info("control plane listening", flog.String("address", l.Addr().String()))
	return gateway.Serve(ctx, l, cluster, queue)
}
