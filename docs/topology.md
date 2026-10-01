# Watchdog Topology & Root-Cause Analysis (RCA) Engine

Watchdog incorporates an in-memory, directed **Topology Graph Engine** (`internal/topology`) and a multi-factor heuristic **Root-Cause Analysis (RCA) Engine** (`internal/intelligence/rootcause.go`). These subsystems provide structural observability, single-point-of-failure (SPOF) risk scoring, cascading blast radius evaluation, and deterministic failure propagation tracing across service architectures and physical infrastructure.

---

## 1. Architectural Invariants & Guarantees

The Topology and Root-Cause Analysis components uphold core operational invariants:

1. **Strictly In-Memory, Cycle-Resistant & Deterministic**:
   - Built on in-memory directed graphs (`Graph`) with concurrent read/write protection (`sync.RWMutex`).
   - All traversals (BFS, DFS, shortest path, transitive closures) implement visited-set tracking to remain safe against arbitrary dependency cycles.
   - Deterministic tie-breaking ensures identical scores, rankings, and shortest paths across multiple evaluations.

2. **Observational & Analytical (Zero Remediation)**:
   - Does not invoke shells, execute binaries, restart services, or alter routing tables.
   - Provides explainable factor decompositions and non-invasive operator recommendations (e.g., check database connection pools, verify disk space, inspect logs).

3. **Multi-Tier Security & Authentication**:
   - REST API endpoints (`/api/v1/topology/*`, `/api/v1/intelligence/incidents/{id}/rootcause`) enforce Bearer token authentication and TLS verification on non-loopback addresses.
   - Model Context Protocol (MCP) integrations expose strictly read-only tools gated by `AllowedReadOperations`.

4. **Resource Bounded**:
   - Zero-CGO pure Go implementation with zero external runtime dependencies.
   - Fast sub-millisecond graph traversals and bounded memory footprint for enterprise-scale topologies (>10,000 nodes).

---

## 2. In-Memory Directed Topology Graph

The core graph (`internal/topology/graph.go`) tracks infrastructure components as nodes and directed relationships as edges.

### Node Types (`NodeType`)

| Type | Description |
| :--- | :--- |
| `service` | Microservice, daemon, or application process |
| `database` | Relational or NoSQL database (PostgreSQL, MySQL, Cassandra) |
| `cache` | In-memory key-value cache (Redis, Memcached) |
| `message_broker` | Distributed queue/stream (Kafka, RabbitMQ, NATS) |
| `gateway` | Ingress proxy or API Gateway (Nginx, Envoy, Traefik) |
| `load_balancer` | L4/L7 load balancer (HAProxy, Cloud LB) |
| `external_api` | Third-party upstream service or external SaaS |
| `physical_host` | Bare-metal server host |
| `virtual_machine` | Hypervisor-hosted VM instance |
| `container` | Containerized pod or task instance |
| `network_device` | Router, switch, or firewall appliance |

### Relationship Types (`RelationshipType`)

| Type | Semantics |
| :--- | :--- |
| `depends_on` | Upstream dependency; source fails if target is unavailable |
| `communicates_with` | Network/RPC communication between components |
| `runs_on` | Hosting relationship (e.g., container runs on physical host) |
| `routes_to` | Traffic forwarding from gateway/load balancer |
| `writes_to` | Data mutation dependency (e.g., service writes to database) |
| `reads_from` | Read-only dependency (e.g., service reads from cache) |

### Graph Operations & Capabilities

- **Cycle-Safe Transitive Traversal**: `GetTransitiveDependencies(nodeID, maxDepth)` and `GetTransitiveDependents(nodeID, maxDepth)` evaluate upstream dependencies and downstream dependents up to bounded depth without infinite loops.
- **Shortest Path Discovery**: `FindShortestPath(sourceID, targetID)` uses Breadth-First Search (BFS) to compute the minimal hop path between any two components.
- **All Paths Enumeration**: `FindAllPaths(sourceID, targetID, maxDepth)` identifies all distinct propagation routes.
- **Connected Components**: `GetConnectedComponents()` isolates disconnected sub-graphs.
- **Graphviz DOT Export**: `ExportDOT()` produces standards-compliant DOT syntax for graph visualization.
- **Sub-graphs & Cloning**: `SubGraph(nodeIDs)` and `Clone()` provide thread-safe graph isolation.

---

## 3. SPOF (Single Point of Failure) Analysis

The SPOF Analyzer (`internal/topology/spof.go`) identifies architectural choke points where component failure causes widespread degradation.

### Criticality Score Formula ($0.0 \le \text{Score} \le 100.0$)

$$\text{CriticalityScore} = \min(100.0, S_{\text{transitive}} + S_{\text{direct}} + S_{\text{type}} + S_{\text{health}} + S_{\text{redundancy}})$$

1. **Transitive Dependents Score ($S_{\text{transitive}}$, max 40.0 pts)**:
   - Evaluates total downstream dependents reachable from the node.
   - Scaled logarithmically and linearly based on fleet impact ratio.
2. **Direct Dependents Score ($S_{\text{direct}}$, max 20.0 pts)**:
   - Points awarded per direct dependent ($4.0\text{ pts}$ per direct dependent up to 20.0 pts).
3. **Node Type Structural Weight ($S_{\text{type}}$, max 20.0 pts)**:
   - Database: $20.0\text{ pts}$
   - Message Broker: $18.0\text{ pts}$
   - Cache: $15.0\text{ pts}$
   - Gateway / Load Balancer: $15.0\text{ pts}$
   - Service: $10.0\text{ pts}$
4. **Health Status Modifier ($S_{\text{health}}$, 0 to +15.0 pts)**:
   - `NodeStatusCritical`: $+15.0\text{ pts}$
   - `NodeStatusDegraded`: $+10.0\text{ pts}$
   - `NodeStatusWarning`: $+5.0\text{ pts}$
5. **Redundancy Penalty/Relief ($S_{\text{redundancy}}$, -15.0 to +15.0 pts)**:
   - `RedundancyNone` ($1\times$ replica): $+15.0\text{ pts}$
   - `RedundancyActivePassive` ($2\times$ replicas): $-5.0\text{ pts}$
   - `RedundancyActiveActive` ($\ge 3\times$ replicas): $-15.0\text{ pts}$

### Risk Classification

- **`CRITICAL`** ($\text{Score} \ge 80.0$): Single point of failure with high blast radius and no redundancy.
- **`WARNING`** ($50.0 \le \text{Score} < 80.0$): Significant failure impact requiring architectural review.
- **`INFO`** ($\text{Score} < 50.0$): Low impact or well-redundant component.

---

## 4. Cascading Blast Radius & Impact Analysis

The Impact Analyzer (`internal/topology/impact.go`) simulates or calculates the downstream fallout of a component outage:

- **Impacted Components**: Computes all direct and transitive downstream dependents.
- **Blast Radius Score ($0.0 \le \text{Score} \le 100.0$)**:
  $$\text{BlastRadiusScore} = \left(\frac{N_{\text{affected}}}{N_{\text{total}}} \times 60.0\right) + \min(20.0, \text{MaxDepth} \times 5.0) + S_{\text{types}}$$
- **Maximum Depth**: Number of transitive hops the failure cascades across.
- **Critical Path Nodes**: High-impact intermediary nodes traversed during propagation.

---

## 5. Multi-Factor Heuristic Root-Cause Analysis (RCA) Engine

The RCA Engine (`internal/intelligence/rootcause.go`) scores all candidate nodes involved in an incident to pinpoint the primary root cause and trace failure propagation chains.

### Multi-Factor Scoring Formulation

For each candidate node $C$, the composite score $Score(C)$ is:

$$Score(C) = w_{\text{time}} S_{\text{time}} + w_{\text{topo}} S_{\text{topo}} + w_{\text{sev}} S_{\text{sev}} + w_{\text{blast}} S_{\text{blast}} + w_{\text{hist}} S_{\text{hist}}$$

Default Weights ($\sum w_i = 1.0$):
- $w_{\text{time}} = 0.30$ (Temporal Precedence)
- $w_{\text{topo}} = 0.25$ (Topological Centrality & Upstream Position)
- $w_{\text{sev}} = 0.20$ (Fault Severity)
- $w_{\text{blast}} = 0.15$ (Blast Radius Coverage)
- $w_{\text{hist}} = 0.10$ (Historical Recurrence & Capacity Predictions)

### Factor Scoring Definitions

1. **Temporal Precedence ($S_{\text{time}}$, 0-100)**:
   - Evaluates the chronological timestamp $t_C$ of the first alert, anomaly, or event on node $C$ relative to incident start $t_0$.
   - Earlier events receive higher scores: $S_{\text{time}} = 100 \cdot e^{-\lambda (t_C - t_0)}$.
2. **Topological Centrality ($S_{\text{topo}}$, 0-100)**:
   - Measures whether $C$ is upstream of other affected nodes. Nodes with zero upstream affected dependencies and multiple downstream affected dependents receive maximum topological score. Includes SPOF weight.
3. **Fault Severity ($S_{\text{sev}}$, 0-100)**:
   - Severity of active alerts and diagnostics on node $C$ (`CRITICAL` = 100, `WARNING` = 60, `INFO` = 20).
4. **Blast Radius Coverage ($S_{\text{blast}}$, 0-100)**:
   - Ratio of incident-affected nodes that can be reached downstream from $C$ in the topology graph.
5. **Historical Context ($S_{\text{hist}}$, 0-100)**:
   - Boosted if node $C$ exhibits statistical flapping recurrence patterns ($CV < 0.5$) or active capacity threshold exhaustion predictions.

### Causal Propagation Chains

The engine computes failure propagation chains from the primary root cause to affected leaf nodes:
$$\text{Root Node} \longrightarrow \text{Intermediary Service} \longrightarrow \text{Edge Gateway}$$
Each chain records the exact hops, relationship types, and arrival time offsets.

---

## 6. REST API Reference

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/v1/topology` | Retrieve full or filtered topology graph |
| `GET` | `/api/v1/topology/summary` | Retrieve node/edge counts, density, and status distributions |
| `GET` | `/api/v1/topology/dot` | Export graph in Graphviz DOT format |
| `GET` | `/api/v1/topology/path?source={src}&target={tgt}` | Compute shortest dependency path between two nodes |
| `GET` | `/api/v1/topology/spof` | List all single points of failure exceeding threshold |
| `GET` | `/api/v1/topology/spof/{node_id}` | Detailed SPOF analysis for a specific node |
| `GET` | `/api/v1/topology/impact/{node_id}` | Cascading blast radius analysis for a specific node |
| `GET` | `/api/v1/topology/nodes/{node_id}` | Retrieve specific node details |
| `GET` | `/api/v1/topology/dependencies/{node_id}` | Retrieve upstream dependencies of a node |
| `GET` | `/api/v1/topology/dependents/{node_id}` | Retrieve downstream dependents of a node |
| `POST` | `/api/v1/topology/nodes` | Declare/register a topology node |
| `POST` | `/api/v1/topology/dependencies` | Declare/register a dependency relationship |
| `DELETE` | `/api/v1/topology/nodes/{node_id}` | Remove a declared topology node |
| `DELETE` | `/api/v1/topology/dependencies` | Remove a declared dependency relationship |
| `GET` | `/api/v1/intelligence/incidents/{id}/rootcause` | Perform root cause analysis on an incident |

---

## 7. Model Context Protocol (MCP) Surface

Watchdog exposes 6 dedicated read-only MCP tools and 2 prompt templates for AI-assisted topology investigation:

### Tools

- `get_topology`: Retrieve graph nodes and dependencies matching type, status, or tag filters.
- `get_topology_summary`: Retrieve node counts, density, and health status summary.
- `get_topology_path`: Compute shortest dependency path between `source_id` and `target_id`.
- `get_spofs`: List single points of failure with optional `min_criticality` threshold.
- `get_node_impact`: Evaluate cascading blast radius and downstream dependents for `node_id`.
- `analyze_root_cause`: Execute multi-factor RCA on `incident_id` with factor breakdowns and causal propagation paths.

### Prompts

- `topology_spof_analysis`: Guided single-point-of-failure and architectural resilience audit.
- `root_cause_analysis`: Step-by-step incident investigation with topological path tracing and non-invasive guidance.

---

## 8. CLI Commands

```bash
# Topology Inspection
watchdog topology inspect --format json
watchdog topology summary
watchdog topology export --format dot > topology.dot

# Shortest Path & Dependency Tracing
watchdog topology path --source api-gateway --target postgres-primary

# Single Point of Failure Analysis
watchdog topology spof --min-criticality 50

# Blast Radius Impact Analysis
watchdog topology impact --node-id redis-cache

# Incident Root-Cause Analysis
watchdog intelligence rootcause --incident-id inc-20260928-01
```

---

## 9. Prometheus Metrics

| Metric | Type | Labels | Description |
| :--- | :--- | :--- | :--- |
| `watchdog_topology_nodes_total` | Gauge | `type`, `status` | Total count of topology nodes by type and status |
| `watchdog_topology_dependencies_total` | Gauge | `type` | Total count of dependency edges by relationship type |
| `watchdog_topology_spof_criticality_score` | Gauge | `node_id`, `risk_level` | Criticality score of single points of failure |
| `watchdog_topology_impact_nodes_count` | Gauge | `node_id`, `impact_level` | Number of downstream nodes affected by node outage |
| `watchdog_rootcause_score` | Gauge | `incident_id`, `node_id`, `rank` | Composite root-cause confidence score for incident candidate |
