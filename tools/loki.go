package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

const (
	lokiDatasourceType  = "loki"
	defaultLokiLogLimit = 10
	maxLokiLogLimit     = 100
)

// resolveLokiDatasourceID resolves a Loki datasource UID to numeric ID.
func resolveLokiDatasourceID(ctx context.Context, uid string) (int64, error) {
	resolved, err := resolveDatasourceRef(ctx, DatasourceRef{UID: uid})
	if err != nil {
		return 0, err
	}
	if !strings.Contains(strings.ToLower(resolved.Datasource.Type), lokiDatasourceType) {
		return 0, fmt.Errorf("datasource %q is type %q, expected loki", resolved.Datasource.Name, resolved.Datasource.Type)
	}
	return resolved.Datasource.ID, nil
}

// lokiDoRequest performs a GET request through the Grafana datasource proxy to Loki.
func lokiDoRequest(ctx context.Context, dsID int64, subpath string, params url.Values) ([]byte, error) {
	path := fmt.Sprintf("/datasources/proxy/%d%s", dsID, subpath)
	respBody, statusCode, err := doAPIRequest(ctx, "GET", path, params, nil)
	if err != nil {
		return nil, wrapRawAPIError(statusCode, respBody, err)
	}
	return respBody, nil
}

// lokiAddTimeRange appends nanosecond start/end params converted from RFC3339.
func lokiAddTimeRange(params url.Values, startRFC3339, endRFC3339 string) {
	if startRFC3339 != "" {
		if t, err := time.Parse(time.RFC3339, startRFC3339); err == nil {
			params.Set("start", fmt.Sprintf("%d", t.UnixNano()))
		}
	}
	if endRFC3339 != "" {
		if t, err := time.Parse(time.RFC3339, endRFC3339); err == nil {
			params.Set("end", fmt.Sprintf("%d", t.UnixNano()))
		}
	}
}

// lokiDefaultTimeRange returns default RFC3339 start/end strings if empty.
func lokiDefaultTimeRange(start, end string) (string, string) {
	if start == "" {
		start = time.Now().Add(-time.Hour).Format(time.RFC3339)
	}
	if end == "" {
		end = time.Now().Format(time.RFC3339)
	}
	return start, end
}

// lokiLabelResponse is the response for Loki label/value list endpoints.
type lokiLabelResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

// lokiQueryResponse is the generic response for Loki query endpoints.
type lokiQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	} `json:"data"`
}

// ─── list_loki_label_names ────────────────────────────────────────────────────

type ListLokiLabelNamesRequest struct {
	DatasourceUID string `json:"datasourceUid" jsonschema:"required,description=The UID of the Loki datasource"`
	StartRFC3339  string `json:"startRfc3339,omitempty" jsonschema:"description=Start time in RFC3339 format (default: 1 hour ago)"`
	EndRFC3339    string `json:"endRfc3339,omitempty" jsonschema:"description=End time in RFC3339 format (default: now)"`
}

func listLokiLabelNames(ctx context.Context, args ListLokiLabelNamesRequest) ([]string, error) {
	dsID, err := resolveLokiDatasourceID(ctx, args.DatasourceUID)
	if err != nil {
		return nil, err
	}

	start, end := lokiDefaultTimeRange(args.StartRFC3339, args.EndRFC3339)
	params := url.Values{}
	lokiAddTimeRange(params, start, end)

	body, err := lokiDoRequest(ctx, dsID, "/loki/api/v1/labels", params)
	if err != nil {
		return nil, fmt.Errorf("list loki label names: %w", err)
	}

	var resp lokiLabelResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode loki labels response: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("loki API error: status=%s", resp.Status)
	}
	if resp.Data == nil {
		return []string{}, nil
	}
	return resp.Data, nil
}

var ListLokiLabelNamesTool = mcpgrafana.MustTool(
	"list_loki_label_names",
	"Lists all available label names (keys) found in logs within a Loki datasource and time range. Defaults to the last hour.",
	listLokiLabelNames,
	mcp.WithTitleAnnotation("List Loki label names"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// ─── list_loki_label_values ───────────────────────────────────────────────────

type ListLokiLabelValuesRequest struct {
	DatasourceUID string `json:"datasourceUid" jsonschema:"required,description=The UID of the Loki datasource"`
	LabelName     string `json:"labelName" jsonschema:"required,description=The name of the label to retrieve values for (e.g. 'app'\\, 'env')"`
	StartRFC3339  string `json:"startRfc3339,omitempty" jsonschema:"description=Start time in RFC3339 format (default: 1 hour ago)"`
	EndRFC3339    string `json:"endRfc3339,omitempty" jsonschema:"description=End time in RFC3339 format (default: now)"`
}

func listLokiLabelValues(ctx context.Context, args ListLokiLabelValuesRequest) ([]string, error) {
	if strings.TrimSpace(args.LabelName) == "" {
		return nil, fmt.Errorf("labelName is required")
	}
	dsID, err := resolveLokiDatasourceID(ctx, args.DatasourceUID)
	if err != nil {
		return nil, err
	}

	start, end := lokiDefaultTimeRange(args.StartRFC3339, args.EndRFC3339)
	params := url.Values{}
	lokiAddTimeRange(params, start, end)

	subpath := fmt.Sprintf("/loki/api/v1/label/%s/values", url.PathEscape(args.LabelName))
	body, err := lokiDoRequest(ctx, dsID, subpath, params)
	if err != nil {
		return nil, fmt.Errorf("list loki label values: %w", err)
	}

	var resp lokiLabelResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode loki label values response: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("loki API error: status=%s", resp.Status)
	}
	if resp.Data == nil {
		return []string{}, nil
	}
	return resp.Data, nil
}

var ListLokiLabelValuesTool = mcpgrafana.MustTool(
	"list_loki_label_values",
	"Retrieves all unique values for a specific label in a Loki datasource. Defaults to the last hour.",
	listLokiLabelValues,
	mcp.WithTitleAnnotation("List Loki label values"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// ─── query_loki_logs ──────────────────────────────────────────────────────────

// LokiLogEntry represents a single log line returned from Loki.
type LokiLogEntry struct {
	Timestamp string            `json:"timestamp,omitempty"`
	Line      string            `json:"line,omitempty"`
	Labels    map[string]string `json:"labels"`
}

type QueryLokiLogsRequest struct {
	DatasourceUID string `json:"datasourceUid" jsonschema:"required,description=The UID of the Loki datasource"`
	LogQL         string `json:"logql" jsonschema:"required,description=The LogQL query (e.g. '{app=\"foo\"} |= \"error\"')"`
	StartRFC3339  string `json:"startRfc3339,omitempty" jsonschema:"description=Start time in RFC3339 format (default: 1 hour ago)"`
	EndRFC3339    string `json:"endRfc3339,omitempty" jsonschema:"description=End time in RFC3339 format (default: now)"`
	Limit         int    `json:"limit,omitempty" jsonschema:"description=Max log lines to return (default: 10\\, max: 100)"`
	Direction     string `json:"direction,omitempty" jsonschema:"description='forward' or 'backward' (default: backward = newest first)"`
}

type QueryLokiLogsResult struct {
	Entries []LokiLogEntry `json:"entries"`
	Total   int            `json:"total"`
}

func queryLokiLogs(ctx context.Context, args QueryLokiLogsRequest) (*QueryLokiLogsResult, error) {
	if strings.TrimSpace(args.LogQL) == "" {
		return nil, fmt.Errorf("logql is required")
	}
	dsID, err := resolveLokiDatasourceID(ctx, args.DatasourceUID)
	if err != nil {
		return nil, err
	}

	start, end := lokiDefaultTimeRange(args.StartRFC3339, args.EndRFC3339)

	limit := args.Limit
	if limit <= 0 {
		limit = defaultLokiLogLimit
	}
	if limit > maxLokiLogLimit {
		limit = maxLokiLogLimit
	}

	direction := args.Direction
	if direction == "" {
		direction = "backward"
	}

	params := url.Values{}
	params.Set("query", args.LogQL)
	lokiAddTimeRange(params, start, end)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("direction", direction)

	body, err := lokiDoRequest(ctx, dsID, "/loki/api/v1/query_range", params)
	if err != nil {
		return nil, fmt.Errorf("query loki logs: %w", err)
	}

	var resp lokiQueryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode loki query response: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("loki query failed: status=%s", resp.Status)
	}

	entries, err := parseLokiLogStreams(resp.Data.Result)
	if err != nil {
		return nil, err
	}
	return &QueryLokiLogsResult{Entries: entries, Total: len(entries)}, nil
}

// parseLokiLogStreams parses the streams result type from a Loki query response.
func parseLokiLogStreams(raw json.RawMessage) ([]LokiLogEntry, error) {
	var streams []struct {
		Stream map[string]string   `json:"stream"`
		Values [][]json.RawMessage `json:"values"`
	}
	if err := json.Unmarshal(raw, &streams); err != nil {
		return nil, fmt.Errorf("decode loki streams: %w", err)
	}

	entries := make([]LokiLogEntry, 0)
	for _, stream := range streams {
		labels := stream.Stream
		if labels == nil {
			labels = map[string]string{}
		}
		for _, value := range stream.Values {
			if len(value) < 2 {
				continue
			}
			var ts, line string
			if err := json.Unmarshal(value[0], &ts); err != nil {
				continue
			}
			if err := json.Unmarshal(value[1], &line); err != nil {
				continue
			}
			entries = append(entries, LokiLogEntry{Timestamp: ts, Line: line, Labels: labels})
		}
	}
	return entries, nil
}

var QueryLokiLogsTool = mcpgrafana.MustTool(
	"query_loki_logs",
	"Execute a LogQL query against a Loki datasource to retrieve log entries. Defaults to the last hour, limit 10, newest first.",
	queryLokiLogs,
	mcp.WithTitleAnnotation("Query Loki logs"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// ─── query_loki_stats ─────────────────────────────────────────────────────────

// LokiStats represents Loki index/stats response fields.
type LokiStats struct {
	Streams int `json:"streams"`
	Chunks  int `json:"chunks"`
	Entries int `json:"entries"`
	Bytes   int `json:"bytes"`
}

type QueryLokiStatsRequest struct {
	DatasourceUID string `json:"datasourceUid" jsonschema:"required,description=The UID of the Loki datasource"`
	LogQL         string `json:"logql" jsonschema:"required,description=LogQL stream selector (e.g. '{app=\"nginx\"}')"`
	StartRFC3339  string `json:"startRfc3339,omitempty" jsonschema:"description=Start time in RFC3339 format (default: 1 hour ago)"`
	EndRFC3339    string `json:"endRfc3339,omitempty" jsonschema:"description=End time in RFC3339 format (default: now)"`
}

func queryLokiStats(ctx context.Context, args QueryLokiStatsRequest) (*LokiStats, error) {
	if strings.TrimSpace(args.LogQL) == "" {
		return nil, fmt.Errorf("logql is required")
	}
	dsID, err := resolveLokiDatasourceID(ctx, args.DatasourceUID)
	if err != nil {
		return nil, err
	}

	start, end := lokiDefaultTimeRange(args.StartRFC3339, args.EndRFC3339)
	params := url.Values{}
	params.Set("query", args.LogQL)
	lokiAddTimeRange(params, start, end)

	body, err := lokiDoRequest(ctx, dsID, "/loki/api/v1/index/stats", params)
	if err != nil {
		return nil, fmt.Errorf("query loki stats: %w", err)
	}

	var stats LokiStats
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("decode loki stats response: %w", err)
	}
	return &stats, nil
}

var QueryLokiStatsTool = mcpgrafana.MustTool(
	"query_loki_stats",
	"Retrieve stream statistics (streams, chunks, entries, bytes) from Loki for a label selector. Use before expensive queries to check if data exists.",
	queryLokiStats,
	mcp.WithTitleAnnotation("Query Loki stats"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// ─── query_loki_patterns ──────────────────────────────────────────────────────

// LokiPattern represents a detected log pattern with its occurrence count.
type LokiPattern struct {
	Pattern    string `json:"pattern"`
	TotalCount int64  `json:"totalCount"`
}

type QueryLokiPatternsRequest struct {
	DatasourceUID string `json:"datasourceUid" jsonschema:"required,description=The UID of the Loki datasource"`
	LogQL         string `json:"logql" jsonschema:"required,description=LogQL stream selector (e.g. '{job=\"nginx\"}')"`
	StartRFC3339  string `json:"startRfc3339,omitempty" jsonschema:"description=Start time in RFC3339 format (default: 1 hour ago)"`
	EndRFC3339    string `json:"endRfc3339,omitempty" jsonschema:"description=End time in RFC3339 format (default: now)"`
	Step          string `json:"step,omitempty" jsonschema:"description=Query resolution step (e.g. '5m')"`
}

func queryLokiPatterns(ctx context.Context, args QueryLokiPatternsRequest) ([]LokiPattern, error) {
	if strings.TrimSpace(args.LogQL) == "" {
		return nil, fmt.Errorf("logql is required")
	}
	dsID, err := resolveLokiDatasourceID(ctx, args.DatasourceUID)
	if err != nil {
		return nil, err
	}

	start, end := lokiDefaultTimeRange(args.StartRFC3339, args.EndRFC3339)
	params := url.Values{}
	params.Set("query", args.LogQL)
	lokiAddTimeRange(params, start, end)
	if args.Step != "" {
		params.Set("step", args.Step)
	}

	body, err := lokiDoRequest(ctx, dsID, "/loki/api/v1/patterns", params)
	if err != nil {
		return nil, fmt.Errorf("query loki patterns: %w", err)
	}

	var raw struct {
		Status string `json:"status"`
		Data   []struct {
			Pattern string     `json:"pattern"`
			Samples [][2]int64 `json:"samples"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode loki patterns response: %w", err)
	}
	if raw.Status != "success" {
		return nil, fmt.Errorf("loki patterns API error: status=%s", raw.Status)
	}

	patterns := make([]LokiPattern, 0, len(raw.Data))
	for _, p := range raw.Data {
		var total int64
		for _, s := range p.Samples {
			total += s[1]
		}
		patterns = append(patterns, LokiPattern{Pattern: p.Pattern, TotalCount: total})
	}
	return patterns, nil
}

var QueryLokiPatternsTool = mcpgrafana.MustTool(
	"query_loki_patterns",
	"Retrieve detected log patterns from a Loki datasource (requires Loki 2.8+). Returns patterns with occurrence counts.",
	queryLokiPatterns,
	mcp.WithTitleAnnotation("Query Loki patterns"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
