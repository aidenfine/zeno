package controlplane

import (
	"testing"
	"zeno/src/config"
)

func TestHealthTransitionsPreserveConfiguredRoles(t *testing.T) {
	cfg := config.Default()
	cfg.Cluster = config.ClusterConfig{Leader: "leader", Nodes: []config.NodeConfig{
		{IP: "leader", State: "read-write"},
		{IP: "reader", State: "read"},
		{IP: "disabled", State: "down"},
	}}
	c, err := New(cfg.Cluster, cfg.RPC)
	if err != nil {
		t.Fatal(err)
	}
	if recovered := c.ReconcileHealth(nil); len(recovered) != 0 {
		t.Fatalf("healthy nodes reported recovered: %v", recovered)
	}
	if c.nodeState["reader"] != ReadState || c.nodeState["disabled"] != DownState {
		t.Fatal("configured roles were overwritten")
	}
	c.ReconcileHealth([]string{"reader"})
	if c.nodeState["reader"] != DownState {
		t.Fatal("failed reader was not marked down")
	}
	recovered := c.ReconcileHealth(nil)
	if len(recovered) != 1 || recovered[0] != "reader" {
		t.Fatalf("unexpected recovered nodes: %v", recovered)
	}
	if c.nodeState["reader"] != ReadState {
		t.Fatal("recovery did not restore read-only role")
	}
	if recovered := c.ReconcileHealth(nil); len(recovered) != 0 {
		t.Fatalf("repeated recovery: %v", recovered)
	}
}
