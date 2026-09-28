package topology

import (
	"fmt"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// Builder passively discovers, constructs, and updates topology graphs
// from system snapshots, fleet telemetry, container data, and declared models.
type Builder struct{}

// NewBuilder creates a new passive topology Builder instance.
func NewBuilder() *Builder {
	return &Builder{}
}

// IngestSnapshot extracts topology nodes and dependency relationships from a single SystemSnapshot.
func (b *Builder) IngestSnapshot(g *Graph, snapshot *model.SystemSnapshot, defaultHostID string) {
	if snapshot == nil {
		return
	}

	now := time.Now().UTC()
	if !snapshot.Timestamp.IsZero() {
		now = snapshot.Timestamp
	}

	// 1. Process Host Node
	hostID := defaultHostID
	hostname := defaultHostID
	if snapshot.System != nil {
		if snapshot.System.HostID != "" {
			hostID = snapshot.System.HostID
		}
		if snapshot.System.Hostname != "" {
			hostname = snapshot.System.Hostname
		}
	}
	if hostID == "" {
		hostID = "local-host"
		hostname = "localhost"
	}

	hostNodeID := fmt.Sprintf("host-%s", hostID)
	hostNode := TopologyNode{
		ID:            hostNodeID,
		Name:          hostname,
		Type:          NodeTypePhysicalHost,
		Status:        NodeStatusHealthy,
		HostID:        hostID,
		Source:        "telemetry_system",
		FirstObserved: now,
		LastObserved:  now,
		Tags: map[string]string{
			"hostname": hostname,
		},
		Metadata: make(map[string]string),
	}
	if snapshot.System != nil {
		if snapshot.System.OS != "" {
			hostNode.Metadata["os"] = snapshot.System.OS
		}
		if snapshot.System.Platform != "" {
			hostNode.Metadata["platform"] = snapshot.System.Platform
		}
		if snapshot.System.KernelVersion != "" {
			hostNode.Metadata["kernel"] = snapshot.System.KernelVersion
		}
	}
	g.AddNode(hostNode)

	// 2. Process Docker Containers
	if snapshot.Docker != nil && snapshot.Docker.Available {
		for _, c := range snapshot.Docker.Containers {
			cID := c.ID
			if len(cID) > 12 {
				cID = cID[:12]
			}
			containerNodeID := fmt.Sprintf("container-%s", cID)

			name := cID
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}

			nodeType := b.inferNodeTypeFromImageOrName(c.Image, name)
			status := b.mapContainerStatus(c.State, c.RestartCount)

			containerNode := TopologyNode{
				ID:            containerNodeID,
				Name:          name,
				Type:          nodeType,
				Status:        status,
				HostID:        hostID,
				Source:        "telemetry_docker",
				FirstObserved: now,
				LastObserved:  now,
				Tags: map[string]string{
					"image": c.Image,
					"state": c.State,
				},
				Metadata: map[string]string{
					"command":       c.Command,
					"restart_count": fmt.Sprintf("%d", c.RestartCount),
				},
			}
			g.AddNode(containerNode)

			// Dependency: Container runs on Host
			g.AddDependency(Dependency{
				SourceID:      containerNodeID,
				TargetID:      hostNodeID,
				Type:          RelRunsOn,
				Weight:        1.0,
				Confidence:    ConfidenceHigh,
				Evidence:      []string{"host_container_mapping", "docker_engine"},
				FirstObserved: now,
				LastObserved:  now,
			})

			// Dependency: Host hosts Container
			g.AddDependency(Dependency{
				SourceID:      hostNodeID,
				TargetID:      containerNodeID,
				Type:          RelHosts,
				Weight:        1.0,
				Confidence:    ConfidenceHigh,
				Evidence:      []string{"host_container_mapping", "docker_engine"},
				FirstObserved: now,
				LastObserved:  now,
			})
		}
	}

	// 3. Process Kubernetes entities
	if snapshot.Kubernetes != nil && snapshot.Kubernetes.Available {
		for _, kn := range snapshot.Kubernetes.Nodes {
			k8sNodeID := fmt.Sprintf("k8s-node-%s", kn.Name)
			k8sStatus := NodeStatusHealthy
			if kn.Status != "Ready" {
				k8sStatus = NodeStatusDegraded
			}

			g.AddNode(TopologyNode{
				ID:            k8sNodeID,
				Name:          kn.Name,
				Type:          NodeTypeK8sNode,
				Status:        k8sStatus,
				HostID:        hostID,
				Source:        "telemetry_kubernetes",
				FirstObserved: now,
				LastObserved:  now,
				Tags: map[string]string{
					"version": kn.Version,
					"runtime": kn.ContainerRuntime,
				},
			})

			g.AddDependency(Dependency{
				SourceID:      k8sNodeID,
				TargetID:      hostNodeID,
				Type:          RelRunsOn,
				Weight:        1.0,
				Confidence:    ConfidenceHigh,
				Evidence:      []string{"k8s_node_binding"},
				FirstObserved: now,
				LastObserved:  now,
			})
		}

		for _, pod := range snapshot.Kubernetes.Pods {
			podID := fmt.Sprintf("k8s-pod-%s-%s", pod.Namespace, pod.Name)
			podStatus := b.mapK8sPodStatus(pod.Status, pod.RestartsTotal)
			podType := b.inferNodeTypeFromImageOrName(pod.Name, pod.Name)
			if podType == NodeTypeCustom || podType == NodeTypeContainer {
				podType = NodeTypeK8sPod
			}

			g.AddNode(TopologyNode{
				ID:            podID,
				Name:          fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
				Type:          podType,
				Status:        podStatus,
				HostID:        hostID,
				Source:        "telemetry_kubernetes",
				FirstObserved: now,
				LastObserved:  now,
				Tags:          pod.Labels,
				Metadata: map[string]string{
					"namespace": pod.Namespace,
					"ip":        pod.IP,
					"restarts":  fmt.Sprintf("%d", pod.RestartsTotal),
				},
			})

			if pod.NodeName != "" {
				targetK8sNode := fmt.Sprintf("k8s-node-%s", pod.NodeName)
				g.AddDependency(Dependency{
					SourceID:      podID,
					TargetID:      targetK8sNode,
					Type:          RelRunsOn,
					Weight:        1.0,
					Confidence:    ConfidenceHigh,
					Evidence:      []string{"k8s_pod_scheduled"},
					FirstObserved: now,
					LastObserved:  now,
				})
			}
		}

		for _, dep := range snapshot.Kubernetes.Deployments {
			deployID := fmt.Sprintf("k8s-svc-%s-%s", dep.Namespace, dep.Name)
			deployStatus := NodeStatusHealthy
			if dep.ReplicasAvailable < dep.ReplicasDesired {
				if dep.ReplicasAvailable == 0 && dep.ReplicasDesired > 0 {
					deployStatus = NodeStatusCritical
				} else {
					deployStatus = NodeStatusDegraded
				}
			}

			g.AddNode(TopologyNode{
				ID:            deployID,
				Name:          fmt.Sprintf("%s/%s", dep.Namespace, dep.Name),
				Type:          NodeTypeService,
				Status:        deployStatus,
				HostID:        hostID,
				Source:        "telemetry_kubernetes",
				FirstObserved: now,
				LastObserved:  now,
			})
		}
	}

	// 4. Process System Services / Daemons
	for _, svc := range snapshot.Services {
		if svc.Name == "" {
			continue
		}
		serviceNodeID := fmt.Sprintf("svc-%s-%s", hostID, svc.Name)
		svcType := b.inferNodeTypeFromImageOrName(svc.Name, svc.DisplayName)
		if svcType == NodeTypeContainer || svcType == NodeTypeCustom {
			svcType = NodeTypeService
		}
		svcStatus := NodeStatusHealthy
		if svc.Status == model.ServiceStateStopped {
			svcStatus = NodeStatusOffline
		} else if svc.Status == model.ServiceStateUnknown {
			svcStatus = NodeStatusUnknown
		}

		g.AddNode(TopologyNode{
			ID:            serviceNodeID,
			Name:          svc.Name,
			Type:          svcType,
			Status:        svcStatus,
			HostID:        hostID,
			Source:        "telemetry_services",
			FirstObserved: now,
			LastObserved:  now,
			Metadata: map[string]string{
				"manager":      svc.Manager,
				"display_name": svc.DisplayName,
			},
		})

		g.AddDependency(Dependency{
			SourceID:      serviceNodeID,
			TargetID:      hostNodeID,
			Type:          RelRunsOn,
			Weight:        1.0,
			Confidence:    ConfidenceHigh,
			Evidence:      []string{"service_manager", svc.Manager},
			FirstObserved: now,
			LastObserved:  now,
		})
	}

	// 5. Process Listening Ports and infer network endpoints
	for _, p := range snapshot.Ports {
		if p.State == "LISTEN" || p.State == "Listening" || p.State == "" {
			endpointID := fmt.Sprintf("endpoint-%s-%s-%d", hostID, p.Protocol, p.Port)
			g.AddNode(TopologyNode{
				ID:            endpointID,
				Name:          fmt.Sprintf("%s:%d", p.BindAddress, p.Port),
				Type:          NodeTypeNetworkEndpoint,
				Status:        NodeStatusHealthy,
				HostID:        hostID,
				Source:        "telemetry_ports",
				FirstObserved: now,
				LastObserved:  now,
				Metadata: map[string]string{
					"protocol":     p.Protocol,
					"bind_address": p.BindAddress,
					"process":      p.ProcessName,
					"pid":          fmt.Sprintf("%d", p.PID),
				},
			})

			g.AddDependency(Dependency{
				SourceID:      endpointID,
				TargetID:      hostNodeID,
				Type:          RelRunsOn,
				Weight:        1.0,
				Confidence:    ConfidenceHigh,
				Evidence:      []string{"port_listening_match"},
				FirstObserved: now,
				LastObserved:  now,
			})
		}
	}
}

// IngestFleet adds fleet nodes to the topology graph.
func (b *Builder) IngestFleet(g *Graph, fleetNodes []model.NodeIdentity) {
	now := time.Now().UTC()
	for _, fn := range fleetNodes {
		hostNodeID := fmt.Sprintf("host-%s", fn.NodeID)
		g.AddNode(TopologyNode{
			ID:            hostNodeID,
			Name:          fn.Hostname,
			Type:          NodeTypePhysicalHost,
			Status:        NodeStatusHealthy,
			HostID:        fn.NodeID,
			Source:        "fleet_inventory",
			FirstObserved: now,
			LastObserved:  now,
			Tags:          fn.Tags,
			Metadata: map[string]string{
				"os":       fn.OS,
				"arch":     fn.Arch,
				"platform": fn.Platform,
			},
		})
	}
}

func (b *Builder) inferNodeTypeFromImageOrName(image, name string) NodeType {
	lower := strings.ToLower(image + " " + name)
	switch {
	case strings.Contains(lower, "postgres"), strings.Contains(lower, "psql"),
		strings.Contains(lower, "mysql"), strings.Contains(lower, "mariadb"),
		strings.Contains(lower, "mongo"), strings.Contains(lower, "cockroach"),
		strings.Contains(lower, "sqlite"), strings.Contains(lower, "clickhouse"):
		return NodeTypeDatabase
	case strings.Contains(lower, "redis"), strings.Contains(lower, "memcached"),
		strings.Contains(lower, "valkey"), strings.Contains(lower, "dragonfly"):
		return NodeTypeCache
	case strings.Contains(lower, "rabbit"), strings.Contains(lower, "kafka"),
		strings.Contains(lower, "nats"), strings.Contains(lower, "activemq"),
		strings.Contains(lower, "pulsar"), strings.Contains(lower, "sqs"):
		return NodeTypeQueue
	case strings.Contains(lower, "minio"), strings.Contains(lower, "ceph"),
		strings.Contains(lower, "s3"), strings.Contains(lower, "nfs"):
		return NodeTypeStorage
	case strings.Contains(lower, "nginx"), strings.Contains(lower, "caddy"),
		strings.Contains(lower, "envoy"), strings.Contains(lower, "traefik"),
		strings.Contains(lower, "haproxy"), strings.Contains(lower, "api"),
		strings.Contains(lower, "app"), strings.Contains(lower, "service"),
		strings.Contains(lower, "backend"), strings.Contains(lower, "frontend"):
		return NodeTypeService
	default:
		return NodeTypeContainer
	}
}

func (b *Builder) mapContainerStatus(state string, restartCount int) NodeStatus {
	switch strings.ToLower(state) {
	case "running":
		if restartCount > 10 {
			return NodeStatusDegraded
		}
		return NodeStatusHealthy
	case "exited", "dead":
		return NodeStatusCritical
	case "paused":
		return NodeStatusDegraded
	default:
		return NodeStatusUnknown
	}
}

func (b *Builder) mapK8sPodStatus(status string, restarts int) NodeStatus {
	switch strings.ToLower(status) {
	case "running", "succeeded":
		if restarts > 5 {
			return NodeStatusDegraded
		}
		return NodeStatusHealthy
	case "pending":
		return NodeStatusDegraded
	case "crashloopbackoff", "failed", "error":
		return NodeStatusCritical
	default:
		return NodeStatusUnknown
	}
}
