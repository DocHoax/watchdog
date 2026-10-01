# Watchdog Governance, Declarative Policy, Compliance Intelligence & Operational Governance Engine

Watchdog provides enterprise-grade organizational governance, hierarchical policy management, deterministic policy resolution, operational compliance evaluation, finding lifecycle tracking, isolated policy simulation, maintenance window scheduling, alert suppression with security guardrails, hierarchical ownership cascading, incident escalation policies, and unified audit trails for large-scale distributed fleets.

---

## 1. Architectural Overview

The governance subsystem establishes a multi-tenant hierarchy and declarative policy framework spanning static policy definition, runtime compliance evaluation, and operational fleet governance:

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
       └────────────────────────────────────────────────────────┘
                                   │
            ┌──────────────────────┴──────────────────────┐
            ▼                                             ▼
┌───────────────────────────────────────┐   ┌────────────────────────────────────────┐
│     Runtime Policy Evaluation Engine  │   │       Operational Governance Engine    │
│  - 5 Modular Rule Evaluators          │   │  - Maintenance Window Scheduler        │
│  - Data Acquisition & Freshness       │   │    (Daily / Weekly / Monthly Clamped)  │
│  - Finding Lifecycle & State Machine  │   │  - Alert Suppression & Security Guard  │
│  - Multi-Level Compliance Rollups     │   │  - Hierarchical Ownership Resolution   │
│  - Isolated Ephemeral Simulation      │   │  - Multi-Stage Incident Escalation     │
│    (Zero Mutation, Zero Alerts)       │   │  - Sanitized Audit Trail (Redaction)   │
└───────────────────────────────────────┘   └────────────────────────────────────────┘
```

---

## 2. Core Concepts & Domain Entities

### 2.1 Multi-Tenant Organization & Group Hierarchy
- **Organizations (`model.Organization`)**: Top-level tenant boundaries. All policies, revisions, assignments, groups, nodes, maintenance windows, and audit events are strictly scoped to an organization. Cross-organization access is rejected.
- **Fleet Groups (`model.FleetGroup`)**: Hierarchical tree-structured groups with cycle prevention and materialized hierarchical paths (e.g. `/root/prod/database`).
- **Fleet Group Membership (`model.FleetGroupMember`)**: Associates nodes with groups using primary, secondary, or observer roles.
- **Node Ownership Metadata (`model.NodeOwnershipMetadata`)**: Captures business criticality (`mission_critical`, `high`, `medium`, `low`), environment, owner team, contact email/channel, data classification, cost center, and lifecycle state (`active`, `maintenance`, `draining`, `decommissioned`).

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

## 6. Runtime Policy Evaluation & Findings Engine

The runtime policy evaluation engine assesses fleet health and operational compliance against resolved effective policies.

```text
┌───────────────────────────────┐
│     Node Telemetry & State    │
└───────────────┬───────────────┘
                │
                ▼
┌───────────────────────────────┐     ┌──────────────────────────────────────────────────┐
│   Data Acquisition Provider   ├────►│ Freshness Classifier: Fresh, Stale, Missing,     │
└───────────────┬───────────────┘     │ Invalid, Unsupported                             │
                │                     └──────────────────────────────────────────────────┘
                ▼
┌───────────────────────────────┐
│      Evaluator Registry       │
│  1. Resource Thresholds       │
│  2. Anomaly Detection         │
│  3. Capacity Planning         │
│  4. Incident Severity         │
│  5. Operational Compliance    │
└───────────────┬───────────────┘
                │
                ▼
┌───────────────────────────────┐     ┌──────────────────────────────────────────────────┐
│   Findings Lifecycle Engine   ├────►│ State Machine:                                   │
└───────────────┬───────────────┘     │   open ──► recurring ──► resolved                │
                │                     │     ▲                      │ (reopen)            │
                │                     │     └──────────────────────┘                     │
                ▼                     └──────────────────────────────────────────────────┘
┌───────────────────────────────┐
│   Compliance Rollup Engine    │
│  - Node / Group / Org Rollups │
└───────────────────────────────┘
```

### 6.1 Modular Rule Evaluators
1. **Resource Threshold Evaluator (`ResourceThresholdEvaluator`)**:
   - Evaluates CPU, memory, disk, network, and swap metrics against warning/critical thresholds across operators (`<`, `<=`, `>`, `>=`, `==`, `!=`).
   - Supports duration windows and consecutive sample evaluation.
2. **Anomaly Detection Evaluator (`AnomalyDetectionEvaluator`)**:
   - Compares observed values against rolling baselines using statistical z-scores.
   - Respects configured excluded hours and seasonal variations.
3. **Capacity Planning Evaluator (`CapacityPlanningEvaluator`)**:
   - Evaluates linear regression and days-to-exhaustion runway estimates against warning and critical horizons.
4. **Incident Severity Evaluator (`IncidentSeverityEvaluator`)**:
   - Audits open incident counts, critical incident durations, and SLA escalation thresholds.
5. **Operational Compliance Evaluator (`OperationalComplianceEvaluator`)**:
   - Audits node heartbeat freshness, agent collector semantic version (`semver`), mandatory metadata tags, and approved operating systems/architectures.

### 6.2 Data Acquisition & Freshness Classification
- **`DataFreshnessFresh`**: Telemetry and heartbeat timestamps are within acceptable tolerance windows (e.g. $\le 5\text{m}$).
- **`DataFreshnessStale`**: Telemetry exists but exceeds freshness thresholds.
- **`DataFreshnessMissing`**: Required telemetry metrics or diagnostic reports are absent.
- **`DataFreshnessInvalid`**: Metric values are malformed or non-numeric.
- **`DataFreshnessUnsupported`**: Check type is not supported on the target node.

### 6.3 Compliance Finding Lifecycle & Deduplication
Every compliance finding has a deterministic compound identity:
$$\text{FindingKey} = (\text{OrgID}, \text{TargetNodeID}, \text{PolicyID}, \text{RuleID})$$

#### State Machine Transitions:
- **`open`**: Triggered on first evaluation failure (`NonCompliant` / `Warning`). Initialized with `OccurrenceCount = 1`, `FirstSeenAt = Now`.
- **`recurring`**: Evaluation continues to fail on subsequent passes. `OccurrenceCount` increments, and `LastSeenAt` updates.
- **`resolved`**: Evaluation passes (`Compliant`). Status updates to `resolved`, and `ResolvedAt` is recorded.
- **Reopen (`open`)**: If a previously `resolved` finding fails evaluation again, it transitions back to `open`, `ResolvedAt` is cleared, and `OccurrenceCount` resumes incrementing.
- **`indeterminate`**: Evaluation returns `InsufficientData` or `Error`.

### 6.4 Compliance Rollups & Metrics
Hierarchical compliance scores are aggregated at Node, Fleet Group, and Organization scopes:
$$\text{ComplianceRatio} = \frac{\text{Compliant Rules}}{\text{Compliant Rules} + \text{NonCompliant Rules} + \text{Warning Rules}}$$
$$\text{CoverageRatio} = \frac{\text{Evaluated Nodes}}{\text{Total Target Nodes}}$$

---

## 7. Isolated Policy Simulation Engine (What-If Analysis)

Watchdog allows operators to test policy additions, modifications, and assignments before applying them to production fleets:

### 7.1 Key Simulation Guarantees
- **Zero Side-Effects**: Operates against an in-memory overlay store without mutating underlying SQLite storage.
- **Zero Alert Firing**: Suppression and alerting pipelines are completely bypassed during simulation runs.
- **Full Hierarchy Resolution**: In-memory policy overlay cleanly replaces target policies during resolution.

### 7.2 Simulation Diff Output
Returns comprehensive before-and-after comparison metrics:
- **`NewFindings`**: Findings that would be introduced by the proposed policy changes.
- **`ResolvedFindings`**: Existing findings that would be resolved by the changes.
- **`UnchangedFindings`**: Findings that remain unaffected.
- **`BaselineComplianceRatio` vs `ProposedComplianceRatio`**: Tenant-wide and group-level compliance impact delta.
- **`NodeImpactSummaries`**: Per-node compliance score changes and state transitions.

---

## 8. Operational Governance & Maintenance Windows

Operational governance controls fleet operations, scheduled downtimes, alert routing, ownership attribution, and auditability.

```text
┌──────────────────────────────────────────────────────────────────────────────────┐
│                             Maintenance Window Scheduler                         │
│  - One-Time & Recurring (Daily, Weekly Bitmask, Monthly Clamped 28-31)          │
│  - IANA Timezone Normalization (e.g. America/New_York ──► UTC)                   │
│  - Scopes: Organization, Fleet Group, Node, Policy                               │
└──────────────────────────────────────┬───────────────────────────────────────────┘
                                       │
            ┌──────────────────────────┴──────────────────────────┐
            ▼                                                     ▼
┌───────────────────────────────────────┐   ┌────────────────────────────────────────┐
│        Alert Suppression Engine       │   │      Hierarchical Ownership Engine     │
│  - Policy & Maintenance Matching      │   │  Cascading Lookup:                     │
│  - Security & Integrity Guardrails:   │   │    1. Node Metadata                    │
│    • CVE Exploits & Vulnerabilities   │   │    2. Fleet Group Subtree (Nearest)    │
│    • Tampering & Privilege Escalation │   │    3. Service Default Metadata         │
│    • Unauthorized Access              │   │    4. Organization Default             │
│    • Data Corruption & Critical Bypass│   └────────────────────────────────────────┘
└───────────────────────────────────────┘
```

### 8.1 Maintenance Window Scheduling & Recurrence Engine
Maintenance windows temporarily place infrastructure into maintenance mode to prevent alert fatigue:

- **Target Scopes**: `organization`, `fleet_group`, `node`, `policy`.
- **Lifecycles**: `scheduled` $\longrightarrow$ `active` $\longrightarrow$ `completed` / `cancelled`.
- **Recurrence Frequencies**:
  - **`none` (One-Time)**: Starts and ends at explicit timestamps.
  - **`daily`**: Repeats every $N$ days with a fixed duration window.
  - **`weekly`**: Repeats on selected days of the week (e.g., Saturday and Sunday) with interval strides.
  - **`monthly`**: Repeats on a specified day of the month (e.g., 31st), automatically clamping to month lengths (e.g. Feb 28/29, Apr 30).
- **Timezone Normalization**: All schedules specify an IANA timezone (e.g., `America/Chicago`, `Europe/London`), and the scheduler normalizes recurrence calculations into UTC.

### 8.2 Alert Suppression Engine with Mandatory Security Guardrails
When alerts or findings are generated, the suppression engine evaluates active maintenance windows:

#### Dynamic Suppression Criteria:
- Alert target matches window scope (`organization`, `fleet_group`, `node`).
- Alert severity is at or below the window's `SeverityThreshold`.
- Alert category is included in the window's `CategoryRestrictions`.

#### Mandatory Security & Integrity Guardrails:
Under no circumstances are critical security, integrity, or authentication events suppressed, even during an active maintenance window:
- **Exempt Rule Patterns**: Rule IDs and names containing `cve-`, `tamper`, `privilege_escalation`, `unauthorized`, `vulnerability`, `data_corruption`, `auth_failure`, or `exploit`.
- **Critical Alert Passthrough**: When `AllowCriticalAlerts = true`, all `SeverityCritical` events bypass suppression.
- **Evidence Immutability**: Suppression decisions never delete alert records or modify telemetry history.

---

## 9. Hierarchical Ownership & Incident Escalation

### 9.1 Cascading Ownership Resolution
Node operational ownership and contact routing are resolved hierarchically with fallback:

$$\text{Node Metadata} \longrightarrow \text{Fleet Group Ancestors (Bottom-Up)} \longrightarrow \text{Service Default} \longrightarrow \text{Organization Default}$$

#### Supported Attributes:
- `owner_team`, `contact_email`, `contact_channel`
- `environment`, `region`, `data_classification`, `cost_center`
- `business_criticality`, `lifecycle`
- Custom key-value properties (`governance.custom.*`)

Supports both canonical `governance.*` prefixed keys and unprefixed fallback keys.

### 9.2 Multi-Stage Incident Escalation Engine
Escalation policies define multi-tier notification workflows for incidents based on severity and duration:

```text
Incident Created (T=0)
  │
  ├─► Stage 1 (Delay: 0m)  ──► Slack: #platform-alerts (Team On-Call)
  │
  ├─► Stage 2 (Delay: 10m) ──► PagerDuty: Platform Secondary Lead
  │
  └─► Stage 3 (Delay: 30m) ──► Email: VP Engineering & SRE Directors
```

- **Stage Resolution**: Evaluates incident duration against cumulative stage delay minutes.
- **Notification Channels**: `slack`, `pagerduty`, `email`, `webhook`.
- **Time-to-Next-Stage**: Computes remaining time until the next escalation stage is triggered.

---

## 10. Unified Governance Audit Trail & Redaction

Watchdog maintains an append-only audit log for all governance actions, policy updates, maintenance operations, and security evaluations.

### 10.1 Automated Credential & Secret Redaction
To prevent sensitive tokens and keys from leaking into audit storage, all audit messages, resource identifiers, and metadata maps are recursively sanitized:

- **Redacted Metadata Keys**: `api_key`, `token`, `secret`, `password`, `auth_header`, `bearer`, `private_key`, `access_token`, `client_secret`, `session_token`.
- **Regex Pattern Redaction**: Automatically matches and redacts:
  - Bearer tokens and JWTs (`Bearer eyJ...` $\longrightarrow$ `Bearer [REDACTED]`)
  - Inline passwords and credentials (`password=...` $\longrightarrow$ `password=[REDACTED]`)
  - API key patterns (`api_key=...`, `secret_token=...` $\longrightarrow$ `[REDACTED]`)
- **Replacement Sentinel**: All redacted values are replaced with `[REDACTED]`.

---

## 11. Storage & Persistence Guarantees

All governance records are persisted in SQLite with WAL mode enabled:
- **Tables**: `organizations`, `fleet_groups`, `fleet_group_members`, `node_ownership_metadata`, `policies`, `policy_revisions`, `policy_assignments`, `policy_resolution_audits`, `policy_evaluations`, `compliance_findings`, `maintenance_windows`, `escalation_policies`, `suppression_decisions`, `audit_events`.
- **Foreign Keys & Cascades**: Foreign keys are enabled with cascading deletion for clean lifecycle teardown.
- **Tenant Isolation**: Queries strictly filter by `org_id` to prevent cross-tenant information leakage.

---

## 12. Programmatic Usage (Go SDK)

### 12.1 Evaluating Node Compliance
```go
service := governance.NewGovernanceService(store)

// Evaluate node compliance
exec, err := service.EvaluateNode(ctx, "org-acme", "srv-node-01")
if err != nil {
    log.Fatalf("Evaluation failed: %v", err)
}

fmt.Printf("Evaluation ID: %s, Compliant: %d, Non-Compliant: %d\n",
    exec.ID, exec.Summary.CompliantCount, exec.Summary.NonCompliantCount)
```

### 12.2 Simulating Policy Changes
```go
simReq := governance.SimulationRequest{
    OrgID: "org-acme",
    ProposedRevisions: []*model.PolicyRevision{
        proposedRev,
    },
}

simResult, err := service.SimulatePolicyChanges(ctx, simReq)
if err != nil {
    log.Fatalf("Simulation failed: %v", err)
}

fmt.Printf("Baseline Compliance: %.2f%%, Proposed: %.2f%%\n",
    simResult.BaselineComplianceRatio*100, simResult.ProposedComplianceRatio*100)
fmt.Printf("New Findings: %d, Resolved Findings: %d\n",
    len(simResult.NewFindings), len(simResult.ResolvedFindings))
```

### 12.3 Maintenance Windows & Alert Suppression
```go
// Create recurring maintenance window
win := &model.MaintenanceWindow{
    ID:          "win-weekend-patch",
    OrgID:       "org-acme",
    Name:        "Weekend Infrastructure Maintenance",
    Status:      model.MaintenanceStatusActive,
    TargetScope: model.MaintenanceTargetScopeFleetGroup,
    TargetID:    "grp-databases",
    Schedule: model.MaintenanceSchedule{
        StartTime: time.Now().UTC(),
        EndTime:   time.Now().UTC().Add(4 * time.Hour),
        TimeZone:  "America/New_York",
        Recurrence: &model.RecurrenceSchedule{
            Frequency:  model.RecurrenceFrequencyWeekly,
            Interval:   1,
            DaysOfWeek: []time.Weekday{time.Saturday},
            Duration:   4 * time.Hour,
        },
    },
    SuppressAlerts:      true,
    SuppressFindings:    true,
    AllowCriticalAlerts: true,
}
_ = service.CreateMaintenanceWindow(ctx, win)

// Evaluate alert suppression
decision, err := service.EvaluateSuppression(ctx, governance.SuppressionEvaluationRequest{
    OrgID:    "org-acme",
    AlertID:  "alt-001",
    NodeID:   "srv-db-01",
    GroupIDs: []string{"grp-databases"},
    RuleID:   "high-cpu",
    Severity: model.SeverityWarning,
})
fmt.Printf("Suppression Outcome: %s (Reason: %s)\n", decision.Outcome, decision.Reason)
```

### 12.4 Resolving Node Ownership & Incident Escalation
```go
// Resolve cascading ownership
ownership, err := service.ResolveNodeOwnership(ctx, "srv-db-01")
if err != nil {
    log.Fatalf("Ownership resolution failed: %v", err)
}
fmt.Printf("Node Owner: %s (Channel: %s, Env: %s)\n",
    ownership.OwnerTeam, ownership.ContactChannel, ownership.Environment)

// Evaluate incident escalation stage
escalation, err := service.EvaluateIncidentEscalation(ctx, "org-acme", incident, time.Now().UTC())
if err != nil {
    log.Fatalf("Escalation evaluation failed: %v", err)
}
fmt.Printf("Incident Stage: %d (Channel: %s, Next in: %v)\n",
    escalation.CurrentStage, escalation.Channel, escalation.TimeUntilNextStage)
```
