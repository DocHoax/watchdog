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
	"github.com/DocHoax/watchdog/internal/topology"
	"github.com/DocHoax/watchdog/pkg/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	topoServerURL      string
	topoToken          string
	topoTokenFile      string
	topoTokenEnv       string
	topoInsecureTLS    bool
	topoTimeout        time.Duration
	topoFormat         string
	topoTypes          []string
	topoStatuses       []string
	topoSource         string
	topoHostID         string
	topoSearch         string
	topoMaxDepth       int
	topoTags           []string
	topoMinCriticality float64
	topoOutFile        string

	// Node declaration flags
	topoNodeID       string
	topoNodeName     string
	topoNodeType     string
	topoNodeStatus   string
	topoNodeHostID   string
	topoNodeTags     []string
	topoNodeMetadata []string

	// Dependency declaration flags
	topoDepSource     string
	topoDepTarget     string
	topoDepType       string
	topoDepConfidence string
	topoDepWeight     float64
	topoDepDependents bool
)

var topologyCmd = &cobra.Command{
	Use:     "topology [command]",
	Aliases: []string{"topo", "graph"},
	Short:   "Query, visualize, analyze, and manage service topology and dependencies",
	Long: `Provides comprehensive service topology mapping, dependency intelligence,
single point of failure (SPOF) resilience analysis, blast radius impact evaluation,
and Graphviz DOT visualization.

All topology discovery operates under strict passive observation principles without
invasive network scanning or host mutation.`,
	Example: `  # View topology graph summary
  watchdog topology summary

  # List full topology graph
  watchdog topology graph

  # Filter topology by node type and status
  watchdog topology graph --type service,database --status healthy

  # Export topology to Graphviz DOT format
  watchdog topology dot --out topology.dot

  # Find shortest dependency path between two services
  watchdog topology path web-frontend auth-db

  # List all Single Points of Failure (SPOFs)
  watchdog topology spof --min-criticality 0.5

  # Analyze upstream blast radius of an outage on a node
  watchdog topology impact redis-cache`,
}

var topologyGraphCmd = &cobra.Command{
	Use:     "graph [flags]",
	Aliases: []string{"list", "show", "get"},
	Short:   "Display topology nodes and dependency relationships",
	Long:    "Retrieves and displays topology nodes and dependency edges matching specified filters.",
	Example: `  watchdog topology graph
  watchdog topology graph --type service --status healthy
  watchdog topology graph --search "redis" --format json`,
	RunE: runTopologyGraph,
}

var topologySummaryCmd = &cobra.Command{
	Use:     "summary [flags]",
	Aliases: []string{"stats", "overview"},
	Short:   "Display high-level topology summary statistics",
	Long:    "Displays node counts, type distributions, status breakdown, edge counts, and connectivity metrics.",
	Example: `  watchdog topology summary
  watchdog topology summary --format yaml`,
	RunE: runTopologySummary,
}

var topologyDOTCmd = &cobra.Command{
	Use:     "dot [flags]",
	Aliases: []string{"export-dot", "graphviz"},
	Short:   "Export topology graph in Graphviz DOT format",
	Long:    "Generates and exports the complete directed dependency graph in standard Graphviz DOT notation.",
	Example: `  watchdog topology dot
  watchdog topology dot --out cluster-topology.dot`,
	RunE: runTopologyDOT,
}

var topologyPathCmd = &cobra.Command{
	Use:     "path <source-id> <target-id> [flags]",
	Aliases: []string{"route", "trace"},
	Short:   "Find shortest dependency path between two nodes",
	Long:    "Performs breadth-first search to find the shortest directed dependency path between source and target nodes.",
	Args:    cobra.ExactArgs(2),
	Example: `  watchdog topology path web-frontend db-primary
  watchdog topology path api-gateway payment-svc --format json`,
	RunE: runTopologyPath,
}

var topologySPOFCmd = &cobra.Command{
	Use:     "spof [node-id] [flags]",
	Aliases: []string{"bottlenecks", "criticality"},
	Short:   "Identify Single Points of Failure and resilience risks",
	Long: `Evaluates structural centrality, downstream dependent count, articulation points,
and bridge dependencies to identify Single Points of Failure and resilience vulnerabilities.`,
	Args: cobra.MaximumNArgs(1),
	Example: `  # List all SPOFs with criticality >= 0.3
  watchdog topology spof --min-criticality 0.3

  # Inspect SPOF details and recommendations for a specific node
  watchdog topology spof api-gateway`,
	RunE: runTopologySPOF,
}

var topologyImpactCmd = &cobra.Command{
	Use:     "impact <node-id> [flags]",
	Aliases: []string{"blast-radius", "blast"},
	Short:   "Assess upstream blast radius of node degradation or outage",
	Long:    "Calculates direct and transitive upstream dependents that would be affected if the target node experiences an outage.",
	Args:    cobra.ExactArgs(1),
	Example: `  watchdog topology impact db-primary
  watchdog topology impact auth-service --format json`,
	RunE: runTopologyImpact,
}

var topologyNodesCmd = &cobra.Command{
	Use:     "nodes [command]",
	Aliases: []string{"node"},
	Short:   "Manage and inspect individual topology nodes",
	Long:    "Provides subcommands to list, inspect, declare, and delete topology nodes.",
}

var topologyNodesListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List topology nodes",
	RunE:    runTopologyGraph,
}

var topologyNodesGetCmd = &cobra.Command{
	Use:     "get <node-id> [flags]",
	Aliases: []string{"show", "inspect"},
	Short:   "Get details for a specific topology node",
	Args:    cobra.ExactArgs(1),
	Example: `  watchdog topology nodes get web-srv`,
	RunE:    runTopologyNodeGet,
}

var topologyNodesAddCmd = &cobra.Command{
	Use:     "add [flags]",
	Aliases: []string{"create", "declare"},
	Short:   "Declare a new topology node",
	Example: `  watchdog topology nodes add --id redis-cache --name "Redis Cache" --type database --status healthy --host-id host-01 --tag env=prod`,
	RunE:    runTopologyNodeAdd,
}

var topologyNodesDeleteCmd = &cobra.Command{
	Use:     "delete <node-id> [flags]",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a declared topology node",
	Args:    cobra.ExactArgs(1),
	Example: `  watchdog topology nodes delete redis-cache`,
	RunE:    runTopologyNodeDelete,
}

var topologyDependenciesCmd = &cobra.Command{
	Use:     "dependencies [command]",
	Aliases: []string{"deps", "edges"},
	Short:   "Inspect and manage dependency edges between nodes",
	Long:    "Provides subcommands to list downstream dependencies/dependents, declare edges, and remove edges.",
}

var topologyDependenciesListCmd = &cobra.Command{
	Use:     "list <node-id> [flags]",
	Aliases: []string{"ls", "show"},
	Short:   "List dependencies (or dependents) for a node",
	Args:    cobra.ExactArgs(1),
	Example: `  # List downstream dependencies that web-srv calls
  watchdog topology dependencies list web-srv

  # List upstream dependents that rely on db-srv
  watchdog topology dependencies list db-srv --dependents`,
	RunE: runTopologyDependenciesList,
}

var topologyDependenciesAddCmd = &cobra.Command{
	Use:     "add [flags]",
	Aliases: []string{"create", "declare"},
	Short:   "Declare a dependency relationship between two nodes",
	Example: `  watchdog topology dependencies add --source web-srv --target api-srv --type communicates_with`,
	RunE:    runTopologyDependencyAdd,
}

var topologyDependenciesDeleteCmd = &cobra.Command{
	Use:     "delete [flags]",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a dependency relationship between two nodes",
	Example: `  watchdog topology dependencies delete --source web-srv --target api-srv`,
	RunE:    runTopologyDependencyDelete,
}

func init() {
	// Persistent flags on topologyCmd
	topologyCmd.PersistentFlags().StringVar(&topoServerURL, "server", "", "centralized fleet server URL (e.g. https://fleet.internal:8443)")
	topologyCmd.PersistentFlags().StringVar(&topoToken, "token", "", "bearer token for fleet server authentication")
	topologyCmd.PersistentFlags().StringVar(&topoTokenFile, "token-file", "", "path to file containing bearer token")
	topologyCmd.PersistentFlags().StringVar(&topoTokenEnv, "token-env", "", "environment variable containing bearer token")
	topologyCmd.PersistentFlags().BoolVar(&topoInsecureTLS, "insecure-tls", false, "skip TLS certificate validation (development only)")
	topologyCmd.PersistentFlags().DurationVar(&topoTimeout, "timeout", 10*time.Second, "HTTP request timeout duration")
	topologyCmd.PersistentFlags().StringVarP(&topoFormat, "format", "f", "text", "output format (text, json, yaml)")

	// Graph command flags
	topologyGraphCmd.Flags().StringSliceVar(&topoTypes, "type", nil, "filter by node type (physical_host, vm, container, k8s_pod, service, database, cache, queue)")
	topologyGraphCmd.Flags().StringSliceVar(&topoStatuses, "status", nil, "filter by node status (healthy, degraded, critical, offline, unknown)")
	topologyGraphCmd.Flags().StringVar(&topoSource, "source", "", "filter by discovery source (telemetry, docker, port, declared)")
	topologyGraphCmd.Flags().StringVar(&topoHostID, "host-id", "", "filter by host ID")
	topologyGraphCmd.Flags().StringVarP(&topoSearch, "search", "q", "", "search keyword in node ID, name, or host ID")
	topologyGraphCmd.Flags().IntVar(&topoMaxDepth, "max-depth", 0, "maximum traversal depth from matching nodes")
	topologyGraphCmd.Flags().StringSliceVar(&topoTags, "tag", nil, "filter by tags (key=value)")

	// DOT export flags
	topologyDOTCmd.Flags().StringVarP(&topoOutFile, "out", "o", "", "file path to save exported DOT notation (default stdout)")

	// SPOF flags
	topologySPOFCmd.Flags().Float64Var(&topoMinCriticality, "min-criticality", 0.0, "minimum criticality threshold filter (0.0 to 1.0)")

	// Node declaration flags
	topologyNodesAddCmd.Flags().StringVar(&topoNodeID, "id", "", "unique node identifier (required)")
	topologyNodesAddCmd.Flags().StringVar(&topoNodeName, "name", "", "human-readable node name")
	topologyNodesAddCmd.Flags().StringVar(&topoNodeType, "type", "service", "node type (physical_host, vm, container, k8s_pod, service, database, cache, queue)")
	topologyNodesAddCmd.Flags().StringVar(&topoNodeStatus, "status", "healthy", "initial node status (healthy, degraded, critical, offline, unknown)")
	topologyNodesAddCmd.Flags().StringVar(&topoNodeHostID, "host-id", "", "parent host ID")
	topologyNodesAddCmd.Flags().StringSliceVar(&topoNodeTags, "tag", nil, "key=value tags")
	topologyNodesAddCmd.Flags().StringSliceVar(&topoNodeMetadata, "meta", nil, "key=value metadata pairs")
	_ = topologyNodesAddCmd.MarkFlagRequired("id")

	// Dependency list flags
	topologyDependenciesListCmd.Flags().BoolVar(&topoDepDependents, "dependents", false, "list upstream dependents instead of downstream dependencies")

	// Dependency declaration flags
	topologyDependenciesAddCmd.Flags().StringVar(&topoDepSource, "source", "", "source node ID (required)")
	topologyDependenciesAddCmd.Flags().StringVar(&topoDepTarget, "target", "", "target node ID (required)")
	topologyDependenciesAddCmd.Flags().StringVar(&topoDepType, "type", "depends_on", "relationship type (depends_on, communicates_with, contains, runs_on, hosts, stores_on, reads_from, writes_to, routes_to, member_of)")
	topologyDependenciesAddCmd.Flags().StringVar(&topoDepConfidence, "confidence", "high", "confidence level (high, medium, low, unknown)")
	topologyDependenciesAddCmd.Flags().Float64Var(&topoDepWeight, "weight", 1.0, "edge weight for path calculations")
	_ = topologyDependenciesAddCmd.MarkFlagRequired("source")
	_ = topologyDependenciesAddCmd.MarkFlagRequired("target")

	// Dependency delete flags
	topologyDependenciesDeleteCmd.Flags().StringVar(&topoDepSource, "source", "", "source node ID (required)")
	topologyDependenciesDeleteCmd.Flags().StringVar(&topoDepTarget, "target", "", "target node ID (required)")
	topologyDependenciesDeleteCmd.Flags().StringVar(&topoDepType, "type", "", "relationship type filter")
	_ = topologyDependenciesDeleteCmd.MarkFlagRequired("source")
	_ = topologyDependenciesDeleteCmd.MarkFlagRequired("target")

	// Assemble node subcommands
	topologyNodesCmd.AddCommand(
		topologyNodesListCmd,
		topologyNodesGetCmd,
		topologyNodesAddCmd,
		topologyNodesDeleteCmd,
	)

	// Assemble dependency subcommands
	topologyDependenciesCmd.AddCommand(
		topologyDependenciesListCmd,
		topologyDependenciesAddCmd,
		topologyDependenciesDeleteCmd,
	)

	// Assemble top-level topology subcommands
	topologyCmd.AddCommand(
		topologyGraphCmd,
		topologySummaryCmd,
		topologyDOTCmd,
		topologyPathCmd,
		topologySPOFCmd,
		topologyImpactCmd,
		topologyNodesCmd,
		topologyDependenciesCmd,
	)

	RootCmd.AddCommand(topologyCmd)
}

func getTopologyClient() (*topology.Client, error) {
	serverURL := topoServerURL
	if serverURL == "" && globalCfg != nil {
		serverURL = globalCfg.Fleet.ServerURL
	}
	if serverURL == "" {
		return nil, NewExitError(ExitConfigError, "fleet server URL is required (specify via --server flag or fleet.server_url in config)")
	}

	token := topoToken
	tokenFile := topoTokenFile
	tokenEnv := topoTokenEnv
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

	var tlsCfg *tls.Config
	if topoInsecureTLS {
		tlsCfg = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}

	client := topology.NewClient(topology.ClientConfig{
		Endpoint:  serverURL,
		Token:     resolvedToken,
		Timeout:   topoTimeout,
		TLSConfig: tlsCfg,
	})
	return client, nil
}

func buildTopologyFilter() topology.TopologyFilter {
	filter := topology.TopologyFilter{
		Source:    topoSource,
		HostID:    topoHostID,
		Search:    topoSearch,
		MaxDepth:  topoMaxDepth,
		TagFilter: make(map[string]string),
	}
	for _, t := range topoTypes {
		filter.Types = append(filter.Types, topology.NodeType(strings.TrimSpace(t)))
	}
	for _, s := range topoStatuses {
		filter.Statuses = append(filter.Statuses, topology.NodeStatus(strings.TrimSpace(s)))
	}
	for _, kv := range topoTags {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			filter.TagFilter[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return filter
}

func outputTopologyFormatted(v any) error {
	switch strings.ToLower(topoFormat) {
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

func runTopologyGraph(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	filter := buildTopologyFilter()
	resp, err := client.GetTopology(ctx, filter)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get topology graph: %w", err)
	}

	if topoFormat == "json" || topoFormat == "yaml" || topoFormat == "yml" {
		return outputTopologyFormatted(resp)
	}

	if len(resp.Nodes) == 0 {
		fmt.Println("No topology nodes found matching the specified filters.")
		return nil
	}

	fmt.Printf("🗺️  Topology Graph (%d Nodes, %d Dependencies)\n\n", len(resp.Nodes), len(resp.Dependencies))
	fmt.Println("Nodes:")
	tw := util.NewTableWriter("ID", "Name", "Type", "Status", "Host ID", "Source", "Tags")
	for _, n := range resp.Nodes {
		var tagStrs []string
		for k, v := range n.Tags {
			tagStrs = append(tagStrs, fmt.Sprintf("%s=%s", k, v))
		}
		name := n.Name
		if name == "" {
			name = "-"
		}
		host := n.HostID
		if host == "" {
			host = "-"
		}
		tw.Append(
			n.ID,
			name,
			string(n.Type),
			string(n.Status),
			host,
			n.Source,
			strings.Join(tagStrs, ", "),
		)
	}
	tw.Render(os.Stdout)

	if len(resp.Dependencies) > 0 {
		fmt.Printf("\nDependencies (%d):\n", len(resp.Dependencies))
		dw := util.NewTableWriter("Source Node", "Relationship", "Target Node", "Confidence", "Weight")
		for _, d := range resp.Dependencies {
			dw.Append(
				d.SourceID,
				string(d.Type),
				d.TargetID,
				string(d.Confidence),
				fmt.Sprintf("%.1f", d.Weight),
			)
		}
		dw.Render(os.Stdout)
	}

	return nil
}

func runTopologySummary(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	summary, err := client.GetTopologySummary(ctx)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get topology summary: %w", err)
	}

	if topoFormat == "json" || topoFormat == "yaml" || topoFormat == "yml" {
		return outputTopologyFormatted(summary)
	}

	fmt.Println("📊 Topology Graph Summary")
	fmt.Printf("  Total Nodes:          %d\n", summary.TotalNodes)
	fmt.Printf("  Total Dependencies:   %d\n", summary.TotalDependencies)
	fmt.Printf("  Graph Density:        %.4f\n", summary.Density)
	fmt.Printf("  Connected Components: %d\n", summary.ConnectedComponents)
	fmt.Printf("  Last Evaluated:       %s\n", summary.EvaluatedAt.Local().Format(time.RFC3339))

	if len(summary.NodeTypeCounts) > 0 {
		fmt.Println("\nNodes by Type:")
		for t, cnt := range summary.NodeTypeCounts {
			fmt.Printf("  %-20s %d\n", t+":", cnt)
		}
	}

	if len(summary.StatusCounts) > 0 {
		fmt.Println("\nNodes by Status:")
		for s, cnt := range summary.StatusCounts {
			fmt.Printf("  %-20s %d\n", s+":", cnt)
		}
	}

	if len(summary.RelationshipTypeCounts) > 0 {
		fmt.Println("\nDependencies by Relationship:")
		for r, cnt := range summary.RelationshipTypeCounts {
			fmt.Printf("  %-20s %d\n", r+":", cnt)
		}
	}

	return nil
}

func runTopologyDOT(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	dotStr, err := client.ExportDOT(ctx)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to export DOT graph: %w", err)
	}

	if topoOutFile != "" {
		if err := os.WriteFile(topoOutFile, []byte(dotStr), 0600); err != nil {
			return NewExitError(ExitGeneralError, "failed to write DOT file %q: %w", topoOutFile, err)
		}
		fmt.Printf("Graphviz DOT notation written to %q (%d bytes).\n", topoOutFile, len(dotStr))
		return nil
	}

	fmt.Print(dotStr)
	return nil
}

func runTopologyPath(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	sourceID := args[0]
	targetID := args[1]

	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	path, err := client.FindPath(ctx, sourceID, targetID)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to find path: %w", err)
	}

	if topoFormat == "json" || topoFormat == "yaml" || topoFormat == "yml" {
		return outputTopologyFormatted(path)
	}

	var nodeNames []string
	for _, n := range path.Nodes {
		nodeNames = append(nodeNames, n.ID)
	}

	fmt.Printf("🛤️  Shortest Dependency Path: %s ➔ %s\n", sourceID, targetID)
	fmt.Printf("  Length (Hops):   %d\n", path.Hops)
	fmt.Printf("  Total Weight:    %.2f\n", path.TotalWeight)
	fmt.Printf("  Route:           %s\n\n", strings.Join(nodeNames, " ➔ "))

	if len(path.Edges) > 0 {
		tw := util.NewTableWriter("Hop", "Source", "Relationship", "Target", "Confidence", "Weight")
		for i, e := range path.Edges {
			tw.Append(
				strconv.Itoa(i+1),
				e.SourceID,
				string(e.Type),
				e.TargetID,
				string(e.Confidence),
				fmt.Sprintf("%.1f", e.Weight),
			)
		}
		tw.Render(os.Stdout)
	}

	return nil
}

func runTopologySPOF(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	if len(args) == 1 {
		nodeID := args[0]
		spof, err := client.AnalyzeSPOF(ctx, nodeID)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to analyze SPOF for node %q: %w", nodeID, err)
		}

		if topoFormat == "json" || topoFormat == "yaml" || topoFormat == "yml" {
			return outputTopologyFormatted(spof)
		}

		fmt.Printf("🔍 SPOF Analysis for Node: %s (%s)\n", spof.NodeID, spof.NodeName)
		fmt.Printf("  Criticality Score:          %.2f / 1.00\n", spof.CriticalityScore)
		fmt.Printf("  Risk Level:                 %s\n", spof.RiskLevel)
		fmt.Printf("  Redundancy Level:           %s\n", spof.RedundancyLevel)
		fmt.Printf("  Direct Dependents:          %d\n", spof.DependentsCount)
		fmt.Printf("  Transitive Dependents:      %d\n", spof.TransitiveDependentsCount)
		fmt.Printf("  Alternative Paths:          %d\n", spof.AlternativePaths)
		if len(spof.AffectedServices) > 0 {
			fmt.Printf("  Affected Services:          %s\n", strings.Join(spof.AffectedServices, ", "))
		}

		if len(spof.Reasoning) > 0 {
			fmt.Printf("\n🛡️  Structural Assessment & Reasoning:\n")
			for _, r := range spof.Reasoning {
				fmt.Printf("  • %s\n", r)
			}
		}
		return nil
	}

	spofs, err := client.FindAllSPOFs(ctx, topoMinCriticality)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to retrieve SPOF list: %w", err)
	}

	if topoFormat == "json" || topoFormat == "yaml" || topoFormat == "yml" {
		return outputTopologyFormatted(spofs)
	}

	if len(spofs) == 0 {
		fmt.Printf("No single points of failure detected matching criticality >= %.2f.\n", topoMinCriticality)
		return nil
	}

	fmt.Printf("⚠️  Single Points of Failure (%d Detected, Min Criticality: %.2f)\n", len(spofs), topoMinCriticality)
	tw := util.NewTableWriter("Node ID", "Name", "Type", "Status", "Criticality", "Risk", "Redundancy", "Direct Deps", "Transitive Deps")
	for _, s := range spofs {
		tw.Append(
			s.NodeID,
			s.NodeName,
			string(s.NodeType),
			string(s.Status),
			fmt.Sprintf("%.2f", s.CriticalityScore),
			string(s.RiskLevel),
			string(s.RedundancyLevel),
			strconv.Itoa(s.DependentsCount),
			strconv.Itoa(s.TransitiveDependentsCount),
		)
	}
	tw.Render(os.Stdout)
	return nil
}

func runTopologyImpact(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	nodeID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	impact, err := client.AnalyzeImpact(ctx, nodeID)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to evaluate impact for node %q: %w", nodeID, err)
	}

	if topoFormat == "json" || topoFormat == "yaml" || topoFormat == "yml" {
		return outputTopologyFormatted(impact)
	}

	fmt.Printf("💥 Upstream Blast Radius Analysis for Node: %s (%s)\n", impact.TargetNodeID, impact.TargetNodeName)
	fmt.Printf("  Target Status:              %s\n", impact.TargetNodeStatus)
	fmt.Printf("  Target Type:                %s\n", impact.TargetNodeType)
	fmt.Printf("  Evaluated At:               %s\n", impact.AnalyzedAt.Local().Format(time.RFC3339))
	fmt.Printf("  Direct Upstream Dependents: %d\n", len(impact.DirectDependents))
	fmt.Printf("  Total Blast Radius Nodes:   %d\n", len(impact.TransitiveDependents))
	fmt.Printf("  Max Propagation Depth:      %d\n", impact.MaxImpactDepth)
	fmt.Printf("  Blast Radius Score:         %.2f\n", impact.BlastRadiusScore)
	fmt.Printf("  Blast Radius Level:         %s\n", impact.BlastRadiusLevel)
	if impact.Summary != "" {
		fmt.Printf("  Summary:                    %s\n", impact.Summary)
	}

	if len(impact.TransitiveDependents) > 0 {
		fmt.Println("\nAffected Upstream Nodes in Blast Radius:")
		tw := util.NewTableWriter("Node ID", "Name", "Type", "Status", "Direct Dependent")
		for _, dep := range impact.TransitiveDependents {
			isDirect := false
			for _, d := range impact.DirectDependents {
				if d.ID == dep.ID {
					isDirect = true
					break
				}
			}
			tw.Append(
				dep.ID,
				dep.Name,
				string(dep.Type),
				string(dep.Status),
				fmt.Sprintf("%v", isDirect),
			)
		}
		tw.Render(os.Stdout)
	}

	return nil
}

func runTopologyNodeGet(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	nodeID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	node, err := client.GetNode(ctx, nodeID)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get node %q: %w", nodeID, err)
	}

	if topoFormat == "json" || topoFormat == "yaml" || topoFormat == "yml" {
		return outputTopologyFormatted(node)
	}

	fmt.Printf("📦 Topology Node: %s\n", node.ID)
	fmt.Printf("  Name:        %s\n", node.Name)
	fmt.Printf("  Type:        %s\n", node.Type)
	fmt.Printf("  Status:      %s\n", node.Status)
	fmt.Printf("  Host ID:     %s\n", node.HostID)
	fmt.Printf("  Source:      %s\n", node.Source)
	fmt.Printf("  First Seen:  %s\n", node.FirstObserved.Local().Format(time.RFC3339))
	fmt.Printf("  Last Seen:   %s\n", node.LastObserved.Local().Format(time.RFC3339))

	if len(node.Tags) > 0 {
		fmt.Println("  Tags:")
		for k, v := range node.Tags {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}
	if len(node.Metadata) > 0 {
		fmt.Println("  Metadata:")
		for k, v := range node.Metadata {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}

	return nil
}

func runTopologyNodeAdd(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	tags := make(map[string]string)
	for _, kv := range topoNodeTags {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			tags[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	metadata := make(map[string]string)
	for _, kv := range topoNodeMetadata {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			metadata[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	node := topology.TopologyNode{
		ID:       topoNodeID,
		Name:     topoNodeName,
		Type:     topology.NodeType(topoNodeType),
		Status:   topology.NodeStatus(topoNodeStatus),
		HostID:   topoNodeHostID,
		Source:   "declared",
		Tags:     tags,
		Metadata: metadata,
	}

	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	created, err := client.AddDeclaredNode(ctx, node)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to declare node: %w", err)
	}

	if topoFormat == "json" || topoFormat == "yaml" || topoFormat == "yml" {
		return outputTopologyFormatted(created)
	}

	fmt.Printf("Successfully declared node %q (Type: %s, Status: %s).\n", created.ID, created.Type, created.Status)
	return nil
}

func runTopologyNodeDelete(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	nodeID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	if err := client.RemoveNode(ctx, nodeID); err != nil {
		return NewExitError(ExitNetworkError, "failed to delete node %q: %w", nodeID, err)
	}

	fmt.Printf("Successfully removed node %q and its associated dependencies.\n", nodeID)
	return nil
}

func runTopologyDependenciesList(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	nodeID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	if topoDepDependents {
		dependents, err := client.GetDependents(ctx, nodeID)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get dependents for %q: %w", nodeID, err)
		}
		if topoFormat == "json" || topoFormat == "yaml" || topoFormat == "yml" {
			return outputTopologyFormatted(dependents)
		}
		if len(dependents) == 0 {
			fmt.Printf("No upstream dependents rely on node %q.\n", nodeID)
			return nil
		}
		fmt.Printf("⬆️  Upstream Dependents of Node %q (%d):\n", nodeID, len(dependents))
		tw := util.NewTableWriter("Node ID", "Name", "Type", "Status", "Host ID")
		for _, n := range dependents {
			tw.Append(n.ID, n.Name, string(n.Type), string(n.Status), n.HostID)
		}
		tw.Render(os.Stdout)
		return nil
	}

	dependencies, err := client.GetDependencies(ctx, nodeID)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to get dependencies for %q: %w", nodeID, err)
	}
	if topoFormat == "json" || topoFormat == "yaml" || topoFormat == "yml" {
		return outputTopologyFormatted(dependencies)
	}
	if len(dependencies) == 0 {
		fmt.Printf("Node %q has no downstream dependencies.\n", nodeID)
		return nil
	}
	fmt.Printf("⬇️  Downstream Dependencies of Node %q (%d):\n", nodeID, len(dependencies))
	tw := util.NewTableWriter("Node ID", "Name", "Type", "Status", "Host ID")
	for _, n := range dependencies {
		tw.Append(n.ID, n.Name, string(n.Type), string(n.Status), n.HostID)
	}
	tw.Render(os.Stdout)
	return nil
}

func runTopologyDependencyAdd(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	dep := topology.Dependency{
		SourceID:   topoDepSource,
		TargetID:   topoDepTarget,
		Type:       topology.RelationshipType(topoDepType),
		Confidence: topology.Confidence(topoDepConfidence),
		Weight:     topoDepWeight,
	}

	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	created, err := client.AddDeclaredDependency(ctx, dep)
	if err != nil {
		return NewExitError(ExitNetworkError, "failed to declare dependency: %w", err)
	}

	if topoFormat == "json" || topoFormat == "yaml" || topoFormat == "yml" {
		return outputTopologyFormatted(created)
	}

	fmt.Printf("Successfully declared dependency: %s ──[%s]──> %s\n", created.SourceID, created.Type, created.TargetID)
	return nil
}

func runTopologyDependencyDelete(cmd *cobra.Command, args []string) error {
	client, err := getTopologyClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()

	var relType topology.RelationshipType
	if topoDepType != "" {
		relType = topology.RelationshipType(topoDepType)
	}

	if err := client.RemoveDependency(ctx, topoDepSource, topoDepTarget, relType); err != nil {
		return NewExitError(ExitNetworkError, "failed to remove dependency: %w", err)
	}

	fmt.Printf("Successfully removed dependency: %s ➔ %s\n", topoDepSource, topoDepTarget)
	return nil
}
