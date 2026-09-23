package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/watchdog-cli/watchdog/internal/config"
	"gopkg.in/yaml.v3"
)

var (
	configInitForce bool
	configShowJSON  bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Watchdog configuration files and settings",
	Long:  `Inspect, initialize, validate, or display the resolved Watchdog configuration file.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display active configuration in YAML or JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := globalCfg
		if cfg == nil {
			cfg = config.DefaultConfig()
		}

		if configShowJSON {
			data, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to format JSON: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}

		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("failed to format YAML: %w", err)
		}
		fmt.Println(string(data))
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the active configuration file path",
	Run: func(cmd *cobra.Command, args []string) {
		if loadedPath != "" {
			fmt.Println(loadedPath)
		} else {
			fmt.Println(config.GetDefaultConfigPath())
		}
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Generate a new default configuration file",
	Long:  `Creates a well-documented default configuration file at $HOME/.watchdog/config.yaml or the specified path.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dest := config.GetDefaultConfigPath()
		if len(args) > 0 {
			dest = args[0]
		}

		if _, err := os.Stat(dest); err == nil && !configInitForce {
			return fmt.Errorf("config file already exists at %s (use --force to overwrite)", dest)
		}

		cfg := config.DefaultConfig()
		if err := cfg.Save(dest); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}

		fmt.Printf("✓ Successfully created default configuration at: %s\n", dest)
		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the current configuration file syntax and values",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := globalCfg
		if cfg == nil {
			return fmt.Errorf("no configuration loaded")
		}

		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("configuration validation failed: %w", err)
		}

		fmt.Println("✓ Configuration is valid.")
		return nil
	},
}

func init() {
	configShowCmd.Flags().BoolVar(&configShowJSON, "json", false, "display configuration in JSON format")
	configInitCmd.Flags().BoolVarP(&configInitForce, "force", "f", false, "overwrite existing configuration file")

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)

	RootCmd.AddCommand(configCmd)
}
