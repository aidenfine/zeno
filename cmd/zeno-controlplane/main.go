package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	"zeno/src/config"
	"zeno/src/controlplane"
	"zeno/src/gateway"
	"zeno/src/utils"
)

func main() {
	path := flag.String("config", "zeno.yaml", "control-plane configuration file")
	flag.Parse()
	if err := run(*path); err != nil {
		slog.Error("control plane failed", "error", err)
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
					slog.Error("heartbeat failed", "error", err)
					continue
				}
				for _, addr := range cluster.ReconcileHealth(failed) {
					if err := cluster.RestartNode(addr, queue); err != nil {
						slog.Error("node recovery failed", "node", addr, "error", err)
					}
				}
			}
		}
	}()
	slog.Info("control plane listening", "address", l.Addr())
	return gateway.Serve(ctx, l, cluster, queue)
}
