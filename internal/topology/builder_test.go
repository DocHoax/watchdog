package topology

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestBuilder_IngestSnapshot(t *testing.T) {
	builder := NewBuilder()
	g := NewGraph()

	snapshot := &model.SystemSnapshot{
		Timestamp: time.Now().UTC(),
		System: &model.SystemInfo{
			HostID:   "host-node-1",
			Hostname: "worker-prod-01",
			OS:       "linux",
			Platform: "ubuntu",
		},
		Docker: &model.DockerSummary{
			Available: true,
			Containers: []model.DockerContainer{
				{
					ID:    "c1234567890ab",
					Names: []string{"/postgres-primary"},
					Image: "postgres:15-alpine",
					State: "running",
				},
			},
		},
		Ports: []model.PortInfo{
			{
				Protocol:    "tcp",
				Port:        5432,
				BindAddress: "0.0.0.0",
				State:       "LISTEN",
				ProcessName: "postgres",
			},
		},
	}

	builder.IngestSnapshot(g, snapshot, "host-node-1")

	// Verify host node exists
	hostNode, found := g.GetNode("host-host-node-1")
	if !found {
		t.Fatalf("expected host-host-node-1 to exist")
	}
	if hostNode.Name != "worker-prod-01" {
		t.Errorf("expected hostname worker-prod-01, got %s", hostNode.Name)
	}

	// Verify container node exists and inferred as NodeTypeDatabase
	containerNode, found := g.GetNode("container-c1234567890a")
	if !found {
		t.Fatalf("expected container-c1234567890a to exist")
	}
	if containerNode.Type != NodeTypeDatabase {
		t.Errorf("expected NodeTypeDatabase for postgres image, got %s", containerNode.Type)
	}

	// Verify endpoint node exists
	endpointNode, found := g.GetNode("endpoint-host-node-1-tcp-5432")
	if !found {
		t.Fatalf("expected endpoint node to exist")
	}
	if endpointNode.Type != NodeTypeNetworkEndpoint {
		t.Errorf("expected NodeTypeNetworkEndpoint, got %s", endpointNode.Type)
	}
}

func TestTopologyService_EndToEnd(t *testing.T) {
	svc := NewService()

	svc.AddDeclaredNode(TopologyNode{
		ID:     "api-gw",
		Name:   "API Gateway",
		Type:   NodeTypeService,
		Status: NodeStatusHealthy,
	})
	svc.AddDeclaredNode(TopologyNode{
		ID:     "user-db",
		Name:   "User Database",
		Type:   NodeTypeDatabase,
		Status: NodeStatusHealthy,
	})
	svc.AddDeclaredDependency(Dependency{
		SourceID: "api-gw",
		TargetID: "user-db",
		Type:     RelDependsOn,
	})

	resp := svc.GetTopology(TopologyFilter{})
	if len(resp.Nodes) != 2 {
		t.Errorf("expected 2 nodes in response, got %d", len(resp.Nodes))
	}
	if len(resp.Dependencies) != 1 {
		t.Errorf("expected 1 dependency in response, got %d", len(resp.Dependencies))
	}

	path, found := svc.FindPath("api-gw", "user-db")
	if !found || path.Hops != 1 {
		t.Errorf("expected 1 hop path between api-gw and user-db")
	}

	spof := svc.AnalyzeSPOF("user-db")
	if spof == nil {
		t.Fatalf("expected non-nil SPOF analysis")
	}

	impact := svc.AnalyzeImpact("user-db")
	if impact == nil || len(impact.DirectDependents) != 1 {
		t.Errorf("expected impact analysis to show 1 direct dependent")
	}
}
