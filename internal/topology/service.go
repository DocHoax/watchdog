package topology

import (
	"sync"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// Service defines the interface for topology graph management and dependency intelligence.
type Service interface {
	GetGraph() *Graph
	GetTopology(filter TopologyFilter) TopologyGraphResponse
	GetNode(id string) (TopologyNode, bool)
	GetDependencies(id string) []TopologyNode
	GetDependents(id string) []TopologyNode
	FindPath(sourceID, targetID string) (*DependencyPath, bool)
	AnalyzeSPOF(nodeID string) *SPOFAnalysis
	FindAllSPOFs(minCriticality float64) []SPOFAnalysis
	AnalyzeImpact(nodeID string) *TopologyImpactAnalysis
	ExportDOT() string
	IngestSnapshot(snapshot *model.SystemSnapshot, hostID string)
	IngestFleet(fleetNodes []model.NodeIdentity)
	AddDeclaredNode(node TopologyNode)
	AddDeclaredDependency(dep Dependency)
	RemoveNode(id string) bool
	RemoveDependency(sourceID, targetID string, relType RelationshipType) bool
}

// TopologyService implements the Service interface.
type TopologyService struct {
	mu             sync.RWMutex
	graph          *Graph
	builder        *Builder
	spofAnalyzer   *SPOFAnalyzer
	impactAnalyzer *ImpactAnalyzer
}

// NewService creates a new initialized TopologyService.
func NewService() *TopologyService {
	return &TopologyService{
		graph:          NewGraph(),
		builder:        NewBuilder(),
		spofAnalyzer:   NewSPOFAnalyzer(),
		impactAnalyzer: NewImpactAnalyzer(),
	}
}

// GetGraph returns the underlying in-memory Graph.
func (s *TopologyService) GetGraph() *Graph {
	return s.graph
}

// GetTopology returns the full or filtered topology response with graph metrics summary.
func (s *TopologyService) GetTopology(filter TopologyFilter) TopologyGraphResponse {
	targetGraph := s.graph
	if len(filter.Types) > 0 || len(filter.Statuses) > 0 || filter.Source != "" ||
		filter.HostID != "" || len(filter.TagFilter) > 0 || filter.Search != "" {
		targetGraph = s.graph.Filter(filter)
	}

	nodes := targetGraph.GetAllNodes()
	deps := targetGraph.GetAllDependencies()
	summary := targetGraph.Summary()

	return TopologyGraphResponse{
		Nodes:        nodes,
		Dependencies: deps,
		Summary:      summary,
		EvaluatedAt:  time.Now().UTC(),
	}
}

// GetNode retrieves a topology node by ID.
func (s *TopologyService) GetNode(id string) (TopologyNode, bool) {
	return s.graph.GetNode(id)
}

// GetDependencies returns the direct upstream dependencies of a node.
func (s *TopologyService) GetDependencies(id string) []TopologyNode {
	return s.graph.GetDirectDependencies(id)
}

// GetDependents returns the direct downstream dependents of a node.
func (s *TopologyService) GetDependents(id string) []TopologyNode {
	return s.graph.GetDirectDependents(id)
}

// FindPath calculates the shortest directed path between two nodes.
func (s *TopologyService) FindPath(sourceID, targetID string) (*DependencyPath, bool) {
	return s.graph.FindShortestPath(sourceID, targetID)
}

// AnalyzeSPOF calculates Single Point of Failure metrics for a specific node.
func (s *TopologyService) AnalyzeSPOF(nodeID string) *SPOFAnalysis {
	return s.spofAnalyzer.AnalyzeNode(s.graph, nodeID)
}

// FindAllSPOFs returns all topology nodes evaluated for SPOF, filtered by minimum criticality.
func (s *TopologyService) FindAllSPOFs(minCriticality float64) []SPOFAnalysis {
	return s.spofAnalyzer.FindAllSPOFs(s.graph, minCriticality)
}

// AnalyzeImpact calculates the downstream blast radius if the target node fails.
func (s *TopologyService) AnalyzeImpact(nodeID string) *TopologyImpactAnalysis {
	return s.impactAnalyzer.AnalyzeImpact(s.graph, nodeID)
}

// ExportDOT renders the topology graph in Graphviz DOT format.
func (s *TopologyService) ExportDOT() string {
	return s.graph.ExportDOT()
}

// IngestSnapshot extracts topology nodes and dependencies from a telemetry snapshot.
func (s *TopologyService) IngestSnapshot(snapshot *model.SystemSnapshot, hostID string) {
	s.builder.IngestSnapshot(s.graph, snapshot, hostID)
}

// IngestFleet adds fleet nodes to the topology graph.
func (s *TopologyService) IngestFleet(fleetNodes []model.NodeIdentity) {
	s.builder.IngestFleet(s.graph, fleetNodes)
}

// AddDeclaredNode adds or updates a declared node in the graph.
func (s *TopologyService) AddDeclaredNode(node TopologyNode) {
	if node.Source == "" {
		node.Source = "declared"
	}
	s.graph.AddNode(node)
}

// AddDeclaredDependency adds or updates a declared dependency edge.
func (s *TopologyService) AddDeclaredDependency(dep Dependency) {
	if len(dep.Evidence) == 0 {
		dep.Evidence = []string{"config_declared"}
	}
	s.graph.AddDependency(dep)
}

// RemoveNode removes a node and its edges from the graph.
func (s *TopologyService) RemoveNode(id string) bool {
	return s.graph.RemoveNode(id)
}

// RemoveDependency removes a specific dependency from the graph.
func (s *TopologyService) RemoveDependency(sourceID, targetID string, relType RelationshipType) bool {
	return s.graph.RemoveDependency(sourceID, targetID, relType)
}
