package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderContainersView() string {
	var sb strings.Builder

	// 1. Docker Containers
	hasDocker := m.currentSnapshot != nil && m.currentSnapshot.Docker != nil
	sb.WriteString(SubTitleStyle.Render("🐳 Docker Containers") + "\n")
	if !hasDocker || len(m.currentSnapshot.Docker.Containers) == 0 {
		dockerMsg := "Docker daemon not running, not installed, or no containers found."
		if hasDocker && m.currentSnapshot.Docker.Error != "" {
			dockerMsg = "Docker error: " + m.currentSnapshot.Docker.Error
		}
		sb.WriteString(CardStyle.Width(m.width-4).Render(MutedStyle.Render(dockerMsg)) + "\n\n")
	} else {
		var dockerSb strings.Builder
		header := fmt.Sprintf("  %-14s %-22s %-20s %-12s %-8s %-8s %s",
			"CONTAINER ID", "NAME", "IMAGE", "STATUS", "CPU %", "MEM %", "PORTS")
		dockerSb.WriteString(TableHeaderStyle.Width(m.width-8).Render(header) + "\n")

		for _, c := range m.currentSnapshot.Docker.Containers {
			cName := c.ID
			if len(c.Names) > 0 {
				cName = strings.TrimPrefix(c.Names[0], "/")
			}
			ports := strings.Join(c.Ports, ", ")
			cpuPct := 0.0
			memPct := 0.0
			if c.Stats != nil {
				cpuPct = c.Stats.CPUPercent
				memPct = c.Stats.MemoryPercent
			}

			dockerSb.WriteString(fmt.Sprintf("  %-14s %-22s %-20s %-12s %6.1f%% %6.1f%% %s\n",
				TruncateString(c.ID, 12),
				TruncateString(cName, 20),
				TruncateString(c.Image, 18),
				TruncateString(c.Status, 10),
				cpuPct,
				memPct,
				TruncateString(ports, 24),
			))
		}
		sb.WriteString(CardStyle.Width(m.width-4).Render(dockerSb.String()) + "\n\n")
	}

	// 2. Kubernetes Pods
	hasK8s := m.currentSnapshot != nil && m.currentSnapshot.Kubernetes != nil
	sb.WriteString(SubTitleStyle.Render("☸️ Kubernetes Pods & Nodes") + "\n")
	if !hasK8s || len(m.currentSnapshot.Kubernetes.Pods) == 0 {
		k8sMsg := "Kubernetes in-cluster or kubeconfig context not available, or no pods found."
		if hasK8s && m.currentSnapshot.Kubernetes.Error != "" {
			k8sMsg = "Kubernetes error: " + m.currentSnapshot.Kubernetes.Error
		}
		sb.WriteString(CardStyle.Width(m.width - 4).Render(MutedStyle.Render(k8sMsg)))
	} else {
		var k8sSb strings.Builder
		header := fmt.Sprintf("  %-16s %-26s %-12s %-10s %-16s %s",
			"NAMESPACE", "POD NAME", "STATUS", "RESTARTS", "NODE", "AGE")
		k8sSb.WriteString(TableHeaderStyle.Width(m.width-8).Render(header) + "\n")

		for _, pod := range m.currentSnapshot.Kubernetes.Pods {
			k8sSb.WriteString(fmt.Sprintf("  %-16s %-26s %-12s %-10d %-16s %s\n",
				TruncateString(pod.Namespace, 14),
				TruncateString(pod.Name, 24),
				TruncateString(pod.Status, 10),
				pod.RestartsTotal,
				TruncateString(pod.NodeName, 14),
				FormatDuration(pod.Age),
			))
		}
		sb.WriteString(CardStyle.Width(m.width - 4).Render(k8sSb.String()))
	}

	return sb.String()
}
