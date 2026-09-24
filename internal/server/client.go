package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// Client is a client for communicating with remote Watchdog agents.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a new remote Watchdog agent client.
func NewClient(baseURL, token string, insecureSkipVerify bool) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify, // #nosec G402 - user configurable for self-signed certificates
		},
	}

	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

// GetHealth checks remote agent health status.
func (c *Client) GetHealth(ctx context.Context) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/health", nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact remote agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote agent returned status %d", resp.StatusCode)
	}

	var res map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode health response: %w", err)
	}
	return res, nil
}

// FetchSnapshot fetches a live SystemSnapshot from the remote agent.
func (c *Client) FetchSnapshot(ctx context.Context) (*model.SystemSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/snapshot", nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote agent error (%d)", resp.StatusCode)
	}

	var snap model.SystemSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot: %w", err)
	}
	return &snap, nil
}

// FetchDiagnostics fetches the latest DiagnosticReport from the remote agent.
func (c *Client) FetchDiagnostics(ctx context.Context) (*model.DiagnosticReport, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/diagnostics", nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch diagnostics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote agent error (%d)", resp.StatusCode)
	}

	var report model.DiagnosticReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return nil, fmt.Errorf("failed to parse diagnostics: %w", err)
	}
	return &report, nil
}

// FetchAlerts fetches active alerts from the remote agent.
func (c *Client) FetchAlerts(ctx context.Context) ([]model.AlertEvent, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/alerts", nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch alerts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote agent error (%d)", resp.StatusCode)
	}

	var res struct {
		ActiveAlerts []model.AlertEvent `json:"active_alerts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to parse alerts: %w", err)
	}
	return res.ActiveAlerts, nil
}

func (c *Client) setAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}
