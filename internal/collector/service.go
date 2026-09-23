package collector

import (
	"context"

	"github.com/watchdog-cli/watchdog/pkg/model"
)

// ServiceCollector gathers status of system services.
type ServiceCollector struct{}

// NewServiceCollector creates a new ServiceCollector instance.
func NewServiceCollector() *ServiceCollector {
	return &ServiceCollector{}
}

// Name returns the identifier of the collector.
func (c *ServiceCollector) Name() string {
	return "service"
}

// Collect retrieves system services.
func (c *ServiceCollector) Collect(ctx context.Context) (any, error) {
	return c.GetServices(ctx)
}
