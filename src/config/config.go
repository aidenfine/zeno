package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Heartbeat HeartbeatConfig `yaml:"heartbeat"`
	RPC       RPCConfig       `yaml:"rpc"`
	Storage   StorageConfig   `yaml:"storage"`
	Cluster   ClusterConfig   `yaml:"cluster"`
}

type ServerConfig struct {
	RESPPort int `yaml:"resp_port"` // RESP/TCP por
	GRPCPort int `yaml:"grpc_port"` // gRPC port
}

type HeartbeatConfig struct {
	Interval Duration `yaml:"interval"` // leader heartbeat ping interval
}

type RPCConfig struct {
	DialPort int      `yaml:"dial_port"` // port used to dial peer nodes
	Timeout  Duration `yaml:"timeout"`   // RPC deadline
}

type StorageConfig struct {
	AOFEnabled    bool     `yaml:"aof_enabled"`
	AOFPath       string   `yaml:"aof_path"`
	FsyncInterval Duration `yaml:"fsync_interval"`
}

type ClusterConfig struct {
	Leader string       `yaml:"leader"` // ip of the initial leader node
	Nodes  []NodeConfig `yaml:"nodes"`  // every node in the cluster (leader included)
}

type NodeConfig struct {
	IP    string `yaml:"ip"`
	Name  string `yaml:"name"`  // human/container name, e.g. "zeno-a1"
	State string `yaml:"state"` // read-write | read | write | none | down
}

func Default() *Config {
	return &Config{
		Server:    ServerConfig{RESPPort: 6379, GRPCPort: 6380},
		Heartbeat: HeartbeatConfig{Interval: Duration{30 * time.Second}},
		RPC:       RPCConfig{DialPort: 6380, Timeout: Duration{2 * time.Second}},
		Storage: StorageConfig{
			AOFEnabled:    false,
			AOFPath:       "database.aof",
			FsyncInterval: Duration{time.Second},
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Server.RESPPort < 1 || c.Server.RESPPort > 65535 {
		return fmt.Errorf("server.resp_port must be between 1 and 65535")
	}
	if c.RPC.DialPort < 1 || c.RPC.DialPort > 65535 {
		return fmt.Errorf("rpc.dial_port must be between 1 and 65535")
	}
	if c.RPC.Timeout.Duration <= 0 {
		return fmt.Errorf("rpc.timeout must be positive")
	}
	if c.Heartbeat.Interval.Duration <= 0 {
		return fmt.Errorf("heartbeat.interval must be positive")
	}

	if c.Cluster.Leader == "" {
		return fmt.Errorf("cluster.leader is required")
	}
	if len(c.Cluster.Nodes) == 0 {
		return fmt.Errorf("cluster.nodes must list at least one node")
	}

	leaderFound := false
	seen := make(map[string]bool)
	for _, n := range c.Cluster.Nodes {
		if n.IP == "" {
			return fmt.Errorf("every node needs an ip")
		}
		if seen[n.IP] {
			return fmt.Errorf("duplicate node %q", n.IP)
		}
		seen[n.IP] = true
		switch n.State {
		case "none", "read", "write", "read-write", "down":
		default:
			return fmt.Errorf("invalid state %q for node %s", n.State, n.IP)
		}
		if n.IP == c.Cluster.Leader {
			leaderFound = true
		}
	}
	if !leaderFound {
		return fmt.Errorf("cluster.leader %q is not listed in cluster.nodes", c.Cluster.Leader)
	}
	return nil
}
