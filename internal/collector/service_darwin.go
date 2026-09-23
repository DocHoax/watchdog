//go:build darwin

package collector

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/watchdog-cli/watchdog/pkg/model"
)

// GetServices queries macOS launchd services.
func (c *ServiceCollector) GetServices(ctx context.Context) ([]model.ServiceInfo, error) {
	cmd := exec.CommandContext(ctx, "launchctl", "list")
	out, err := cmd.Output()
	if err != nil {
		return []model.ServiceInfo{}, nil
	}

	lines := strings.Split(string(out), "\n")
	var services []model.ServiceInfo

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if i == 0 || line == "" {
			continue // skip header PID Status Label
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		pidStr := fields[0]
		label := fields[2]

		status := model.ServiceStateStopped
		if pidStr != "-" {
			status = model.ServiceStateRunning
		}

		services = append(services, model.ServiceInfo{
			Name:        label,
			DisplayName: label,
			Status:      status,
			Manager:     "launchd",
			CollectedAt: time.Now(),
		})
	}

	return services, nil
}
