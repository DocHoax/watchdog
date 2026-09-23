package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/watchdog-cli/watchdog/internal/collector"
	"github.com/watchdog-cli/watchdog/internal/diagnostics"
	"github.com/watchdog-cli/watchdog/internal/logger"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

var (
	diagCategory string
	diagFix      bool
	diagJSON     bool
	diagPlain    bool
	diagTimeout  time.Duration
)

var diagnoseCmd = &cobra.Command{
	Use:     "diagnose",
	Aliases: []string{"diag", "check", "doctor"},
	Short:   "Execute automated system health diagnostics and checks",
	Long: `Runs an extensive suite of concurrent system health checks evaluating:
- CPU saturation and core run-queue load average
- RAM utilization, paging pressure, and swap exhaustion
- Disk partition space utilization and filesystem inode limits
- Network connectivity, gateway reachability, and DNS resolution latency
- Process table anomalies, zombie tasks, and runaway rogue processes
- Container runtime health, crash-loops, and pod stability

Optionally provides automated remediation recommendations or interactive fixes.`,
	RunE: runDiagnose,
}

func runDiagnose(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	col := collector.NewManager(cfg)

	if diagTimeout <= 0 {
		diagTimeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), diagTimeout)
	defer cancel()

	logger.Infof("Collecting system snapshot for diagnostic evaluation...")
	snap, err := col.Collect(ctx)
	if err != nil {
		logger.Warnf("Partial metrics collection: %v", err)
	}

	diagEngine := diagnostics.NewEngine(cfg)
	report, err := diagEngine.Run(ctx, snap)
	if err != nil {
		return fmt.Errorf("diagnostic execution failed: %w", err)
	}

	// Filter by category if requested
	if diagCategory != "" {
		var filtered []model.DiagnosticResult
		for _, r := range report.Results {
			if strings.EqualFold(r.Category, diagCategory) {
				filtered = append(filtered, r)
			}
		}
		report.Results = filtered
		// Recalculate summary counts
		report.TotalChecks = len(filtered)
		report.PassedChecks = 0
		report.WarningChecks = 0
		report.CriticalChecks = 0
		report.OverallStatus = model.StatusPass
		for _, r := range filtered {
			switch r.Status {
			case model.StatusPass:
				report.PassedChecks++
			case model.StatusWarning:
				report.WarningChecks++
				if report.OverallStatus != model.StatusFail {
					report.OverallStatus = model.StatusWarning
				}
			case model.StatusFail:
				report.CriticalChecks++
				report.OverallStatus = model.StatusFail
			}
		}
	}

	// Output format
	if diagJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		renderDiagnosticResults(report, diagPlain)
	}

	// Remediations if requested
	if diagFix && (report.WarningChecks > 0 || report.CriticalChecks > 0) {
		runRemediation(report)
	}

	if report.OverallStatus == model.StatusFail {
		os.Exit(1)
	}

	return nil
}

func renderDiagnosticResults(report *model.DiagnosticReport, plain bool) {
	cReset := "\033[0m"
	cBold := "\033[1m"
	cDim := "\033[2m"
	cRed := "\033[31m"
	cGreen := "\033[32m"
	cYellow := "\033[33m"
	cCyan := "\033[36m"
	cWhite := "\033[37m"

	if plain || noColor {
		cReset = ""
		cBold = ""
		cDim = ""
		cRed = ""
		cGreen = ""
		cYellow = ""
		cCyan = ""
		cWhite = ""
	}

	fmt.Println()
	fmt.Printf("%s%s=== WATCHDOG SYSTEM DIAGNOSTICS ===%s\n", cBold, cCyan, cReset)
	fmt.Printf("%sEvaluated at: %s | Total Checks: %d%s\n\n",
		cDim, report.GeneratedAt.Format("2006-01-02 15:04:05 MST"), report.TotalChecks, cReset)

	// Status badge
	var statusBadge string
	switch report.OverallStatus {
	case model.StatusPass:
		statusBadge = fmt.Sprintf("%s%s[ PASS - SYSTEM HEALTHY ]%s", cBold, cGreen, cReset)
	case model.StatusWarning:
		statusBadge = fmt.Sprintf("%s%s[ WARN - ATTENTION NEEDED ]%s", cBold, cYellow, cReset)
	case model.StatusFail:
		statusBadge = fmt.Sprintf("%s%s[ FAIL - CRITICAL ISSUES DETECTED ]%s", cBold, cRed, cReset)
	default:
		statusBadge = fmt.Sprintf("%s[ UNKNOWN ]%s", cDim, cReset)
	}

	fmt.Printf("  Overall Status : %s\n", statusBadge)
	fmt.Printf("  Summary        : %s%d Passed%s | %s%d Warnings%s | %s%d Critical%s\n\n",
		cGreen, report.PassedChecks, cReset,
		cYellow, report.WarningChecks, cReset,
		cRed, report.CriticalChecks, cReset,
	)

	fmt.Printf("%s%-10s %-8s %-26s %-24s %s%s\n",
		cBold+cWhite, "STATUS", "SEVERITY", "CHECK", "METRIC / VALUE", "DESCRIPTION", cReset)
	fmt.Println(strings.Repeat("-", 90))

	for _, res := range report.Results {
		if res.Status == model.StatusSkip {
			continue
		}

		var stTag string
		switch res.Status {
		case model.StatusPass:
			stTag = fmt.Sprintf("%s[PASS]%s", cGreen, cReset)
		case model.StatusWarning:
			stTag = fmt.Sprintf("%s[WARN]%s", cYellow+cBold, cReset)
		case model.StatusFail:
			stTag = fmt.Sprintf("%s[FAIL]%s", cRed+cBold, cReset)
		default:
			stTag = fmt.Sprintf("%s[SKIP]%s", cDim, cReset)
		}

		var sevTag string
		switch res.Severity {
		case model.SeverityCritical:
			sevTag = fmt.Sprintf("%sCRIT%s", cRed+cBold, cReset)
		case model.SeverityWarning:
			sevTag = fmt.Sprintf("%sWARN%s", cYellow, cReset)
		default:
			sevTag = fmt.Sprintf("%sINFO%s", cDim, cReset)
		}

		val := res.MetricValue
		if len(val) > 22 {
			val = val[:19] + "..."
		}

		fmt.Printf("%-19s %-16s %-26s %-24s %s\n",
			stTag, sevTag, res.Name, val, res.Description)

		if (res.Status == model.StatusWarning || res.Status == model.StatusFail) && res.Recommendation != "" {
			fmt.Printf("   %s└─ Remediation:%s %s%s%s\n",
				cCyan, cReset, cYellow, res.Recommendation, cReset)
		}
	}
	fmt.Println()
}

func runRemediation(report *model.DiagnosticReport) {
	fmt.Printf("\n\033[1m\033[36m=== AUTOMATED REMEDIATION PLAN ===\033[0m\n")
	remediationSteps := 0

	for _, res := range report.Results {
		if res.Status == model.StatusWarning || res.Status == model.StatusFail {
			if res.Recommendation != "" {
				remediationSteps++
				fmt.Printf("[%d] %s (%s)\n", remediationSteps, res.Name, res.Category)
				fmt.Printf("    Problem : %s\n", res.Description)
				fmt.Printf("    Action  : %s\n\n", res.Recommendation)
			}
		}
	}

	if remediationSteps == 0 {
		fmt.Println("No automated remediation required. System is healthy.")
	} else {
		fmt.Printf("Total %d actionable remediation steps identified.\n", remediationSteps)
	}
}

func init() {
	diagnoseCmd.Flags().StringVarP(&diagCategory, "category", "C", "", "filter checks by category (CPU, Memory, Disk, Network, Process, Docker)")
	diagnoseCmd.Flags().BoolVarP(&diagFix, "fix", "F", false, "display actionable remediation steps for detected issues")
	diagnoseCmd.Flags().BoolVar(&diagJSON, "json", false, "output diagnostic results in JSON format")
	diagnoseCmd.Flags().BoolVar(&diagPlain, "plain", false, "disable ANSI styling for plaintext output")
	diagnoseCmd.Flags().DurationVarP(&diagTimeout, "timeout", "t", 10*time.Second, "maximum execution timeout for all diagnostic checks")

	RootCmd.AddCommand(diagnoseCmd)
}
