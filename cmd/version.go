package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// Version is the current semantic version of Watchdog.
	Version = "1.0.0"
	// GitCommit is the git commit hash injected at build time via ldflags.
	GitCommit = "dev"
	// Commit is maintained for backwards-compatibility with legacy ldflags.
	Commit = "dev"
	// BuildDate is the timestamp when the binary was built.
	BuildDate = "2026-09-23"
	// BuiltBy is the entity or tool that compiled the binary (e.g. goreleaser, docker).
	BuiltBy = ""
)

var (
	versionShort bool
	versionJSON  bool
)

// VersionInfo encapsulates detailed version metadata.
type VersionInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit,omitempty"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	BuiltBy   string `json:"built_by,omitempty"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
	Compiler  string `json:"compiler"`
}

var versionCmd = &cobra.Command{
	Use:   "version [flags]",
	Short: "Print Watchdog version and build information",
	Long:  `Displays semantic version number, git commit hash, build timestamp, Go runtime version, target architecture, and compiler.`,
	Example: `  # Print human-readable version summary
  watchdog version

  # Print only the semantic version string (e.g. for scripts/CI)
  watchdog version --short

  # Print version details formatted as JSON
  watchdog version --json`,
	Run: func(cmd *cobra.Command, args []string) {
		effectiveCommit := GitCommit
		if effectiveCommit == "dev" && Commit != "dev" && Commit != "" {
			effectiveCommit = Commit
		}

		info := VersionInfo{
			Version:   Version,
			GitCommit: effectiveCommit,
			Commit:    effectiveCommit,
			BuildDate: BuildDate,
			BuiltBy:   BuiltBy,
			GoVersion: runtime.Version(),
			Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			Compiler:  runtime.Compiler,
		}

		if versionShort {
			fmt.Println(info.Version)
			return
		}

		if versionJSON {
			data, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Printf("🐺 Watchdog v%s (%s)\n", strings.TrimPrefix(info.Version, "v"), info.Commit)
		fmt.Printf("   Build Date : %s\n", info.BuildDate)
		if info.BuiltBy != "" {
			fmt.Printf("   Built By   : %s\n", info.BuiltBy)
		}
		fmt.Printf("   Go Version : %s\n", info.GoVersion)
		fmt.Printf("   Platform   : %s\n", info.Platform)
		fmt.Printf("   Compiler   : %s\n", info.Compiler)
	},
}

func init() {
	versionCmd.Flags().BoolVarP(&versionShort, "short", "s", false, "display only the version number")
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "display version information as JSON")

	RootCmd.AddCommand(versionCmd)
}
