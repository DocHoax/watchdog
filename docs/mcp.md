# 🤖 Watchdog Model Context Protocol (MCP) Server & AI Integration

> **Standardized, Read-Only Model Context Protocol (MCP) Interface for AI Assistants, Claude Desktop, IDE Extensions, and Autonomous Diagnostic Agents**

---

## 🌟 Overview

The **Watchdog Model Context Protocol (MCP) Server** provides a native, standardized bridge between Watchdog's systems observability engine and modern AI assistants (such as Anthropic Claude Desktop, Claude Code, Cursor, Windsurf, JetBrains AI, and custom autonomous agents).

Operating strictly as a **read-only observability interface**, Watchdog MCP enables AI models to safely inspect real-time system metrics, fleet topology, diagnostic findings, active alerts, and statistical anomalies using the open [Model Context Protocol](https://modelcontextprotocol.io/) specification (`ProtocolVersion = "2024-11-05"`).

```text
┌─────────────────────────────────────────────────────────────┐
│                       AI Assistant                          │
│     (Claude Desktop / Cursor / Autonomous Agent / CLI)     │
└──────────────┬───────────────────────────────┬──────────────┘
               │                               │
    (stdio / JSON-RPC 2.0)            (HTTP / SSE Transport)
               │                               │
               ▼                               ▼
┌─────────────────────────────────────────────────────────────┐
│               Watchdog MCP Server (Daemon)                  │
│  ├─ JSON-RPC 2.0 Message Dispatcher                         │
│  ├─ Token-Bucket Rate Limiter & Pre-Flight Security Guard   │
│  └─ Security Audit Logging Engine                           │
└──────────────┬───────────────┬───────────────┬──────────────┘
               │               │               │
               ▼               ▼               ▼
      ┌────────────────┐┌──────────────┐┌──────────────┐
      │  8 Core Tools  ││ 5 Resources  ││  4 Prompts   │
      │ (list_nodes,   ││ (watchdog:// ││ (system_     │
      │  get_node,     ││  fleet,      ││  health_     │
      │  metrics, ...) ││  snapshot,..)││  audit, ...) │
      └────────────────┘└──────────────┘└──────────────┘
```

---

## 🔒 Security Principles & Invariants

1. **Strict Read-Only Guarantee**: The MCP server is fundamentally incapable of modifying system state, terminating processes, modifying firewall rules, changing configuration files, or executing arbitrary commands. Structural interface segregation (`ReadOnlyFleetService` and `ReadOnlyStorage`) ensures mutation methods cannot be compiled or called.
2. **Static Tool Allowlist**: Enforces a compile-time allowlist (`AllowedReadOperations`) strictly limiting execution to the 8 approved read tools. Unrecognized or mutation requests return JSON-RPC `-32601` (`CodeMethodNotFound`).
3. **Secure-by-Default Networking**: Binding to external or non-loopback network interfaces (`0.0.0.0`, LAN IPs) strictly mandates both Bearer token authentication and valid TLS certificates (`--tls-cert` and `--tls-key`).
4. **Stdio Protocol Isolation**: In `stdio` transport mode, all application logs and operational banners are strictly redirected to `stderr`, keeping `stdout` dedicated to framed JSON-RPC 2.0 messages.
5. **Token-Bucket Rate Limiting**: Embedded per-client token-bucket rate limiter prevents runaway AI tool loops from exhausting host memory or CPU resources.
6. **Comprehensive Audit Logging**: Every tool invocation, resource read, prompt retrieval, and authentication event is audited and recorded with caller identity and timing metrics.
7. **Security Audit Report**: For full threat modeling, interface proofs, fuzzing reports, and compliance matrices, see [`docs/security-audit.md`](security-audit.md).

---

## 🚀 Transport Modes

Watchdog MCP supports dual transport mechanisms:

### 1. Standard I/O Transport (`stdio`)
Ideal for local AI clients (such as Claude Desktop or local CLI tools) spawning Watchdog as a child subprocess. Protocol messages are exchanged line-by-line over standard input and standard output.

```bash
watchdog mcp serve --transport stdio
```

### 2. Network Transport (`http` & `sse`)
Provides authenticated network endpoints for remote agents and web-based AI platforms:
- **HTTP POST Endpoint** (`/mcp`): Standard JSON-RPC 2.0 request/response API.
- **Server-Sent Events Stream** (`/sse` & `/mcp/message`): Full-duplex SSE streaming connection.

```bash
# Localhost loopback HTTP server
watchdog mcp serve --transport http --port 8444

# Production remote server with TLS and Bearer Token authentication
watchdog mcp serve --transport sse --host 0.0.0.0 --port 8444 \
  --token secret-token-12345 \
  --tls-cert /etc/ssl/certs/watchdog.crt \
  --tls-key /etc/ssl/private/watchdog.key
```

---

## 🛠️ MCP Tools (8 Registered)

AI assistants can invoke any of the following 8 registered tools:

| Tool Name | Description | Parameters |
| :--- | :--- | :--- |
| `list_nodes` | Lists registered fleet nodes with health status and tags | `status` (optional), `limit` (optional), `offset` (optional) |
| `get_node` | Retrieves metadata and hardware specs for a node | `node_id` (required) |
| `get_node_health` | Evaluates multi-subsystem health scores and diagnostics | `node_id` (required) |
| `get_node_snapshot` | Returns real-time CPU, RAM, disk, network, and process snapshot | `node_id` (required) |
| `get_fleet_health` | Computes cluster-wide aggregate health summary | *(none)* |
| `get_node_metrics` | Queries historical time-series metric data points | `node_id` (required), `metric` (required), `duration` (optional) |
| `get_recent_diagnostics` | Runs diagnostic rules and returns active issues | `node_id` (optional), `severity` (optional) |
| `get_active_alerts` | Queries currently firing threshold alerts | `severity` (optional), `subsystem` (optional) |

---

## 📦 MCP Resources (5 Registered)

Readable resources exposed under the `watchdog://` URI scheme:

| Resource URI | MIME Type | Description |
| :--- | :--- | :--- |
| `watchdog://fleet` | `application/json` | Cluster topology and registered node list |
| `watchdog://fleet/{node_id}` | `application/json` | Detailed node identity and system specifications |
| `watchdog://fleet/{node_id}/health` | `application/json` | Multi-subsystem health evaluation |
| `watchdog://fleet/{node_id}/snapshot` | `application/json` | Latest point-in-time telemetry snapshot |
| `watchdog://fleet/{node_id}/alerts` | `application/json` | Active alerts for specified node |

---

## 💡 MCP Prompt Templates (4 Registered)

Pre-configured diagnostic workflows and incident triage prompt recipes:

| Prompt Name | Arguments | Workflow Description |
| :--- | :--- | :--- |
| `system_health_audit` | `severity` (optional) | Comprehensive system health and performance analysis |
| `diagnose_node` | `node_id` (required), `focus` (optional) | In-depth troubleshooting workflow for an unhealthy host |
| `incident_triage` | `alert_id` (optional), `subsystem` (optional) | Root-cause analysis and remediation runbook generation |
| `fleet_status_report` | `cluster` (optional) | High-level executive overview of fleet capacity and health |

---

## 💻 Claude Desktop Setup

To integrate Watchdog with Claude Desktop, add the following configuration to your Claude Desktop config file:

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "watchdog": {
      "command": "watchdog",
      "args": ["mcp", "serve", "--transport", "stdio"]
    }
  }
}
```

---

## 🖥️ CLI Commands

```bash
# Display server status, configuration, and registered capability counts
watchdog mcp status

# Output status as JSON
watchdog mcp status --json

# List registered tool definitions and parameter schemas
watchdog mcp tools

# List registered resources and URI templates
watchdog mcp resources

# List registered prompt templates
watchdog mcp prompts

# Start MCP daemon in stdio mode (default)
watchdog mcp serve

# Start authenticated HTTP daemon on port 8444
watchdog mcp serve --transport http --port 8444 --token <auth-token>
```
