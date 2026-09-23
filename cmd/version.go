package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// Version is the current version of Watchdog.
	Version = "1.0.0"
	// Commit is the git commit hash at build time.
	Commit = "dev"
	// BuildDate is the timestamp when the binary was built.
	BuildDate = "2026-09-23"
)

var (
	versionShort bool
	versionJSON  bool
)

// VersionInfo encapsulates detailed version metadata.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
	Compiler  string `json:"compiler"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print Watchdog version and build information",
	Long:  `Displays version, git commit, build date, Go compiler version, and target platform.`,
	Run: func(cmd *cobra.Command, args []string) {
		info := VersionInfo{
			Version:   Version,
			Commit:    Commit,
			BuildDate: BuildDate,
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

		fmt.Printf("🐺 Watchdog v%s (%s)\n", info.Version, info.Commit)
		fmt.Printf("   Build Date : %s\n", info.BuildDate)
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
