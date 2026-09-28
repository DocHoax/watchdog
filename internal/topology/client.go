package topology

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// Client communicates with the Watchdog Topology REST API.
type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// ClientConfig holds configuration parameters for Topology Client.
type ClientConfig struct {
	Endpoint  string
	Token     string
	Timeout   time.Duration
	TLSConfig *tls.Config
}

// NewClient creates a new Client for topology queries.
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

// GetTopology retrieves the full or filtered topology graph.
func (c *Client) GetTopology(ctx context.Context, filter TopologyFilter) (*TopologyGraphResponse, error) {
	reqURL := fmt.Sprintf("%s/api/v1/topology", c.endpoint)
	params := url.Values{}

	if len(filter.Types) > 0 {
		var typeStrs []string
		for _, t := range filter.Types {
			typeStrs = append(typeStrs, string(t))
		}
		params.Set("type", strings.Join(typeStrs, ","))
	}
	if len(filter.Statuses) > 0 {
		var statusStrs []string
		for _, s := range filter.Statuses {
			statusStrs = append(statusStrs, string(s))
		}
		params.Set("status", strings.Join(statusStrs, ","))
	}
	if filter.Source != "" {
		params.Set("source", filter.Source)
	}
	if filter.HostID != "" {
		params.Set("host_id", filter.HostID)
	}
	if filter.Search != "" {
		params.Set("search", filter.Search)
	}
	if filter.MaxDepth > 0 {
		params.Set("max_depth", strconv.Itoa(filter.MaxDepth))
	}
	for k, v := range filter.TagFilter {
		params.Set("tag:"+k, v)
	}

	if len(params) > 0 {
		reqURL = fmt.Sprintf("%s?%s", reqURL, params.Encode())
	}

	var resp TopologyGraphResponse
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetTopologySummary retrieves high-level topology summary statistics.
func (c *Client) GetTopologySummary(ctx context.Context) (*TopologyGraphSummary, error) {
	reqURL := fmt.Sprintf("%s/api/v1/topology/summary", c.endpoint)
	var resp TopologyGraphSummary
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ExportDOT retrieves the Graphviz DOT representation of the topology.
func (c *Client) ExportDOT(ctx context.Context) (string, error) {
	reqURL := fmt.Sprintf("%s/api/v1/topology/dot", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return string(body), nil
}

// FindPath searches for the shortest dependency path between two nodes.
func (c *Client) FindPath(ctx context.Context, sourceID, targetID string) (*DependencyPath, error) {
	if sourceID == "" || targetID == "" {
		return nil, fmt.Errorf("both source and target node IDs are required")
	}
	params := url.Values{}
	params.Set("source", sourceID)
	params.Set("target", targetID)
	reqURL := fmt.Sprintf("%s/api/v1/topology/path?%s", c.endpoint, params.Encode())

	var resp DependencyPath
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// FindAllSPOFs retrieves all single points of failure exceeding the minimum criticality threshold.
func (c *Client) FindAllSPOFs(ctx context.Context, minCriticality float64) ([]SPOFAnalysis, error) {
	reqURL := fmt.Sprintf("%s/api/v1/topology/spof", c.endpoint)
	if minCriticality > 0 {
		reqURL = fmt.Sprintf("%s?min_criticality=%f", reqURL, minCriticality)
	}

	var res struct {
		SPOFs []SPOFAnalysis `json:"spofs"`
		Count int            `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &res); err != nil {
		return nil, err
	}
	return res.SPOFs, nil
}

// AnalyzeSPOF retrieves detailed single point of failure analysis for a specific node.
func (c *Client) AnalyzeSPOF(ctx context.Context, nodeID string) (*SPOFAnalysis, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	reqURL := fmt.Sprintf("%s/api/v1/topology/spof/%s", c.endpoint, nodeID)
	var resp SPOFAnalysis
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AnalyzeImpact evaluates upstream blast radius for degradation or outage of a node.
func (c *Client) AnalyzeImpact(ctx context.Context, nodeID string) (*TopologyImpactAnalysis, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	reqURL := fmt.Sprintf("%s/api/v1/topology/impact/%s", c.endpoint, nodeID)
	var resp TopologyImpactAnalysis
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetNode retrieves a single topology node by ID.
func (c *Client) GetNode(ctx context.Context, nodeID string) (*TopologyNode, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	reqURL := fmt.Sprintf("%s/api/v1/topology/nodes/%s", c.endpoint, nodeID)
	var resp TopologyNode
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetDependencies retrieves downstream dependencies for a given node.
func (c *Client) GetDependencies(ctx context.Context, nodeID string) ([]TopologyNode, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	reqURL := fmt.Sprintf("%s/api/v1/topology/dependencies/%s", c.endpoint, nodeID)
	var res struct {
		NodeID       string         `json:"node_id"`
		Dependencies []TopologyNode `json:"dependencies"`
		Count        int            `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &res); err != nil {
		return nil, err
	}
	return res.Dependencies, nil
}

// GetDependents retrieves upstream dependents that rely on a given node.
func (c *Client) GetDependents(ctx context.Context, nodeID string) ([]TopologyNode, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	reqURL := fmt.Sprintf("%s/api/v1/topology/dependents/%s", c.endpoint, nodeID)
	var res struct {
		NodeID     string         `json:"node_id"`
		Dependents []TopologyNode `json:"dependents"`
		Count      int            `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &res); err != nil {
		return nil, err
	}
	return res.Dependents, nil
}

// AddDeclaredNode registers a declared node in the topology.
func (c *Client) AddDeclaredNode(ctx context.Context, node TopologyNode) (*TopologyNode, error) {
	reqURL := fmt.Sprintf("%s/api/v1/topology/nodes", c.endpoint)
	var resp TopologyNode
	if err := c.doJSON(ctx, http.MethodPost, reqURL, node, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AddDeclaredDependency registers a declared dependency edge in the topology.
func (c *Client) AddDeclaredDependency(ctx context.Context, dep Dependency) (*Dependency, error) {
	reqURL := fmt.Sprintf("%s/api/v1/topology/dependencies", c.endpoint)
	var resp Dependency
	if err := c.doJSON(ctx, http.MethodPost, reqURL, dep, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RemoveNode removes a node and all connected dependencies.
func (c *Client) RemoveNode(ctx context.Context, nodeID string) error {
	if nodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	reqURL := fmt.Sprintf("%s/api/v1/topology/nodes/%s", c.endpoint, nodeID)
	return c.doJSON(ctx, http.MethodDelete, reqURL, nil, nil)
}

// RemoveDependency deletes a dependency relationship between two nodes.
func (c *Client) RemoveDependency(ctx context.Context, sourceID, targetID string, relType RelationshipType) error {
	if sourceID == "" || targetID == "" {
		return fmt.Errorf("both source and target IDs are required")
	}
	params := url.Values{}
	params.Set("source", sourceID)
	params.Set("target", targetID)
	if relType != "" {
		params.Set("type", string(relType))
	}
	reqURL := fmt.Sprintf("%s/api/v1/topology/dependencies?%s", c.endpoint, params.Encode())
	return c.doJSON(ctx, http.MethodDelete, reqURL, nil, nil)
}

func (c *Client) doJSON(ctx context.Context, method, urlStr string, payload any, result any) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal request payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
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
