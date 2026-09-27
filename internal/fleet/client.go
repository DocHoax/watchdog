package fleet

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// FleetClient communicates with the centralized Watchdog fleet management server.
type FleetClient struct {
	endpoint   string
	token      string
	nodeID     string
	version    string
	httpClient *http.Client
	buffer     *TelemetryBuffer
}

// ClientConfig holds configuration parameters for FleetClient.
type ClientConfig struct {
	Endpoint   string
	Token      string
	NodeID     string
	Version    string
	Timeout    time.Duration
	TLSConfig  *tls.Config
	Buffer     *TelemetryBuffer
}

// NewFleetClient initializes a new FleetClient.
func NewFleetClient(cfg ClientConfig) *FleetClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig:     cfg.TLSConfig,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	endpoint := strings.TrimRight(cfg.Endpoint, "/")

	return &FleetClient{
		endpoint: endpoint,
		token:    cfg.Token,
		nodeID:   cfg.NodeID,
		version:  cfg.Version,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		buffer: cfg.Buffer,
	}
}

// Endpoint returns the configured base URL for the fleet server.
func (c *FleetClient) Endpoint() string {
	return c.endpoint
}

// NodeID returns the configured NodeID for the client.
func (c *FleetClient) NodeID() string {
	return c.nodeID
}

// Register sends a node registration request to the central server.
func (c *FleetClient) Register(ctx context.Context, req *model.NodeRegistrationRequest) (*model.NodeRegistrationResponse, error) {
	url := fmt.Sprintf("%s/api/v1/fleet/register", c.endpoint)
	var resp model.NodeRegistrationResponse
	if err := c.doJSON(ctx, http.MethodPost, url, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendHeartbeat dispatches a periodic liveness heartbeat to the server.
func (c *FleetClient) SendHeartbeat(ctx context.Context, hb *model.HeartbeatRequest) (*model.HeartbeatResponse, error) {
	url := fmt.Sprintf("%s/api/v1/heartbeat", c.endpoint)
	var resp model.HeartbeatResponse
	if err := c.doJSON(ctx, http.MethodPost, url, hb, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendTelemetry transmits a single telemetry submission to the central server.
// If the server is unreachable and a buffer is configured, it enqueues the submission.
func (c *FleetClient) SendTelemetry(ctx context.Context, sub *model.TelemetrySubmission) error {
	url := fmt.Sprintf("%s/api/v1/telemetry", c.endpoint)
	err := c.doJSON(ctx, http.MethodPost, url, sub, nil)
	if err != nil {
		if c.buffer != nil {
			c.buffer.Push(sub)
		}
		return err
	}
	return nil
}

// FlushBuffer attempts to drain queued submissions from the buffer to the server.
func (c *FleetClient) FlushBuffer(ctx context.Context, batchSize int) (int, error) {
	if c.buffer == nil || c.buffer.Len() == 0 {
		return 0, nil
	}

	if batchSize <= 0 {
		batchSize = 25
	}

	batch := c.buffer.PopBatch(batchSize)
	if len(batch) == 0 {
		return 0, nil
	}

	url := fmt.Sprintf("%s/api/v1/telemetry", c.endpoint)
	sent := 0

	for i, sub := range batch {
		if err := c.doJSON(ctx, http.MethodPost, url, sub, nil); err != nil {
			// Push remaining items back into the buffer
			for j := len(batch) - 1; j >= i; j-- {
				c.buffer.Push(batch[j])
			}
			return sent, fmt.Errorf("buffer flush interrupted after %d items: %w", sent, err)
		}
		sent++
	}

	return sent, nil
}

// ListNodes queries the fleet management server for registered nodes matching the filter.
func (c *FleetClient) ListNodes(ctx context.Context, filter model.FleetFilter) (*model.FleetListResponse, error) {
	url := fmt.Sprintf("%s/api/v1/fleet", c.endpoint)
	params := make([]string, 0)
	if filter.Status != "" {
		params = append(params, fmt.Sprintf("status=%s", filter.Status))
	}
	if filter.Search != "" {
		params = append(params, fmt.Sprintf("search=%s", filter.Search))
	}
	if filter.Limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", filter.Limit))
	}
	if filter.Offset > 0 {
		params = append(params, fmt.Sprintf("offset=%d", filter.Offset))
	}
	if filter.SortBy != "" {
		params = append(params, fmt.Sprintf("sort_by=%s", filter.SortBy))
	}
	if filter.SortDirection != "" {
		params = append(params, fmt.Sprintf("sort_direction=%s", filter.SortDirection))
	}
	if !filter.Since.IsZero() {
		params = append(params, fmt.Sprintf("since=%s", filter.Since.Format(time.RFC3339)))
	}
	if len(params) > 0 {
		url = fmt.Sprintf("%s?%s", url, strings.Join(params, "&"))
	}

	var resp model.FleetListResponse
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetNode retrieves detailed information and recent telemetry for a single node.
func (c *FleetClient) GetNode(ctx context.Context, nodeID string) (*model.NodeDetailResponse, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	url := fmt.Sprintf("%s/api/v1/fleet/%s", c.endpoint, nodeID)
	var resp model.NodeDetailResponse
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteNode removes/deregisters a node from the fleet registry.
func (c *FleetClient) DeleteNode(ctx context.Context, nodeID string) error {
	if nodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	url := fmt.Sprintf("%s/api/v1/fleet/%s", c.endpoint, nodeID)
	return c.doJSON(ctx, http.MethodDelete, url, nil, nil)
}

// GetSummary retrieves aggregated health and fleet metrics.
func (c *FleetClient) GetSummary(ctx context.Context) (*model.FleetSummary, error) {
	url := fmt.Sprintf("%s/api/v1/fleet/summary", c.endpoint)
	var resp model.FleetSummary
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetNodeIdentity retrieves the node identity from /api/v1/node.
func (c *FleetClient) GetNodeIdentity(ctx context.Context) (*model.NodeIdentity, error) {
	url := fmt.Sprintf("%s/api/v1/node", c.endpoint)
	var resp model.NodeIdentity
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *FleetClient) doJSON(ctx context.Context, method, url string, payload any, result any) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal request payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.nodeID != "" {
		req.Header.Set("X-Watchdog-Node-ID", c.nodeID)
	}
	if c.version != "" {
		req.Header.Set("X-Watchdog-Agent-Version", c.version)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr model.APIErrorResponse
		if jsonErr := json.Unmarshal(respBody, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
			return fmt.Errorf("server error (%d %s): %s", resp.StatusCode, apiErr.Error.Code, apiErr.Error.Message)
		}
		return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response payload: %w", err)
		}
	}

	return nil
}
