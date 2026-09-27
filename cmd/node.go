package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/fleet"
	"github.com/DocHoax/watchdog/pkg/util"
	"github.com/spf13/cobra"
)

var (
	nodeJSON       bool
	nodeShort      bool
	nodeIDOverride string
	nodeIDFile     string
	nodeTags       []string
)

var nodeCmd = &cobra.Command{
	Use:   "node [flags]",
	Short: "Inspect the persistent identity and hardware specs of this node",
	Long: `Discovers and displays the persistent cryptographic identity, hardware capabilities,
network interfaces, and operational tags of the local machine.`,
	Example: `  # Display local node identity in human-readable format
  watchdog node

  # Output only the persistent Node UUID (for scripting/automation)
  watchdog node --short

  # Output full node identity in structured JSON
  watchdog node --json

  # Specify operational tags
  watchdog node --tags env=production,role=database`,
	RunE: runNode,
}

func init() {
	nodeCmd.Flags().BoolVar(&nodeJSON, "json", false, "output node identity in structured JSON format")
	nodeCmd.Flags().BoolVarP(&nodeShort, "short", "s", false, "output only the persistent node ID")
	nodeCmd.Flags().StringVar(&nodeIDOverride, "node-id", "", "explicit node ID to use instead of auto-generated UUID")
	nodeCmd.Flags().StringVar(&nodeIDFile, "node-id-file", "", "file path for persistent node ID storage")
	nodeCmd.Flags().StringSliceVar(&nodeTags, "tags", nil, "comma-separated key=value operational tags (e.g. env=prod,dc=us-east)")

	RootCmd.AddCommand(nodeCmd)
}

func runNode(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := globalCfg
	nodeID := nodeIDOverride
	idFile := nodeIDFile
	tags := make(map[string]string)

	if cfg != nil {
		if nodeID == "" {
			nodeID = cfg.Fleet.NodeID
		}
		if idFile == "" {
			idFile = cfg.Fleet.NodeIDFile
		}
		for k, v := range cfg.Fleet.Tags {
			tags[k] = v
		}
	}

	if nodeID == "" && idFile != "" {
		var err error
		nodeID, err = fleet.GetOrGenerateNodeID(idFile)
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to get or generate node ID: %w", err)
		}
	}

	for _, tagPair := range nodeTags {
		parts := strings.SplitN(tagPair, "=", 2)
		if len(parts) == 2 {
			tags[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	identity, err := fleet.DiscoverNodeIdentity(ctx, nodeID, "1.0.0", tags)
	if err != nil {
		return NewExitError(ExitGeneralError, "failed to discover node identity: %w", err)
	}

	if nodeShort {
		fmt.Println(identity.NodeID)
		return nil
	}

	if nodeJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(identity); err != nil {
			return NewExitError(ExitGeneralError, "failed to encode json: %w", err)
		}
		return nil
	}

	// Human-readable formatted output
	fmt.Println("🐺 Watchdog Node Identity")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Printf("  Node ID:         %s\n", identity.NodeID)
	fmt.Printf("  Hostname:        %s\n", identity.Hostname)
	fmt.Printf("  OS:              %s\n", identity.OS)
	fmt.Printf("  Platform:        %s %s\n", identity.Platform, identity.PlatformVer)
	fmt.Printf("  Architecture:    %s\n", identity.Arch)
	if identity.KernelVer != "" {
		fmt.Printf("  Kernel Version:  %s\n", identity.KernelVer)
	}
	fmt.Printf("  Agent Version:   %s\n", identity.Version)
	fmt.Printf("  CPU Cores:       %d\n", identity.CPUCores)
	fmt.Printf("  Total Memory:    %s\n", util.FormatBytes(identity.TotalMemory))

	if len(identity.IPAddresses) > 0 {
		fmt.Printf("  IP Addresses:    %s\n", strings.Join(identity.IPAddresses, ", "))
	}
	if len(identity.MACAddresses) > 0 {
		fmt.Printf("  MAC Addresses:   %s\n", strings.Join(identity.MACAddresses, ", "))
	}
	if len(identity.Tags) > 0 {
		tagList := make([]string, 0, len(identity.Tags))
		for k, v := range identity.Tags {
			tagList = append(tagList, fmt.Sprintf("%s=%s", k, v))
		}
		fmt.Printf("  Tags:            %s\n", strings.Join(tagList, ", "))
	}
	fmt.Printf("  Discovered At:   %s\n", identity.CreatedAt.Local().Format(time.RFC3339))
	return nil
}
