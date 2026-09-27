package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/fleet"
	"github.com/DocHoax/watchdog/internal/intelligence"
	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/DocHoax/watchdog/pkg/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	intelServerURL   string
	intelToken       string
	intelTokenFile   string
	intelTokenEnv    string
	intelInsecureTLS bool
	intelTimeout     time.Duration
	intelFormat      string
	intelWindow      string
	intelCategory    string
	intelMinSeverity string
	intelIncidentID  string
)

var intelligenceCmd = &cobra.Command{
	Use:     "intelligence [command]",
	Aliases: []string{"intel", "ai"},
	Short:   "Query explainable health intelligence, correlations, trends, and incidents",
	Long: `Provides administrative CLI capabilities for interacting with the Watchdog Intelligence Layer:
evaluating fleet and node health scores with explainable factor deductions, analyzing historical
metric trends and baselines, inspecting clustered incidents, and viewing cross-node pattern findings.`,
	Example: `  # Show fleet health score and summary overview
  watchdog intelligence fleet --server https://fleet.internal:8443

  # Inspect explainable health score breakdown and trajectory for a node
  watchdog intelligence node node-prod-01 --server https://fleet.internal:8443

  # List active clustered incidents across the fleet
  watchdog intelligence incidents --server https://fleet.internal:8443

  # Inspect metric trends and rate-of-change over a 1-hour window
  watchdog intelligence trends node-prod-01 --window 1h

  # View cross-node findings filtered by category and minimum severity
  watchdog intelligence findings --category fleet_pattern --min-severity warning`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIntelligenceFleet(cmd, args)
	},
}

var intelligenceFleetCmd = &cobra.Command{
	Use:     "fleet [flags]",
	Aliases: []string{"summary", "status"},
	Short:   "Display aggregated fleet health score, status distribution, and top findings",
	Long:    `Queries the fleet management server for aggregated fleet health intelligence.`,
	Example: `  # Display fleet health overview
  watchdog intelligence fleet

  # Output as structured JSON
  watchdog intelligence fleet --format json`,
	RunE: runIntelligenceFleet,
}

var intelligenceNodeCmd = &cobra.Command{
	Use:   "node <node-id> [flags]",
	Short: "Inspect explainable health score, factor deductions, and trajectory for a node",
	Long:  `Evaluates a specific node's health score (0-100) with detailed factor deductions and historical trajectory.`,
	Example: `  # Inspect node health breakdown
  watchdog intelligence node node-01

  # Output as structured YAML
  watchdog intelligence node node-01 --format yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runIntelligenceNode,
}

var intelligenceIncidentsCmd = &cobra.Command{
	Use:     "incidents [incident-id] [flags]",
	Aliases: []string{"incident", "inc"},
	Short:   "List active clustered incidents or inspect details and timeline of a specific incident",
	Long:    `Queries the intelligence service for active multi-signal incidents across the fleet.`,
	Example: `  # List all active incidents
  watchdog intelligence incidents

  # Inspect a single incident with timeline events
  watchdog intelligence incidents inc-node-01-mem-swap`,
	Args: cobra.MaximumNArgs(1),
	RunE: runIntelligenceIncidents,
}

var intelligenceTrendsCmd = &cobra.Command{
	Use:   "trends <node-id> [flags]",
	Short: "Display historical metric trends and rates of change for a node",
	Long:  `Calculates linear regression trends and rates of change for key telemetry metrics over a time window.`,
	Example: `  # View 1-hour metric trends
  watchdog intelligence trends node-01 --window 1h

  # View 24-hour metric trends
  watchdog intelligence trends node-01 --window 24h`,
	Args: cobra.ExactArgs(1),
	RunE: runIntelligenceTrends,
}

var intelligenceBaselinesCmd = &cobra.Command{
	Use:   "baselines <node-id> [flags]",
	Short: "Display historical baseline distributions and percentiles for a node",
	Long:  `Calculates statistical baseline distributions (P50, P90, P95, P99, mean, stddev) over a time window.`,
	Example: `  # View 24-hour baselines for a node
  watchdog intelligence baselines node-01 --window 24h`,
	Args: cobra.ExactArgs(1),
	RunE: runIntelligenceBaselines,
}

var intelligenceCorrelationsCmd = &cobra.Command{
	Use:   "correlations [flags]",
	Short: "Display temporal correlations and associations between metric signals",
	Long:  `Evaluates non-causal temporal correlations between metric streams over a sliding time window.`,
	Example: `  # View correlations over 1 hour
  watchdog intelligence correlations --window 1h`,
	RunE: runIntelligenceCorrelations,
}

var intelligenceFindingsCmd = &cobra.Command{
	Use:   "findings [flags]",
	Short: "Display cross-node and node-specific intelligence findings",
	Long:  `Lists analytical findings including resource exhaustion, fleet patterns, and stability risks with non-invasive suggestions.`,
	Example: `  # View all findings
  watchdog intelligence findings

  # Filter by category and severity
  watchdog intelligence findings --category fleet_pattern --min-severity warning`,
	RunE: runIntelligenceFindings,
}

func init() {
	// Persistent flags on intelligenceCmd
	intelligenceCmd.PersistentFlags().StringVar(&intelServerURL, "server", "", "centralized fleet server URL (e.g. https://fleet.internal:8443)")
	intelligenceCmd.PersistentFlags().StringVar(&intelToken, "token", "", "bearer token for authenticating with the fleet server")
	intelligenceCmd.PersistentFlags().StringVar(&intelTokenFile, "token-file", "", "path to file containing the authentication token")
	intelligenceCmd.PersistentFlags().StringVar(&intelTokenEnv, "token-env", "", "environment variable name containing authentication token")
	intelligenceCmd.PersistentFlags().BoolVar(&intelInsecureTLS, "insecure-tls", false, "skip TLS certificate validation (development only)")
	intelligenceCmd.PersistentFlags().DurationVar(&intelTimeout, "timeout", 10*time.Second, "HTTP request timeout duration")
	intelligenceCmd.PersistentFlags().StringVarP(&intelFormat, "format", "f", "text", "output format (text, json, yaml)")
	intelligenceCmd.PersistentFlags().StringVarP(&intelWindow, "window", "w", "1h", "time window duration (e.g. 15m, 1h, 6h, 24h)")

	// Findings specific flags
	intelligenceFindingsCmd.Flags().StringVar(&intelCategory, "category", "", "filter findings by category (resource_exhaustion, performance_degradation, fleet_pattern, stability_risk, anomaly_cluster)")
	intelligenceFindingsCmd.Flags().StringVar(&intelMinSeverity, "min-severity", "", "minimum severity filter (info, warning, critical)")

	// Incident specific flags
	intelligenceIncidentsCmd.Flags().StringVar(&intelIncidentID, "id", "", "incident ID to inspect")

	intelligenceCmd.AddCommand(intelligenceFleetCmd)
	intelligenceCmd.AddCommand(intelligenceNodeCmd)
	intelligenceCmd.AddCommand(intelligenceIncidentsCmd)
	intelligenceCmd.AddCommand(intelligenceTrendsCmd)
	intelligenceCmd.AddCommand(intelligenceBaselinesCmd)
	intelligenceCmd.AddCommand(intelligenceCorrelationsCmd)
	intelligenceCmd.AddCommand(intelligenceFindingsCmd)

	RootCmd.AddCommand(intelligenceCmd)
}

func getIntelligenceClient() (*intelligence.Client, error) {
	serverURL := intelServerURL
	if serverURL == "" && globalCfg != nil {
		serverURL = globalCfg.Fleet.ServerURL
	}
	if serverURL == "" {
		return nil, NewExitError(ExitConfigError, "fleet server URL is required (specify via --server flag or fleet.server_url in config)")
	}

	token := intelToken
	tokenFile := intelTokenFile
	tokenEnv := intelTokenEnv
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

	var tlsConfig *tls.Config
	if intelInsecureTLS {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}

	timeout := intelTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	client := intelligence.NewClient(intelligence.ClientConfig{
		Endpoint:  serverURL,
		Token:     resolvedToken,
		Timeout:   timeout,
		TLSConfig: tlsConfig,
	})
	return client, nil
}

func outputFormatted(v any) error {
	switch strings.ToLower(intelFormat) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case "yaml", "yml":
		enc := yaml.NewEncoder(os.Stdout)
		defer enc.Close()
		return enc.Encode(v)
	default:
		return nil
	}
}

func parseDurationFlag(val string, defaultVal time.Duration) time.Duration {
	if val == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil || d <= 0 {
		return defaultVal
	}
	return d
}

func runIntelligenceFleet(cmd *cobra.Command, args []string) error {
	client, err := getIntelligenceClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), intelTimeout)
	defer cancel()

	summary, err := client.GetFleetHealth(ctx)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get fleet health summary: %w", err)
	}

	if intelFormat == "json" || intelFormat == "yaml" || intelFormat == "yml" {
		return outputFormatted(summary)
	}

	fmt.Println("🧠 Watchdog Fleet Intelligence Summary")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  Evaluated At:       %s\n", summary.EvaluatedAt.Local().Format(time.RFC3339))
	fmt.Printf("  Total Nodes:        %d\n", summary.TotalNodes)
	fmt.Printf("  Fleet Health Score: %.1f / 100.0\n", summary.AverageScore)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("  Healthy: %d | Warning: %d | Critical: %d | Stale: %d | Offline: %d\n",
		summary.HealthyCount, summary.WarningCount, summary.CriticalCount, summary.StaleCount, summary.OfflineCount)
	fmt.Printf("  Active Incidents:   %d\n", len(summary.ActiveIncidents))
	fmt.Printf("  Fleet Findings:     %d\n", len(summary.FleetFindings))
	fmt.Println()

	if len(summary.LowestScoringNodes) > 0 {
		fmt.Println("🔻 Lowest Scoring Nodes:")
		tw := util.NewTableWriter("Node ID", "Hostname", "Status", "Score", "Trajectory", "Primary Concerns")
		for _, n := range summary.LowestScoringNodes {
			concerns := "-"
			if len(n.HealthScore.PrimaryConcerns) > 0 {
				concerns = strings.Join(n.HealthScore.PrimaryConcerns, "; ")
			}
			tw.Append(
				n.NodeID,
				n.Hostname,
				string(n.Status),
				fmt.Sprintf("%.1f", n.HealthScore.Score),
				string(n.HealthScore.Trajectory),
				concerns,
			)
		}
		tw.Render(os.Stdout)
		fmt.Println()
	}

	if len(summary.FleetFindings) > 0 {
		fmt.Println("🔍 Fleet Pattern Findings:")
		tw := util.NewTableWriter("ID", "Category", "Severity", "Confidence", "Title", "Affected Nodes")
		for _, f := range summary.FleetFindings {
			tw.Append(
				f.ID,
				string(f.Category),
				string(f.Severity),
				string(f.Confidence),
				f.Title,
				strings.Join(f.AffectedNodes, ", "),
			)
		}
		tw.Render(os.Stdout)
	}

	return nil
}

func runIntelligenceNode(cmd *cobra.Command, args []string) error {
	client, err := getIntelligenceClient()
	if err != nil {
		return err
	}

	nodeID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), intelTimeout)
	defer cancel()

	summary, err := client.GetNodeHealth(ctx, nodeID)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get node health: %w", err)
	}

	if intelFormat == "json" || intelFormat == "yaml" || intelFormat == "yml" {
		return outputFormatted(summary)
	}

	fmt.Printf("🧠 Node Health Summary: %s (%s)\n", summary.Hostname, summary.NodeID)
	fmt.Println(strings.Repeat("=", 65))
	fmt.Printf("  Status:          %s\n", summary.Status)
	fmt.Printf("  Health Score:    %.1f / 100.0\n", summary.HealthScore.Score)
	fmt.Printf("  Trajectory:      %s\n", summary.HealthScore.Trajectory)
	fmt.Printf("  Evaluated At:    %s\n", summary.EvaluatedAt.Local().Format(time.RFC3339))
	if len(summary.HealthScore.PrimaryConcerns) > 0 {
		fmt.Printf("  Primary Concerns:\n")
		for _, c := range summary.HealthScore.PrimaryConcerns {
			fmt.Printf("    • %s\n", c)
		}
	}
	fmt.Println()

	if len(summary.HealthScore.Breakdown) > 0 {
		fmt.Println("📊 Explainable Score Factor Breakdown:")
		tw := util.NewTableWriter("Factor", "Category", "Weight", "Deduction", "Impact", "Explanation")
		for _, f := range summary.HealthScore.Breakdown {
			tw.Append(
				f.Name,
				f.Category,
				fmt.Sprintf("%.0f", f.Weight),
				fmt.Sprintf("-%.1f", f.Deduction),
				string(f.Impact),
				f.Explanation,
			)
		}
		tw.Render(os.Stdout)
		fmt.Println()
	}

	if len(summary.ActiveIncidents) > 0 {
		fmt.Println("⚠️ Active Incidents Affecting Node:")
		tw := util.NewTableWriter("ID", "Severity", "Status", "Title", "Start Time")
		for _, inc := range summary.ActiveIncidents {
			tw.Append(
				inc.ID,
				string(inc.Severity),
				string(inc.Status),
				inc.Title,
				inc.StartTime.Local().Format(time.RFC3339),
			)
		}
		tw.Render(os.Stdout)
		fmt.Println()
	}

	if len(summary.Findings) > 0 {
		fmt.Println("💡 Node Findings & Non-Invasive Suggestions:")
		for i, f := range summary.Findings {
			fmt.Printf("  [%d] %s (%s, %s)\n", i+1, f.Title, f.Severity, f.Confidence)
			fmt.Printf("      Description: %s\n", f.Description)
			if len(f.NonInvasiveSuggestions) > 0 {
				fmt.Printf("      Suggestions:\n")
				for _, s := range f.NonInvasiveSuggestions {
					fmt.Printf("        - %s\n", s)
				}
			}
		}
	}

	return nil
}

func runIntelligenceIncidents(cmd *cobra.Command, args []string) error {
	client, err := getIntelligenceClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), intelTimeout)
	defer cancel()

	targetID := intelIncidentID
	if len(args) > 0 {
		targetID = args[0]
	}

	if targetID != "" {
		incident, err := client.GetIncident(ctx, targetID)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get incident %q: %w", targetID, err)
		}

		if intelFormat == "json" || intelFormat == "yaml" || intelFormat == "yml" {
			return outputFormatted(incident)
		}

		fmt.Printf("🚨 Incident: %s\n", incident.Title)
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("  ID:             %s\n", incident.ID)
		fmt.Printf("  Severity:       %s\n", incident.Severity)
		fmt.Printf("  Status:         %s\n", incident.Status)
		fmt.Printf("  Start Time:     %s\n", incident.StartTime.Local().Format(time.RFC3339))
		if incident.EndTime != nil {
			fmt.Printf("  End Time:       %s\n", incident.EndTime.Local().Format(time.RFC3339))
		}
		fmt.Printf("  Affected Nodes: %s\n", strings.Join(incident.AffectedNodes, ", "))
		if len(incident.PrimarySymptoms) > 0 {
			fmt.Println("  Primary Symptoms:")
			for _, s := range incident.PrimarySymptoms {
				fmt.Printf("    • %s\n", s)
			}
		}
		fmt.Println()

		if len(incident.Timeline) > 0 {
			fmt.Println("⏱️ Incident Timeline Events:")
			tw := util.NewTableWriter("Timestamp", "Node ID", "Event Type", "Severity", "Description")
			for _, ev := range incident.Timeline {
				tw.Append(
					ev.Timestamp.Local().Format(time.RFC3339),
					ev.NodeID,
					ev.EventType,
					string(ev.Severity),
					ev.Description,
				)
			}
			tw.Render(os.Stdout)
		}
		return nil
	}

	incidents, err := client.GetActiveIncidents(ctx)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to list active incidents: %w", err)
	}

	if intelFormat == "json" || intelFormat == "yaml" || intelFormat == "yml" {
		return outputFormatted(incidents)
	}

	if len(incidents) == 0 {
		fmt.Println("No active incidents detected across the fleet.")
		return nil
	}

	fmt.Printf("🚨 Active Fleet Incidents (%d)\n", len(incidents))
	tw := util.NewTableWriter("ID", "Severity", "Status", "Title", "Affected Nodes", "Start Time")
	for _, inc := range incidents {
		tw.Append(
			inc.ID,
			string(inc.Severity),
			string(inc.Status),
			inc.Title,
			strings.Join(inc.AffectedNodes, ", "),
			inc.StartTime.Local().Format(time.RFC3339),
		)
	}
	tw.Render(os.Stdout)
	return nil
}

func runIntelligenceTrends(cmd *cobra.Command, args []string) error {
	client, err := getIntelligenceClient()
	if err != nil {
		return err
	}

	nodeID := args[0]
	window := parseDurationFlag(intelWindow, 1*time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), intelTimeout)
	defer cancel()

	trends, err := client.GetNodeTrends(ctx, nodeID, window)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get node trends: %w", err)
	}

	if intelFormat == "json" || intelFormat == "yaml" || intelFormat == "yml" {
		return outputFormatted(trends)
	}

	if len(trends) == 0 {
		fmt.Printf("No trend data available for node %q over window %s.\n", nodeID, window)
		return nil
	}

	fmt.Printf("📈 Metric Trends for Node %q (Window: %s)\n", nodeID, window)
	tw := util.NewTableWriter("Metric", "Direction", "Rate of Change", "Start Value", "End Value", "Confidence")
	for _, t := range trends {
		rateStr := fmt.Sprintf("%+.2f %s", t.RateOfChange, t.Unit)
		tw.Append(
			t.Metric,
			string(t.Direction),
			rateStr,
			fmt.Sprintf("%.2f", t.StartValue),
			fmt.Sprintf("%.2f", t.EndValue),
			fmt.Sprintf("%.0f%%", t.Confidence*100),
		)
	}
	tw.Render(os.Stdout)
	return nil
}

func runIntelligenceBaselines(cmd *cobra.Command, args []string) error {
	client, err := getIntelligenceClient()
	if err != nil {
		return err
	}

	nodeID := args[0]
	window := parseDurationFlag(intelWindow, 24*time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), intelTimeout)
	defer cancel()

	baselines, err := client.GetNodeBaselines(ctx, nodeID, window)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get node baselines: %w", err)
	}

	if intelFormat == "json" || intelFormat == "yaml" || intelFormat == "yml" {
		return outputFormatted(baselines)
	}

	if len(baselines) == 0 {
		fmt.Printf("No historical baselines available for node %q over window %s.\n", nodeID, window)
		return nil
	}

	fmt.Printf("📏 Historical Baselines for Node %q (Window: %s)\n", nodeID, window)
	tw := util.NewTableWriter("Metric", "Samples", "Min", "Max", "Mean", "StdDev", "P50", "P90", "P95", "P99")
	for _, b := range baselines {
		tw.Append(
			b.Metric,
			fmt.Sprintf("%d", b.SampleCount),
			fmt.Sprintf("%.2f", b.Min),
			fmt.Sprintf("%.2f", b.Max),
			fmt.Sprintf("%.2f", b.Mean),
			fmt.Sprintf("%.2f", b.StdDev),
			fmt.Sprintf("%.2f", b.P50),
			fmt.Sprintf("%.2f", b.P90),
			fmt.Sprintf("%.2f", b.P95),
			fmt.Sprintf("%.2f", b.P99),
		)
	}
	tw.Render(os.Stdout)
	return nil
}

func runIntelligenceCorrelations(cmd *cobra.Command, args []string) error {
	client, err := getIntelligenceClient()
	if err != nil {
		return err
	}

	window := parseDurationFlag(intelWindow, 1*time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), intelTimeout)
	defer cancel()

	correlations, err := client.GetCorrelations(ctx, window)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get correlations: %w", err)
	}

	if intelFormat == "json" || intelFormat == "yaml" || intelFormat == "yml" {
		return outputFormatted(correlations)
	}

	if len(correlations) == 0 {
		fmt.Printf("No cross-signal correlations detected over window %s.\n", window)
		return nil
	}

	fmt.Printf("🔗 Signal Temporal Correlations (Window: %s)\n", window)
	tw := util.NewTableWriter("Primary Signal", "Secondary Signal", "Coefficient", "Offset (s)", "Co-Occurrences", "Confidence", "Description")
	for _, c := range correlations {
		tw.Append(
			c.PrimarySignal,
			c.SecondarySignal,
			fmt.Sprintf("%.2f", c.Coefficient),
			fmt.Sprintf("%d", c.TimeOffsetSeconds),
			fmt.Sprintf("%d", c.CoOccurrenceCount),
			string(c.Confidence),
			c.Description,
		)
	}
	tw.Render(os.Stdout)
	return nil
}

func runIntelligenceFindings(cmd *cobra.Command, args []string) error {
	client, err := getIntelligenceClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), intelTimeout)
	defer cancel()

	category := intelligence.FindingCategory(intelCategory)
	minSev := model.Severity(intelMinSeverity)

	findings, err := client.GetFindings(ctx, category, minSev)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get findings: %w", err)
	}

	if intelFormat == "json" || intelFormat == "yaml" || intelFormat == "yml" {
		return outputFormatted(findings)
	}

	if len(findings) == 0 {
		fmt.Println("No intelligence findings matching the specified criteria.")
		return nil
	}

	fmt.Printf("💡 Intelligence Findings (%d)\n", len(findings))
	for i, f := range findings {
		fmt.Printf("\n[%d] %s (%s, %s)\n", i+1, f.Title, f.Severity, f.Confidence)
		fmt.Printf("    ID:             %s\n", f.ID)
		fmt.Printf("    Category:       %s\n", f.Category)
		fmt.Printf("    Detected At:    %s\n", f.DetectedAt.Local().Format(time.RFC3339))
		fmt.Printf("    Affected Nodes: %s\n", strings.Join(f.AffectedNodes, ", "))
		fmt.Printf("    Description:    %s\n", f.Description)
		if len(f.SupportingEvidence) > 0 {
			fmt.Println("    Supporting Evidence:")
			for _, ev := range f.SupportingEvidence {
				fmt.Printf("      • %s\n", ev)
			}
		}
		if len(f.NonInvasiveSuggestions) > 0 {
			fmt.Println("    Non-Invasive Suggestions:")
			for _, s := range f.NonInvasiveSuggestions {
				fmt.Printf("      - %s\n", s)
			}
		}
	}
	return nil
}
