package incidents

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

// InvestigationReport is an alias for IncidentInvestigationReport for ergonomic API usage.
type InvestigationReport = IncidentInvestigationReport

// RecurrencePattern is an alias for RecurrenceAnalysis.
type RecurrencePattern = RecurrenceAnalysis

// IncidentListResponse represents the JSON response returned by the ListIncidents API.
type IncidentListResponse struct {
	Incidents []Incident `json:"incidents"`
	Total     int        `json:"total"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
	Timestamp string     `json:"timestamp"`
}

// SimilarIncidentsResponse represents the JSON response returned by the Similar Incidents API.
type SimilarIncidentsResponse struct {
	IncidentID       string                  `json:"incident_id"`
	SimilarIncidents []SimilarIncidentResult `json:"similar_incidents"`
	Count            int                     `json:"count"`
	MinSimilarityCut float64                 `json:"min_similarity_cut"`
	Timestamp        string                  `json:"timestamp"`
}

// TimelineResponse represents the JSON response returned by the Incident Timeline API.
type TimelineResponse struct {
	IncidentID string                  `json:"incident_id"`
	Timeline   []IncidentTimelineEntry `json:"timeline"`
	Count      int                     `json:"count"`
	Timestamp  string                  `json:"timestamp"`
}

// RelatedIncidentsResponse represents the JSON response returned by the Related Incidents API.
type RelatedIncidentsResponse struct {
	IncidentID       string                  `json:"incident_id"`
	Signals          []IncidentSignal        `json:"signals"`
	Recurrence       *RecurrenceAnalysis     `json:"recurrence,omitempty"`
	SimilarIncidents []SimilarIncidentResult `json:"similar_incidents"`
	Timestamp        string                  `json:"timestamp"`
}

// FindingsResponse represents the JSON response returned by the Incident Findings API.
type FindingsResponse struct {
	IncidentID string                `json:"incident_id"`
	Findings   []IntelligenceFinding `json:"findings"`
	Count      int                   `json:"count"`
	Timestamp  string                `json:"timestamp"`
}

// Client communicates with the Watchdog Incident Operations REST API.
type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// ClientConfig holds configuration parameters for the Incidents Client.
type ClientConfig struct {
	Endpoint  string
	Token     string
	Timeout   time.Duration
	TLSConfig *tls.Config
}

// NewClient creates a new Client for interacting with the Incident Operations API.
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

// ListIncidents queries and returns a paginated list of incidents matching the filter.
func (c *Client) ListIncidents(ctx context.Context, filter IncidentFilter) (*IncidentListResponse, error) {
	apiURL := fmt.Sprintf("%s/api/v1/incidents", c.endpoint)
	params := url.Values{}

	for _, st := range filter.Status {
		params.Add("status", string(st))
	}
	for _, sv := range filter.Severity {
		params.Add("severity", string(sv))
	}
	for _, sc := range filter.Scope {
		params.Add("scope", string(sc))
	}
	if filter.NodeID != "" {
		params.Set("node_id", filter.NodeID)
	}
	if filter.Search != "" {
		params.Set("search", filter.Search)
	}
	if !filter.StartTime.IsZero() {
		params.Set("start_time", filter.StartTime.UTC().Format(time.RFC3339))
	}
	if !filter.EndTime.IsZero() {
		params.Set("end_time", filter.EndTime.UTC().Format(time.RFC3339))
	}
	if filter.Limit > 0 {
		params.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Offset > 0 {
		params.Set("offset", strconv.Itoa(filter.Offset))
	}
	if filter.SortBy != "" {
		params.Set("sort_by", filter.SortBy)
	}
	if filter.SortOrder != "" {
		params.Set("sort_order", filter.SortOrder)
	}

	queryString := params.Encode()
	if queryString != "" {
		apiURL = fmt.Sprintf("%s?%s", apiURL, queryString)
	}

	var resp IncidentListResponse
	if err := c.doJSON(ctx, http.MethodGet, apiURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetSummary retrieves the aggregated incident summary.
func (c *Client) GetSummary(ctx context.Context) (*IncidentSummary, error) {
	apiURL := fmt.Sprintf("%s/api/v1/incidents/summary", c.endpoint)
	var resp IncidentSummary
	if err := c.doJSON(ctx, http.MethodGet, apiURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetSimilar finds similar historical incidents based on multi-factor Jaccard scoring.
func (c *Client) GetSimilar(ctx context.Context, incidentID string, minSimilarity float64, limit int) (*SimilarIncidentsResponse, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	apiURL := fmt.Sprintf("%s/api/v1/incidents/similar", c.endpoint)
	params := url.Values{}
	params.Set("id", incidentID)
	if minSimilarity > 0 {
		params.Set("min_similarity", fmt.Sprintf("%.2f", minSimilarity))
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	apiURL = fmt.Sprintf("%s?%s", apiURL, params.Encode())

	var resp SimilarIncidentsResponse
	if err := c.doJSON(ctx, http.MethodGet, apiURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetIncident retrieves a single incident by ID.
func (c *Client) GetIncident(ctx context.Context, incidentID string) (*Incident, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	apiURL := fmt.Sprintf("%s/api/v1/incidents/%s", c.endpoint, url.PathEscape(incidentID))
	var resp Incident
	if err := c.doJSON(ctx, http.MethodGet, apiURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetTimeline retrieves chronological timeline entries for an incident.
func (c *Client) GetTimeline(ctx context.Context, incidentID string, filter TimelineFilter) (*TimelineResponse, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	apiURL := fmt.Sprintf("%s/api/v1/incidents/%s/timeline", c.endpoint, url.PathEscape(incidentID))
	params := url.Values{}
	if filter.NodeID != "" {
		params.Set("node_id", filter.NodeID)
	}
	if filter.MinSeverity != "" {
		params.Set("min_severity", string(filter.MinSeverity))
	}
	if !filter.StartTime.IsZero() {
		params.Set("start_time", filter.StartTime.UTC().Format(time.RFC3339))
	}
	if !filter.EndTime.IsZero() {
		params.Set("end_time", filter.EndTime.UTC().Format(time.RFC3339))
	}
	if filter.Limit > 0 {
		params.Set("limit", strconv.Itoa(filter.Limit))
	}
	queryString := params.Encode()
	if queryString != "" {
		apiURL = fmt.Sprintf("%s?%s", apiURL, queryString)
	}

	var resp TimelineResponse
	if err := c.doJSON(ctx, http.MethodGet, apiURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRelated retrieves related root signals, recurrence statistics, and similar incidents.
func (c *Client) GetRelated(ctx context.Context, incidentID string) (*RelatedIncidentsResponse, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	apiURL := fmt.Sprintf("%s/api/v1/incidents/%s/related", c.endpoint, url.PathEscape(incidentID))
	var resp RelatedIncidentsResponse
	if err := c.doJSON(ctx, http.MethodGet, apiURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetImpact retrieves blast radius and subsystem impact analysis for an incident.
func (c *Client) GetImpact(ctx context.Context, incidentID string) (*ImpactAnalysis, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	apiURL := fmt.Sprintf("%s/api/v1/incidents/%s/impact", c.endpoint, url.PathEscape(incidentID))
	var resp ImpactAnalysis
	if err := c.doJSON(ctx, http.MethodGet, apiURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFindings retrieves non-invasive intelligence findings for an incident.
func (c *Client) GetFindings(ctx context.Context, incidentID string) (*FindingsResponse, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	apiURL := fmt.Sprintf("%s/api/v1/incidents/%s/findings", c.endpoint, url.PathEscape(incidentID))
	var resp FindingsResponse
	if err := c.doJSON(ctx, http.MethodGet, apiURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Investigate generates a comprehensive investigation report for an incident.
func (c *Client) Investigate(ctx context.Context, incidentID string) (*IncidentInvestigationReport, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	apiURL := fmt.Sprintf("%s/api/v1/incidents/%s/investigate", c.endpoint, url.PathEscape(incidentID))
	var resp IncidentInvestigationReport
	if err := c.doJSON(ctx, http.MethodGet, apiURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateIncidentStatusPayload defines the payload for changing incident status.
type UpdateIncidentStatusPayload struct {
	Status IncidentStatus `json:"status"`
	Reason string         `json:"reason"`
}

// UpdateStatus transitions an incident's lifecycle status.
func (c *Client) UpdateStatus(ctx context.Context, incidentID string, status IncidentStatus, reason string) (*Incident, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	apiURL := fmt.Sprintf("%s/api/v1/incidents/%s/status", c.endpoint, url.PathEscape(incidentID))
	payload := UpdateIncidentStatusPayload{
		Status: status,
		Reason: reason,
	}
	var resp Incident
	if err := c.doJSON(ctx, http.MethodPost, apiURL, payload, &resp); err != nil {
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
