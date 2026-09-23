package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/watchdog-cli/watchdog/pkg/model"
)

// DockerCollector gathers container metrics from the local Docker daemon.
type DockerCollector struct {
	client *http.Client
}

// NewDockerCollector creates a new DockerCollector.
func NewDockerCollector() *DockerCollector {
	return &DockerCollector{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Name returns the collector identifier.
func (c *DockerCollector) Name() string {
	return "docker"
}

// Collect retrieves Docker metrics.
func (c *DockerCollector) Collect(ctx context.Context) (any, error) {
	return c.GetDockerSummary(ctx)
}

// GetDockerSummary queries Docker container states and summary info.
// It gracefully degrades if Docker is not installed or not running.
func (c *DockerCollector) GetDockerSummary(ctx context.Context) (*model.DockerSummary, error) {
	summary := &model.DockerSummary{
		Available:   false,
		CollectedAt: time.Now(),
		Containers:  []model.DockerContainer{},
	}

	// First attempt via docker CLI if available for fast cross-platform socket access
	cmd := exec.CommandContext(ctx, "docker", "version", "--format", "{{json .}}")
	out, err := cmd.Output()
	if err != nil {
		// Docker is likely not running or not installed
		summary.Error = "Docker daemon is not running or docker CLI is not installed"
		return summary, nil
	}

	summary.Available = true
	var versionInfo struct {
		Client struct {
			Version    string `json:"Version"`
			APIVersion string `json:"ApiVersion"`
		} `json:"Client"`
		Server struct {
			Version    string `json:"Version"`
			APIVersion string `json:"ApiVersion"`
		} `json:"Server"`
	}
	if err := json.Unmarshal(out, &versionInfo); err == nil {
		summary.Version = versionInfo.Server.Version
		if summary.Version == "" {
			summary.Version = versionInfo.Client.Version
		}
		summary.APIVersion = versionInfo.Server.APIVersion
		if summary.APIVersion == "" {
			summary.APIVersion = versionInfo.Client.APIVersion
		}
	}

	// Fetch container list
	psCmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{json .}}")
	psOut, err := psCmd.Output()
	if err != nil {
		summary.Error = fmt.Sprintf("Failed to list containers: %v", err)
		return summary, nil
	}

	lines := strings.Split(string(psOut), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var raw struct {
			ID        string `json:"ID"`
			Names     string `json:"Names"`
			Image     string `json:"Image"`
			CreatedAt string `json:"CreatedAt"`
			State     string `json:"State"`
			Status    string `json:"Status"`
			Ports     string `json:"Ports"`
			Command   string `json:"Command"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		summary.ContainersTotal++
		st := strings.ToLower(raw.State)
		if st == "running" {
			summary.RunningCount++
		} else if st == "paused" {
			summary.PausedCount++
		} else {
			summary.StoppedCount++
		}

		var names []string
		if raw.Names != "" {
			names = strings.Split(raw.Names, ",")
		}

		var portList []string
		if raw.Ports != "" {
			portList = strings.Split(raw.Ports, ", ")
		}

		container := model.DockerContainer{
			ID:      raw.ID,
			Names:   names,
			Image:   raw.Image,
			Command: raw.Command,
			State:   raw.State,
			Status:  raw.Status,
			Ports:   portList,
		}

		summary.Containers = append(summary.Containers, container)
	}

	// Get images count
	imgCmd := exec.CommandContext(ctx, "docker", "images", "-q")
	if imgOut, err := imgCmd.Output(); err == nil {
		imgLines := strings.Split(strings.TrimSpace(string(imgOut)), "\n")
		count := 0
		for _, l := range imgLines {
			if strings.TrimSpace(l) != "" {
				count++
			}
		}
		summary.ImagesCount = count
	}

	// If there are running containers, optionally sample stats for the first top containers
	c.enrichContainerStats(ctx, summary)

	return summary, nil
}

// enrichContainerStats queries docker stats for running containers
func (c *DockerCollector) enrichContainerStats(ctx context.Context, summary *model.DockerSummary) {
	if summary.RunningCount == 0 {
		return
	}

	statsCmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", "{{json .}}")
	out, err := statsCmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(out), "\n")
	statsMap := make(map[string]*model.ContainerStats)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var raw struct {
			ID       string `json:"ID"`
			CPUPerc  string `json:"CPUPerc"`
			MemUsage string `json:"MemUsage"`
			MemPerc  string `json:"MemPerc"`
			NetIO    string `json:"NetIO"`
			BlockIO  string `json:"BlockIO"`
			PIDs     string `json:"PIDs"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		cpuVal, _ := strconv.ParseFloat(strings.TrimSuffix(raw.CPUPerc, "%"), 64)
		memPercVal, _ := strconv.ParseFloat(strings.TrimSuffix(raw.MemPerc, "%"), 64)
		pidsVal, _ := strconv.ParseUint(raw.PIDs, 10, 64)

		statsMap[raw.ID] = &model.ContainerStats{
			CPUPercent:    cpuVal,
			MemoryPercent: memPercVal,
			PIDs:          pidsVal,
		}
	}

	for i := range summary.Containers {
		id := summary.Containers[i].ID
		if st, ok := statsMap[id]; ok {
			summary.Containers[i].Stats = st
		}
	}
}

// InspectContainer gets detailed metrics for a single container.
func (c *DockerCollector) InspectContainer(ctx context.Context, containerID string) (*model.DockerContainer, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerID)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	var data []map[string]any
	if err := json.Unmarshal(out, &data); err != nil || len(data) == 0 {
		return nil, fmt.Errorf("invalid inspect payload for container %s", containerID)
	}

	target := data[0]
	name, _ := target["Name"].(string)
	id, _ := target["Id"].(string)
	if len(id) > 12 {
		id = id[:12]
	}

	stateMap, _ := target["State"].(map[string]any)
	stateStr := "unknown"
	statusStr := ""
	if stateMap != nil {
		if s, ok := stateMap["Status"].(string); ok {
			stateStr = s
			statusStr = s
		}
	}

	configMap, _ := target["Config"].(map[string]any)
	imageStr := ""
	if configMap != nil {
		if img, ok := configMap["Image"].(string); ok {
			imageStr = img
		}
	}

	return &model.DockerContainer{
		ID:      id,
		Names:   []string{strings.TrimPrefix(name, "/")},
		Image:   imageStr,
		State:   stateStr,
		Status:  statusStr,
		Created: time.Now(),
	}, nil
}
