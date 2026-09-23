package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/watchdog-cli/watchdog/pkg/model"
)

// KubernetesCollector queries cluster metadata and health from a reachable Kubernetes cluster.
type KubernetesCollector struct{}

// NewKubernetesCollector creates a new KubernetesCollector.
func NewKubernetesCollector() *KubernetesCollector {
	return &KubernetesCollector{}
}

// Name returns the collector identifier.
func (c *KubernetesCollector) Name() string {
	return "kubernetes"
}

// Collect retrieves Kubernetes cluster summary metrics.
func (c *KubernetesCollector) Collect(ctx context.Context) (any, error) {
	return c.GetClusterSummary(ctx)
}

// GetClusterSummary collects node, pod, and deployment statuses from the configured cluster context.
func (c *KubernetesCollector) GetClusterSummary(ctx context.Context) (*model.K8sSummary, error) {
	summary := &model.K8sSummary{
		Available:   false,
		CollectedAt: time.Now(),
		Nodes:       []model.K8sNode{},
		Pods:        []model.K8sPod{},
		Deployments: []model.K8sDeployment{},
	}

	// Test kubectl connectivity and cluster info
	verCmd := exec.CommandContext(ctx, "kubectl", "version", "--output=json")
	verOut, err := verCmd.Output()
	if err != nil {
		summary.Error = "Kubernetes cluster is not configured or kubectl CLI is unavailable"
		return summary, nil
	}

	summary.Available = true
	var versionData struct {
		ServerVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"serverVersion"`
	}
	if err := json.Unmarshal(verOut, &versionData); err == nil {
		summary.ServerVersion = versionData.ServerVersion.GitVersion
	}

	// Get current context name
	ctxCmd := exec.CommandContext(ctx, "kubectl", "config", "current-context")
	if ctxOut, err := ctxCmd.Output(); err == nil {
		summary.ClusterName = strings.TrimSpace(string(ctxOut))
	}

	// Fetch Nodes
	c.collectNodes(ctx, summary)

	// Fetch Pods
	c.collectPods(ctx, summary)

	// Fetch Deployments
	c.collectDeployments(ctx, summary)

	// Fetch Events
	c.collectEvents(ctx, summary)

	return summary, nil
}

func (c *KubernetesCollector) collectNodes(ctx context.Context, summary *model.K8sSummary) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "nodes", "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	var raw struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
				NodeInfo struct {
					KubeletVersion          string `json:"kubeletVersion"`
					OSImage                 string `json:"osImage"`
					KernelVersion           string `json:"kernelVersion"`
					ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
				} `json:"nodeInfo"`
				Capacity struct {
					CPU    string `json:"cpu"`
					Memory string `json:"memory"`
					Pods   string `json:"pods"`
				} `json:"capacity"`
				Addresses []struct {
					Type    string `json:"type"`
					Address string `json:"address"`
				} `json:"addresses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(out, &raw); err != nil {
		return
	}

	summary.TotalNodes = len(raw.Items)
	for _, item := range raw.Items {
		nodeStatus := "NotReady"
		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				nodeStatus = "Ready"
				summary.ReadyNodes++
				break
			}
		}

		var roles []string
		for k := range item.Metadata.Labels {
			if strings.HasPrefix(k, "node-role.kubernetes.io/") {
				role := strings.TrimPrefix(k, "node-role.kubernetes.io/")
				roles = append(roles, role)
			}
		}
		if len(roles) == 0 {
			roles = append(roles, "worker")
		}

		addrMap := make(map[string]string)
		for _, addr := range item.Status.Addresses {
			addrMap[addr.Type] = addr.Address
		}

		summary.Nodes = append(summary.Nodes, model.K8sNode{
			Name:             item.Metadata.Name,
			Status:           nodeStatus,
			Roles:            roles,
			Version:          item.Status.NodeInfo.KubeletVersion,
			OSImage:          item.Status.NodeInfo.OSImage,
			KernelVersion:    item.Status.NodeInfo.KernelVersion,
			ContainerRuntime: item.Status.NodeInfo.ContainerRuntimeVersion,
			CPUCapacity:      item.Status.Capacity.CPU,
			MemoryCapacity:   item.Status.Capacity.Memory,
			Addresses:        addrMap,
		})
	}
}

func (c *KubernetesCollector) collectPods(ctx context.Context, summary *model.K8sSummary) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "--all-namespaces", "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	var raw struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				Namespace         string            `json:"namespace"`
				CreationTimestamp time.Time         `json:"creationTimestamp"`
				Labels            map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				NodeName   string `json:"nodeName"`
				Containers []struct {
					Name string `json:"name"`
				} `json:"containers"`
			} `json:"spec"`
			Status struct {
				Phase             string `json:"phase"`
				PodIP             string `json:"podIP"`
				ContainerStatuses []struct {
					RestartCount int `json:"restartCount"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(out, &raw); err != nil {
		return
	}

	summary.TotalPods = len(raw.Items)
	for _, item := range raw.Items {
		phase := item.Status.Phase
		if strings.EqualFold(phase, "Running") {
			summary.RunningPods++
		}

		var containers []string
		for _, cn := range item.Spec.Containers {
			containers = append(containers, cn.Name)
		}

		totalRestarts := 0
		for _, cs := range item.Status.ContainerStatuses {
			totalRestarts += cs.RestartCount
		}

		age := time.Since(item.Metadata.CreationTimestamp)
		if age < 0 {
			age = 0
		}

		summary.Pods = append(summary.Pods, model.K8sPod{
			Namespace:     item.Metadata.Namespace,
			Name:          item.Metadata.Name,
			Status:        phase,
			IP:            item.Status.PodIP,
			NodeName:      item.Spec.NodeName,
			RestartsTotal: totalRestarts,
			Age:           age,
			Containers:    containers,
			Labels:        item.Metadata.Labels,
		})
	}
}

func (c *KubernetesCollector) collectDeployments(ctx context.Context, summary *model.K8sSummary) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "deployments", "--all-namespaces", "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	var raw struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Spec struct {
				Replicas int32 `json:"replicas"`
			} `json:"spec"`
			Status struct {
				AvailableReplicas int32 `json:"availableReplicas"`
				UpdatedReplicas   int32 `json:"updatedReplicas"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(out, &raw); err != nil {
		return
	}

	for _, item := range raw.Items {
		summary.Deployments = append(summary.Deployments, model.K8sDeployment{
			Namespace:         item.Metadata.Namespace,
			Name:              item.Metadata.Name,
			ReplicasDesired:   item.Spec.Replicas,
			ReplicasAvailable: item.Status.AvailableReplicas,
			ReplicasUpdated:   item.Status.UpdatedReplicas,
		})
	}
}

func (c *KubernetesCollector) collectEvents(ctx context.Context, summary *model.K8sSummary) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "events", "--all-namespaces", "--sort-by=.metadata.creationTimestamp", "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	var raw struct {
		Items []struct {
			Type           string `json:"type"`
			Reason         string `json:"reason"`
			Message        string `json:"message"`
			Count          int32  `json:"count"`
			InvolvedObject struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			} `json:"involvedObject"`
			LastTimestamp time.Time `json:"lastTimestamp"`
		} `json:"items"`
	}

	if err := json.Unmarshal(out, &raw); err != nil {
		return
	}

	// Capture latest 20 events
	startIdx := 0
	if len(raw.Items) > 20 {
		startIdx = len(raw.Items) - 20
	}

	for i := len(raw.Items) - 1; i >= startIdx; i-- {
		item := raw.Items[i]
		objStr := fmt.Sprintf("%s/%s", item.InvolvedObject.Kind, item.InvolvedObject.Name)
		summary.RecentEvents = append(summary.RecentEvents, model.K8sEvent{
			Type:      item.Type,
			Reason:    item.Reason,
			Message:   item.Message,
			Object:    objStr,
			Count:     item.Count,
			Timestamp: item.LastTimestamp,
		})
	}
}
