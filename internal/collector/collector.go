package collector

import (
	"context"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

// Status represents the operational status of a collector.
type Status struct {
	Name          string        `json:"name"`
	Enabled       bool          `json:"enabled"`
	LastSuccess   time.Time     `json:"last_success"`
	LastError     string        `json:"last_error,omitempty"`
	Duration      time.Duration `json:"duration"`
	Failures      int64         `json:"failures"`
	SuccessCount  int64         `json:"success_count"`
	TotalCount    int64         `json:"total_count"`
	TotalDuration time.Duration `json:"total_duration"`
	Buckets       [12]int64     `json:"buckets"`
	HasRun        bool          `json:"has_run"`
}

// Collector is the base interface that all metric collectors must implement.
type Collector interface {
	Name() string
	Collect(ctx context.Context) (any, error)
}

// Manager orchestrates and coordinates multiple metric collectors concurrently.
type Manager struct {
	mu         sync.RWMutex
	collectors map[string]Collector
	statuses   map[string]*Status
}

// NewManager creates a new empty collector manager.
func NewManager() *Manager {
	return &Manager{
		collectors: make(map[string]Collector),
		statuses:   make(map[string]*Status),
	}
}

// NewDefaultManager creates a new collector manager populated with standard collectors based on configuration.
func NewDefaultManager(cfg *config.Config) *Manager {
	m := NewManager()

	// Always register host system collector
	m.Register(NewSystemCollector())

	if cfg == nil || cfg.Collectors.CPU {
		m.Register(NewCPUCollector())
	}
	if cfg == nil || cfg.Collectors.Memory {
		m.Register(NewMemoryCollector())
	}
	if cfg == nil || cfg.Collectors.Disk {
		m.Register(NewDiskCollector())
	}
	if cfg == nil || cfg.Collectors.Network {
		m.Register(NewNetworkCollector())
	}
	if cfg == nil || cfg.Collectors.Process {
		m.Register(NewProcessCollector())
	}
	if cfg == nil || cfg.Collectors.Service {
		m.Register(NewServiceCollector())
	}
	if cfg == nil || cfg.Collectors.Port {
		m.Register(NewPortCollector())
	}
	if cfg == nil || cfg.Collectors.Docker {
		m.Register(NewDockerCollector())
	}
	if cfg != nil && cfg.Collectors.Kubernetes {
		m.Register(NewKubernetesCollector())
	}

	return m
}

// Register adds a collector to the manager.
func (m *Manager) Register(c Collector) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := c.Name()
	m.collectors[name] = c
	m.statuses[name] = &Status{
		Name:    name,
		Enabled: true,
	}
}

// GetCollector retrieves a registered collector by name.
func (m *Manager) GetCollector(name string) (Collector, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.collectors[name]
	return c, ok
}

// SetEnabled toggles a collector on or off.
func (m *Manager) SetEnabled(name string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.statuses[name]; ok {
		st.Enabled = enabled
	}
}

// GetStatuses returns current status snapshots of all registered collectors.
func (m *Manager) GetStatuses() []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Status
	for _, st := range m.statuses {
		result = append(result, *st)
	}
	return result
}

// CollectAll queries all enabled collectors in parallel and returns a consolidated SystemSnapshot.
func (m *Manager) CollectAll(ctx context.Context) (*model.SystemSnapshot, error) {
	m.mu.RLock()
	activeCollectors := make(map[string]Collector)
	for name, c := range m.collectors {
		if st, ok := m.statuses[name]; ok && st.Enabled {
			activeCollectors[name] = c
		}
	}
	m.mu.RUnlock()

	snapshot := &model.SystemSnapshot{
		Timestamp: time.Now(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, col := range activeCollectors {
		wg.Add(1)
		go func(n string, c Collector) {
			defer wg.Done()
			start := time.Now()
			res, err := c.Collect(ctx)
			dur := time.Since(start)

			m.mu.Lock()
			if st, ok := m.statuses[n]; ok {
				st.Duration = dur
				st.TotalCount++
				st.TotalDuration += dur
				sec := dur.Seconds()
				bounds := [11]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}
				for i, b := range bounds {
					if sec <= b {
						st.Buckets[i]++
					}
				}
				st.Buckets[11]++ // +Inf bucket

				if err != nil {
					st.Failures++
					st.LastError = err.Error()
					st.HasRun = true
				} else {
					st.LastSuccess = time.Now()
					st.LastError = ""
					st.SuccessCount++
					st.HasRun = true
				}
			}
			m.mu.Unlock()

			if err != nil {
				return
			}

			mu.Lock()
			defer mu.Unlock()

			switch v := res.(type) {
			case *model.SystemInfo:
				snapshot.System = v
			case *model.CPUInfo:
				snapshot.CPU = v
			case *model.MemoryInfo:
				snapshot.Memory = v
			case *model.DiskInfo:
				snapshot.Disk = v
			case *model.NetworkInfo:
				snapshot.Network = v
			case *model.ProcessSummary:
				snapshot.Processes = v
			case []model.ServiceInfo:
				snapshot.Services = v
			case []model.PortInfo:
				snapshot.Ports = v
			case *model.DockerSummary:
				snapshot.Docker = v
			case *model.K8sSummary:
				snapshot.Kubernetes = v
			}
		}(name, col)
	}

	wg.Wait()
	return snapshot, nil
}
