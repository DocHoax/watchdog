//go:build windows

package collector

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// GetServices queries Windows services using PowerShell / sc.exe.
func (c *ServiceCollector) GetServices(ctx context.Context) ([]model.ServiceInfo, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
		"Get-Service | Select-Object -Property Name, DisplayName, Status, StartType | ConvertTo-Csv -NoTypeInformation")

	out, err := cmd.Output()
	if err != nil {
		// Fallback to minimal placeholder if PowerShell is restricted
		return []model.ServiceInfo{}, nil
	}

	lines := strings.Split(string(out), "\n")
	var services []model.ServiceInfo

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if i == 0 || line == "" {
			continue // skip header
		}

		parts := parseCSVLine(line)
		if len(parts) < 3 {
			continue
		}

		name := parts[0]
		displayName := parts[1]
		statusStr := strings.ToLower(parts[2])
		startType := ""
		if len(parts) > 3 {
			startType = parts[3]
		}

		status := model.ServiceStateUnknown
		if strings.Contains(statusStr, "running") {
			status = model.ServiceStateRunning
		} else if strings.Contains(statusStr, "stopped") {
			status = model.ServiceStateStopped
		} else if strings.Contains(statusStr, "paused") {
			status = model.ServiceStatePaused
		}

		services = append(services, model.ServiceInfo{
			Name:        name,
			DisplayName: displayName,
			Status:      status,
			StartType:   startType,
			Manager:     "windows-scm",
			CollectedAt: time.Now(),
		})
	}

	return services, nil
}

func parseCSVLine(line string) []string {
	var parts []string
	var cur strings.Builder
	inQuotes := false

	for _, ch := range line {
		if ch == '"' {
			inQuotes = !inQuotes
		} else if ch == ',' && !inQuotes {
			parts = append(parts, strings.TrimSpace(cur.String()))
			cur.Reset()
		} else {
			cur.WriteRune(ch)
		}
	}
	parts = append(parts, strings.TrimSpace(cur.String()))
	return parts
}
