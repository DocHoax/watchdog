package diagnostics

import (
	"context"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

// Engine executes system diagnostic rules and aggregates health status.
type Engine struct {
	mu    sync.RWMutex
	rules []Rule
	cfg   *config.Config
}

// NewEngine creates a new diagnostic engine with default built-in rules.
func NewEngine(cfg *config.Config) *Engine {
	e := &Engine{
		cfg: cfg,
	}

	// Register standard diagnostic rules
	e.RegisterRule(&CPURule{})
	e.RegisterRule(&CPULoadRule{})
	e.RegisterRule(&MemoryRule{})
	e.RegisterRule(&SwapRule{})
	e.RegisterRule(&DiskSpaceRule{})
	e.RegisterRule(&InodeRule{})
	e.RegisterRule(&DNSResolutionRule{})
	e.RegisterRule(&NetworkConnectivityRule{})
	e.RegisterRule(&ProcessHealthRule{})
	e.RegisterRule(&DockerHealthRule{})

	return e
}

// RegisterRule registers a custom or additional diagnostic rule.
func (e *Engine) RegisterRule(r Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, r)
}

// Run executes all registered diagnostic checks against a snapshot.
func (e *Engine) Run(ctx context.Context, snapshot *model.SystemSnapshot) (*model.DiagnosticReport, error) {
	e.mu.RLock()
	rulesToRun := make([]Rule, len(e.rules))
	copy(rulesToRun, e.rules)
	e.mu.RUnlock()

	report := &model.DiagnosticReport{
		GeneratedAt: time.Now(),
		Results:     make([]model.DiagnosticResult, len(rulesToRun)),
	}

	var wg sync.WaitGroup
	for i, r := range rulesToRun {
		wg.Add(1)
		go func(idx int, rule Rule) {
			defer wg.Done()
			report.Results[idx] = rule.Evaluate(ctx, snapshot, e.cfg)
		}(i, r)
	}

	wg.Wait()

	// Aggregate status counts
	overall := model.StatusPass
	for _, res := range report.Results {
		if res.Status == model.StatusSkip {
			continue
		}

		report.TotalChecks++
		switch res.Status {
		case model.StatusPass:
			report.PassedChecks++
		case model.StatusWarning:
			report.WarningChecks++
			if overall != model.StatusFail {
				overall = model.StatusWarning
			}
		case model.StatusFail:
			report.CriticalChecks++
			overall = model.StatusFail
		}
	}

	report.OverallStatus = overall
	return report, nil
}
