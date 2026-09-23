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
	Use:   "config [command]",
	Short: "Manage Watchdog configuration files and settings",
	Long:  `Inspect, initialize, validate, or display resolved Watchdog configuration settings.`,
	Example: `  # Print the active configuration in YAML format
  watchdog config show

  # Print the active configuration file path on disk
  watchdog config path

  # Initialize a new default config file in $HOME/.watchdog/config.yaml
  watchdog config init

  # Validate the current configuration file syntax and rule thresholds
  watchdog config validate`,
}

var configShowCmd = &cobra.Command{
	Use:   "show [flags]",
	Short: "Display active configuration in YAML or JSON",
	Long:  `Renders the loaded configuration structure with all resolved default values.`,
	Example: `  # Display configuration in YAML format
  watchdog config show

  # Display configuration in JSON format
  watchdog config show --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := globalCfg
		if cfg == nil {
			cfg = config.DefaultConfig()
		}

		if configShowJSON {
			data, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return NewExitError(ExitConfigError, "failed to format JSON: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}

		data, err := yaml.Marshal(cfg)
		if err != nil {
			return NewExitError(ExitConfigError, "failed to format YAML: %w", err)
		}
		fmt.Println(string(data))
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the active configuration file path",
	Long:  `Outputs the filesystem path of the loaded configuration file, or the default path if no file was loaded.`,
	Example: `  # Print config path
  watchdog config path`,
	Run: func(cmd *cobra.Command, args []string) {
		if loadedPath != "" {
			fmt.Println(loadedPath)
		} else {
			fmt.Println(config.GetDefaultConfigPath())
		}
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init [path] [flags]",
	Short: "Generate a new default configuration file",
	Long:  `Creates a well-documented default configuration file at $HOME/.watchdog/config.yaml or the specified target path.`,
	Example: `  # Initialize configuration at default location (~/.watchdog/config.yaml)
  watchdog config init

  # Initialize at custom path with overwrite
  watchdog config init /etc/watchdog/config.yaml --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dest := config.GetDefaultConfigPath()
		if len(args) > 0 {
			dest = args[0]
		}

		if _, err := os.Stat(dest); err == nil && !configInitForce {
			return NewExitError(ExitUsageError, "config file already exists at %s (use --force to overwrite)", dest)
		}

		cfg := config.DefaultConfig()
		if err := cfg.Save(dest); err != nil {
			return NewExitError(ExitGeneralError, "failed to write config file: %w", err)
		}

		fmt.Printf("✓ Successfully created default configuration at: %s\n", dest)
		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate current configuration file syntax and values",
	Long:  `Checks the configuration file for syntax errors, missing fields, and out-of-range thresholds.`,
	Example: `  # Validate default configuration
  watchdog config validate

  # Validate specific configuration file
  watchdog --config /etc/watchdog.yaml config validate`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := globalCfg
		if cfg == nil {
			return NewExitError(ExitConfigError, "no configuration loaded")
		}

		if err := cfg.Validate(); err != nil {
			return NewExitError(ExitConfigError, "configuration validation failed: %w", err)
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
