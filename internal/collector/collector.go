package collector

import (
	"context"
	"sync"
	"time"

	"github.com/watchdog-cli/watchdog/pkg/model"
)

// Status represents the operational status of a collector.
type Status struct {
	Name        string        `json:"name"`
	Enabled     bool          `json:"enabled"`
	LastSuccess time.Time     `json:"last_success"`
	LastError   string        `json:"last_error,omitempty"`
	Duration    time.Duration `json:"duration"`
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

// NewManager creates a new collector manager.
func NewManager() *Manager {
	return &Manager{
		collectors: make(map[string]Collector),
		statuses:   make(map[string]*Status),
	}
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
				if err != nil {
					st.LastError = err.Error()
				} else {
					st.LastSuccess = time.Now()
					st.LastError = ""
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
