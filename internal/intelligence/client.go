package intelligence

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

// Client communicates with the Watchdog Intelligence REST API.
type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// ClientConfig holds configuration parameters for Intelligence Client.
type ClientConfig struct {
	Endpoint  string
	Token     string
	Timeout   time.Duration
	TLSConfig *tls.Config
}

// NewClient creates a new Client for intelligence queries.
func NewClient(cfg ClientConfig) *Client {
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

	return &Client{
		endpoint: endpoint,
		token:    cfg.Token,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// Endpoint returns the configured base URL.
func (c *Client) Endpoint() string {
	return c.endpoint
}

// GetFleetHealth retrieves the fleet-wide health summary.
func (c *Client) GetFleetHealth(ctx context.Context) (*FleetHealthSummary, error) {
	url := fmt.Sprintf("%s/api/v1/intelligence/fleet", c.endpoint)
	var resp FleetHealthSummary
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetNodeHealth retrieves the health summary for a specific node.
func (c *Client) GetNodeHealth(ctx context.Context, nodeID string) (*NodeHealthSummary, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	url := fmt.Sprintf("%s/api/v1/intelligence/nodes/%s", c.endpoint, nodeID)
	var resp NodeHealthSummary
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetNodeTrends retrieves historical metric trends for a specific node.
func (c *Client) GetNodeTrends(ctx context.Context, nodeID string, window time.Duration) ([]HealthTrend, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	url := fmt.Sprintf("%s/api/v1/intelligence/nodes/%s/trends", c.endpoint, nodeID)
	if window > 0 {
		url = fmt.Sprintf("%s?window=%s", url, window.String())
	}
	var resp []HealthTrend
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetNodeBaselines retrieves historical metric baselines for a specific node.
func (c *Client) GetNodeBaselines(ctx context.Context, nodeID string, window time.Duration) ([]HistoricalBaseline, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	url := fmt.Sprintf("%s/api/v1/intelligence/nodes/%s/baselines", c.endpoint, nodeID)
	if window > 0 {
		url = fmt.Sprintf("%s?window=%s", url, window.String())
	}
	var resp []HistoricalBaseline
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetActiveIncidents retrieves active incidents across the fleet.
func (c *Client) GetActiveIncidents(ctx context.Context) ([]Incident, error) {
	url := fmt.Sprintf("%s/api/v1/intelligence/incidents", c.endpoint)
	var resp []Incident
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetIncident retrieves a single incident by ID.
func (c *Client) GetIncident(ctx context.Context, incidentID string) (*Incident, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	url := fmt.Sprintf("%s/api/v1/intelligence/incidents/%s", c.endpoint, incidentID)
	var resp Incident
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetCorrelations retrieves metric temporal correlations.
func (c *Client) GetCorrelations(ctx context.Context, window time.Duration) ([]Correlation, error) {
	url := fmt.Sprintf("%s/api/v1/intelligence/correlations", c.endpoint)
	if window > 0 {
		url = fmt.Sprintf("%s?window=%s", url, window.String())
	}
	var resp []Correlation
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetFindings retrieves intelligence findings filtered by category and minimum severity.
func (c *Client) GetFindings(ctx context.Context, category FindingCategory, minSeverity model.Severity) ([]IntelligenceFinding, error) {
	url := fmt.Sprintf("%s/api/v1/intelligence/findings", c.endpoint)
	var params []string
	if category != "" {
		params = append(params, fmt.Sprintf("category=%s", category))
	}
	if minSeverity != "" {
		params = append(params, fmt.Sprintf("severity=%s", minSeverity))
	}
	if len(params) > 0 {
		url = fmt.Sprintf("%s?%s", url, strings.Join(params, "&"))
	}
	var resp []IntelligenceFinding
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetNodePredictions retrieves deterministic threshold predictions for a specific node.
func (c *Client) GetNodePredictions(ctx context.Context, nodeID string, horizon time.Duration) ([]Prediction, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	url := fmt.Sprintf("%s/api/v1/intelligence/nodes/%s/predictions", c.endpoint, nodeID)
	if horizon > 0 {
		url = fmt.Sprintf("%s?horizon=%s", url, horizon.String())
	}
	var resp []Prediction
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetNodeCapacityForecast retrieves multi-resource capacity forecasts for a specific node.
func (c *Client) GetNodeCapacityForecast(ctx context.Context, nodeID string, horizon time.Duration) (*NodeCapacityReport, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	url := fmt.Sprintf("%s/api/v1/intelligence/nodes/%s/capacity", c.endpoint, nodeID)
	if horizon > 0 {
		url = fmt.Sprintf("%s?horizon=%s", url, horizon.String())
	}
	var resp NodeCapacityReport
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFleetPredictions retrieves fleet-wide capacity forecasting and aggregated threshold crossings.
func (c *Client) GetFleetPredictions(ctx context.Context, horizon time.Duration) (*FleetCapacitySummary, error) {
	url := fmt.Sprintf("%s/api/v1/intelligence/fleet/predictions", c.endpoint)
	if horizon > 0 {
		url = fmt.Sprintf("%s?horizon=%s", url, horizon.String())
	}
	var resp FleetCapacitySummary
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRecurringIncidents retrieves recurring incident patterns detected over a lookback window.
func (c *Client) GetRecurringIncidents(ctx context.Context, since time.Duration) ([]RecurrencePattern, error) {
	url := fmt.Sprintf("%s/api/v1/intelligence/recurrence", c.endpoint)
	if since > 0 {
		url = fmt.Sprintf("%s?since=%s", url, since.String())
	}
	var resp []RecurrencePattern
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetPrediction retrieves a single prediction by ID.
func (c *Client) GetPrediction(ctx context.Context, predictionID string) (*Prediction, error) {
	if predictionID == "" {
		return nil, fmt.Errorf("prediction ID is required")
	}
	url := fmt.Sprintf("%s/api/v1/intelligence/predictions/%s", c.endpoint, predictionID)
	var resp Prediction
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRootCauseAnalysis retrieves root cause analysis for an incident.
func (c *Client) GetRootCauseAnalysis(ctx context.Context, incidentID string) (*RootCauseReport, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	url := fmt.Sprintf("%s/api/v1/intelligence/root-cause/%s", c.endpoint, incidentID)
	var resp RootCauseReport
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) doJSON(ctx context.Context, method, url string, payload any, result any) error {
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

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse JSON response: %w", err)
		}
	}

	return nil
}
