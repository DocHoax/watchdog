//go:build linux

package collector

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// GetServices queries Linux systemd / sysvinit services.
func (c *ServiceCollector) GetServices(ctx context.Context) ([]model.ServiceInfo, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "list-units", "--type=service", "--all", "--no-pager", "--no-legend")
	out, err := cmd.Output()
	if err != nil {
		// systemctl might not be present (e.g. containers, alpine)
		return []model.ServiceInfo{}, nil
	}

	lines := strings.Split(string(out), "\n")
	var services []model.ServiceInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		unitName := fields[0]
		loadState := fields[1]
		activeState := fields[2]
		subState := fields[3]
		desc := ""
		if len(fields) > 4 {
			desc = strings.Join(fields[4:], " ")
		}

		status := model.ServiceStateUnknown
		if activeState == "active" && subState == "running" {
			status = model.ServiceStateRunning
		} else if activeState == "inactive" || subState == "dead" {
			status = model.ServiceStateStopped
		}

		services = append(services, model.ServiceInfo{
			Name:        unitName,
			DisplayName: desc,
			Status:      status,
			StartType:   loadState,
			Description: desc,
			Manager:     "systemd",
			CollectedAt: time.Now(),
		})
	}

	return services, nil
}
