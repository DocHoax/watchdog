# Watchdog Governance, Declarative Policy & Compliance Engine

Watchdog provides enterprise-grade organizational governance, hierarchical policy management, deterministic policy resolution, and operational compliance evaluation for large-scale distributed fleets.

---

## 1. Architectural Overview

The governance subsystem establishes a multi-tenant hierarchy and declarative policy framework:

```text
       ┌────────────────────────────────────────────────────────┐
       │               Organization (Tenant Root)               │
       │           (Baseline Policies - Hierarchy L1)           │
       └────────────────────────────────────────────────────────┘
                                   │
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │               Fleet Groups Tree (L2..L99)              │
       │    /infrastructure/prod/us-east (Inherited Policies)   │
       └────────────────────────────────────────────────────────┘
                                   │
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │              Fleet Node (Hierarchy L100)               │
       │             (Direct Node Policy Overrides)             │
       └────────────────────────────────────────────────────────┘
                                   │
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │              Deterministic Policy Resolver             │
       │  - Lexer/AST Selector Evaluation (Depth <= 16)         │
       │  - Precedence Tie-Breaking (Level ASC, Priority ASC)   │
       │  - Inheritance Modes (Override, Strict, Additive)      │
       │  - Conflict Detection (Definite vs Potential)          │
       │  - Operational Compliance Verification                 │
       └────────────────────────────────────────────────────────┘
```

---

## 2. Core Concepts

### 2.1 Multi-Tenant Organization & Group Hierarchy
- **Organizations (`model.Organization`)**: Top-level tenant boundaries. All policies, revisions, assignments, groups, and nodes are strictly scoped to an organization. Cross-organization access is rejected.
- **Fleet Groups (`model.FleetGroup`)**: Hierarchical tree-structured groups with cycle prevention and materialized hierarchical paths (e.g. `/root/prod/database`).
- **Fleet Group Membership (`model.FleetGroupMember`)**: Associates nodes with groups using primary and secondary roles.
- **Node Ownership Metadata (`model.NodeOwnershipMetadata`)**: Captures business criticality (`tier-0` through `tier-3`), environment, owner team, data classification, and cost center.

### 2.2 Declarative Policy Domain Model
- **Policy (`model.Policy`)**: Top-level policy container categorized by domain (`resource_thresholds`, `anomaly_detection`, `capacity_planning`, `incident_severity`, `operational_compliance`).
- **Policy Revision (`model.PolicyRevision`)**: Immutable, versioned revisions containing rules, selector expressions, priority, inheritance mode, and enforcement mode.
  - **Canonical SHA-256 Content Digest**: Every revision calculates a deterministic SHA-256 digest over normalized and sorted rule definitions and configuration metadata.
- **Policy Assignment (`model.PolicyAssignment`)**: Polymorphic assignment linking a policy to an `organization`, `fleet_group`, or direct `node`.

---

## 3. Selector Expression Engine (Safe AST)

Watchdog includes a bounded recursive-descent lexer, parser, and evaluator for node targeting:

### 3.1 Syntax and Operators
- **Field Accessors**: `hostname`, `status`, `architecture`, `os_name`, `os_family`, `cpu_cores`, `memory_gb`, `environment`, `business_criticality`, `owner_team`, `data_classification`, `cost_center`, `tags.<key>`, `group_ids`, `group_paths`.
- **Comparison Operators**: `==`, `!=`, `<`, `<=`, `>`, `>=`
- **Set Membership**: `IN [...]`, `NOT IN [...]`
- **Existence Checks**: `EXISTS <field>`, `NOT EXISTS <field>`
- **Boolean Logic**: `AND`, `OR`, `NOT`, and parenthesized sub-expressions `(...)`
- **Prefix / Path Matching**: Glob suffix matching on tags and paths (e.g., `tags.env == "prod*"` or `group_paths == "/root/prod/*"`).

### 3.2 Safety and Bounded Guarantees
- **Max Expression Length**: 1,024 characters.
- **Max Parser Recursion Depth**: 16 levels.
- **Reflection-Free Execution**: Constant-time AST node evaluation against strongly typed node contexts.

---

## 4. Deterministic Hierarchical Resolution

When resolving effective policies for any node:

### 4.1 Hierarchy Precedence Levels
1. **Organization Baseline**: Precedence Level `1`
2. **Fleet Group Hierarchy**: Precedence Levels `2` through `99` (deeper groups have higher hierarchy levels)
3. **Direct Node Assignments**: Precedence Level `100`

### 4.2 Deterministic Ordering
Policies and assignments are evaluated in strict order:
$$\text{HierarchyLevel ASC} \longrightarrow \text{Priority ASC} \longrightarrow \text{PolicyID ASC}$$

### 4.3 Inheritance Modes
- **`inherit_and_override` (Default)**: Ancestor rules apply unless an identical rule (matched by rule type and specific target, e.g. metric name or check type) is redefined at a higher precedence. Overridden rules maintain provenance tracking via `OverrodeRule`.
- **`strict_override`**: Higher-precedence policy completely wipes all rules previously accumulated in its category.
- **`additive`**: All rules accumulate monotonically across the hierarchy without replacing previous rules.

### 4.4 Enforcement Modes
- **`enforce`**: Active rules evaluated for operational alerting and monitoring.
- **`monitor_only`**: Rules tracked for telemetry and compliance dry-runs without firing active mitigation alerts.
- **`disabled`**: Policy is inactive.

---

## 5. Conflict Detection Engine

Watchdog statically analyzes policy revisions and resolved rules:

| Severity | Type | Description |
| :--- | :--- | :--- |
| **`definite`** | Effective Rule Contradiction | Conflicting thresholds or check expectations defined at the exact same hierarchy level and priority. |
| **`potential`** | Revision Divergence | Divergent metric thresholds or compliance requirements detected between policy revision pairs before assignment. |
| **`compatible`** | Complementary Overlap | Overlapping rules that do not contradict each other. |
| **`indeterminate`** | Selector Overlap | Dynamic selector conditions that cannot be statically proven disjoint. |

---

## 6. Operational Compliance Engine

The compliance engine evaluates fleet nodes against operational governance standards:

### 6.1 Supported Check Types
- **`heartbeat_freshness`**: Verifies node heartbeat is within `MaxAgeSeconds` (e.g., $\le 120\text{s}$).
- **`collector_version`**: Verifies agent collector semantic version against minimum required version expressions (e.g., `>= 1.5.0`).
- **`mandatory_tags`**: Enforces presence of required tags (e.g., `["env", "owner", "service"]`).
- **`approved_platforms`**: Ensures OS family and architecture match authorized platform lists (e.g., `linux/amd64`, `linux/arm64`).

### 6.2 Compliance Evaluation Output
Returns overall status (`COMPLIANT` vs `NON_COMPLIANT`), compliance ratio ($0.0 \dots 1.0$), and detailed per-rule evaluation logs including observed vs expected values.

---

## 7. Storage & Persistence Guarantees

All governance records are persisted in SQLite (WAL mode):
- **Tables**: `organizations`, `fleet_groups`, `fleet_group_members`, `node_ownership_metadata`, `policies`, `policy_revisions`, `policy_assignments`, `policy_resolution_audits`.
- **Foreign Keys & Cascades**: Foreign keys are enabled with cascading deletion for clean lifecycle teardown.
- **Tenant Isolation**: Queries strictly filter by `org_id` to prevent cross-tenant information leakage.

---

## 8. CLI & API Reference

### 8.1 CLI Commands
```bash
# Manage organizations
watchdog governance org list
watchdog governance org create --id "org-acme" --name "Acme Corp"

# Manage fleet group hierarchies
watchdog governance group create --org "org-acme" --id "grp-prod" --name "Production" --parent "grp-root"
watchdog governance group members add --group "grp-prod" --node "srv-node-01" --role "primary"

# Resolve and explain effective policies
watchdog governance policy resolve --node "srv-node-01"
watchdog governance policy explain --node "srv-node-01"

# Run compliance check
watchdog governance compliance scan --node "srv-node-01"
```

### 8.2 Programmatic Usage (Go SDK)
```go
service := governance.NewGovernanceService(store)

// Resolve node policies
result, err := service.ResolveNodePolicies(ctx, "org-acme", "srv-node-01")
if err != nil {
    log.Fatalf("Policy resolution failed: %v", err)
}

fmt.Printf("Effective Rules: %d\n", len(result.EffectiveRules))
for _, r := range result.EffectiveRules {
    fmt.Printf(" - %s (from policy %s)\n", r.Rule.Name, r.SourcePolicyID)
}

// Evaluate compliance
compliance, err := service.EvaluateNodeCompliance(ctx, "org-acme", "srv-node-01")
if err != nil {
    log.Fatalf("Compliance check failed: %v", err)
}
fmt.Printf("Compliance Status: %s (Ratio: %.2f%%)\n", compliance.OverallStatus, compliance.ComplianceRatio*100)
```
