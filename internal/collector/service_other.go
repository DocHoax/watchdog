//go:build !windows && !linux && !darwin

package collector

import (
	"context"

	"github.com/DocHoax/watchdog/pkg/model"
)

// GetServices returns empty list for other operating systems.
func (c *ServiceCollector) GetServices(ctx context.Context) ([]model.ServiceInfo, error) {
	return []model.ServiceInfo{}, nil
}
