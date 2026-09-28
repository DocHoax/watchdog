package topology

import (
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// NodeType enumerates the types of infrastructure and service entities in the topology.
type NodeType string

const (
	NodeTypePhysicalHost    NodeType = "physical_host"
	NodeTypeVM              NodeType = "vm"
	NodeTypeContainer       NodeType = "container"
	NodeTypeK8sNode         NodeType = "k8s_node"
	NodeTypeK8sPod          NodeType = "k8s_pod"
	NodeTypeService         NodeType = "service"
	NodeTypeDatabase        NodeType = "database"
	NodeTypeCache           NodeType = "cache"
	NodeTypeQueue           NodeType = "queue"
	NodeTypeStorage         NodeType = "storage"
	NodeTypeNetworkEndpoint NodeType = "network_endpoint"
	NodeTypeCustom          NodeType = "custom"
)

// NodeStatus defines the operational health status of a topology node.
type NodeStatus string

const (
	NodeStatusHealthy  NodeStatus = "healthy"
	NodeStatusDegraded NodeStatus = "degraded"
	NodeStatusCritical NodeStatus = "critical"
	NodeStatusUnknown  NodeStatus = "unknown"
	NodeStatusOffline  NodeStatus = "offline"
)

// RelationshipType specifies the directional dependency relationship between topology nodes.
type RelationshipType string

const (
	RelHosts            RelationshipType = "hosts"             // Source hosts Target (e.g. host -> container)
	RelRunsOn           RelationshipType = "runs_on"           // Source runs on Target (e.g. service -> host)
	RelDependsOn        RelationshipType = "depends_on"        // Source depends on Target for functionality
	RelCommunicatesWith RelationshipType = "communicates_with" // Source communicates with Target over network
	RelStoresOn         RelationshipType = "stores_on"         // Source stores data on Target
	RelReadsFrom        RelationshipType = "reads_from"        // Source reads data from Target
	RelWritesTo         RelationshipType = "writes_to"         // Source writes data to Target
	RelRoutesTo         RelationshipType = "routes_to"         // Source routes traffic to Target
	RelContains         RelationshipType = "contains"          // Source contains Target
	RelMemberOf         RelationshipType = "member_of"         // Source is a member of cluster/group Target
)

// Confidence rates the certainty of an observed or inferred dependency relationship.
type Confidence string

const (
	ConfidenceHigh    Confidence = "high"
	ConfidenceMedium  Confidence = "medium"
	ConfidenceLow     Confidence = "low"
	ConfidenceUnknown Confidence = "unknown"
)

// TopologyNode represents an individual infrastructure, container, service, or logical entity.
type TopologyNode struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Type          NodeType          `json:"type"`
	Status        NodeStatus        `json:"status"`
	HostID        string            `json:"host_id,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Source        string            `json:"source"`
	FirstObserved time.Time         `json:"first_observed"`
	LastObserved  time.Time         `json:"last_observed"`
}

// Dependency represents a directional dependency edge between two topology nodes.
type Dependency struct {
	ID            string            `json:"id"`
	SourceID      string            `json:"source_id"`
	TargetID      string            `json:"target_id"`
	Type          RelationshipType  `json:"type"`
	Weight        float64           `json:"weight"`
	Confidence    Confidence        `json:"confidence"`
	Evidence      []string          `json:"evidence,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	FirstObserved time.Time         `json:"first_observed"`
	LastObserved  time.Time         `json:"last_observed"`
}

// DependencyPath represents a directed traversal path between two nodes in the topology graph.
type DependencyPath struct {
	SourceID    string         `json:"source_id"`
	TargetID    string         `json:"target_id"`
	Nodes       []TopologyNode `json:"nodes"`
	Edges       []Dependency   `json:"edges"`
	TotalWeight float64        `json:"total_weight"`
	Hops        int            `json:"hops"`
}

// TopologyFilter defines query parameters for filtering nodes and subgraphs.
type TopologyFilter struct {
	Types     []NodeType        `json:"types,omitempty"`
	Statuses  []NodeStatus      `json:"statuses,omitempty"`
	Source    string            `json:"source,omitempty"`
	HostID    string            `json:"host_id,omitempty"`
	TagFilter map[string]string `json:"tag_filter,omitempty"`
	Search    string            `json:"search,omitempty"`
	MaxDepth  int               `json:"max_depth,omitempty"`
}

// TopologyGraphSummary aggregates structural metrics and statistics about the topology graph.
type TopologyGraphSummary struct {
	TotalNodes             int            `json:"total_nodes"`
	TotalDependencies      int            `json:"total_dependencies"`
	NodeTypeCounts         map[string]int `json:"node_type_counts"`
	RelationshipTypeCounts map[string]int `json:"relationship_type_counts"`
	StatusCounts           map[string]int `json:"status_counts"`
	ConnectedComponents   int            `json:"connected_components"`
	Density                float64        `json:"density"`
	EvaluatedAt            time.Time      `json:"evaluated_at"`
}

// TopologyGraphResponse represents the payload for topology graph queries.
type TopologyGraphResponse struct {
	Nodes        []TopologyNode       `json:"nodes"`
	Dependencies []Dependency         `json:"dependencies"`
	Summary      TopologyGraphSummary `json:"summary"`
	EvaluatedAt  time.Time            `json:"evaluated_at"`
}

// RedundancyClassification classifies the backup/failover redundancy level for a node.
type RedundancyClassification string

const (
	RedundancyNone      RedundancyClassification = "none"
	RedundancyPartial   RedundancyClassification = "partial"
	RedundancyRedundant RedundancyClassification = "redundant"
)

// SPOFAnalysis details a detected Single Point of Failure in the topology.
type SPOFAnalysis struct {
	NodeID                    string                   `json:"node_id"`
	NodeName                  string                   `json:"node_name"`
	NodeType                  NodeType                 `json:"node_type"`
	Status                    NodeStatus               `json:"status"`
	CriticalityScore          float64                  `json:"criticality_score"`
	DependentsCount           int                      `json:"dependents_count"`
	TransitiveDependentsCount int                      `json:"transitive_dependents_count"`
	AffectedServices          []string                 `json:"affected_services"`
	RedundancyLevel           RedundancyClassification `json:"redundancy_level"`
	AlternativePaths          int                      `json:"alternative_paths"`
	RiskLevel                 model.Severity           `json:"risk_level"`
	Reasoning                 []string                 `json:"reasoning"`
}

// BlastRadiusLevel categorizes the severity of potential impact.
type BlastRadiusLevel string

const (
	BlastRadiusCritical BlastRadiusLevel = "critical"
	BlastRadiusHigh     BlastRadiusLevel = "high"
	BlastRadiusMedium   BlastRadiusLevel = "medium"
	BlastRadiusLow      BlastRadiusLevel = "low"
	BlastRadiusMinimal  BlastRadiusLevel = "minimal"
)

// TopologyImpactAnalysis details the blast radius and downstream consequences of a node failure.
type TopologyImpactAnalysis struct {
	TargetNodeID          string            `json:"target_node_id"`
	TargetNodeName        string            `json:"target_node_name"`
	TargetNodeType        NodeType          `json:"target_node_type"`
	TargetNodeStatus      NodeStatus        `json:"target_node_status"`
	DirectDependents      []TopologyNode    `json:"direct_dependents"`
	TransitiveDependents  []TopologyNode    `json:"transitive_dependents"`
	MaxImpactDepth        int               `json:"max_impact_depth"`
	BlastRadiusScore      float64           `json:"blast_radius_score"`
	BlastRadiusLevel      BlastRadiusLevel  `json:"blast_radius_level"`
	AffectedNodeTypes     map[string]int    `json:"affected_node_types"`
	AffectedStatuses      map[string]int    `json:"affected_statuses"`
	CriticalNodesAffected int               `json:"critical_nodes_affected"`
	WarningNodesAffected  int               `json:"warning_nodes_affected"`
	Summary               string            `json:"summary"`
	AnalyzedAt            time.Time         `json:"analyzed_at"`
}
