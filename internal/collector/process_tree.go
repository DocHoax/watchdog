package collector

import (
	"context"
	"fmt"
	"strings"

	"github.com/DocHoax/watchdog/pkg/model"
)

// GetProcessTree retrieves the complete process hierarchy.
func (c *ProcessCollector) GetProcessTree(ctx context.Context) ([]*model.ProcessInfo, error) {
	return c.BuildProcessTree(ctx, "")
}

// BuildProcessTree constructs a tree hierarchy of processes.
func (c *ProcessCollector) BuildProcessTree(ctx context.Context, filter string) ([]*model.ProcessInfo, error) {
	summary, err := c.GetProcesses(ctx, 0, "pid", "")
	if err != nil {
		return nil, err
	}

	procMap := make(map[int32]*model.ProcessInfo)
	var rootProcs []*model.ProcessInfo

	for i := range summary.Processes {
		pCopy := summary.Processes[i]
		procMap[pCopy.PID] = &pCopy
	}

	for _, p := range procMap {
		parent, exists := procMap[p.PPID]
		if exists && p.PPID != p.PID {
			parent.Children = append(parent.Children, p)
		} else {
			rootProcs = append(rootProcs, p)
		}
	}

	if filter != "" {
		filterLower := strings.ToLower(filter)
		var filteredRoots []*model.ProcessInfo
		for _, r := range rootProcs {
			if treeContains(r, filterLower) {
				filteredRoots = append(filteredRoots, r)
			}
		}
		return filteredRoots, nil
	}

	return rootProcs, nil
}

func treeContains(p *model.ProcessInfo, filter string) bool {
	if strings.Contains(strings.ToLower(p.Name), filter) || strings.Contains(fmt.Sprintf("%d", p.PID), filter) {
		return true
	}
	for _, child := range p.Children {
		if treeContains(child, filter) {
			return true
		}
	}
	return false
}

// RenderProcessTreeText formats a process tree into text for CLI output.
func RenderProcessTreeText(roots []*model.ProcessInfo, maxDepth int) string {
	var sb strings.Builder
	for _, r := range roots {
		renderTreeNode(&sb, r, "", true, 0, maxDepth)
	}
	return sb.String()
}

func renderTreeNode(sb *strings.Builder, p *model.ProcessInfo, prefix string, isLast bool, depth int, maxDepth int) {
	if maxDepth > 0 && depth > maxDepth {
		return
	}

	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if depth == 0 {
		connector = ""
	}

	sb.WriteString(fmt.Sprintf("%s%s[%d] %s (CPU: %.1f%%, MEM: %.1f%%)\n",
		prefix, connector, p.PID, p.Name, p.CPUPercent, p.MemoryPercent))

	childPrefix := prefix
	if depth > 0 {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, child := range p.Children {
		isLastChild := i == len(p.Children)-1
		renderTreeNode(sb, child, childPrefix, isLastChild, depth+1, maxDepth)
	}
}
