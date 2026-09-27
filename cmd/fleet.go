package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/fleet"
	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/DocHoax/watchdog/pkg/util"
	"github.com/spf13/cobra"
)

var (
	fleetServerURL   string
	fleetToken       string
	fleetTokenFile   string
	fleetTokenEnv    string
	fleetInsecureTLS bool
	fleetJSON        bool
	fleetTimeout     time.Duration

	// List filters
	fleetFilterStatus        string
	fleetFilterSearch        string
	fleetFilterLimit         int
	fleetFilterOffset        int
	fleetFilterSortBy        string
	fleetFilterSortDirection string
	fleetFilterSince         string

	// Register flags
	regNodeID   string
	regHostname string
	regTags     []string
	regMetadata []string

	// Heartbeat flags
	hbNodeID string
	hbStatus string

	// Deregister flags
	deregForce bool
)

var fleetCmd = &cobra.Command{
	Use:     "fleet [command]",
	Aliases: []string{"fleets"},
	Short:   "Manage, inspect, and query centralized fleet nodes and telemetry",
	Long: `Provides administrative CLI capabilities for interacting with the centralized
Watchdog fleet management server: listing nodes, inspecting node telemetry, registering
nodes, sending heartbeats, and viewing aggregated fleet summaries.`,
	Example: `  # Show fleet health summary
  watchdog fleet status --server https://fleet.internal:8443

  # List all healthy nodes in the fleet
  watchdog fleet list --status healthy --server https://fleet.internal:8443

  # Inspect full details and recent telemetry for a node
  watchdog fleet get node-prod-01 --server https://fleet.internal:8443

  # Register this machine with the central fleet controller
  watchdog fleet register --server https://fleet.internal:8443 --token-file /etc/watchdog/token

  # Deregister / remove a decommissioned node
  watchdog fleet deregister node-old-01 --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runFleetStatus(cmd, args)
	},
}

var fleetStatusCmd = &cobra.Command{
	Use:     "status [flags]",
	Aliases: []string{"summary"},
	Short:   "Display aggregated fleet health overview and metric statistics",
	Long:    `Queries the fleet management server for aggregated cluster overview metrics.`,
	Example: `  # Display summary of all managed fleet nodes
  watchdog fleet status

  # Output summary as structured JSON
  watchdog fleet status --json`,
	RunE: runFleetStatus,
}

var fleetListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List registered fleet nodes with status and telemetry filters",
	Long:    `Lists all nodes registered with the fleet server, supporting filtering by status, search term, and sorting.`,
	Example: `  # List all nodes
  watchdog fleet list

  # List only warning or critical nodes
  watchdog fleet list --status warning

  # Search nodes by hostname or IP address
  watchdog fleet list --search worker-01

  # Output list in JSON format
  watchdog fleet list --json`,
	RunE: runFleetList,
}

var fleetGetCmd = &cobra.Command{
	Use:     "get <node_id> [flags]",
	Aliases: []string{"inspect", "show"},
	Short:   "Inspect detailed telemetry, diagnostics, and status for a single node",
	Long:    `Retrieves comprehensive point-in-time state, hardware specs, diagnostics, and recent telemetry history for a specific node.`,
	Args:    cobra.ExactArgs(1),
	Example: `  # Inspect details for node-prod-01
  watchdog fleet get node-prod-01

  # Output detailed node state in JSON format
  watchdog fleet get node-prod-01 --json`,
	RunE: runFleetGet,
}

var fleetRegisterCmd = &cobra.Command{
	Use:   "register [flags]",
	Short: "Register this node (or specified identity) with the central fleet controller",
	Long:  `Sends a registration payload to the fleet server to enroll a node in centralized monitoring.`,
	Example: `  # Register current node with auto-discovered hardware specs
  watchdog fleet register

  # Register with custom tags and metadata
  watchdog fleet register --tags env=prod,role=web --metadata dc=us-east-1`,
	RunE: runFleetRegister,
}

var fleetHeartbeatCmd = &cobra.Command{
	Use:     "heartbeat [flags]",
	Aliases: []string{"ping"},
	Short:   "Send an ad-hoc heartbeat liveness signal to the fleet server",
	Long:    `Submits a point-in-time heartbeat to attest node liveness and update health status.`,
	Example: `  # Send a healthy heartbeat
  watchdog fleet heartbeat

  # Send heartbeat with custom status
  watchdog fleet heartbeat --status warning`,
	RunE: runFleetHeartbeat,
}

var fleetDeregisterCmd = &cobra.Command{
	Use:     "deregister <node_id> [flags]",
	Aliases: []string{"delete", "rm"},
	Short:   "Deregister and remove a node from the fleet registry",
	Long:    `Removes a node and its telemetry records from the central fleet management server.`,
	Args:    cobra.ExactArgs(1),
	Example: `  # Deregister a node with confirmation
  watchdog fleet deregister node-old-01

  # Force deregistration without confirmation prompt
  watchdog fleet deregister node-old-01 --force`,
	RunE: runFleetDeregister,
}

func init() {
	// Persistent flags on fleetCmd
	fleetCmd.PersistentFlags().StringVarP(&fleetServerURL, "server", "s", "", "fleet management server URL (e.g. https://fleet.internal:8443)")
	fleetCmd.PersistentFlags().StringVarP(&fleetToken, "token", "t", "", "bearer authentication token")
	fleetCmd.PersistentFlags().StringVar(&fleetTokenFile, "token-file", "", "path to file containing bearer authentication token")
	fleetCmd.PersistentFlags().StringVar(&fleetTokenEnv, "token-env", "", "environment variable name containing bearer authentication token")
	fleetCmd.PersistentFlags().BoolVarP(&fleetInsecureTLS, "insecure", "k", false, "skip TLS certificate validation (insecure)")
	fleetCmd.PersistentFlags().BoolVar(&fleetJSON, "json", false, "output response in structured JSON format")
	fleetCmd.PersistentFlags().DurationVar(&fleetTimeout, "timeout", 10*time.Second, "HTTP request timeout")

	// List flags
	fleetListCmd.Flags().StringVar(&fleetFilterStatus, "status", "", "filter nodes by status (healthy, warning, critical, stale, offline)")
	fleetListCmd.Flags().StringVar(&fleetFilterSearch, "search", "", "search nodes by node ID, hostname, or IP address")
	fleetListCmd.Flags().IntVar(&fleetFilterLimit, "limit", 50, "maximum number of nodes to return")
	fleetListCmd.Flags().IntVar(&fleetFilterOffset, "offset", 0, "offset for pagination")
	fleetListCmd.Flags().StringVar(&fleetFilterSortBy, "sort-by", "", "sort column (hostname, last_heartbeat, cpu, memory, status)")
	fleetListCmd.Flags().StringVar(&fleetFilterSortDirection, "sort-direction", "", "sort direction (asc, desc)")
	fleetListCmd.Flags().StringVar(&fleetFilterSince, "since", "", "filter nodes active since duration (e.g. 5m, 1h, 24h)")

	// Register flags
	fleetRegisterCmd.Flags().StringVar(&regNodeID, "node-id", "", "node ID (defaults to persistent local node ID)")
	fleetRegisterCmd.Flags().StringVar(&regHostname, "hostname", "", "hostname (defaults to local hostname)")
	fleetRegisterCmd.Flags().StringSliceVar(&regTags, "tags", nil, "operational tags as key=value pairs (e.g. env=prod,dc=us-east)")
	fleetRegisterCmd.Flags().StringSliceVar(&regMetadata, "metadata", nil, "custom metadata as key=value pairs")

	// Heartbeat flags
	fleetHeartbeatCmd.Flags().StringVar(&hbNodeID, "node-id", "", "node ID (defaults to persistent local node ID)")
	fleetHeartbeatCmd.Flags().StringVar(&hbStatus, "status", "healthy", "node health status (healthy, warning, critical)")

	// Deregister flags
	fleetDeregisterCmd.Flags().BoolVarP(&deregForce, "force", "f", false, "force deregistration without interactive confirmation")

	// Subcommands
	fleetCmd.AddCommand(fleetStatusCmd)
	fleetCmd.AddCommand(fleetListCmd)
	fleetCmd.AddCommand(fleetGetCmd)
	fleetCmd.AddCommand(fleetRegisterCmd)
	fleetCmd.AddCommand(fleetHeartbeatCmd)
	fleetCmd.AddCommand(fleetDeregisterCmd)

	RootCmd.AddCommand(fleetCmd)
}

func getFleetClient() (*fleet.FleetClient, error) {
	serverURL := fleetServerURL
	if serverURL == "" && globalCfg != nil {
		serverURL = globalCfg.Fleet.ServerURL
	}
	if serverURL == "" {
		return nil, NewExitError(ExitConfigError, "fleet server URL is required (specify via --server flag or fleet.server_url in config)")
	}

	token := fleetToken
	tokenFile := fleetTokenFile
	tokenEnv := fleetTokenEnv
	if token == "" && tokenFile == "" && tokenEnv == "" && globalCfg != nil {
		token = globalCfg.Fleet.Token
		tokenFile = globalCfg.Fleet.TokenFile
		tokenEnv = globalCfg.Fleet.TokenEnv
	}

	fleetCfg := config.FleetConfig{
		Token:     token,
		TokenFile: tokenFile,
		TokenEnv:  tokenEnv,
	}
	resolvedToken, err := fleetCfg.ResolveToken()
	if err != nil {
		return nil, NewExitError(ExitConfigError, "failed to resolve fleet token: %w", err)
	}

	nodeID := ""
	if globalCfg != nil {
		nodeID = globalCfg.Fleet.NodeID
		if nodeID == "" && globalCfg.Fleet.NodeIDFile != "" {
			nodeID, _ = fleet.GetOrGenerateNodeID(globalCfg.Fleet.NodeIDFile)
		}
	}

	var tlsConfig *tls.Config
	if fleetInsecureTLS {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}

	timeout := fleetTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	client := fleet.NewFleetClient(fleet.ClientConfig{
		Endpoint:  serverURL,
		Token:     resolvedToken,
		NodeID:    nodeID,
		Version:   "1.0.0",
		Timeout:   timeout,
		TLSConfig: tlsConfig,
	})
	return client, nil
}

func runFleetStatus(cmd *cobra.Command, args []string) error {
	client, err := getFleetClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), fleetTimeout)
	defer cancel()

	summary, err := client.GetSummary(ctx)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get fleet summary: %w", err)
	}

	if fleetJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	fmt.Println("🐺 Watchdog Fleet Summary")
	fmt.Println(strings.Repeat("=", 45))
	fmt.Printf("  Total Nodes:       %d\n", summary.TotalNodes)
	fmt.Printf("  Healthy Nodes:     %d\n", summary.HealthyNodes)
	fmt.Printf("  Warning Nodes:     %d\n", summary.WarningNodes)
	fmt.Printf("  Critical Nodes:    %d\n", summary.CriticalNodes)
	fmt.Printf("  Stale Nodes:       %d\n", summary.StaleNodes)
	fmt.Printf("  Offline Nodes:     %d\n", summary.OfflineNodes)
	fmt.Printf("  Unknown Nodes:     %d\n", summary.UnknownNodes)
	fmt.Println(strings.Repeat("-", 45))
	fmt.Printf("  Average CPU:       %.1f%%\n", summary.AvgCPUPercent)
	fmt.Printf("  Average Memory:    %.1f%%\n", summary.AvgMemoryPct)
	fmt.Printf("  Active Alerts:     %d\n", summary.TotalAlerts)
	fmt.Printf("  Last Updated:      %s\n", summary.LastUpdated.Local().Format(time.RFC3339))
	return nil
}

func runFleetList(cmd *cobra.Command, args []string) error {
	client, err := getFleetClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), fleetTimeout)
	defer cancel()

	filter := model.FleetFilter{
		Status:        model.NodeStatus(fleetFilterStatus),
		Search:        fleetFilterSearch,
		Limit:         fleetFilterLimit,
		Offset:        fleetFilterOffset,
		SortBy:        fleetFilterSortBy,
		SortDirection: fleetFilterSortDirection,
	}

	if fleetFilterSince != "" {
		if d, err := time.ParseDuration(fleetFilterSince); err == nil {
			filter.Since = time.Now().Add(-d)
		} else if t, err := time.Parse(time.RFC3339, fleetFilterSince); err == nil {
			filter.Since = t
		} else {
			return NewExitError(ExitUsageError, "invalid --since value %q: must be duration (e.g. 10m, 2h) or RFC3339 timestamp", fleetFilterSince)
		}
	}

	resp, err := client.ListNodes(ctx, filter)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to list fleet nodes: %w", err)
	}

	if fleetJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	if len(resp.Nodes) == 0 {
		fmt.Println("No fleet nodes found matching the specified criteria.")
		return nil
	}

	tw := util.NewTableWriter("Node ID", "Hostname", "Status", "CPU %", "Memory %", "Disk %", "Alerts", "Last Heartbeat", "Tags")
	for _, n := range resp.Nodes {
		cpuStr := "-"
		memStr := "-"
		diskStr := "-"
		alertsStr := "0"
		if n.Summary != nil {
			cpuStr = fmt.Sprintf("%.1f%%", n.Summary.CPUUsagePercent)
			memStr = fmt.Sprintf("%.1f%%", n.Summary.MemoryUsagePercent)
			diskStr = fmt.Sprintf("%.1f%%", n.Summary.DiskUsagePercent)
			alertsStr = strconv.Itoa(n.Summary.ActiveAlertsCount)
		}

		hbStr := "-"
		if !n.LastHeartbeat.IsZero() {
			hbStr = time.Since(n.LastHeartbeat).Truncate(time.Second).String() + " ago"
		}

		tagsStr := "-"
		if len(n.Identity.Tags) > 0 {
			var tagPairs []string
			for k, v := range n.Identity.Tags {
				tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", k, v))
			}
			tagsStr = strings.Join(tagPairs, ",")
		}

		statusDisplay := string(n.Status)
		switch n.Status {
		case model.NodeStatusHealthy:
			statusDisplay = "HEALTHY"
		case model.NodeStatusWarning:
			statusDisplay = "WARNING"
		case model.NodeStatusCritical:
			statusDisplay = "CRITICAL"
		case model.NodeStatusStale:
			statusDisplay = "STALE"
		case model.NodeStatusOffline:
			statusDisplay = "OFFLINE"
		}

		tw.Append(
			util.TruncateString(n.Identity.NodeID, 20),
			util.TruncateString(n.Identity.Hostname, 20),
			statusDisplay,
			cpuStr,
			memStr,
			diskStr,
			alertsStr,
			hbStr,
			util.TruncateString(tagsStr, 25),
		)
	}

	tw.Render(os.Stdout)
	fmt.Printf("\nTotal Nodes: %d (Showing %d)\n", resp.Total, len(resp.Nodes))
	return nil
}

func runFleetGet(cmd *cobra.Command, args []string) error {
	nodeID := args[0]
	client, err := getFleetClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), fleetTimeout)
	defer cancel()

	detail, err := client.GetNode(ctx, nodeID)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get node %q: %w", nodeID, err)
	}

	if fleetJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(detail)
	}

	n := detail.Node
	fmt.Println("🐺 Watchdog Fleet Node Detail")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("  Node ID:           %s\n", n.Identity.NodeID)
	fmt.Printf("  Hostname:          %s\n", n.Identity.Hostname)
	fmt.Printf("  Status:            %s\n", strings.ToUpper(string(n.Status)))
	if n.StatusMessage != "" {
		fmt.Printf("  Status Message:    %s\n", n.StatusMessage)
	}
	fmt.Printf("  OS / Platform:     %s / %s %s\n", n.Identity.OS, n.Identity.Platform, n.Identity.PlatformVer)
	fmt.Printf("  Architecture:      %s\n", n.Identity.Arch)
	fmt.Printf("  CPU Cores:         %d\n", n.Identity.CPUCores)
	fmt.Printf("  Total Memory:      %s\n", util.FormatBytes(n.Identity.TotalMemory))
	fmt.Printf("  Agent Version:     %s\n", n.Identity.Version)
	fmt.Printf("  Registered At:     %s\n", n.RegisteredAt.Local().Format(time.RFC3339))
	if !n.LastHeartbeat.IsZero() {
		fmt.Printf("  Last Heartbeat:    %s (%s ago)\n", n.LastHeartbeat.Local().Format(time.RFC3339), time.Since(n.LastHeartbeat).Truncate(time.Second))
	}
	if n.LastTelemetry != nil && !n.LastTelemetry.IsZero() {
		fmt.Printf("  Last Telemetry:    %s (%s ago)\n", n.LastTelemetry.Local().Format(time.RFC3339), time.Since(*n.LastTelemetry).Truncate(time.Second))
	}

	if len(n.Identity.Tags) > 0 {
		var tagPairs []string
		for k, v := range n.Identity.Tags {
			tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", k, v))
		}
		fmt.Printf("  Tags:              %s\n", strings.Join(tagPairs, ", "))
	}
	if len(n.Metadata) > 0 {
		var metaPairs []string
		for k, v := range n.Metadata {
			metaPairs = append(metaPairs, fmt.Sprintf("%s=%s", k, v))
		}
		fmt.Printf("  Metadata:          %s\n", strings.Join(metaPairs, ", "))
	}

	if n.Summary != nil {
		fmt.Println("\n📊 Resource Usage Summary")
		fmt.Println(strings.Repeat("-", 50))
		fmt.Printf("  CPU Usage:         %.1f%%\n", n.Summary.CPUUsagePercent)
		fmt.Printf("  Memory Usage:      %.1f%%\n", n.Summary.MemoryUsagePercent)
		fmt.Printf("  Disk Usage:        %.1f%%\n", n.Summary.DiskUsagePercent)
		if n.Summary.Load1 > 0 {
			fmt.Printf("  Load Average (1m): %.2f\n", n.Summary.Load1)
		}
		fmt.Printf("  Active Alerts:     %d\n", n.Summary.ActiveAlertsCount)
		if n.Summary.DiagnosticStatus != "" {
			fmt.Printf("  Diagnostic Status: %s\n", n.Summary.DiagnosticStatus)
		}
	}

	if len(detail.ActiveAlerts) > 0 {
		fmt.Printf("\n🚨 Active Alerts (%d):\n", len(detail.ActiveAlerts))
		for _, a := range detail.ActiveAlerts {
			fmt.Printf("  - [%s] %s: %s (Triggered: %s)\n", strings.ToUpper(string(a.Severity)), a.RuleName, a.Message, a.FiredAt.Format(time.RFC3339))
		}
	}

	return nil
}

func runFleetRegister(cmd *cobra.Command, args []string) error {
	client, err := getFleetClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), fleetTimeout)
	defer cancel()

	nodeID := regNodeID
	if nodeID == "" && globalCfg != nil {
		nodeID = globalCfg.Fleet.NodeID
		if nodeID == "" && globalCfg.Fleet.NodeIDFile != "" {
			nodeID, _ = fleet.GetOrGenerateNodeID(globalCfg.Fleet.NodeIDFile)
		}
	}

	tags := make(map[string]string)
	if globalCfg != nil {
		for k, v := range globalCfg.Fleet.Tags {
			tags[k] = v
		}
	}
	for _, t := range regTags {
		parts := strings.SplitN(t, "=", 2)
		if len(parts) == 2 {
			tags[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	metadata := make(map[string]string)
	for _, m := range regMetadata {
		parts := strings.SplitN(m, "=", 2)
		if len(parts) == 2 {
			metadata[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	identity, err := fleet.DiscoverNodeIdentity(ctx, nodeID, "1.0.0", tags)
	if err != nil {
		return NewExitError(ExitGeneralError, "failed to discover local node identity: %w", err)
	}

	if regHostname != "" {
		identity.Hostname = regHostname
	}

	req := &model.NodeRegistrationRequest{
		Identity: *identity,
		Metadata: metadata,
	}

	resp, err := client.Register(ctx, req)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to register node: %w", err)
	}

	if fleetJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	fmt.Println("✅ Successfully registered node with fleet controller!")
	fmt.Printf("  Node ID:                    %s\n", resp.NodeID)
	fmt.Printf("  Registered At:              %s\n", resp.RegisteredAt.Local().Format(time.RFC3339))
	fmt.Printf("  Heartbeat Interval:         %d seconds\n", resp.HeartbeatIntervalSeconds)
	fmt.Printf("  Telemetry Interval:         %d seconds\n", resp.TelemetryIntervalSeconds)
	if resp.Message != "" {
		fmt.Printf("  Server Message:             %s\n", resp.Message)
	}
	return nil
}

func runFleetHeartbeat(cmd *cobra.Command, args []string) error {
	client, err := getFleetClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), fleetTimeout)
	defer cancel()

	nodeID := hbNodeID
	if nodeID == "" && globalCfg != nil {
		nodeID = globalCfg.Fleet.NodeID
		if nodeID == "" && globalCfg.Fleet.NodeIDFile != "" {
			nodeID, _ = fleet.GetOrGenerateNodeID(globalCfg.Fleet.NodeIDFile)
		}
	}
	if nodeID == "" {
		return NewExitError(ExitUsageError, "node ID is required for heartbeat (specify via --node-id or fleet config)")
	}

	req := &model.HeartbeatRequest{
		NodeID:    nodeID,
		Timestamp: time.Now().UTC(),
		Status:    model.NodeStatus(hbStatus),
	}

	resp, err := client.SendHeartbeat(ctx, req)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to send heartbeat: %w", err)
	}

	if fleetJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	fmt.Println("💓 Heartbeat acknowledged by fleet server")
	fmt.Printf("  Node Status:                %s\n", strings.ToUpper(string(resp.NodeStatus)))
	fmt.Printf("  Next Heartbeat Interval:    %d seconds\n", resp.NextHeartbeatInterval)
	fmt.Printf("  Server Timestamp:           %s\n", resp.Timestamp.Local().Format(time.RFC3339))
	return nil
}

func runFleetDeregister(cmd *cobra.Command, args []string) error {
	nodeID := args[0]
	client, err := getFleetClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), fleetTimeout)
	defer cancel()

	if err := client.DeleteNode(ctx, nodeID); err != nil {
		return NewExitError(ExitNetworkError, "failed to deregister node %q: %w", nodeID, err)
	}

	if fleetJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"deleted":   true,
			"node_id":   nodeID,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}

	fmt.Printf("🗑️  Successfully deregistered node %q from fleet registry.\n", nodeID)
	return nil
}
