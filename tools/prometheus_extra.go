package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

// --- list_prometheus_label_names ---

type ListPrometheusLabelNamesRequest struct {
	Datasource *DatasourceRef `json:"datasource,omitempty" jsonschema:"description=Datasource reference (id/uid/name). Omit to use default Prometheus datasource"`
	Start      string         `json:"start,omitempty" jsonschema:"description=Optional start time for filtering"`
	End        string         `json:"end,omitempty" jsonschema:"description=Optional end time for filtering"`
}

type ListPrometheusLabelNamesResponse struct {
	Labels []string `json:"labels"`
	Total  int      `json:"total"`
}

func listPrometheusLabelNames(ctx context.Context, args ListPrometheusLabelNamesRequest) (*ListPrometheusLabelNamesResponse, error) {
	dsID, err := resolvePrometheusDatasourceID(ctx, args.Datasource)
	if err != nil {
		return nil, fmt.Errorf("resolve datasource: %w", err)
	}

	path := fmt.Sprintf("/datasources/proxy/%d/api/v1/labels", dsID)
	query := url.Values{}
	if strings.TrimSpace(args.Start) != "" {
		t, err := parsePromTime(args.Start)
		if err != nil {
			return nil, fmt.Errorf("parse start: %w", err)
		}
		query.Set("start", formatPromTimestamp(t))
	}
	if strings.TrimSpace(args.End) != "" {
		t, err := parsePromTime(args.End)
		if err != nil {
			return nil, fmt.Errorf("parse end: %w", err)
		}
		query.Set("end", formatPromTimestamp(t))
	}

	respBody, statusCode, err := doAPIRequest(ctx, "GET", path, query, nil)
	if err != nil {
		return nil, fmt.Errorf("list label names: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	var payload promLabelValuesResponse
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("decode label names response: %w", err)
	}
	if payload.Status != "" && payload.Status != "success" {
		msg := payload.Error
		if msg == "" {
			msg = "prometheus label names query failed"
		}
		return nil, fmt.Errorf("%s", msg)
	}

	labels := payload.Data
	if labels == nil {
		labels = []string{}
	}
	return &ListPrometheusLabelNamesResponse{Labels: labels, Total: len(labels)}, nil
}

var ListPrometheusLabelNamesTool = mcpgrafana.MustTool(
	"list_prometheus_label_names",
	"List all label names (keys) available in a Prometheus datasource. Optionally filter by time range.",
	listPrometheusLabelNames,
	mcp.WithTitleAnnotation("List Prometheus label names"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// --- list_prometheus_metric_metadata ---

type ListPrometheusMetricMetadataRequest struct {
	Datasource *DatasourceRef `json:"datasource,omitempty" jsonschema:"description=Datasource reference (id/uid/name). Omit to use default Prometheus datasource"`
	Metric     string         `json:"metric,omitempty" jsonschema:"description=Filter by exact metric name"`
	Limit      int            `json:"limit,omitempty" jsonschema:"description=Max number of metrics to return (default 100)"`
}

type MetricMetadata struct {
	Type string `json:"type,omitempty"`
	Help string `json:"help,omitempty"`
	Unit string `json:"unit,omitempty"`
}

type MetricMetadataEntry struct {
	Metric   string           `json:"metric"`
	Metadata []MetricMetadata `json:"metadata"`
}

type ListPrometheusMetricMetadataResponse struct {
	Metrics []MetricMetadataEntry `json:"metrics"`
	Total   int                   `json:"total"`
}

func listPrometheusMetricMetadata(ctx context.Context, args ListPrometheusMetricMetadataRequest) (*ListPrometheusMetricMetadataResponse, error) {
	dsID, err := resolvePrometheusDatasourceID(ctx, args.Datasource)
	if err != nil {
		return nil, fmt.Errorf("resolve datasource: %w", err)
	}

	path := fmt.Sprintf("/datasources/proxy/%d/api/v1/metadata", dsID)
	query := url.Values{}
	if strings.TrimSpace(args.Metric) != "" {
		query.Set("metric", args.Metric)
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 100
	}
	query.Set("limit", fmt.Sprintf("%d", limit))

	respBody, statusCode, err := doAPIRequest(ctx, "GET", path, query, nil)
	if err != nil {
		return nil, fmt.Errorf("list metric metadata: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	// Response: {"status":"success","data":{"metric_name":[{"type":"...","help":"...","unit":""}]}}
	var raw struct {
		Status string `json:"status"`
		Data   map[string][]struct {
			Type string `json:"type"`
			Help string `json:"help"`
			Unit string `json:"unit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("decode metadata response: %w", err)
	}
	if raw.Status != "" && raw.Status != "success" {
		return nil, fmt.Errorf("prometheus metadata query failed")
	}

	entries := make([]MetricMetadataEntry, 0, len(raw.Data))
	for metricName, metas := range raw.Data {
		entry := MetricMetadataEntry{
			Metric:   metricName,
			Metadata: make([]MetricMetadata, 0, len(metas)),
		}
		for _, m := range metas {
			entry.Metadata = append(entry.Metadata, MetricMetadata{
				Type: m.Type,
				Help: m.Help,
				Unit: m.Unit,
			})
		}
		entries = append(entries, entry)
	}
	return &ListPrometheusMetricMetadataResponse{Metrics: entries, Total: len(entries)}, nil
}

var ListPrometheusMetricMetadataTool = mcpgrafana.MustTool(
	"list_prometheus_metric_metadata",
	"Get type and help text metadata for Prometheus metrics. Useful for understanding metric semantics before querying.",
	listPrometheusMetricMetadata,
	mcp.WithTitleAnnotation("List Prometheus metric metadata"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// --- query_prometheus_histogram ---

type QueryPrometheusHistogramRequest struct {
	Datasource *DatasourceRef `json:"datasource,omitempty" jsonschema:"description=Datasource reference (id/uid/name). Omit to use default Prometheus datasource"`
	Metric     string         `json:"metric" jsonschema:"required,description=Histogram metric base name (e.g. 'http_request_duration_seconds'). The '_bucket' suffix is appended automatically."`
	Quantile   float64        `json:"quantile,omitempty" jsonschema:"description=Quantile to compute (0.0-1.0\\, e.g. 0.95 for p95). Default: 0.95"`
	Selector   string         `json:"selector,omitempty" jsonschema:"description=Optional label selector (e.g. '{job=\"api-server\"}')"`
	Start      string         `json:"start" jsonschema:"required,description=Start time: 'now-1h'\\, 'now'\\, or RFC3339"`
	End        string         `json:"end,omitempty" jsonschema:"description=End time (default: now)"`
	Step       string         `json:"step,omitempty" jsonschema:"description=Query resolution step (e.g. '30s'\\, '1m'). Default: auto"`
	RateWindow string         `json:"rateWindow,omitempty" jsonschema:"description=Rate window for histogram_quantile (e.g. '5m'). Default: '5m'"`
	GroupBy    []string       `json:"groupBy,omitempty" jsonschema:"description=Additional labels to group by (always includes 'le')"`
}

func queryPrometheusHistogram(ctx context.Context, args QueryPrometheusHistogramRequest) (*QueryPrometheusResponse, error) {
	if strings.TrimSpace(args.Metric) == "" {
		return nil, fmt.Errorf("metric is required")
	}
	if strings.TrimSpace(args.Start) == "" {
		return nil, fmt.Errorf("start is required")
	}

	quantile := args.Quantile
	if quantile <= 0 || quantile > 1 {
		quantile = 0.95
	}

	rateWindow := "5m"
	if strings.TrimSpace(args.RateWindow) != "" {
		rateWindow = strings.TrimSpace(args.RateWindow)
	}

	bucketMetric := strings.TrimSuffix(strings.TrimSpace(args.Metric), "_bucket") + "_bucket"

	selector := strings.TrimSpace(args.Selector)
	if selector == "" {
		selector = "{}"
	}

	groupBy := append([]string{"le"}, args.GroupBy...)
	byClause := strings.Join(groupBy, ", ")

	// histogram_quantile(q, sum(rate(metric_bucket{selector}[window])) by (le, ...))
	metricExpr := bucketMetric + selector
	expr := fmt.Sprintf("histogram_quantile(%g, sum(rate(%s[%s])) by (%s))", quantile, metricExpr, rateWindow, byClause)

	return queryPrometheus(ctx, QueryPrometheusRequest{
		Datasource: args.Datasource,
		Expr:       expr,
		Start:      args.Start,
		End:        args.End,
		Step:       args.Step,
	})
}

var QueryPrometheusHistogramTool = mcpgrafana.MustTool(
	"query_prometheus_histogram",
	"Compute a histogram quantile (e.g. p95 latency) from a Prometheus histogram metric. Automatically generates the histogram_quantile PromQL expression.",
	queryPrometheusHistogram,
	mcp.WithTitleAnnotation("Query Prometheus histogram quantile"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
