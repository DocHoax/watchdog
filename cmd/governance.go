package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/governance"
	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/DocHoax/watchdog/pkg/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	govServerURL   string
	govToken       string
	govTokenFile   string
	govTokenEnv    string
	govInsecureTLS bool
	govTimeout     time.Duration
	govFormat      string
	govOrgID       string

	// Common filtering & pagination flags
	govLimit     int
	govOffset    int
	govSearch    string
	govSeverity  string
	govStatus    string
	govCategory  string
	govNodeID    string
	govGroupID   string
	govPolicyID  string
	govTargetTyp string
	govTargetID  string
	govStartTime string
	govEndTime   string
	govEnabled   string
	govAtTime    string
	govReason    string

	// Specific flags
	govName           string
	govDescription    string
	govTags           []string
	govParentID       string
	govAssignedBy     string
	govAuthor         string
	govChangelog      string
	govRulesFile      string
	govStagesFile     string
	govPoliciesFile   string
	govSourceFile     string
	govRevisionNum    int
	govSchemaVersion  string
	govScopeFilter    string
	govTriggerType    string
	govRuleID         string
	govDiffOnly       bool
	govTargetScope    string
	govWindowType     string
	govWindowStart    string
	govWindowEnd      string
	govDailyStart     string
	govDailyEnd       string
	govDaysOfWeek     []int
	govDayOfMonth     int
	govTimezone       string
	govAllowDisrupt   bool
	govCreatedBy      string
	govIncludeExpired bool
	govDuration       time.Duration
	govAcknowledged   bool
	govAlertID        string
	govAlertName      string
	govEventType      string
	govOutcome        string
	govActorType      string
	govActorID        string
	govAction         string
	govResourceType   string
	govResourceID     string
	govMessage        string
	govPolicyIDs      []string
	govNodeIDs        []string
	govGroupIDs       []string

	// Ownership flags
	govOwnerTeam    string
	govOwnerService string
	govOwnerEmail   string
	govOwnerSlack   string
	govOwnerPager   string
	govOwnerEscTier string
	govOwnerLifeStg string
	govOwnerCostCtr string
	govOwnerEnv     string
	govOwnerLead    string
)

// governanceCmd represents the base command for governance operations.
var governanceCmd = &cobra.Command{
	Use:     "governance [command]",
	Aliases: []string{"gov", "govs"},
	Short:   "Manage organizations, hierarchical fleet groups, policies, compliance, and operational governance",
	Long: `Provides administrative, policy management, compliance auditing, simulation,
and operational governance CLI capabilities for Watchdog.

Governance Capabilities:
  - Multi-Tenant Organization & Fleet Group Hierarchies
  - Declarative Policy Lifecycle, SHA-256 Revisions, and Assignments
  - Runtime Rule Evaluation, Findings Lifecycle, and Compliance Rollups
  - Isolated What-If Policy Simulation
  - Maintenance Windows & Safe Alert Suppression Guardrails
  - Hierarchical Node Ownership Metadata Cascading
  - Multi-Stage Incident Escalation Chains
  - Sanitized & Redacted Governance Audit Trail`,
	Example: `  # List all registered organizations
  watchdog governance org list

  # List fleet groups under the default organization
  watchdog governance group list

  # Check compliance summary for a specific node
  watchdog governance compliance node node-prod-01

  # Run isolated policy simulation against candidate policies
  watchdog governance simulate run --policies-file /tmp/candidate_policies.json

  # Evaluate whether an alert is currently suppressed by maintenance windows
  watchdog governance suppression evaluate --node-id node-01 --category disk_full --severity warning`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// -----------------------------------------------------------------------------
// Helper: Get GovernanceClient
// -----------------------------------------------------------------------------

func getGovernanceClient() (*governance.GovernanceClient, error) {
	serverURL := govServerURL
	if serverURL == "" && globalCfg != nil {
		serverURL = globalCfg.Fleet.ServerURL
	}
	if serverURL == "" {
		return nil, NewExitError(ExitConfigError, "server URL is required (specify via --server flag or fleet.server_url in config)")
	}

	token := govToken
	tokenFile := govTokenFile
	tokenEnv := govTokenEnv
	if token == "" && tokenFile == "" && tokenEnv == "" && globalCfg != nil {
		token = globalCfg.Fleet.Token
		tokenFile = globalCfg.Fleet.TokenFile
		tokenEnv = globalCfg.Fleet.TokenEnv
	}

	resolvedToken := token
	if resolvedToken == "" && tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			return nil, NewExitError(ExitConfigError, "failed to read token from file %q: %w", tokenFile, err)
		}
		resolvedToken = strings.TrimSpace(string(data))
	}
	if resolvedToken == "" && tokenEnv != "" {
		resolvedToken = os.Getenv(tokenEnv)
	}

	var tlsConfig *tls.Config
	if govInsecureTLS {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}

	timeout := govTimeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	return governance.NewGovernanceClient(governance.ClientConfig{
		Endpoint:  serverURL,
		Token:     resolvedToken,
		Timeout:   timeout,
		TLSConfig: tlsConfig,
	}), nil
}

func outputGovernanceFormatted(v any) error {
	switch strings.ToLower(govFormat) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case "yaml", "yml":
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(v)
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
}

func isGovFormatted() bool {
	f := strings.ToLower(govFormat)
	return f == "json" || f == "yaml" || f == "yml"
}

// -----------------------------------------------------------------------------
// 1. Organization Subcommands
// -----------------------------------------------------------------------------

var orgCmd = &cobra.Command{
	Use:     "org [command]",
	Aliases: []string{"orgs", "organization", "organizations"},
	Short:   "Manage multi-tenant organizations",
}

var orgListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List all organizations",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgs, err := client.ListOrganizations(ctx)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to list organizations: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(orgs)
		}
		if len(orgs) == 0 {
			fmt.Println("No organizations found.")
			return nil
		}
		tw := util.NewTableWriter("Org ID", "Name", "Description", "Created At", "Updated At")
		for _, o := range orgs {
			tw.Append(o.ID, o.Name, o.Description, o.CreatedAt.Format(time.RFC3339), o.UpdatedAt.Format(time.RFC3339))
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var orgGetCmd = &cobra.Command{
	Use:   "get <org_id>",
	Short: "Get organization details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		org, err := client.GetOrganization(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get organization %q: %w", args[0], err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(org)
		}
		fmt.Printf("🏢 Organization: %s (%s)\n", org.Name, org.ID)
		fmt.Printf("Description: %s\n", org.Description)
		fmt.Printf("Created At:  %s\n", org.CreatedAt.Format(time.RFC3339))
		fmt.Printf("Updated At:  %s\n", org.UpdatedAt.Format(time.RFC3339))
		if len(org.Tags) > 0 {
			fmt.Printf("Tags:        %s\n", strings.Join(org.Tags, ", "))
		}
		return nil
	},
}

var orgCreateCmd = &cobra.Command{
	Use:   "create <org_id> <name>",
	Short: "Create a new organization",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		req := model.Organization{
			ID:          args[0],
			Name:        args[1],
			Description: govDescription,
			Tags:        govTags,
		}
		org, err := client.CreateOrganization(ctx, req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to create organization: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(org)
		}
		fmt.Printf("✓ Created organization %q (%s)\n", org.Name, org.ID)
		return nil
	},
}

var orgUpdateCmd = &cobra.Command{
	Use:   "update <org_id>",
	Short: "Update an organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		req := model.Organization{
			Name:        govName,
			Description: govDescription,
			Tags:        govTags,
		}
		org, err := client.UpdateOrganization(ctx, args[0], req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to update organization: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(org)
		}
		fmt.Printf("✓ Updated organization %q (%s)\n", org.Name, org.ID)
		return nil
	},
}

var orgDeleteCmd = &cobra.Command{
	Use:     "delete <org_id>",
	Aliases: []string{"rm"},
	Short:   "Delete an organization",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		if err := client.DeleteOrganization(ctx, args[0]); err != nil {
			return NewExitError(ExitNetworkError, "failed to delete organization %q: %w", args[0], err)
		}
		fmt.Printf("✓ Deleted organization %q\n", args[0])
		return nil
	},
}

// -----------------------------------------------------------------------------
// 2. Fleet Group Subcommands
// -----------------------------------------------------------------------------

var groupCmd = &cobra.Command{
	Use:     "group [command]",
	Aliases: []string{"groups", "fleet-group", "fleet-groups"},
	Short:   "Manage hierarchical fleet groups",
}

var groupListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List fleet groups in an organization",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		groups, err := client.ListFleetGroups(ctx, orgID, govParentID)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to list fleet groups: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(groups)
		}
		if len(groups) == 0 {
			fmt.Println("No fleet groups found.")
			return nil
		}
		tw := util.NewTableWriter("Group ID", "Name", "Parent ID", "Org ID", "Description")
		for _, g := range groups {
			parent := g.ParentGroupID
			if parent == "" {
				parent = "<root>"
			}
			tw.Append(g.ID, g.Name, parent, g.OrgID, g.Description)
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var groupGetCmd = &cobra.Command{
	Use:   "get <group_id>",
	Short: "Get fleet group details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		g, err := client.GetFleetGroup(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get fleet group %q: %w", args[0], err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(g)
		}
		fmt.Printf("📁 Fleet Group: %s (%s)\n", g.Name, g.ID)
		fmt.Printf("Org ID:      %s\n", g.OrgID)
		if g.ParentGroupID != "" {
			fmt.Printf("Parent ID:   %s\n", g.ParentGroupID)
		}
		fmt.Printf("Description: %s\n", g.Description)
		fmt.Printf("Created At:  %s\n", g.CreatedAt.Format(time.RFC3339))
		if len(g.Tags) > 0 {
			fmt.Printf("Tags:        %s\n", strings.Join(g.Tags, ", "))
		}
		return nil
	},
}

var groupCreateCmd = &cobra.Command{
	Use:   "create <group_id> <name>",
	Short: "Create a new fleet group",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		req := model.FleetGroup{
			ID:            args[0],
			OrgID:         orgID,
			Name:          args[1],
			ParentGroupID: govParentID,
			Description:   govDescription,
			Tags:          govTags,
		}
		g, err := client.CreateFleetGroup(ctx, req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to create fleet group: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(g)
		}
		fmt.Printf("✓ Created fleet group %q (%s)\n", g.Name, g.ID)
		return nil
	},
}

var groupUpdateCmd = &cobra.Command{
	Use:   "update <group_id>",
	Short: "Update a fleet group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		req := model.FleetGroup{
			Name:          govName,
			ParentGroupID: govParentID,
			Description:   govDescription,
			Tags:          govTags,
		}
		g, err := client.UpdateFleetGroup(ctx, args[0], req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to update fleet group: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(g)
		}
		fmt.Printf("✓ Updated fleet group %q (%s)\n", g.Name, g.ID)
		return nil
	},
}

var groupDeleteCmd = &cobra.Command{
	Use:     "delete <group_id>",
	Aliases: []string{"rm"},
	Short:   "Delete a fleet group",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		if err := client.DeleteFleetGroup(ctx, args[0]); err != nil {
			return NewExitError(ExitNetworkError, "failed to delete fleet group %q: %w", args[0], err)
		}
		fmt.Printf("✓ Deleted fleet group %q\n", args[0])
		return nil
	},
}

var groupHierarchyCmd = &cobra.Command{
	Use:   "hierarchy [flags]",
	Short: "Display organizational fleet group hierarchy tree",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		nodes, err := client.GetHierarchy(ctx, orgID)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get group hierarchy: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(nodes)
		}
		if len(nodes) == 0 {
			fmt.Printf("No hierarchy found for organization %s.\n", orgID)
			return nil
		}
		fmt.Printf("🌳 Governance Fleet Group Hierarchy (Org: %s):\n", orgID)
		printHierarchyTree(nodes, 0)
		return nil
	},
}

func printHierarchyTree(nodes []model.GroupHierarchyNode, indent int) {
	for _, n := range nodes {
		prefix := strings.Repeat("  ", indent)
		fmt.Printf("%s└─ 📁 %s (%s) [Nodes: %d, Subgroups: %d]\n",
			prefix, n.Group.Name, n.Group.ID, len(n.DirectNodes), len(n.Children))
		for _, nodeID := range n.DirectNodes {
			fmt.Printf("%s   ├─ 🖥️ %s\n", prefix, nodeID)
		}
		if len(n.Children) > 0 {
			printHierarchyTree(n.Children, indent+1)
		}
	}
}

var groupMembersCmd = &cobra.Command{
	Use:   "members <group_id>",
	Short: "List direct node members in a group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		members, err := client.GetGroupMembers(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get group members: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(members)
		}
		fmt.Printf("Members of Group %s (%d nodes):\n", args[0], len(members))
		for _, m := range members {
			fmt.Printf("  • %s\n", m)
		}
		return nil
	},
}

var groupAddMemberCmd = &cobra.Command{
	Use:   "add-member <group_id> <node_id>",
	Short: "Add a node to a fleet group",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		if err := client.AddMember(ctx, args[0], args[1]); err != nil {
			return NewExitError(ExitNetworkError, "failed to add member: %w", err)
		}
		fmt.Printf("✓ Added node %q to group %q\n", args[1], args[0])
		return nil
	},
}

var groupRemoveMemberCmd = &cobra.Command{
	Use:   "remove-member <group_id> <node_id>",
	Short: "Remove a node from a fleet group",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		if err := client.RemoveMember(ctx, args[0], args[1]); err != nil {
			return NewExitError(ExitNetworkError, "failed to remove member: %w", err)
		}
		fmt.Printf("✓ Removed node %q from group %q\n", args[1], args[0])
		return nil
	},
}

var groupSubtreeCmd = &cobra.Command{
	Use:   "subtree <group_id>",
	Short: "List all descendant node members recursively in a group subtree",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		nodes, err := client.GetSubtreeNodes(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get subtree nodes: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(nodes)
		}
		fmt.Printf("Subtree Node Members for Group %s (%d nodes):\n", args[0], len(nodes))
		for _, n := range nodes {
			fmt.Printf("  • %s\n", n)
		}
		return nil
	},
}

// -----------------------------------------------------------------------------
// 3. Ownership Subcommands
// -----------------------------------------------------------------------------

var ownershipCmd = &cobra.Command{
	Use:     "ownership [command]",
	Aliases: []string{"owner"},
	Short:   "Manage and resolve hierarchical node ownership metadata",
}

var ownershipGetCmd = &cobra.Command{
	Use:   "get <node_id>",
	Short: "Get direct ownership metadata on a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		own, err := client.GetNodeOwnership(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get node ownership: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(own)
		}
		fmt.Printf("👤 Node Ownership: %s\n", args[0])
		fmt.Printf("Service:         %s\n", own.Service)
		fmt.Printf("Team:            %s\n", own.Team)
		fmt.Printf("Lead / Owner:    %s\n", own.Owner)
		fmt.Printf("Contact Email:   %s\n", own.ContactEmail)
		fmt.Printf("Slack Channel:   %s\n", own.SlackChannel)
		fmt.Printf("PagerDuty:       %s\n", own.PagerDutyService)
		fmt.Printf("Escalation Tier: %s\n", own.EscalationTier)
		fmt.Printf("Lifecycle Stage: %s\n", own.LifecycleStage)
		fmt.Printf("Cost Center:     %s\n", own.CostCenter)
		fmt.Printf("Environment:     %s\n", own.Environment)
		return nil
	},
}

var ownershipSetCmd = &cobra.Command{
	Use:   "set <node_id>",
	Short: "Set direct ownership metadata on a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		req := model.NodeOwnership{
			Service:          govOwnerService,
			Team:             govOwnerTeam,
			Owner:            govOwnerLead,
			ContactEmail:     govOwnerEmail,
			SlackChannel:     govOwnerSlack,
			PagerDutyService: govOwnerPager,
			EscalationTier:   govOwnerEscTier,
			LifecycleStage:   govOwnerLifeStg,
			CostCenter:       govOwnerCostCtr,
			Environment:      govOwnerEnv,
		}
		if err := client.SetNodeOwnership(ctx, args[0], req); err != nil {
			return NewExitError(ExitNetworkError, "failed to set node ownership: %w", err)
		}
		fmt.Printf("✓ Set ownership metadata for node %q\n", args[0])
		return nil
	},
}

var ownershipResolveCmd = &cobra.Command{
	Use:   "resolve <node_id>",
	Short: "Resolve cascading hierarchical ownership for a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		res, err := client.ResolveNodeOwnership(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to resolve ownership: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(res)
		}
		fmt.Printf("👤 Resolved Hierarchical Ownership for Node: %s\n", args[0])
		fmt.Printf("Service:         %s\n", res.Ownership.Service)
		fmt.Printf("Team:            %s\n", res.Ownership.Team)
		fmt.Printf("Lead / Owner:    %s\n", res.Ownership.Owner)
		fmt.Printf("Contact Email:   %s\n", res.Ownership.ContactEmail)
		fmt.Printf("Slack Channel:   %s\n", res.Ownership.SlackChannel)
		fmt.Printf("PagerDuty:       %s\n", res.Ownership.PagerDutyService)
		fmt.Printf("Escalation Tier: %s\n", res.Ownership.EscalationTier)
		fmt.Printf("Lifecycle Stage: %s\n", res.Ownership.LifecycleStage)
		fmt.Printf("Cost Center:     %s\n", res.Ownership.CostCenter)
		fmt.Printf("Environment:     %s\n", res.Ownership.Environment)
		if len(res.SourceLineage) > 0 {
			fmt.Printf("\nResolution Lineage Sources:\n")
			for k, src := range res.SourceLineage {
				fmt.Printf("  • %-16s -> %s\n", k, src)
			}
		}
		return nil
	},
}

// -----------------------------------------------------------------------------
// 4. Policy Subcommands
// -----------------------------------------------------------------------------

var policyCmd = &cobra.Command{
	Use:     "policy [command]",
	Aliases: []string{"policies", "pol"},
	Short:   "Manage declarative policies and revisions",
}

var policyListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List policies matching filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		filter := model.PolicyFilter{
			OrgID:    orgID,
			Category: govCategory,
			Status:   model.PolicyStatus(govStatus),
			Search:   govSearch,
		}
		policies, err := client.ListPolicies(ctx, filter)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to list policies: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(policies)
		}
		if len(policies) == 0 {
			fmt.Println("No policies found.")
			return nil
		}
		tw := util.NewTableWriter("Policy ID", "Name", "Category", "Severity", "Status", "Active Rev")
		for _, p := range policies {
			tw.Append(p.ID, p.Name, p.Category, string(p.Severity), string(p.Status), strconv.Itoa(p.ActiveRevision))
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var policyGetCmd = &cobra.Command{
	Use:   "get <policy_id>",
	Short: "Get policy details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		p, err := client.GetPolicy(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get policy %q: %w", args[0], err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(p)
		}
		fmt.Printf("📜 Policy: %s (%s)\n", p.Name, p.ID)
		fmt.Printf("Org ID:          %s\n", p.OrgID)
		fmt.Printf("Category:        %s\n", p.Category)
		fmt.Printf("Severity:        %s\n", p.Severity)
		fmt.Printf("Status:          %s\n", p.Status)
		fmt.Printf("Active Revision: %d\n", p.ActiveRevision)
		fmt.Printf("Description:     %s\n", p.Description)
		fmt.Printf("Created At:      %s\n", p.CreatedAt.Format(time.RFC3339))
		fmt.Printf("Updated At:      %s\n", p.UpdatedAt.Format(time.RFC3339))
		return nil
	},
}

var policyCreateCmd = &cobra.Command{
	Use:   "create <policy_id> <name>",
	Short: "Create a new policy",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		req := model.Policy{
			ID:            args[0],
			OrgID:         orgID,
			Name:          args[1],
			Category:      govCategory,
			Severity:      model.SeverityLevel(govSeverity),
			Description:   govDescription,
			SchemaVersion: govSchemaVersion,
		}
		if req.Severity == "" {
			req.Severity = model.SeverityWarning
		}
		p, err := client.CreatePolicy(ctx, req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to create policy: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(p)
		}
		fmt.Printf("✓ Created policy %q (%s)\n", p.Name, p.ID)
		return nil
	},
}

var policyUpdateCmd = &cobra.Command{
	Use:   "update <policy_id>",
	Short: "Update policy metadata",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		req := model.Policy{
			Name:        govName,
			Category:    govCategory,
			Severity:    model.SeverityLevel(govSeverity),
			Description: govDescription,
			Status:      model.PolicyStatus(govStatus),
		}
		p, err := client.UpdatePolicy(ctx, args[0], req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to update policy: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(p)
		}
		fmt.Printf("✓ Updated policy %q (%s)\n", p.Name, p.ID)
		return nil
	},
}

var policyDeleteCmd = &cobra.Command{
	Use:     "delete <policy_id>",
	Aliases: []string{"rm"},
	Short:   "Delete a policy",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		if err := client.DeletePolicy(ctx, args[0]); err != nil {
			return NewExitError(ExitNetworkError, "failed to delete policy %q: %w", args[0], err)
		}
		fmt.Printf("✓ Deleted policy %q\n", args[0])
		return nil
	},
}

var policyRevisionsCmd = &cobra.Command{
	Use:   "revisions <policy_id>",
	Short: "List revisions for a policy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		revs, err := client.ListPolicyRevisions(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to list revisions: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(revs)
		}
		if len(revs) == 0 {
			fmt.Printf("No revisions found for policy %s.\n", args[0])
			return nil
		}
		tw := util.NewTableWriter("Rev #", "SHA-256 Hash", "Rules", "Author", "Created At", "Changelog")
		for _, r := range revs {
			hashShort := r.ContentHash
			if len(hashShort) > 12 {
				hashShort = hashShort[:12]
			}
			tw.Append(strconv.Itoa(r.Revision), hashShort, strconv.Itoa(len(r.Rules)), r.Author, r.CreatedAt.Format(time.RFC3339), r.Changelog)
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var policyPublishRevCmd = &cobra.Command{
	Use:   "publish-revision <policy_id>",
	Short: "Publish a new revision with rules for a policy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		var rules []model.PolicyRule
		if govRulesFile != "" {
			data, err := os.ReadFile(govRulesFile)
			if err != nil {
				return NewExitError(ExitUsageError, "failed to read rules file %q: %w", govRulesFile, err)
			}
			if err := json.Unmarshal(data, &rules); err != nil {
				return NewExitError(ExitUsageError, "failed to parse rules JSON: %w", err)
			}
		}

		rev, err := client.PublishRevision(ctx, args[0], rules, govAuthor, govChangelog)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to publish revision: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(rev)
		}
		fmt.Printf("✓ Published revision %d for policy %s (SHA-256: %s, Rules: %d)\n",
			rev.Revision, rev.PolicyID, rev.ContentHash, len(rev.Rules))
		return nil
	},
}

var policySetStatusCmd = &cobra.Command{
	Use:   "set-status <policy_id> <status>",
	Short: "Set policy lifecycle status (draft, published, deprecated, archived)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		if err := client.SetPolicyStatus(ctx, args[0], model.PolicyStatus(args[1])); err != nil {
			return NewExitError(ExitNetworkError, "failed to set policy status: %w", err)
		}
		fmt.Printf("✓ Set policy %s status to %s\n", args[0], args[1])
		return nil
	},
}

var policySetActiveCmd = &cobra.Command{
	Use:   "set-active <policy_id> <revision_number>",
	Short: "Set the active revision number for a policy",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		revNum, err := strconv.Atoi(args[1])
		if err != nil {
			return NewExitError(ExitUsageError, "invalid revision number %q: %w", args[1], err)
		}
		if err := client.SetActiveRevision(ctx, args[0], revNum); err != nil {
			return NewExitError(ExitNetworkError, "failed to set active revision: %w", err)
		}
		fmt.Printf("✓ Set policy %s active revision to %d\n", args[0], revNum)
		return nil
	},
}

var policyConflictsCmd = &cobra.Command{
	Use:   "conflicts [flags]",
	Short: "Detect conflicting rule definitions across active policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		conflicts, err := client.DetectPolicyConflicts(ctx, orgID, govPolicyIDs)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to detect policy conflicts: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(conflicts)
		}
		if len(conflicts) == 0 {
			fmt.Println("✓ No policy conflicts detected.")
			return nil
		}
		fmt.Printf("⚠️ Detected %d Policy Conflicts:\n\n", len(conflicts))
		tw := util.NewTableWriter("Type", "Severity", "Policy A", "Policy B", "Rule / Field", "Description")
		for _, c := range conflicts {
			tw.Append(string(c.ConflictType), string(c.Severity), c.PolicyA, c.PolicyB, c.RuleOrField, c.Description)
		}
		tw.Render(os.Stdout)
		return nil
	},
}

// -----------------------------------------------------------------------------
// 5. Policy Assignment Subcommands
// -----------------------------------------------------------------------------

var assignmentCmd = &cobra.Command{
	Use:     "assignment [command]",
	Aliases: []string{"assignments", "assign"},
	Short:   "Manage policy assignments to nodes, groups, or organizations",
}

var assignmentListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List policy assignments matching filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		var enabledPtr *bool
		if govEnabled != "" {
			b, err := strconv.ParseBool(govEnabled)
			if err == nil {
				enabledPtr = &b
			}
		}
		filter := model.AssignmentFilter{
			OrgID:      orgID,
			PolicyID:   govPolicyID,
			TargetType: model.AssignmentTargetType(govTargetTyp),
			TargetID:   govTargetID,
			Enabled:    enabledPtr,
		}
		assignments, err := client.ListAssignments(ctx, filter)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to list assignments: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(assignments)
		}
		if len(assignments) == 0 {
			fmt.Println("No policy assignments found.")
			return nil
		}
		tw := util.NewTableWriter("Assignment ID", "Policy ID", "Target Type", "Target ID", "Enabled", "Assigned By")
		for _, a := range assignments {
			tw.Append(a.ID, a.PolicyID, string(a.TargetType), a.TargetID, strconv.FormatBool(a.Enabled), a.AssignedBy)
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var assignmentGetCmd = &cobra.Command{
	Use:   "get <assignment_id>",
	Short: "Get policy assignment details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		a, err := client.GetAssignment(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get assignment: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(a)
		}
		fmt.Printf("📌 Policy Assignment: %s\n", a.ID)
		fmt.Printf("Policy ID:    %s\n", a.PolicyID)
		fmt.Printf("Target Type:  %s\n", a.TargetType)
		fmt.Printf("Target ID:    %s\n", a.TargetID)
		fmt.Printf("Enabled:      %v\n", a.Enabled)
		fmt.Printf("Assigned By:  %s\n", a.AssignedBy)
		fmt.Printf("Created At:   %s\n", a.CreatedAt.Format(time.RFC3339))
		return nil
	},
}

var assignmentCreateCmd = &cobra.Command{
	Use:   "create <assignment_id> <policy_id> <target_type> <target_id>",
	Short: "Create a policy assignment (target_type: organization, fleet_group, node)",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		req := model.PolicyAssignment{
			ID:          args[0],
			OrgID:       orgID,
			PolicyID:    args[1],
			TargetType:  model.AssignmentTargetType(args[2]),
			TargetID:    args[3],
			Enabled:     true,
			AssignedBy:  govAssignedBy,
			ScopeFilter: govScopeFilter,
		}
		a, err := client.CreateAssignment(ctx, req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to create assignment: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(a)
		}
		fmt.Printf("✓ Assigned policy %s to %s %s (Assignment ID: %s)\n", a.PolicyID, a.TargetType, a.TargetID, a.ID)
		return nil
	},
}

var assignmentDeleteCmd = &cobra.Command{
	Use:     "delete <assignment_id>",
	Aliases: []string{"rm"},
	Short:   "Delete a policy assignment",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		if err := client.DeleteAssignment(ctx, args[0]); err != nil {
			return NewExitError(ExitNetworkError, "failed to delete assignment: %w", err)
		}
		fmt.Printf("✓ Deleted policy assignment %q\n", args[0])
		return nil
	},
}

var assignmentEnableCmd = &cobra.Command{
	Use:   "enable <assignment_id>",
	Short: "Enable or disable a policy assignment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		enabled := true
		if govEnabled != "" {
			b, err := strconv.ParseBool(govEnabled)
			if err == nil {
				enabled = b
			}
		}
		if err := client.SetAssignmentEnabled(ctx, args[0], enabled); err != nil {
			return NewExitError(ExitNetworkError, "failed to update assignment enabled state: %w", err)
		}
		fmt.Printf("✓ Set assignment %s enabled = %v\n", args[0], enabled)
		return nil
	},
}

// -----------------------------------------------------------------------------
// 6. Evaluation Subcommands
// -----------------------------------------------------------------------------

var evaluateCmd = &cobra.Command{
	Use:     "evaluate [command]",
	Aliases: []string{"eval"},
	Short:   "Execute runtime policy evaluation and query execution history",
}

var evalNodeCmd = &cobra.Command{
	Use:   "node <node_id>",
	Short: "Trigger runtime policy evaluation on a specific node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		trigger := model.TriggerManual
		if govTriggerType != "" {
			trigger = model.EvaluationTriggerType(govTriggerType)
		}
		exec, err := client.EvaluateNode(ctx, args[0], trigger)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to evaluate node: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(exec)
		}
		fmt.Printf("⚡ Evaluation Complete for Node %s (Execution ID: %s)\n", args[0], exec.ID)
		fmt.Printf("Status:             %s\n", exec.Status)
		fmt.Printf("Compliance Score:   %.1f%%\n", exec.ComplianceScore)
		fmt.Printf("Evaluated Policies: %d\n", exec.TotalPolicies)
		fmt.Printf("Passed Rules:       %d\n", exec.PassedRules)
		fmt.Printf("Failed Rules:       %d\n", exec.FailedRules)
		fmt.Printf("Findings Created:   %d\n", exec.FindingsCreated)
		return nil
	},
}

var evalGroupCmd = &cobra.Command{
	Use:   "group <group_id>",
	Short: "Trigger runtime policy evaluation across a fleet group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		trigger := model.TriggerManual
		if govTriggerType != "" {
			trigger = model.EvaluationTriggerType(govTriggerType)
		}
		execs, err := client.EvaluateFleetGroup(ctx, args[0], trigger)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to evaluate fleet group: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(execs)
		}
		fmt.Printf("⚡ Group Evaluation Complete (%d nodes evaluated):\n", len(execs))
		tw := util.NewTableWriter("Execution ID", "Node ID", "Status", "Score", "Policies", "Passed", "Failed")
		for _, e := range execs {
			tw.Append(e.ID, e.TargetID, string(e.Status), fmt.Sprintf("%.1f%%", e.ComplianceScore),
				strconv.Itoa(e.TotalPolicies), strconv.Itoa(e.PassedRules), strconv.Itoa(e.FailedRules))
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var evalOrgCmd = &cobra.Command{
	Use:   "org <org_id>",
	Short: "Trigger runtime policy evaluation across an entire organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		trigger := model.TriggerManual
		if govTriggerType != "" {
			trigger = model.EvaluationTriggerType(govTriggerType)
		}
		execs, err := client.EvaluateOrganization(ctx, args[0], trigger)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to evaluate organization: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(execs)
		}
		fmt.Printf("⚡ Org Evaluation Complete (%d nodes evaluated):\n", len(execs))
		tw := util.NewTableWriter("Execution ID", "Node ID", "Status", "Score", "Policies", "Passed", "Failed")
		for _, e := range execs {
			tw.Append(e.ID, e.TargetID, string(e.Status), fmt.Sprintf("%.1f%%", e.ComplianceScore),
				strconv.Itoa(e.TotalPolicies), strconv.Itoa(e.PassedRules), strconv.Itoa(e.FailedRules))
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var evalListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List historical evaluation execution records",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		filter := model.EvaluationFilter{
			OrgID:       orgID,
			TargetType:  model.AssignmentTargetType(govTargetTyp),
			TargetID:    govTargetID,
			TriggerType: model.EvaluationTriggerType(govTriggerType),
			Limit:       govLimit,
			Offset:      govOffset,
		}
		execs, err := client.ListEvaluationExecutions(ctx, filter)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to list evaluation executions: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(execs)
		}
		if len(execs) == 0 {
			fmt.Println("No evaluation execution history found.")
			return nil
		}
		tw := util.NewTableWriter("Execution ID", "Target ID", "Trigger", "Status", "Score", "Passed", "Failed", "Started At")
		for _, e := range execs {
			tw.Append(e.ID, e.TargetID, string(e.TriggerType), string(e.Status),
				fmt.Sprintf("%.1f%%", e.ComplianceScore), strconv.Itoa(e.PassedRules),
				strconv.Itoa(e.FailedRules), e.StartedAt.Format(time.RFC3339))
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var evalGetCmd = &cobra.Command{
	Use:   "get <execution_id>",
	Short: "Get evaluation execution details and rule results",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		exec, err := client.GetEvaluationExecution(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get evaluation execution: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(exec)
		}
		fmt.Printf("⚡ Evaluation Execution: %s\n", exec.ID)
		fmt.Printf("Target:           %s (%s)\n", exec.TargetID, exec.TargetType)
		fmt.Printf("Trigger:          %s\n", exec.TriggerType)
		fmt.Printf("Status:           %s\n", exec.Status)
		fmt.Printf("Score:            %.1f%%\n", exec.ComplianceScore)
		fmt.Printf("Passed / Failed:  %d / %d\n", exec.PassedRules, exec.FailedRules)
		fmt.Printf("Findings Created: %d (Resolved: %d)\n", exec.FindingsCreated, exec.FindingsResolved)
		fmt.Printf("Started At:       %s\n", exec.StartedAt.Format(time.RFC3339))
		if exec.CompletedAt != nil {
			fmt.Printf("Completed At:     %s (Duration: %s)\n", exec.CompletedAt.Format(time.RFC3339), exec.Duration)
		}
		if len(exec.RuleResults) > 0 {
			fmt.Printf("\nRule Evaluation Results (%d):\n", len(exec.RuleResults))
			tw := util.NewTableWriter("Rule ID", "Policy ID", "Status", "Severity", "Message")
			for _, r := range exec.RuleResults {
				tw.Append(r.RuleID, r.PolicyID, string(r.Status), string(r.Severity), r.Message)
			}
			tw.Render(os.Stdout)
		}
		return nil
	},
}

var evalLatestCmd = &cobra.Command{
	Use:   "latest <node_id>",
	Short: "Get latest evaluation execution for a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		exec, err := client.GetLatestNodeEvaluation(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get latest evaluation for node %s: %w", args[0], err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(exec)
		}
		fmt.Printf("⚡ Latest Evaluation for Node %s (Execution ID: %s)\n", args[0], exec.ID)
		fmt.Printf("Status:           %s\n", exec.Status)
		fmt.Printf("Compliance Score: %.1f%%\n", exec.ComplianceScore)
		fmt.Printf("Passed / Failed:  %d / %d\n", exec.PassedRules, exec.FailedRules)
		fmt.Printf("Started At:       %s\n", exec.StartedAt.Format(time.RFC3339))
		return nil
	},
}

// -----------------------------------------------------------------------------
// 7. Findings Subcommands
// -----------------------------------------------------------------------------

var findingsCmd = &cobra.Command{
	Use:     "findings [command]",
	Aliases: []string{"finding"},
	Short:   "Inspect and query compliance findings and violations",
}

var findingsListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List compliance findings matching filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		filter := model.FindingFilter{
			OrgID:    orgID,
			NodeID:   govNodeID,
			PolicyID: govPolicyID,
			RuleID:   govRuleID,
			Severity: model.SeverityLevel(govSeverity),
			Status:   model.FindingStatus(govStatus),
			Search:   govSearch,
			Limit:    govLimit,
			Offset:   govOffset,
		}
		findings, err := client.ListComplianceFindings(ctx, filter)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to list findings: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(findings)
		}
		if len(findings) == 0 {
			fmt.Println("No compliance findings found matching criteria.")
			return nil
		}
		tw := util.NewTableWriter("Finding ID", "Node ID", "Policy ID", "Rule ID", "Severity", "Status", "First Seen", "Message")
		for _, f := range findings {
			msgShort := f.Message
			if len(msgShort) > 35 {
				msgShort = msgShort[:32] + "..."
			}
			tw.Append(f.ID, f.NodeID, f.PolicyID, f.RuleID, string(f.Severity), string(f.Status), f.FirstSeenAt.Format("01/02 15:04"), msgShort)
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var findingsGetCmd = &cobra.Command{
	Use:   "get <finding_id>",
	Short: "Get compliance finding details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		f, err := client.GetComplianceFinding(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get finding %q: %w", args[0], err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(f)
		}
		fmt.Printf("🔍 Compliance Finding: %s\n", f.ID)
		fmt.Printf("Node ID:       %s\n", f.NodeID)
		fmt.Printf("Policy ID:     %s\n", f.PolicyID)
		fmt.Printf("Rule ID:       %s\n", f.RuleID)
		fmt.Printf("Severity:      %s\n", f.Severity)
		fmt.Printf("Status:        %s\n", f.Status)
		fmt.Printf("Message:       %s\n", f.Message)
		fmt.Printf("First Seen At: %s\n", f.FirstSeenAt.Format(time.RFC3339))
		fmt.Printf("Last Seen At:  %s\n", f.LastSeenAt.Format(time.RFC3339))
		if f.ResolvedAt != nil {
			fmt.Printf("Resolved At:   %s\n", f.ResolvedAt.Format(time.RFC3339))
		}
		if f.RemediationAdvice != "" {
			fmt.Printf("\n💡 Remediation Advice:\n  %s\n", f.RemediationAdvice)
		}
		return nil
	},
}

// -----------------------------------------------------------------------------
// 8. Compliance Subcommands
// -----------------------------------------------------------------------------

var complianceCmd = &cobra.Command{
	Use:     "compliance [command]",
	Aliases: []string{"comp"},
	Short:   "Query hierarchical compliance rollups and policy resolution",
}

var compNodeCmd = &cobra.Command{
	Use:   "node <node_id>",
	Short: "Get compliance posture summary for a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		summary, err := client.GetNodeComplianceSummary(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get node compliance summary: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(summary)
		}
		fmt.Printf("📊 Node Compliance Summary: %s\n", args[0])
		fmt.Printf("Overall Score:       %.1f%%\n", summary.OverallScore)
		fmt.Printf("Total Rules:         %d\n", summary.TotalRules)
		fmt.Printf("Passed / Failed:     %d / %d\n", summary.PassedRules, summary.FailedRules)
		fmt.Printf("Active Findings:     %d (Critical: %d, High: %d, Medium: %d, Low: %d)\n",
			summary.ActiveFindings, summary.CriticalFindings, summary.HighFindings, summary.MediumFindings, summary.LowFindings)
		return nil
	},
}

var compGroupCmd = &cobra.Command{
	Use:   "group <group_id>",
	Short: "Get aggregated compliance posture summary for a fleet group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		summary, err := client.GetGroupComplianceSummary(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get group compliance summary: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(summary)
		}
		fmt.Printf("📊 Fleet Group Compliance Summary: %s\n", args[0])
		fmt.Printf("Overall Score:       %.1f%%\n", summary.OverallScore)
		fmt.Printf("Total Nodes:         %d (Compliant: %d, Non-Compliant: %d)\n",
			summary.TotalNodes, summary.CompliantNodes, summary.NonCompliantNodes)
		fmt.Printf("Active Findings:     %d (Critical: %d, High: %d, Medium: %d, Low: %d)\n",
			summary.ActiveFindings, summary.CriticalFindings, summary.HighFindings, summary.MediumFindings, summary.LowFindings)
		return nil
	},
}

var compOrgCmd = &cobra.Command{
	Use:   "org [org_id]",
	Short: "Get organization-wide compliance posture rollup",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if len(args) > 0 {
			orgID = args[0]
		}
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		summary, err := client.GetOrgComplianceSummary(ctx, orgID)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get org compliance summary: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(summary)
		}
		fmt.Printf("📊 Organization Compliance Summary: %s\n", orgID)
		fmt.Printf("Overall Score:       %.1f%%\n", summary.OverallScore)
		fmt.Printf("Total Nodes:         %d (Compliant: %d, Non-Compliant: %d)\n",
			summary.TotalNodes, summary.CompliantNodes, summary.NonCompliantNodes)
		fmt.Printf("Total Groups:        %d\n", summary.TotalGroups)
		fmt.Printf("Total Policies:      %d\n", summary.TotalPolicies)
		fmt.Printf("Active Findings:     %d (Critical: %d, High: %d, Medium: %d, Low: %d)\n",
			summary.ActiveFindings, summary.CriticalFindings, summary.HighFindings, summary.MediumFindings, summary.LowFindings)
		return nil
	},
}

var compResolvePoliciesCmd = &cobra.Command{
	Use:   "resolve-policies <node_id>",
	Short: "Resolve all active policies applicable to a node via group and org hierarchy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		policies, err := client.ResolveNodePolicies(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to resolve policies for node %s: %w", args[0], err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(policies)
		}
		fmt.Printf("📜 Resolved Policies for Node %s (%d policies):\n", args[0], len(policies))
		tw := util.NewTableWriter("Policy ID", "Name", "Category", "Severity", "Active Rev")
		for _, p := range policies {
			tw.Append(p.ID, p.Name, p.Category, string(p.Severity), strconv.Itoa(p.ActiveRevision))
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var compExplainCmd = &cobra.Command{
	Use:   "explain <node_id>",
	Short: "Explain hierarchical policy and ownership resolution lineage for a node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		expl, err := client.ExplainResolution(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to explain resolution for node %s: %w", args[0], err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(expl)
		}
		fmt.Printf("💡 Resolution Explanation for Node: %s\n", args[0])
		fmt.Printf("Organization: %s\n", expl.OrgID)
		if len(expl.GroupLineage) > 0 {
			fmt.Printf("Group Hierarchy Lineage: %s\n", strings.Join(expl.GroupLineage, " -> "))
		}
		if len(expl.AssignedPolicies) > 0 {
			fmt.Printf("\nAssigned Policies Lineage (%d):\n", len(expl.AssignedPolicies))
			tw := util.NewTableWriter("Policy ID", "Scope Source", "Inherited From")
			for _, ap := range expl.AssignedPolicies {
				tw.Append(ap.PolicyID, ap.SourceScope, ap.InheritedFrom)
			}
			tw.Render(os.Stdout)
		}
		return nil
	},
}

// -----------------------------------------------------------------------------
// 9. Simulation Subcommands
// -----------------------------------------------------------------------------

var simulateCmd = &cobra.Command{
	Use:     "simulate [command]",
	Aliases: []string{"sim"},
	Short:   "Perform what-if policy simulations without database side effects",
}

var simulateRunCmd = &cobra.Command{
	Use:   "run [flags]",
	Short: "Run an isolated what-if policy simulation",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		var candidatePolicies []model.Policy
		if govPoliciesFile != "" {
			data, err := os.ReadFile(govPoliciesFile)
			if err != nil {
				return NewExitError(ExitUsageError, "failed to read candidate policies file: %w", err)
			}
			if err := json.Unmarshal(data, &candidatePolicies); err != nil {
				return NewExitError(ExitUsageError, "failed to parse candidate policies JSON: %w", err)
			}
		}

		req := model.SimulationRequest{
			OrgID:             orgID,
			NodeIDs:           govNodeIDs,
			GroupIDs:          govGroupIDs,
			CandidatePolicies: candidatePolicies,
			DiffOnly:          govDiffOnly,
		}
		result, err := client.SimulatePolicyChanges(ctx, req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to execute policy simulation: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(result)
		}
		fmt.Println("🧪 What-If Policy Simulation Results:")
		fmt.Printf("Nodes Evaluated:     %d\n", result.NodesEvaluated)
		fmt.Printf("Baseline Score:      %.1f%%\n", result.BaselineScore)
		fmt.Printf("Simulated Score:     %.1f%%\n", result.SimulatedScore)
		fmt.Printf("Score Delta:         %+.1f%%\n", result.ScoreDelta)
		fmt.Printf("New Violations:      %d\n", result.NewViolations)
		fmt.Printf("Resolved Violations: %d\n", result.ResolvedViolations)
		if len(result.FindingsDiff) > 0 {
			fmt.Printf("\nFindings Impact Diff (%d changes):\n", len(result.FindingsDiff))
			tw := util.NewTableWriter("Diff Type", "Node ID", "Policy ID", "Rule ID", "Severity", "Message")
			for _, d := range result.FindingsDiff {
				tw.Append(string(d.DiffType), d.NodeID, d.PolicyID, d.RuleID, string(d.Severity), d.Message)
			}
			tw.Render(os.Stdout)
		}
		return nil
	},
}

// -----------------------------------------------------------------------------
// 10. Maintenance Windows Subcommands
// -----------------------------------------------------------------------------

var maintenanceCmd = &cobra.Command{
	Use:     "maintenance [command]",
	Aliases: []string{"maint", "window", "windows"},
	Short:   "Manage maintenance windows and schedule alert suppressions",
}

var maintListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List maintenance windows matching filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		filter := model.MaintenanceWindowFilter{
			OrgID:          orgID,
			Status:         model.MaintenanceStatus(govStatus),
			TargetScope:    model.MaintenanceScope(govTargetScope),
			TargetID:       govTargetID,
			IncludeExpired: govIncludeExpired,
			Limit:          govLimit,
			Offset:         govOffset,
		}
		windows, err := client.ListMaintenanceWindows(ctx, filter)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to list maintenance windows: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(windows)
		}
		if len(windows) == 0 {
			fmt.Println("No maintenance windows found.")
			return nil
		}
		tw := util.NewTableWriter("Window ID", "Name", "Scope", "Target ID", "Status", "Schedule Type", "Start Time", "End Time")
		for _, w := range windows {
			tw.Append(w.ID, w.Name, string(w.Scope), w.TargetID, string(w.Status), string(w.Schedule.Type),
				w.Schedule.StartTime.Format(time.RFC3339), w.Schedule.EndTime.Format(time.RFC3339))
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var maintGetCmd = &cobra.Command{
	Use:   "get <window_id>",
	Short: "Get maintenance window details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		w, err := client.GetMaintenanceWindow(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get maintenance window: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(w)
		}
		fmt.Printf("🛠️ Maintenance Window: %s (%s)\n", w.Name, w.ID)
		fmt.Printf("Org ID:          %s\n", w.OrgID)
		fmt.Printf("Scope / Target:  %s / %s\n", w.Scope, w.TargetID)
		fmt.Printf("Status:          %s\n", w.Status)
		fmt.Printf("Schedule Type:   %s\n", w.Schedule.Type)
		fmt.Printf("Start / End:     %s -> %s\n", w.Schedule.StartTime.Format(time.RFC3339), w.Schedule.EndTime.Format(time.RFC3339))
		if w.Schedule.Timezone != "" {
			fmt.Printf("Timezone:        %s\n", w.Schedule.Timezone)
		}
		fmt.Printf("Allow Disrupt:   %v\n", w.AllowDisruptiveOps)
		fmt.Printf("Created By:      %s\n", w.CreatedBy)
		if len(w.SuppressionRules) > 0 {
			fmt.Printf("Suppression Rules: %d configured\n", len(w.SuppressionRules))
		}
		return nil
	},
}

var maintCreateCmd = &cobra.Command{
	Use:   "create <window_id> <name>",
	Short: "Create a maintenance window",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		var st, et time.Time
		if govWindowStart != "" {
			st, _ = time.Parse(time.RFC3339, govWindowStart)
		} else {
			st = time.Now()
		}
		if govWindowEnd != "" {
			et, _ = time.Parse(time.RFC3339, govWindowEnd)
		} else {
			et = st.Add(2 * time.Hour)
		}

		scope := model.MaintenanceScope(govTargetScope)
		if scope == "" {
			scope = model.MaintenanceScopeNode
		}
		schedType := model.ScheduleOneTime
		if govWindowType != "" {
			schedType = model.ScheduleType(govWindowType)
		}

		req := model.MaintenanceWindow{
			ID:       args[0],
			OrgID:    orgID,
			Name:     args[1],
			Scope:    scope,
			TargetID: govTargetID,
			Status:   model.MaintenanceStatusScheduled,
			Schedule: model.ScheduleConfig{
				Type:        schedType,
				StartTime:   st,
				EndTime:     et,
				DailyStart:  govDailyStart,
				DailyEnd:    govDailyEnd,
				DaysOfWeek:  govDaysOfWeek,
				DayOfMonth:  govDayOfMonth,
				Timezone:    govTimezone,
			},
			AllowDisruptiveOps: govAllowDisrupt,
			CreatedBy:          govCreatedBy,
		}
		w, err := client.CreateMaintenanceWindow(ctx, req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to create maintenance window: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(w)
		}
		fmt.Printf("✓ Created maintenance window %q (%s)\n", w.Name, w.ID)
		return nil
	},
}

var maintUpdateCmd = &cobra.Command{
	Use:   "update <window_id>",
	Short: "Update a maintenance window",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		req := model.MaintenanceWindow{
			Name:   govName,
			Status: model.MaintenanceStatus(govStatus),
		}
		w, err := client.UpdateMaintenanceWindow(ctx, args[0], req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to update maintenance window: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(w)
		}
		fmt.Printf("✓ Updated maintenance window %q (%s)\n", w.Name, w.ID)
		return nil
	},
}

var maintDeleteCmd = &cobra.Command{
	Use:     "delete <window_id>",
	Aliases: []string{"rm"},
	Short:   "Delete a maintenance window",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		if err := client.DeleteMaintenanceWindow(ctx, args[0]); err != nil {
			return NewExitError(ExitNetworkError, "failed to delete maintenance window: %w", err)
		}
		fmt.Printf("✓ Deleted maintenance window %q\n", args[0])
		return nil
	},
}

var maintCancelCmd = &cobra.Command{
	Use:   "cancel <window_id>",
	Short: "Cancel an active or scheduled maintenance window",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		if err := client.CancelMaintenanceWindow(ctx, args[0], govReason); err != nil {
			return NewExitError(ExitNetworkError, "failed to cancel maintenance window: %w", err)
		}
		fmt.Printf("✓ Cancelled maintenance window %q\n", args[0])
		return nil
	},
}

var maintEvaluateCmd = &cobra.Command{
	Use:   "evaluate [flags]",
	Short: "Evaluate active maintenance windows for a target at a given point in time",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		at := time.Now()
		if govAtTime != "" {
			if t, err := time.Parse(time.RFC3339, govAtTime); err == nil {
				at = t
			}
		}
		windows, err := client.EvaluateMaintenanceWindows(ctx, orgID, model.MaintenanceScope(govTargetScope), govTargetID, at)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to evaluate maintenance windows: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(windows)
		}
		if len(windows) == 0 {
			fmt.Printf("No active maintenance windows at %s.\n", at.Format(time.RFC3339))
			return nil
		}
		fmt.Printf("Active Maintenance Windows at %s (%d):\n", at.Format(time.RFC3339), len(windows))
		tw := util.NewTableWriter("Window ID", "Name", "Scope", "Target ID", "Status", "End Time")
		for _, w := range windows {
			tw.Append(w.ID, w.Name, string(w.Scope), w.TargetID, string(w.Status), w.Schedule.EndTime.Format(time.RFC3339))
		}
		tw.Render(os.Stdout)
		return nil
	},
}

// -----------------------------------------------------------------------------
// 11. Escalation Subcommands
// -----------------------------------------------------------------------------

var escalationCmd = &cobra.Command{
	Use:     "escalation [command]",
	Aliases: []string{"escalate", "esc"},
	Short:   "Manage incident escalation policies and evaluate multi-stage chains",
}

var escListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List incident escalation policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		var enabledPtr *bool
		if govEnabled != "" {
			b, err := strconv.ParseBool(govEnabled)
			if err == nil {
				enabledPtr = &b
			}
		}
		filter := model.EscalationPolicyFilter{
			OrgID:    orgID,
			Severity: model.SeverityLevel(govSeverity),
			Enabled:  enabledPtr,
			Limit:    govLimit,
			Offset:   govOffset,
		}
		policies, err := client.ListEscalationPolicies(ctx, filter)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to list escalation policies: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(policies)
		}
		if len(policies) == 0 {
			fmt.Println("No escalation policies found.")
			return nil
		}
		tw := util.NewTableWriter("Policy ID", "Name", "Severity", "Enabled", "Stages", "Created By")
		for _, p := range policies {
			tw.Append(p.ID, p.Name, string(p.Severity), strconv.FormatBool(p.Enabled), strconv.Itoa(len(p.Stages)), p.CreatedBy)
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var escGetCmd = &cobra.Command{
	Use:   "get <policy_id>",
	Short: "Get escalation policy details and stages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		p, err := client.GetEscalationPolicy(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get escalation policy: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(p)
		}
		fmt.Printf("🚨 Escalation Policy: %s (%s)\n", p.Name, p.ID)
		fmt.Printf("Org ID:      %s\n", p.OrgID)
		fmt.Printf("Severity:    %s\n", p.Severity)
		fmt.Printf("Enabled:     %v\n", p.Enabled)
		fmt.Printf("Description: %s\n", p.Description)
		if len(p.Stages) > 0 {
			fmt.Printf("\nEscalation Stages (%d):\n", len(p.Stages))
			tw := util.NewTableWriter("Stage #", "Delay", "Channels", "Targets", "Notify Lead")
			for _, st := range p.Stages {
				chans := strings.Join(st.Channels, ", ")
				targets := strings.Join(st.Targets, ", ")
				tw.Append(strconv.Itoa(st.StageNumber), st.DelayDuration.String(), chans, targets, strconv.FormatBool(st.NotifyOwnerLead))
			}
			tw.Render(os.Stdout)
		}
		return nil
	},
}

var escCreateCmd = &cobra.Command{
	Use:   "create <policy_id> <name>",
	Short: "Create an escalation policy",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		var stages []model.EscalationStage
		if govStagesFile != "" {
			data, err := os.ReadFile(govStagesFile)
			if err != nil {
				return NewExitError(ExitUsageError, "failed to read stages file: %w", err)
			}
			if err := json.Unmarshal(data, &stages); err != nil {
				return NewExitError(ExitUsageError, "failed to parse stages JSON: %w", err)
			}
		}

		req := model.EscalationPolicy{
			ID:          args[0],
			OrgID:       orgID,
			Name:        args[1],
			Description: govDescription,
			Severity:    model.SeverityLevel(govSeverity),
			Enabled:     true,
			Stages:      stages,
			CreatedBy:   govCreatedBy,
		}
		p, err := client.CreateEscalationPolicy(ctx, req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to create escalation policy: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(p)
		}
		fmt.Printf("✓ Created escalation policy %q (%s)\n", p.Name, p.ID)
		return nil
	},
}

var escUpdateCmd = &cobra.Command{
	Use:   "update <policy_id>",
	Short: "Update an escalation policy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		var stages []model.EscalationStage
		if govStagesFile != "" {
			data, err := os.ReadFile(govStagesFile)
			if err != nil {
				return NewExitError(ExitUsageError, "failed to read stages file: %w", err)
			}
			if err := json.Unmarshal(data, &stages); err != nil {
				return NewExitError(ExitUsageError, "failed to parse stages JSON: %w", err)
			}
		}

		req := model.EscalationPolicy{
			Name:        govName,
			Description: govDescription,
			Stages:      stages,
		}
		p, err := client.UpdateEscalationPolicy(ctx, args[0], req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to update escalation policy: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(p)
		}
		fmt.Printf("✓ Updated escalation policy %q (%s)\n", p.Name, p.ID)
		return nil
	},
}

var escDeleteCmd = &cobra.Command{
	Use:     "delete <policy_id>",
	Aliases: []string{"rm"},
	Short:   "Delete an escalation policy",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		if err := client.DeleteEscalationPolicy(ctx, args[0]); err != nil {
			return NewExitError(ExitNetworkError, "failed to delete escalation policy: %w", err)
		}
		fmt.Printf("✓ Deleted escalation policy %q\n", args[0])
		return nil
	},
}

var escEvaluateCmd = &cobra.Command{
	Use:   "evaluate [flags]",
	Short: "Evaluate incident escalation status against active policy stages",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		status, err := client.EvaluateIncidentEscalation(ctx, govPolicyID, govDuration, govAcknowledged)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to evaluate escalation: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(status)
		}
		fmt.Printf("🚨 Incident Escalation Status (Policy: %s):\n", govPolicyID)
		fmt.Printf("Active Stage:       %d\n", status.ActiveStageNumber)
		fmt.Printf("Time to Next Stage: %s\n", status.TimeToNextStage)
		if len(status.ActiveTargets) > 0 {
			fmt.Printf("Active Targets:     %s\n", strings.Join(status.ActiveTargets, ", "))
		}
		if len(status.ActiveChannels) > 0 {
			fmt.Printf("Active Channels:    %s\n", strings.Join(status.ActiveChannels, ", "))
		}
		return nil
	},
}

// -----------------------------------------------------------------------------
// 12. Alert Suppression Subcommands
// -----------------------------------------------------------------------------

var suppressionCmd = &cobra.Command{
	Use:     "suppression [command]",
	Aliases: []string{"suppress"},
	Short:   "Evaluate and inspect alert suppression decisions with safety guardrails",
}

var suppEvaluateCmd = &cobra.Command{
	Use:   "evaluate [flags]",
	Short: "Evaluate alert suppression against active maintenance windows and safety guardrails",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		at := time.Now()
		if govAtTime != "" {
			if t, err := time.Parse(time.RFC3339, govAtTime); err == nil {
				at = t
			}
		}

		req := model.SuppressionEvalRequest{
			OrgID:         orgID,
			NodeID:        govNodeID,
			FleetGroupID:  govGroupID,
			AlertID:       govAlertID,
			AlertName:     govAlertName,
			AlertCategory: govCategory,
			Severity:      model.SeverityLevel(govSeverity),
			Timestamp:     at,
		}
		dec, err := client.EvaluateSuppression(ctx, req)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to evaluate alert suppression: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(dec)
		}
		fmt.Printf("🛡️ Alert Suppression Decision:\n")
		fmt.Printf("Suppressed:       %v\n", dec.Suppressed)
		fmt.Printf("Decision ID:      %s\n", dec.ID)
		fmt.Printf("Reason / Guard:   %s\n", dec.Reason)
		if dec.MatchingWindowID != "" {
			fmt.Printf("Matching Window:  %s\n", dec.MatchingWindowID)
		}
		if dec.GuardrailTriggered {
			fmt.Printf("⚠️ Guardrail Triggered: Critical alert class cannot be suppressed!\n")
		}
		return nil
	},
}

var suppListCmd = &cobra.Command{
	Use:     "list [flags]",
	Aliases: []string{"ls"},
	Short:   "List historical suppression decisions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		var suppPtr *bool
		if govEnabled != "" {
			b, err := strconv.ParseBool(govEnabled)
			if err == nil {
				suppPtr = &b
			}
		}
		filter := model.SuppressionFilter{
			OrgID:              orgID,
			NodeID:             govNodeID,
			AlertID:            govAlertID,
			MatchingWindowID:   govTargetID,
			SuppressedOnly:     suppPtr,
			Limit:              govLimit,
			Offset:             govOffset,
		}
		decs, err := client.ListSuppressionDecisions(ctx, filter)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to list suppression decisions: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(decs)
		}
		if len(decs) == 0 {
			fmt.Println("No suppression decisions recorded.")
			return nil
		}
		tw := util.NewTableWriter("Decision ID", "Node ID", "Alert Name", "Suppressed", "Window ID", "Reason", "Timestamp")
		for _, d := range decs {
			tw.Append(d.ID, d.NodeID, d.AlertName, strconv.FormatBool(d.Suppressed), d.MatchingWindowID, d.Reason, d.Timestamp.Format("01/02 15:04:05"))
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var suppGetCmd = &cobra.Command{
	Use:   "get <decision_id>",
	Short: "Get suppression decision details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		d, err := client.GetSuppressionDecision(ctx, args[0])
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to get suppression decision: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(d)
		}
		fmt.Printf("🛡️ Suppression Decision: %s\n", d.ID)
		fmt.Printf("Node ID:          %s\n", d.NodeID)
		fmt.Printf("Alert ID:         %s\n", d.AlertID)
		fmt.Printf("Alert Name:       %s\n", d.AlertName)
		fmt.Printf("Suppressed:       %v\n", d.Suppressed)
		fmt.Printf("Guardrail Hit:    %v\n", d.GuardrailTriggered)
		fmt.Printf("Matching Window:  %s\n", d.MatchingWindowID)
		fmt.Printf("Reason:           %s\n", d.Reason)
		fmt.Printf("Timestamp:        %s\n", d.Timestamp.Format(time.RFC3339))
		return nil
	},
}

// -----------------------------------------------------------------------------
// 13. Audit Subcommands
// -----------------------------------------------------------------------------

var govAuditCmd = &cobra.Command{
	Use:     "audit [command]",
	Aliases: []string{"audits", "trail"},
	Short:   "Query and record sanitized governance audit events",
}

var auditQueryCmd = &cobra.Command{
	Use:     "query [flags]",
	Aliases: []string{"list", "ls"},
	Short:   "Query sanitized governance audit event history",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		var st, et time.Time
		if govStartTime != "" {
			st, _ = time.Parse(time.RFC3339, govStartTime)
		}
		if govEndTime != "" {
			et, _ = time.Parse(time.RFC3339, govEndTime)
		}

		filter := governance.AuditQueryFilter{
			EventType:     govEventType,
			Severity:      govSeverity,
			Outcome:       govOutcome,
			ActorType:     govActorType,
			ActorIdentity: govActorID,
			StartTime:     st,
			EndTime:       et,
			Limit:         govLimit,
			Offset:        govOffset,
		}
		events, err := client.QueryAuditEvents(ctx, filter)
		if err != nil {
			return NewExitError(ExitNetworkError, "failed to query audit events: %w", err)
		}
		if isGovFormatted() {
			return outputGovernanceFormatted(events)
		}
		if len(events) == 0 {
			fmt.Println("No governance audit events found.")
			return nil
		}
		tw := util.NewTableWriter("Event ID", "Timestamp", "Event Type", "Actor", "Outcome", "Action", "Resource")
		for _, e := range events {
			res := fmt.Sprintf("%s:%s", e.ResourceType, e.ResourceID)
			tw.Append(e.ID, e.Timestamp.Format("01/02 15:04:05"), string(e.EventType), e.ActorIdentity, string(e.Outcome), e.Action, res)
		}
		tw.Render(os.Stdout)
		return nil
	},
}

var auditRecordCmd = &cobra.Command{
	Use:   "record [flags]",
	Short: "Record a governance audit event (passwords and tokens automatically redacted)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getGovernanceClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), govTimeout)
		defer cancel()

		orgID := govOrgID
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}

		event := model.AuditEvent{
			EventType:     model.AuditEventType(govEventType),
			Severity:      model.SeverityLevel(govSeverity),
			Outcome:       model.AuditOutcome(govOutcome),
			ActorType:     govActorType,
			ActorIdentity: govActorID,
			Action:        govAction,
			ResourceType:  govResourceType,
			ResourceID:    govResourceID,
			Message:       govMessage,
			Metadata:      map[string]string{"org_id": orgID},
		}
		if event.EventType == "" {
			event.EventType = model.AuditPolicyCreated
		}
		if event.Severity == "" {
			event.Severity = model.SeverityInfo
		}
		if event.Outcome == "" {
			event.Outcome = model.AuditOutcomeSuccess
		}

		if err := client.RecordAuditEvent(ctx, event); err != nil {
			return NewExitError(ExitNetworkError, "failed to record audit event: %w", err)
		}
		fmt.Printf("✓ Recorded governance audit event %s\n", event.Action)
		return nil
	},
}

// -----------------------------------------------------------------------------
// init: Wiring all commands and flags
// -----------------------------------------------------------------------------

func init() {
	// Persistent flags on governanceCmd
	governanceCmd.PersistentFlags().StringVar(&govServerURL, "server", "", "centralized server URL (e.g. https://fleet.internal:8443)")
	governanceCmd.PersistentFlags().StringVar(&govToken, "token", "", "bearer token for server authentication")
	governanceCmd.PersistentFlags().StringVar(&govTokenFile, "token-file", "", "path to file containing bearer token")
	governanceCmd.PersistentFlags().StringVar(&govTokenEnv, "token-env", "", "environment variable containing bearer token")
	governanceCmd.PersistentFlags().BoolVar(&govInsecureTLS, "insecure-tls", false, "skip TLS certificate validation (development only)")
	governanceCmd.PersistentFlags().DurationVar(&govTimeout, "timeout", 15*time.Second, "HTTP request timeout duration")
	governanceCmd.PersistentFlags().StringVarP(&govFormat, "format", "f", "text", "output format (text, json, yaml)")
	governanceCmd.PersistentFlags().StringVar(&govOrgID, "org", "", "organization ID scope")

	// Subcommand: org
	orgCmd.AddCommand(orgListCmd, orgGetCmd, orgCreateCmd, orgUpdateCmd, orgDeleteCmd)
	orgCreateCmd.Flags().StringVar(&govDescription, "description", "", "organization description")
	orgCreateCmd.Flags().StringSliceVar(&govTags, "tags", nil, "tags for organization")
	orgUpdateCmd.Flags().StringVar(&govName, "name", "", "updated organization name")
	orgUpdateCmd.Flags().StringVar(&govDescription, "description", "", "updated organization description")
	orgUpdateCmd.Flags().StringSliceVar(&govTags, "tags", nil, "updated tags")

	// Subcommand: group
	groupCmd.AddCommand(groupListCmd, groupGetCmd, groupCreateCmd, groupUpdateCmd, groupDeleteCmd,
		groupHierarchyCmd, groupMembersCmd, groupAddMemberCmd, groupRemoveMemberCmd, groupSubtreeCmd)
	groupListCmd.Flags().StringVar(&govParentID, "parent-id", "", "filter groups by parent group ID")
	groupCreateCmd.Flags().StringVar(&govParentID, "parent-id", "", "parent group ID")
	groupCreateCmd.Flags().StringVar(&govDescription, "description", "", "group description")
	groupCreateCmd.Flags().StringSliceVar(&govTags, "tags", nil, "group tags")
	groupUpdateCmd.Flags().StringVar(&govName, "name", "", "updated group name")
	groupUpdateCmd.Flags().StringVar(&govParentID, "parent-id", "", "updated parent group ID")
	groupUpdateCmd.Flags().StringVar(&govDescription, "description", "", "updated description")
	groupUpdateCmd.Flags().StringSliceVar(&govTags, "tags", nil, "updated tags")

	// Subcommand: ownership
	ownershipCmd.AddCommand(ownershipGetCmd, ownershipSetCmd, ownershipResolveCmd)
	ownershipSetCmd.Flags().StringVar(&govOwnerService, "service", "", "service name")
	ownershipSetCmd.Flags().StringVar(&govOwnerTeam, "team", "", "team name")
	ownershipSetCmd.Flags().StringVar(&govOwnerLead, "owner", "", "lead/owner name")
	ownershipSetCmd.Flags().StringVar(&govOwnerEmail, "email", "", "contact email")
	ownershipSetCmd.Flags().StringVar(&govOwnerSlack, "slack", "", "slack channel")
	ownershipSetCmd.Flags().StringVar(&govOwnerPager, "pagerduty", "", "pagerduty service ID")
	ownershipSetCmd.Flags().StringVar(&govOwnerEscTier, "escalation-tier", "", "escalation tier")
	ownershipSetCmd.Flags().StringVar(&govOwnerLifeStg, "lifecycle-stage", "", "lifecycle stage")
	ownershipSetCmd.Flags().StringVar(&govOwnerCostCtr, "cost-center", "", "cost center")
	ownershipSetCmd.Flags().StringVar(&govOwnerEnv, "environment", "", "environment name")

	// Subcommand: policy
	policyCmd.AddCommand(policyListCmd, policyGetCmd, policyCreateCmd, policyUpdateCmd, policyDeleteCmd,
		policyRevisionsCmd, policyPublishRevCmd, policySetStatusCmd, policySetActiveCmd, policyConflictsCmd)
	policyListCmd.Flags().StringVar(&govCategory, "category", "", "filter policies by category")
	policyListCmd.Flags().StringVar(&govStatus, "status", "", "filter policies by status (draft, published, deprecated, archived)")
	policyListCmd.Flags().StringVar(&govSearch, "search", "", "search keyword")
	policyCreateCmd.Flags().StringVar(&govCategory, "category", "", "policy category")
	policyCreateCmd.Flags().StringVar(&govSeverity, "severity", "warning", "policy severity level")
	policyCreateCmd.Flags().StringVar(&govDescription, "description", "", "policy description")
	policyCreateCmd.Flags().StringVar(&govSchemaVersion, "schema-version", "1.0", "policy schema version")
	policyUpdateCmd.Flags().StringVar(&govName, "name", "", "updated policy name")
	policyUpdateCmd.Flags().StringVar(&govCategory, "category", "", "updated category")
	policyUpdateCmd.Flags().StringVar(&govSeverity, "severity", "", "updated severity level")
	policyUpdateCmd.Flags().StringVar(&govDescription, "description", "", "updated description")
	policyUpdateCmd.Flags().StringVar(&govStatus, "status", "", "updated status")
	policyPublishRevCmd.Flags().StringVar(&govAuthor, "author", "", "author publishing the revision")
	policyPublishRevCmd.Flags().StringVar(&govChangelog, "changelog", "", "revision changelog message")
	policyPublishRevCmd.Flags().StringVar(&govRulesFile, "rules-file", "", "path to JSON file containing policy rules")
	policyConflictsCmd.Flags().StringSliceVar(&govPolicyIDs, "policy-ids", nil, "policy IDs to check for conflicts")

	// Subcommand: assignment
	assignmentCmd.AddCommand(assignmentListCmd, assignmentGetCmd, assignmentCreateCmd, assignmentDeleteCmd, assignmentEnableCmd)
	assignmentListCmd.Flags().StringVar(&govPolicyID, "policy-id", "", "filter by policy ID")
	assignmentListCmd.Flags().StringVar(&govTargetTyp, "target-type", "", "filter by target type (organization, fleet_group, node)")
	assignmentListCmd.Flags().StringVar(&govTargetID, "target-id", "", "filter by target ID")
	assignmentListCmd.Flags().StringVar(&govEnabled, "enabled", "", "filter by enabled state (true/false)")
	assignmentCreateCmd.Flags().StringVar(&govAssignedBy, "assigned-by", "", "user or service creating assignment")
	assignmentCreateCmd.Flags().StringVar(&govScopeFilter, "scope-filter", "", "optional CEL filter expression")
	assignmentEnableCmd.Flags().StringVar(&govEnabled, "enabled", "true", "enabled state (true/false)")

	// Subcommand: evaluate
	evaluateCmd.AddCommand(evalNodeCmd, evalGroupCmd, evalOrgCmd, evalListCmd, evalGetCmd, evalLatestCmd)
	evalNodeCmd.Flags().StringVar(&govTriggerType, "trigger", "manual", "trigger type (manual, schedule, event, telemetry)")
	evalGroupCmd.Flags().StringVar(&govTriggerType, "trigger", "manual", "trigger type")
	evalOrgCmd.Flags().StringVar(&govTriggerType, "trigger", "manual", "trigger type")
	evalListCmd.Flags().StringVar(&govTargetTyp, "target-type", "", "filter by target type")
	evalListCmd.Flags().StringVar(&govTargetID, "target-id", "", "filter by target ID")
	evalListCmd.Flags().StringVar(&govTriggerType, "trigger", "", "filter by trigger type")
	evalListCmd.Flags().IntVar(&govLimit, "limit", 50, "max results")
	evalListCmd.Flags().IntVar(&govOffset, "offset", 0, "offset")

	// Subcommand: findings
	findingsCmd.AddCommand(findingsListCmd, findingsGetCmd)
	findingsListCmd.Flags().StringVar(&govNodeID, "node-id", "", "filter by node ID")
	findingsListCmd.Flags().StringVar(&govPolicyID, "policy-id", "", "filter by policy ID")
	findingsListCmd.Flags().StringVar(&govRuleID, "rule-id", "", "filter by rule ID")
	findingsListCmd.Flags().StringVar(&govSeverity, "severity", "", "filter by severity (info, warning, critical)")
	findingsListCmd.Flags().StringVar(&govStatus, "status", "", "filter by status (active, resolved, suppressed, muted)")
	findingsListCmd.Flags().StringVar(&govSearch, "search", "", "search keyword")
	findingsListCmd.Flags().IntVar(&govLimit, "limit", 50, "max results")
	findingsListCmd.Flags().IntVar(&govOffset, "offset", 0, "offset")

	// Subcommand: compliance
	complianceCmd.AddCommand(compNodeCmd, compGroupCmd, compOrgCmd, compResolvePoliciesCmd, compExplainCmd)

	// Subcommand: simulate
	simulateCmd.AddCommand(simulateRunCmd)
	simulateRunCmd.Flags().StringSliceVar(&govNodeIDs, "node-ids", nil, "node IDs to include in simulation")
	simulateRunCmd.Flags().StringSliceVar(&govGroupIDs, "group-ids", nil, "fleet group IDs to include in simulation")
	simulateRunCmd.Flags().StringVar(&govPoliciesFile, "policies-file", "", "path to JSON file containing candidate policies")
	simulateRunCmd.Flags().BoolVar(&govDiffOnly, "diff-only", false, "only return findings diff instead of full evaluation records")

	// Subcommand: maintenance
	maintenanceCmd.AddCommand(maintListCmd, maintGetCmd, maintCreateCmd, maintUpdateCmd, maintDeleteCmd, maintCancelCmd, maintEvaluateCmd)
	maintListCmd.Flags().StringVar(&govStatus, "status", "", "filter by status (scheduled, active, completed, cancelled)")
	maintListCmd.Flags().StringVar(&govTargetScope, "target-scope", "", "filter by scope (node, fleet_group, organization, policy)")
	maintListCmd.Flags().StringVar(&govTargetID, "target-id", "", "filter by target ID")
	maintListCmd.Flags().BoolVar(&govIncludeExpired, "include-expired", false, "include expired windows")
	maintListCmd.Flags().IntVar(&govLimit, "limit", 50, "max results")
	maintListCmd.Flags().IntVar(&govOffset, "offset", 0, "offset")
	maintCreateCmd.Flags().StringVar(&govTargetScope, "scope", "node", "target scope (node, fleet_group, organization, policy)")
	maintCreateCmd.Flags().StringVar(&govTargetID, "target-id", "", "target ID")
	maintCreateCmd.Flags().StringVar(&govWindowType, "type", "one_time", "schedule type (one_time, daily, weekly, monthly)")
	maintCreateCmd.Flags().StringVar(&govWindowStart, "start", "", "start time (RFC3339)")
	maintCreateCmd.Flags().StringVar(&govWindowEnd, "end", "", "end time (RFC3339)")
	maintCreateCmd.Flags().StringVar(&govDailyStart, "daily-start", "", "daily recurring start time (HH:MM)")
	maintCreateCmd.Flags().StringVar(&govDailyEnd, "daily-end", "", "daily recurring end time (HH:MM)")
	maintCreateCmd.Flags().IntSliceVar(&govDaysOfWeek, "days-of-week", nil, "weekly recurring days (0=Sunday ... 6=Saturday)")
	maintCreateCmd.Flags().IntVar(&govDayOfMonth, "day-of-month", 0, "monthly recurring day of month (1-31)")
	maintCreateCmd.Flags().StringVar(&govTimezone, "timezone", "UTC", "IANA timezone")
	maintCreateCmd.Flags().BoolVar(&govAllowDisrupt, "allow-disruptive", false, "allow disruptive operations during window")
	maintCreateCmd.Flags().StringVar(&govCreatedBy, "created-by", "", "author who created window")
	maintUpdateCmd.Flags().StringVar(&govName, "name", "", "updated name")
	maintUpdateCmd.Flags().StringVar(&govStatus, "status", "", "updated status")
	maintCancelCmd.Flags().StringVar(&govReason, "reason", "", "cancellation reason")
	maintEvaluateCmd.Flags().StringVar(&govTargetScope, "target-scope", "node", "scope to evaluate")
	maintEvaluateCmd.Flags().StringVar(&govTargetID, "target-id", "", "target ID")
	maintEvaluateCmd.Flags().StringVar(&govAtTime, "at", "", "point in time to evaluate (RFC3339, default now)")

	// Subcommand: escalation
	escalationCmd.AddCommand(escListCmd, escGetCmd, escCreateCmd, escUpdateCmd, escDeleteCmd, escEvaluateCmd)
	escListCmd.Flags().StringVar(&govSeverity, "severity", "", "filter by severity")
	escListCmd.Flags().StringVar(&govEnabled, "enabled", "", "filter by enabled state")
	escListCmd.Flags().IntVar(&govLimit, "limit", 50, "max results")
	escListCmd.Flags().IntVar(&govOffset, "offset", 0, "offset")
	escCreateCmd.Flags().StringVar(&govSeverity, "severity", "critical", "incident severity level")
	escCreateCmd.Flags().StringVar(&govDescription, "description", "", "policy description")
	escCreateCmd.Flags().StringVar(&govStagesFile, "stages-file", "", "path to JSON file with escalation stages")
	escCreateCmd.Flags().StringVar(&govCreatedBy, "created-by", "", "created by user/service")
	escUpdateCmd.Flags().StringVar(&govName, "name", "", "updated name")
	escUpdateCmd.Flags().StringVar(&govDescription, "description", "", "updated description")
	escUpdateCmd.Flags().StringVar(&govStagesFile, "stages-file", "", "updated stages file")
	escEvaluateCmd.Flags().StringVar(&govPolicyID, "policy-id", "", "escalation policy ID")
	escEvaluateCmd.Flags().DurationVar(&govDuration, "duration", 0, "incident duration (e.g. 30m, 1h)")
	escEvaluateCmd.Flags().BoolVar(&govAcknowledged, "acknowledged", false, "whether incident has been acknowledged")

	// Subcommand: suppression
	suppressionCmd.AddCommand(suppEvaluateCmd, suppListCmd, suppGetCmd)
	suppEvaluateCmd.Flags().StringVar(&govNodeID, "node-id", "", "node ID")
	suppEvaluateCmd.Flags().StringVar(&govGroupID, "group-id", "", "fleet group ID")
	suppEvaluateCmd.Flags().StringVar(&govAlertID, "alert-id", "", "alert ID")
	suppEvaluateCmd.Flags().StringVar(&govAlertName, "alert-name", "", "alert name")
	suppEvaluateCmd.Flags().StringVar(&govCategory, "category", "", "alert category")
	suppEvaluateCmd.Flags().StringVar(&govSeverity, "severity", "warning", "alert severity")
	suppEvaluateCmd.Flags().StringVar(&govAtTime, "at", "", "timestamp to evaluate at (RFC3339)")
	suppListCmd.Flags().StringVar(&govNodeID, "node-id", "", "filter by node ID")
	suppListCmd.Flags().StringVar(&govAlertID, "alert-id", "", "filter by alert ID")
	suppListCmd.Flags().StringVar(&govTargetID, "window-id", "", "filter by matching maintenance window ID")
	suppListCmd.Flags().StringVar(&govEnabled, "suppressed", "", "filter by suppressed state (true/false)")
	suppListCmd.Flags().IntVar(&govLimit, "limit", 50, "max results")
	suppListCmd.Flags().IntVar(&govOffset, "offset", 0, "offset")

	// Subcommand: audit
	govAuditCmd.AddCommand(auditQueryCmd, auditRecordCmd)
	auditQueryCmd.Flags().StringVar(&govEventType, "event-type", "", "filter by event type")
	auditQueryCmd.Flags().StringVar(&govSeverity, "severity", "", "filter by severity")
	auditQueryCmd.Flags().StringVar(&govOutcome, "outcome", "", "filter by outcome (success, failure, denied, error)")
	auditQueryCmd.Flags().StringVar(&govActorType, "actor-type", "", "filter by actor type")
	auditQueryCmd.Flags().StringVar(&govActorID, "actor-identity", "", "filter by actor identity")
	auditQueryCmd.Flags().StringVar(&govStartTime, "start-time", "", "filter events starting after (RFC3339)")
	auditQueryCmd.Flags().StringVar(&govEndTime, "end-time", "", "filter events starting before (RFC3339)")
	auditQueryCmd.Flags().IntVar(&govLimit, "limit", 50, "max results")
	auditQueryCmd.Flags().IntVar(&govOffset, "offset", 0, "offset")
	auditRecordCmd.Flags().StringVar(&govEventType, "event-type", "policy.created", "event type")
	auditRecordCmd.Flags().StringVar(&govSeverity, "severity", "info", "severity")
	auditRecordCmd.Flags().StringVar(&govOutcome, "outcome", "success", "outcome")
	auditRecordCmd.Flags().StringVar(&govActorType, "actor-type", "user", "actor type")
	auditRecordCmd.Flags().StringVar(&govActorID, "actor-identity", "cli-operator", "actor identity")
	auditRecordCmd.Flags().StringVar(&govAction, "action", "", "action performed")
	auditRecordCmd.Flags().StringVar(&govResourceType, "resource-type", "", "resource type")
	auditRecordCmd.Flags().StringVar(&govResourceID, "resource-id", "", "resource ID")
	auditRecordCmd.Flags().StringVar(&govMessage, "message", "", "audit message")

	// Attach subcommands to governanceCmd
	governanceCmd.AddCommand(
		orgCmd,
		groupCmd,
		ownershipCmd,
		policyCmd,
		assignmentCmd,
		evaluateCmd,
		findingsCmd,
		complianceCmd,
		simulateCmd,
		maintenanceCmd,
		escalationCmd,
		suppressionCmd,
		govAuditCmd,
	)

	// Attach governanceCmd to RootCmd
	RootCmd.AddCommand(governanceCmd)
}
