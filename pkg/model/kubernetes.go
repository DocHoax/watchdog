package model

import "time"

// K8sNode represents a Kubernetes worker/control-plane node.
type K8sNode struct {
	Name             string            `json:"name" yaml:"name"`
	Status           string            `json:"status" yaml:"status"` // Ready, NotReady
	Roles            []string          `json:"roles" yaml:"roles"`
	Version          string            `json:"version" yaml:"version"`
	OSImage          string            `json:"os_image" yaml:"os_image"`
	KernelVersion    string            `json:"kernel_version" yaml:"kernel_version"`
	ContainerRuntime string            `json:"container_runtime" yaml:"container_runtime"`
	CPUCapacity      string            `json:"cpu_capacity" yaml:"cpu_capacity"`
	MemoryCapacity   string            `json:"memory_capacity" yaml:"memory_capacity"`
	PodsCount        int               `json:"pods_count" yaml:"pods_count"`
	Addresses        map[string]string `json:"addresses" yaml:"addresses"`
}

// K8sPod represents a Pod.
type K8sPod struct {
	Namespace     string            `json:"namespace" yaml:"namespace"`
	Name          string            `json:"name" yaml:"name"`
	Status        string            `json:"status" yaml:"status"` // Running, Pending, CrashLoopBackOff, Succeeded, Failed
	IP            string            `json:"ip,omitempty" yaml:"ip,omitempty"`
	NodeName      string            `json:"node_name" yaml:"node_name"`
	RestartsTotal int               `json:"restarts_total" yaml:"restarts_total"`
	Age           time.Duration     `json:"age" yaml:"age"`
	Containers    []string          `json:"containers" yaml:"containers"`
	Labels        map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// K8sDeployment represents a Deployment.
type K8sDeployment struct {
	Namespace         string `json:"namespace" yaml:"namespace"`
	Name              string `json:"name" yaml:"name"`
	ReplicasDesired   int32  `json:"replicas_desired" yaml:"replicas_desired"`
	ReplicasAvailable int32  `json:"replicas_available" yaml:"replicas_available"`
	ReplicasUpdated   int32  `json:"replicas_updated" yaml:"replicas_updated"`
}

// K8sEvent represents a cluster event.
type K8sEvent struct {
	Type      string    `json:"type" yaml:"type"` // Normal, Warning
	Reason    string    `json:"reason" yaml:"reason"`
	Message   string    `json:"message" yaml:"message"`
	Object    string    `json:"object" yaml:"object"`
	Count     int32     `json:"count" yaml:"count"`
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
}

// K8sSummary represents cluster overview.
type K8sSummary struct {
	Available        bool            `json:"available" yaml:"available"`
	ClusterName      string          `json:"cluster_name,omitempty" yaml:"cluster_name,omitempty"`
	ServerVersion    string          `json:"server_version,omitempty" yaml:"server_version,omitempty"`
	Nodes            []K8sNode       `json:"nodes" yaml:"nodes"`
	Pods             []K8sPod        `json:"pods" yaml:"pods"`
	Deployments      []K8sDeployment `json:"deployments" yaml:"deployments"`
	RecentEvents     []K8sEvent      `json:"recent_events,omitempty" yaml:"recent_events,omitempty"`
	TotalNodes       int             `json:"total_nodes" yaml:"total_nodes"`
	ReadyNodes       int             `json:"ready_nodes" yaml:"ready_nodes"`
	TotalPods        int             `json:"total_pods" yaml:"total_pods"`
	RunningPods      int             `json:"running_pods" yaml:"running_pods"`
	Error            string          `json:"error,omitempty" yaml:"error,omitempty"`
	CollectedAt      time.Time       `json:"collected_at" yaml:"collected_at"`
}
