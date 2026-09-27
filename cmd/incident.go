package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/DocHoax/watchdog/pkg/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	incServerURL     string
	incToken         string
	incTokenFile     string
	incTokenEnv      string
	incInsecureTLS   bool
	incTimeout       time.Duration
	incFormat        string
	incStatuses      []string
	incSeverities    []string
	incScopes        []string
	incNodeID        string
	incSearch        string
	incStartTime     string
	incEndTime       string
	incLimit         int
	incOffset        int
	incSortBy        string
	incSortOrder     string
	incMinSimilarity float64
	incMinSeverity   string
	incReason        string
)

var incidentCmd = &cobra.Command{
	Use:     "incident [command]",
	Aliases: []string{"incidents", "inc"},
	Short:   "Query, correlate, track, and investigate cluster-wide incidents",
	Long: `Provides comprehensive operational incident management, multi-node correlation,
and root-cause investigation capabilities for the Watchdog fleet.

The incident operations layer answers six fundamental operational questions:
  1. What happened and when did it happen? (Root signals, symptoms, start/update/resolution times)
  2. Which nodes and infrastructure are within the blast radius? (Fleet coverage, affected subsystems)
  3. What contributing signals triggered the incident? (Alerts, check failures, anomalies)
  4. What happened immediately before the incident? (Deterministic chronological timeline)
  5. Has this incident occurred before? (Statistical recurrence, periodicity, flapping, Jaccard similarity)
  6. What non-invasive investigation findings and explanations exist? (Explainable severity score, findings)`,
	Example: `  # List active and recent incidents
  watchdog incident list

  # Filter incidents by severity and status
  watchdog incident list --severity critical,warning --status detected,investigating

  # View detailed incident information and root signals
  watchdog incident get inc-20260927-001

  # View chronological timeline leading up to an incident
  watchdog incident timeline inc-20260927-001

  # Analyze blast radius and affected infrastructure
  watchdog incident impact inc-20260927-001

  # Generate a full investigation dossier
  watchdog incident investigate inc-20260927-001

  # Transition incident lifecycle status
  watchdog incident status inc-20260927-001 investigating --reason "Investigating root causes"

  # Find historical incidents matching current symptoms
  watchdog incident similar inc-20260927-001 --min-similarity 0.4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIncidentList(cmd, args)
	},
}

var incidentListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List and filter cluster incidents",
	Long:    "Queries and displays a paginated list of correlated cluster incidents matching specified filters.",
	Example: `  watchdog incident list
  watchdog incident list --status detected,investigating --severity critical
  watchdog incident list --node-id node-01 --limit 20
  watchdog incident list --search "memory leak" --format json`,
	RunE: runIncidentList,
}

var incidentGetCmd = &cobra.Command{
	Use:     "get <incident-id> [flags]",
	Aliases: []string{"show", "view", "inspect"},
	Short:   "Get detailed information for a specific incident",
	Long:    "Retrieves and displays complete details, root signals, impact scope, and severity explanations for an incident.",
	Args:    cobra.ExactArgs(1),
	Example: `  watchdog incident get inc-20260927-001
  watchdog incident get inc-20260927-001 --format json`,
	RunE: runIncidentGet,
}

var incidentSummaryCmd = &cobra.Command{
	Use:     "summary [flags]",
	Aliases: []string{"stats", "overview"},
	Short:   "Display aggregated cluster-wide incident summary",
	Long:    "Displays statistical aggregations of active, investigating, and resolved incidents across the fleet.",
	Example: `  watchdog incident summary
  watchdog incident summary --format yaml`,
	RunE: runIncidentSummary,
}

var incidentTimelineCmd = &cobra.Command{
	Use:     "timeline <incident-id> [flags]",
	Aliases: []string{"tl", "events"},
	Short:   "Display chronological timeline of events for an incident",
	Long:    "Retrieves and renders the deterministic chronological event timeline leading up to and during an incident.",
	Args:    cobra.ExactArgs(1),
	Example: `  watchdog incident timeline inc-20260927-001
  watchdog incident timeline inc-20260927-001 --min-severity warning --limit 50`,
	RunE: runIncidentTimeline,
}

var incidentRelatedCmd = &cobra.Command{
	Use:     "related <incident-id> [flags]",
	Aliases: []string{"signals", "context"},
	Short:   "Display related root signals, recurrence statistics, and similar incidents",
	Long:    "Retrieves correlated root signals, recurrence patterns, and historical matches for an incident.",
	Args:    cobra.ExactArgs(1),
	Example: `  watchdog incident related inc-20260927-001`,
	RunE: runIncidentRelated,
}

var incidentImpactCmd = &cobra.Command{
	Use:     "impact <incident-id> [flags]",
	Aliases: []string{"blast-radius", "blast"},
	Short:   "Display blast radius and subsystem impact analysis",
	Long:    "Calculates and displays fleet coverage, affected node lists, and subsystem distribution for an incident.",
	Args:    cobra.ExactArgs(1),
	Example: `  watchdog incident impact inc-20260927-001`,
	RunE: runIncidentImpact,
}

var incidentFindingsCmd = &cobra.Command{
	Use:     "findings <incident-id> [flags]",
	Aliases: []string{"intelligence", "advice"},
	Short:   "Display non-invasive intelligence findings and operator advisory recommendations",
	Long:    "Retrieves intelligence explanations, risk assessments, and non-invasive operator guidance for an incident.",
	Args:    cobra.ExactArgs(1),
	Example: `  watchdog incident findings inc-20260927-001`,
	RunE: runIncidentFindings,
}

var incidentInvestigateCmd = &cobra.Command{
	Use:     "investigate <incident-id> [flags]",
	Aliases: []string{"dossier", "report"},
	Short:   "Generate a comprehensive investigation dossier for an incident",
	Long:    "Consolidates incident details, blast radius, chronological timeline, recurrence analysis, and findings into a unified investigation report.",
	Args:    cobra.ExactArgs(1),
	Example: `  watchdog incident investigate inc-20260927-001
  watchdog incident investigate inc-20260927-001 --format json`,
	RunE: runIncidentInvestigate,
}

var incidentStatusCmd = &cobra.Command{
	Use:     "status <incident-id> <new-status> [reason] [flags]",
	Aliases: []string{"update-status", "set-status", "transition"},
	Short:   "Transition an incident's lifecycle status",
	Long: `Transitions an incident's operational status according to the valid lifecycle state machine:
  detected -> acknowledged -> investigating -> resolved -> closed
  detected / investigating -> suppressed
  resolved / closed -> reopened`,
	Args: cobra.RangeArgs(2, 3),
	Example: `  watchdog incident status inc-20260927-001 acknowledged
  watchdog incident status inc-20260927-001 investigating -r "Engineers reviewing logs"
  watchdog incident status inc-20260927-001 resolved -r "Memory leak patched and service restarted"`,
	RunE: runIncidentStatus,
}

var incidentSimilarCmd = &cobra.Command{
	Use:     "similar <incident-id> [flags]",
	Aliases: []string{"match", "history"},
	Short:   "Find similar historical incidents based on multi-factor Jaccard scoring",
	Long:    "Computes multi-factor Jaccard similarity across symptoms (40%), subsystems (30%), nodes (20%), and severity (10%).",
	Args:    cobra.ExactArgs(1),
	Example: `  watchdog incident similar inc-20260927-001 --min-similarity 0.35 --limit 10`,
	RunE: runIncidentSimilar,
}

func init() {
	// Persistent flags on incidentCmd
	incidentCmd.PersistentFlags().StringVar(&incServerURL, "server", "", "centralized fleet server URL (e.g. https://fleet.internal:8443)")
	incidentCmd.PersistentFlags().StringVar(&incToken, "token", "", "bearer token for fleet server authentication")
	incidentCmd.PersistentFlags().StringVar(&incTokenFile, "token-file", "", "path to file containing bearer token")
	incidentCmd.PersistentFlags().StringVar(&incTokenEnv, "token-env", "", "environment variable containing bearer token")
	incidentCmd.PersistentFlags().BoolVar(&incInsecureTLS, "insecure-tls", false, "skip TLS certificate validation (development only)")
	incidentCmd.PersistentFlags().DurationVar(&incTimeout, "timeout", 10*time.Second, "HTTP request timeout duration")
	incidentCmd.PersistentFlags().StringVarP(&incFormat, "format", "f", "text", "output format (text, json, yaml)")

	// Flags for list
	incidentListCmd.Flags().StringSliceVarP(&incStatuses, "status", "s", nil, "filter by status (detected, acknowledged, investigating, resolved, suppressed, reopened)")
	incidentListCmd.Flags().StringSliceVar(&incSeverities, "severity", nil, "filter by severity (info, warning, critical)")
	incidentListCmd.Flags().StringSliceVar(&incScopes, "scope", nil, "filter by blast radius scope (node, multi_node, fleet)")
	incidentListCmd.Flags().StringVarP(&incNodeID, "node-id", "n", "", "filter incidents affecting specific node")
	incidentListCmd.Flags().StringVarP(&incSearch, "search", "q", "", "search keywords in title or ID")
	incidentListCmd.Flags().StringVar(&incStartTime, "start-time", "", "filter incidents starting after timestamp or relative duration (e.g. 24h, 2026-09-27T00:00:00Z)")
	incidentListCmd.Flags().StringVar(&incEndTime, "end-time", "", "filter incidents starting before timestamp or relative duration")
	incidentListCmd.Flags().IntVarP(&incLimit, "limit", "l", 50, "maximum number of incidents to return")
	incidentListCmd.Flags().IntVar(&incOffset, "offset", 0, "pagination offset")
	incidentListCmd.Flags().StringVar(&incSortBy, "sort-by", "start_time", "sort field (start_time, updated_at, severity, status, title)")
	incidentListCmd.Flags().StringVar(&incSortOrder, "sort-order", "desc", "sort order (asc, desc)")

	// Flags for timeline
	incidentTimelineCmd.Flags().StringVarP(&incNodeID, "node-id", "n", "", "filter events by node ID")
	incidentTimelineCmd.Flags().StringVar(&incMinSeverity, "min-severity", "", "minimum event severity filter (info, warning, critical)")
	incidentTimelineCmd.Flags().StringVar(&incStartTime, "start-time", "", "filter events starting after timestamp or relative duration")
	incidentTimelineCmd.Flags().StringVar(&incEndTime, "end-time", "", "filter events starting before timestamp or relative duration")
	incidentTimelineCmd.Flags().IntVarP(&incLimit, "limit", "l", 100, "maximum number of timeline events to display")

	// Flags for status
	incidentStatusCmd.Flags().StringVarP(&incReason, "reason", "r", "", "reason description for status transition")

	// Flags for similar
	incidentSimilarCmd.Flags().Float64VarP(&incMinSimilarity, "min-similarity", "m", 0.3, "minimum Jaccard similarity cutoff (0.0 - 1.0)")
	incidentSimilarCmd.Flags().IntVarP(&incLimit, "limit", "l", 10, "maximum number of similar incidents to return")

	// Add subcommands
	incidentCmd.AddCommand(
		incidentListCmd,
		incidentGetCmd,
		incidentSummaryCmd,
		incidentTimelineCmd,
		incidentRelatedCmd,
		incidentImpactCmd,
		incidentFindingsCmd,
		incidentInvestigateCmd,
		incidentStatusCmd,
		incidentSimilarCmd,
	)

	RootCmd.AddCommand(incidentCmd)
}

func getIncidentClient() (*incidents.Client, error) {
	serverURL := incServerURL
	if serverURL == "" && globalCfg != nil {
		serverURL = globalCfg.Fleet.ServerURL
	}
	if serverURL == "" {
		return nil, NewExitError(ExitConfigError, "fleet server URL is required (specify via --server flag or fleet.server_url in config)")
	}

	token := incToken
	tokenFile := incTokenFile
	tokenEnv := incTokenEnv
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
	if incInsecureTLS {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}

	timeout := incTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	client := incidents.NewClient(incidents.ClientConfig{
		Endpoint:  serverURL,
		Token:     resolvedToken,
		Timeout:   timeout,
		TLSConfig: tlsConfig,
	})
	return client, nil
}

func outputIncidentFormatted(v any) error {
	switch strings.ToLower(incFormat) {
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

func parseTimeOrDurationFlag(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}

	// Try relative duration (e.g., "1h", "24h", "30m")
	if d, err := time.ParseDuration(s); err == nil {
		if d > 0 {
			return time.Now().UTC().Add(-d), nil
		}
		return time.Now().UTC().Add(d), nil
	}

	// Try RFC3339 / ISO8601
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time value %q (use RFC3339 or duration like 1h, 24h)", s)
}

func runIncidentList(cmd *cobra.Command, _ []string) error {
	client, err := getIncidentClient()
	if err != nil {
		return err
	}

	var filter incidents.IncidentFilter
	for _, st := range incStatuses {
		for _, s := range strings.Split(st, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				filter.Status = append(filter.Status, incidents.IncidentStatus(s))
			}
		}
	}
	for _, sv := range incSeverities {
		for _, s := range strings.Split(sv, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				filter.Severity = append(filter.Severity, model.Severity(strings.ToUpper(s)))
			}
		}
	}
	for _, sc := range incScopes {
		for _, s := range strings.Split(sc, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				filter.Scope = append(filter.Scope, incidents.IncidentScope(s))
			}
		}
	}
	filter.NodeID = incNodeID
	filter.Search = incSearch

	if incStartTime != "" {
		t, err := parseTimeOrDurationFlag(incStartTime)
		if err != nil {
			return NewExitError(ExitUsageError, "invalid --start-time: %w", err)
		}
		filter.StartTime = t
	}
	if incEndTime != "" {
		t, err := parseTimeOrDurationFlag(incEndTime)
		if err != nil {
			return NewExitError(ExitUsageError, "invalid --end-time: %w", err)
		}
		filter.EndTime = t
	}

	filter.Limit = incLimit
	filter.Offset = incOffset
	filter.SortBy = incSortBy
	filter.SortOrder = incSortOrder

	ctx, cancel := context.WithTimeout(context.Background(), incTimeout)
	defer cancel()

	resp, err := client.ListIncidents(ctx, filter)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to list incidents: %w", err)
	}

	if incFormat == "json" || incFormat == "yaml" || incFormat == "yml" {
		return outputIncidentFormatted(resp)
	}

	fmt.Println("🚨 Watchdog Correlated Cluster Incidents")
	fmt.Println(strings.Repeat("=", 80))

	if len(resp.Incidents) == 0 {
		fmt.Println("No incidents matching the specified filters were found.")
		return nil
	}

	tw := util.NewTableWriter("Incident ID", "Severity", "Score", "Status", "Scope", "Nodes", "Started", "Title")
	for _, inc := range resp.Incidents {
		nodeDisplay := fmt.Sprintf("%d node(s)", len(inc.AffectedNodes))
		if len(inc.AffectedNodes) == 1 {
			nodeDisplay = inc.AffectedNodes[0]
		}

		startTimeStr := inc.StartTime.Local().Format("2006-01-02 15:04:05")
		scoreStr := fmt.Sprintf("%.1f", inc.SeverityScore)
		if inc.SeverityScore == 0 && inc.SeverityExplanation.BaseScore > 0 {
			scoreStr = fmt.Sprintf("%.1f", inc.SeverityExplanation.BaseScore)
		}

		tw.Append(
			inc.ID,
			string(inc.Severity),
			scoreStr,
			string(inc.Status),
			string(inc.Scope),
			nodeDisplay,
			startTimeStr,
			inc.Title,
		)
	}
	tw.Render(os.Stdout)
	fmt.Printf("\nShowing %d of %d total incidents (Offset: %d, Limit: %d)\n",
		len(resp.Incidents), resp.Total, resp.Offset, resp.Limit)

	return nil
}

func runIncidentGet(cmd *cobra.Command, args []string) error {
	client, err := getIncidentClient()
	if err != nil {
		return err
	}

	incidentID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), incTimeout)
	defer cancel()

	inc, err := client.GetIncident(ctx, incidentID)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get incident %s: %w", incidentID, err)
	}

	if incFormat == "json" || incFormat == "yaml" || incFormat == "yml" {
		return outputIncidentFormatted(inc)
	}

	printIncidentDetails(inc)
	return nil
}

func printIncidentDetails(inc *incidents.Incident) {
	fmt.Printf("🚨 Incident Dossier: %s\n", inc.ID)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("  Title:           %s\n", inc.Title)
	fmt.Printf("  Status:          %s\n", inc.Status)
	fmt.Printf("  Severity:        %s (Score: %.1f/100.0, Confidence: %s)\n",
		inc.Severity, inc.SeverityScore, inc.Confidence)
	fmt.Printf("  Blast Scope:     %s (Affected Nodes: %d)\n", inc.Scope, len(inc.AffectedNodes))
	fmt.Printf("  Started At:      %s\n", inc.StartTime.Local().Format(time.RFC3339))
	fmt.Printf("  Last Updated:    %s\n", inc.UpdatedAt.Local().Format(time.RFC3339))
	if inc.ResolvedAt != nil && !inc.ResolvedAt.IsZero() {
		dur := inc.ResolvedAt.Sub(inc.StartTime).Round(time.Second)
		fmt.Printf("  Resolved At:     %s (Duration: %s)\n", inc.ResolvedAt.Local().Format(time.RFC3339), dur)
	}
	if inc.Summary != "" {
		fmt.Printf("  Summary:         %s\n", inc.Summary)
	}

	if len(inc.PrimarySymptoms) > 0 {
		fmt.Println("\n🩺 Primary Symptoms:")
		for _, s := range inc.PrimarySymptoms {
			fmt.Printf("    • %s\n", s)
		}
	}

	if len(inc.AffectedNodes) > 0 {
		fmt.Printf("\n🖥️ Affected Nodes (%d):\n", len(inc.AffectedNodes))
		fmt.Printf("    %s\n", strings.Join(inc.AffectedNodes, ", "))
	}

	if len(inc.SeverityExplanation.Factors) > 0 {
		fmt.Println("\n📊 Severity Score Breakdown:")
		tw := util.NewTableWriter("Factor Name", "Category", "Weight", "Points", "Description")
		for _, f := range inc.SeverityExplanation.Factors {
			tw.Append(
				f.Name,
				f.Category,
				fmt.Sprintf("%.2f", f.Weight),
				fmt.Sprintf("+%.1f", f.Points),
				f.Description,
			)
		}
		tw.Render(os.Stdout)
	}

	if len(inc.RootSignals) > 0 {
		fmt.Printf("\n⚡ Root Contributing Signals (%d):\n", len(inc.RootSignals))
		tw := util.NewTableWriter("Signal Type", "Source", "Node ID", "Severity", "Timestamp", "Description")
		for _, sig := range inc.RootSignals {
			tw.Append(
				string(sig.Type),
				sig.Source,
				sig.NodeID,
				string(sig.Severity),
				sig.Timestamp.Local().Format("15:04:05"),
				sig.Description,
			)
		}
		tw.Render(os.Stdout)
	}

	if len(inc.Findings) > 0 {
		fmt.Printf("\n💡 Non-Invasive Intelligence Findings (%d):\n", len(inc.Findings))
		for _, f := range inc.Findings {
			fmt.Printf("  [%s] %s (%s, %s confidence)\n", f.ID, f.Title, f.Severity, f.Confidence)
			fmt.Printf("    Description: %s\n", f.Description)
			if len(f.NonInvasiveSuggestions) > 0 {
				fmt.Printf("    Advisory:    %s\n", strings.Join(f.NonInvasiveSuggestions, "; "))
			}
		}
	}

	if len(inc.Tags) > 0 {
		fmt.Println("\n🏷️ Tags:")
		var tagPairs []string
		for k, v := range inc.Tags {
			tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(tagPairs)
		fmt.Printf("    %s\n", strings.Join(tagPairs, ", "))
	}
}

func runIncidentSummary(cmd *cobra.Command, _ []string) error {
	client, err := getIncidentClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), incTimeout)
	defer cancel()

	summary, err := client.GetSummary(ctx)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get incident summary: %w", err)
	}

	if incFormat == "json" || incFormat == "yaml" || incFormat == "yml" {
		return outputIncidentFormatted(summary)
	}

	fmt.Println("📊 Watchdog Cluster Incident Summary")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("  Total Incidents Recorded: %d\n", summary.TotalCount)
	fmt.Printf("  Active (Detected):         %d\n", summary.DetectedCount)
	fmt.Printf("  Acknowledged:              %d\n", summary.AcknowledgedCount)
	fmt.Printf("  Under Investigation:       %d\n", summary.InvestigatingCount)
	fmt.Printf("  Resolved Incidents:        %d\n", summary.ResolvedCount)
	if summary.SuppressedCount > 0 {
		fmt.Printf("  Suppressed:                %d\n", summary.SuppressedCount)
	}
	if summary.ReopenedCount > 0 {
		fmt.Printf("  Reopened:                  %d\n", summary.ReopenedCount)
	}
	if summary.AverageResolutionTime > 0 {
		fmt.Printf("  Mean Resolution Time:      %s\n", summary.AverageResolutionTime.Round(time.Second))
	}
	fmt.Println(strings.Repeat("-", 70))

	fmt.Println("\n📈 Distribution By Severity:")
	fmt.Printf("  • Critical : %d\n", summary.CriticalCount)
	fmt.Printf("  • Warning  : %d\n", summary.WarningCount)
	fmt.Printf("  • Info     : %d\n", summary.InfoCount)

	fmt.Println("\n🌐 Distribution By Scope:")
	for sc, count := range summary.ScopeDistribution {
		fmt.Printf("  • %-12s : %d\n", sc, count)
	}

	if len(summary.TopAffectedNodes) > 0 {
		fmt.Println("\n🖥️ Top Affected Nodes:")
		tw := util.NewTableWriter("Node ID", "Hostname", "Incident Count", "Critical", "Warning")
		for _, n := range summary.TopAffectedNodes {
			hostname := n.Hostname
			if hostname == "" {
				hostname = "-"
			}
			tw.Append(n.NodeID, hostname, strconv.Itoa(n.IncidentCount), strconv.Itoa(n.CriticalCount), strconv.Itoa(n.WarningCount))
		}
		tw.Render(os.Stdout)
	}

	if len(summary.RecentIncidents) > 0 {
		fmt.Println("\n🚨 Recent Incidents:")
		tw := util.NewTableWriter("Incident ID", "Severity", "Status", "Scope", "Started", "Title")
		for _, inc := range summary.RecentIncidents {
			tw.Append(
				inc.ID,
				string(inc.Severity),
				string(inc.Status),
				string(inc.Scope),
				inc.StartTime.Local().Format("2006-01-02 15:04"),
				inc.Title,
			)
		}
		tw.Render(os.Stdout)
	}

	return nil
}

func runIncidentTimeline(cmd *cobra.Command, args []string) error {
	client, err := getIncidentClient()
	if err != nil {
		return err
	}

	incidentID := args[0]
	var filter incidents.TimelineFilter
	filter.NodeID = incNodeID
	if incMinSeverity != "" {
		filter.MinSeverity = model.Severity(strings.ToUpper(incMinSeverity))
	}
	if incStartTime != "" {
		t, err := parseTimeOrDurationFlag(incStartTime)
		if err != nil {
			return NewExitError(ExitUsageError, "invalid --start-time: %w", err)
		}
		filter.StartTime = t
	}
	if incEndTime != "" {
		t, err := parseTimeOrDurationFlag(incEndTime)
		if err != nil {
			return NewExitError(ExitUsageError, "invalid --end-time: %w", err)
		}
		filter.EndTime = t
	}
	filter.Limit = incLimit

	ctx, cancel := context.WithTimeout(context.Background(), incTimeout)
	defer cancel()

	resp, err := client.GetTimeline(ctx, incidentID, filter)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get timeline for incident %s: %w", incidentID, err)
	}

	if incFormat == "json" || incFormat == "yaml" || incFormat == "yml" {
		return outputIncidentFormatted(resp)
	}

	fmt.Printf("⏱️ Chronological Event Timeline for Incident: %s\n", incidentID)
	fmt.Println(strings.Repeat("=", 80))

	if len(resp.Timeline) == 0 {
		fmt.Println("No timeline events recorded for this incident.")
		return nil
	}

	tw := util.NewTableWriter("Event ID", "Timestamp", "Severity", "Type", "Node ID", "Source", "Title")
	for _, e := range resp.Timeline {
		nodeID := e.NodeID
		if nodeID == "" {
			nodeID = "-"
		}
		tw.Append(
			e.ID,
			e.Timestamp.Local().Format("2006-01-02 15:04:05.000"),
			string(e.Severity),
			string(e.EventType),
			nodeID,
			e.Source,
			e.Title,
		)
	}
	tw.Render(os.Stdout)
	fmt.Printf("\nTotal Timeline Events: %d (Ordered Ascending by Timestamp)\n", resp.Count)

	return nil
}

func runIncidentRelated(cmd *cobra.Command, args []string) error {
	client, err := getIncidentClient()
	if err != nil {
		return err
	}

	incidentID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), incTimeout)
	defer cancel()

	resp, err := client.GetRelated(ctx, incidentID)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get related incident signals: %w", err)
	}

	if incFormat == "json" || incFormat == "yaml" || incFormat == "yml" {
		return outputIncidentFormatted(resp)
	}

	fmt.Printf("🔗 Correlated Context & Signals for Incident: %s\n", incidentID)
	fmt.Println(strings.Repeat("=", 80))

	if len(resp.Signals) > 0 {
		fmt.Printf("\n⚡ Root Contributing Signals (%d):\n", len(resp.Signals))
		tw := util.NewTableWriter("Signal Type", "Source", "Node ID", "Severity", "Timestamp", "Description")
		for _, sig := range resp.Signals {
			tw.Append(
				string(sig.Type),
				sig.Source,
				sig.NodeID,
				string(sig.Severity),
				sig.Timestamp.Local().Format("15:04:05"),
				sig.Description,
			)
		}
		tw.Render(os.Stdout)
	}

	if resp.Recurrence != nil {
		fmt.Println("\n🔁 Statistical Recurrence Pattern:")
		fmt.Printf("  Pattern Key:       %s\n", resp.Recurrence.PatternKey)
		fmt.Printf("  Occurrences:       %d\n", resp.Recurrence.OccurrenceCount)
		fmt.Printf("  Periodicity:       %s (CV: %.2f)\n", resp.Recurrence.Periodicity, resp.Recurrence.CoefficientOfVariation)
		fmt.Printf("  Flapping Detected: %v\n", resp.Recurrence.IsFlapping)
		if resp.Recurrence.AverageInterval > 0 {
			fmt.Printf("  Average Interval:  %s (Median: %s, StdDev: %s)\n",
				resp.Recurrence.AverageInterval.Round(time.Second),
				resp.Recurrence.MedianInterval.Round(time.Second),
				resp.Recurrence.StandardDeviation.Round(time.Second),
			)
		}
		if resp.Recurrence.Summary != "" {
			fmt.Printf("  Pattern Summary:   %s\n", resp.Recurrence.Summary)
		}
	}

	if len(resp.SimilarIncidents) > 0 {
		fmt.Printf("\n👥 Similar Historical Incidents (%d):\n", len(resp.SimilarIncidents))
		tw := util.NewTableWriter("Incident ID", "Similarity", "Status", "Severity", "Started", "Title")
		for _, sim := range resp.SimilarIncidents {
			tw.Append(
				sim.Incident.ID,
				fmt.Sprintf("%.1f%%", sim.SimilarityScore*100.0),
				string(sim.Incident.Status),
				string(sim.Incident.Severity),
				sim.Incident.StartTime.Local().Format("2006-01-02 15:04"),
				sim.Incident.Title,
			)
		}
		tw.Render(os.Stdout)
	}

	return nil
}

func runIncidentImpact(cmd *cobra.Command, args []string) error {
	client, err := getIncidentClient()
	if err != nil {
		return err
	}

	incidentID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), incTimeout)
	defer cancel()

	impact, err := client.GetImpact(ctx, incidentID)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get impact analysis for incident %s: %w", incidentID, err)
	}

	if incFormat == "json" || incFormat == "yaml" || incFormat == "yml" {
		return outputIncidentFormatted(impact)
	}

	fmt.Printf("💥 Blast Radius & Impact Analysis for Incident: %s\n", incidentID)
	fmt.Println(strings.Repeat("=", 75))
	fmt.Printf("  Blast Scope:       %s (%s)\n", impact.Scope, impact.EstimatedBlastRadius)
	fmt.Printf("  Affected Nodes:    %d of %d (%.1f%% Fleet Coverage)\n",
		len(impact.Impact.AffectedNodeIDs), impact.Impact.TotalFleetNodes, impact.Impact.FleetPercentage)
	fmt.Printf("  Critical Nodes:    %d | Warning Nodes: %d\n", impact.CriticalNodesCount, impact.WarningNodesCount)
	if impact.Summary != "" {
		fmt.Printf("  Summary:           %s\n", impact.Summary)
	}
	fmt.Println(strings.Repeat("-", 75))

	if len(impact.Impact.AffectedNodeIDs) > 0 {
		fmt.Println("\n🖥️ Affected Nodes:")
		tw := util.NewTableWriter("Node ID", "Hostname")
		for i, id := range impact.Impact.AffectedNodeIDs {
			hostname := "-"
			if i < len(impact.Impact.AffectedHostnames) && impact.Impact.AffectedHostnames[i] != "" {
				hostname = impact.Impact.AffectedHostnames[i]
			}
			tw.Append(id, hostname)
		}
		tw.Render(os.Stdout)
	}

	if len(impact.Impact.Subsystems) > 0 {
		fmt.Println("\n⚙️ Impacted Subsystems:")
		for _, sub := range impact.Impact.Subsystems {
			fmt.Printf("  • %s\n", sub)
		}
	}

	if len(impact.Impact.Resources) > 0 {
		fmt.Println("\n📦 Impacted Resources:")
		for _, res := range impact.Impact.Resources {
			fmt.Printf("  • %s\n", res)
		}
	}

	return nil
}

func runIncidentFindings(cmd *cobra.Command, args []string) error {
	client, err := getIncidentClient()
	if err != nil {
		return err
	}

	incidentID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), incTimeout)
	defer cancel()

	resp, err := client.GetFindings(ctx, incidentID)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get findings for incident %s: %w", incidentID, err)
	}

	if incFormat == "json" || incFormat == "yaml" || incFormat == "yml" {
		return outputIncidentFormatted(resp)
	}

	fmt.Printf("💡 Intelligence Findings & Advisory for Incident: %s\n", incidentID)
	fmt.Println(strings.Repeat("=", 80))

	if len(resp.Findings) == 0 {
		fmt.Println("No non-invasive intelligence findings reported for this incident.")
		return nil
	}

	for i, f := range resp.Findings {
		fmt.Printf("\n[%d] Finding: %s (%s)\n", i+1, f.Title, f.ID)
		fmt.Printf("    Category:       %s\n", f.Category)
		fmt.Printf("    Severity:       %s\n", f.Severity)
		fmt.Printf("    Confidence:     %s\n", f.Confidence)
		fmt.Printf("    Description:    %s\n", f.Description)
		if len(f.SupportingEvidence) > 0 {
			fmt.Printf("    Evidence:       %s\n", strings.Join(f.SupportingEvidence, "; "))
		}
		if len(f.NonInvasiveSuggestions) > 0 {
			fmt.Printf("    Advisory:       %s\n", strings.Join(f.NonInvasiveSuggestions, "; "))
		}
		if len(f.AffectedNodes) > 0 {
			fmt.Printf("    Affected Nodes: %s\n", strings.Join(f.AffectedNodes, ", "))
		}
	}

	return nil
}

func runIncidentInvestigate(cmd *cobra.Command, args []string) error {
	client, err := getIncidentClient()
	if err != nil {
		return err
	}

	incidentID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), incTimeout)
	defer cancel()

	report, err := client.Investigate(ctx, incidentID)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to investigate incident %s: %w", incidentID, err)
	}

	if incFormat == "json" || incFormat == "yaml" || incFormat == "yml" {
		return outputIncidentFormatted(report)
	}

	fmt.Println("🔬 Watchdog Comprehensive Incident Investigation Dossier")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Incident ID:        %s\n", report.Incident.ID)
	fmt.Printf("Title:              %s\n", report.Incident.Title)
	fmt.Printf("Severity:           %s (Composite Score: %.1f/100.0, Confidence: %s)\n",
		report.Incident.Severity, report.Incident.SeverityScore, report.Incident.Confidence)
	fmt.Printf("Status:             %s\n", report.Incident.Status)
	fmt.Printf("Started At:         %s\n", report.Incident.StartTime.Local().Format(time.RFC3339))
	if report.Incident.ResolvedAt != nil && !report.Incident.ResolvedAt.IsZero() {
		dur := report.Incident.ResolvedAt.Sub(report.Incident.StartTime).Round(time.Second)
		fmt.Printf("Resolved At:        %s (Duration: %s)\n", report.Incident.ResolvedAt.Local().Format(time.RFC3339), dur)
	}
	fmt.Println(strings.Repeat("-", 80))

	// Impact section
	fmt.Println("\n💥 Blast Radius & Infrastructure Impact:")
	fmt.Printf("  Scope:             %s (%s)\n", report.Impact.Scope, report.Impact.EstimatedBlastRadius)
	fmt.Printf("  Affected Nodes:    %d of %d (%.1f%% of fleet)\n",
		len(report.Impact.Impact.AffectedNodeIDs), report.Impact.Impact.TotalFleetNodes, report.Impact.Impact.FleetPercentage)
	if len(report.Impact.Impact.AffectedNodeIDs) > 0 {
		fmt.Printf("  Node List:         %s\n", strings.Join(report.Impact.Impact.AffectedNodeIDs, ", "))
	}
	if len(report.Impact.Impact.Subsystems) > 0 {
		fmt.Printf("  Subsystems:        %s\n", strings.Join(report.Impact.Impact.Subsystems, ", "))
	}

	// Recurrence section
	if report.Recurrence != nil {
		fmt.Println("\n🔁 Statistical Recurrence Analysis:")
		fmt.Printf("  Pattern Key:       %s\n", report.Recurrence.PatternKey)
		fmt.Printf("  Occurrences:       %d times\n", report.Recurrence.OccurrenceCount)
		fmt.Printf("  Periodicity:       %s (Coefficient of Variation: %.2f)\n",
			report.Recurrence.Periodicity, report.Recurrence.CoefficientOfVariation)
		fmt.Printf("  Flapping Status:   %v\n", report.Recurrence.IsFlapping)
		if report.Recurrence.AverageInterval > 0 {
			fmt.Printf("  Average Interval:  %s (Median: %s, StdDev: %s)\n",
				report.Recurrence.AverageInterval.Round(time.Second),
				report.Recurrence.MedianInterval.Round(time.Second),
				report.Recurrence.StandardDeviation.Round(time.Second),
			)
		}
	}

	// Similar incidents
	if len(report.SimilarIncidents) > 0 {
		fmt.Printf("\n👥 Historical Similar Incidents (%d matches):\n", len(report.SimilarIncidents))
		tw := util.NewTableWriter("Incident ID", "Similarity", "Status", "Severity", "Started", "Title")
		for _, sim := range report.SimilarIncidents {
			tw.Append(
				sim.Incident.ID,
				fmt.Sprintf("%.1f%%", sim.SimilarityScore*100.0),
				string(sim.Incident.Status),
				string(sim.Incident.Severity),
				sim.Incident.StartTime.Local().Format("2006-01-02 15:04"),
				sim.Incident.Title,
			)
		}
		tw.Render(os.Stdout)
	}

	// Timeline events
	if len(report.TimelineHighlights) > 0 {
		fmt.Printf("\n⏱️ Chronological Timeline Highlights (%d events):\n", len(report.TimelineHighlights))
		tw := util.NewTableWriter("Timestamp", "Severity", "Event Type", "Node ID", "Source", "Title")
		for _, e := range report.TimelineHighlights {
			nodeID := e.NodeID
			if nodeID == "" {
				nodeID = "-"
			}
			tw.Append(
				e.Timestamp.Local().Format("15:04:05"),
				string(e.Severity),
				string(e.EventType),
				nodeID,
				e.Source,
				e.Title,
			)
		}
		tw.Render(os.Stdout)
	}

	// Findings and recommendations
	if len(report.Findings) > 0 {
		fmt.Printf("\n💡 Intelligence Findings & Action Items (%d):\n", len(report.Findings))
		for i, f := range report.Findings {
			fmt.Printf("  %d. [%s] %s\n", i+1, f.Severity, f.Title)
			fmt.Printf("     Description:    %s\n", f.Description)
			if len(f.NonInvasiveSuggestions) > 0 {
				fmt.Printf("     Advisory:       %s\n", strings.Join(f.NonInvasiveSuggestions, "; "))
			}
		}
	}

	return nil
}

func runIncidentStatus(cmd *cobra.Command, args []string) error {
	client, err := getIncidentClient()
	if err != nil {
		return err
	}

	incidentID := args[0]
	newStatusStr := args[1]
	reason := incReason
	if len(args) >= 3 && reason == "" {
		reason = args[2]
	}

	newStatus := incidents.IncidentStatus(strings.ToLower(newStatusStr))
	if !incidents.IsValidStatus(newStatus) {
		return NewExitError(ExitUsageError, "invalid incident status '%s' (allowed: %s)",
			newStatusStr, strings.Join([]string{
				string(incidents.IncidentStatusDetected),
				string(incidents.IncidentStatusAcknowledged),
				string(incidents.IncidentStatusInvestigating),
				string(incidents.IncidentStatusResolved),
				string(incidents.IncidentStatusSuppressed),
				string(incidents.IncidentStatusReopened),
			}, ", "))
	}

	ctx, cancel := context.WithTimeout(context.Background(), incTimeout)
	defer cancel()

	updated, err := client.UpdateStatus(ctx, incidentID, newStatus, reason)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to update status for incident %s: %w", incidentID, err)
	}

	if incFormat == "json" || incFormat == "yaml" || incFormat == "yml" {
		return outputIncidentFormatted(updated)
	}

	fmt.Printf("✓ Successfully transitioned incident '%s' status to '%s'\n", incidentID, updated.Status)
	if reason != "" {
		fmt.Printf("  Reason: %s\n", reason)
	}
	fmt.Printf("  Updated At: %s\n", updated.UpdatedAt.Local().Format(time.RFC3339))
	return nil
}

func runIncidentSimilar(cmd *cobra.Command, args []string) error {
	client, err := getIncidentClient()
	if err != nil {
		return err
	}

	incidentID := args[0]
	minSimilarity := incMinSimilarity
	if minSimilarity < 0.0 || minSimilarity > 1.0 {
		minSimilarity = 0.3
	}
	limit := incLimit
	if limit <= 0 {
		limit = 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), incTimeout)
	defer cancel()

	resp, err := client.GetSimilar(ctx, incidentID, minSimilarity, limit)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to find similar incidents for %s: %w", incidentID, err)
	}

	if incFormat == "json" || incFormat == "yaml" || incFormat == "yml" {
		return outputIncidentFormatted(resp)
	}

	fmt.Printf("🔍 Multi-Factor Jaccard Similarity Analysis for Incident: %s\n", incidentID)
	fmt.Printf("Minimum Similarity Cutoff: %.1f%% | Limit: %d\n", resp.MinSimilarityCut*100.0, limit)
	fmt.Println(strings.Repeat("=", 80))

	if len(resp.SimilarIncidents) == 0 {
		fmt.Println("No historical incidents exceeded the similarity threshold.")
		return nil
	}

	tw := util.NewTableWriter("Similar ID", "Similarity", "Symptom", "Subsystem", "Node", "Severity", "Title")
	for _, sim := range resp.SimilarIncidents {
		simScoreStr := fmt.Sprintf("%.1f%%", sim.SimilarityScore*100.0)
		symptomScoreStr := fmt.Sprintf("%.0f%%", math.Round(sim.Breakdown.SymptomSimilarity*100.0))
		subsysScoreStr := fmt.Sprintf("%.0f%%", math.Round(sim.Breakdown.SubsystemSimilarity*100.0))
		nodeScoreStr := fmt.Sprintf("%.0f%%", math.Round(sim.Breakdown.NodeSimilarity*100.0))
		sevScoreStr := fmt.Sprintf("%.0f%%", math.Round(sim.Breakdown.SeveritySimilarity*100.0))

		tw.Append(
			sim.Incident.ID,
			simScoreStr,
			symptomScoreStr,
			subsysScoreStr,
			nodeScoreStr,
			sevScoreStr,
			sim.Incident.Title,
		)
	}
	tw.Render(os.Stdout)
	fmt.Printf("\nTotal Matching Similar Incidents: %d\n", resp.Count)

	return nil
}
