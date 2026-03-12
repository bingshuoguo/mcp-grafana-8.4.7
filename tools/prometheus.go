package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

type QueryPrometheusRequest struct {
	Datasource *DatasourceRef `json:"datasource,omitempty" jsonschema:"description=Datasource reference (id/uid/name). Omit to use default Prometheus datasource"`
	Expr       string         `json:"expr" jsonschema:"required,description=PromQL expression"`
	Start      string         `json:"start" jsonschema:"required,description=Start time: 'now-1h'\\, 'now'\\, or RFC3339"`
	End        string         `json:"end,omitempty" jsonschema:"description=End time (default: now). Required for range queries."`
	Step       string         `json:"step,omitempty" jsonschema:"description=Step interval (default: auto). E.g. '30s'\\, '1m'\\, '1d'"`
	QueryType  string         `json:"queryType,omitempty" jsonschema:"description=Query type: 'range' (default) or 'instant'"`
}

type PromSample struct {
	Time  string `json:"time"`
	Value string `json:"value"`
}

type PromSeriesResult struct {
	Metric map[string]string `json:"metric"`
	Values []PromSample      `json:"values,omitempty"`
	Value  *PromSample       `json:"value,omitempty"`
}

type QueryPrometheusResponse struct {
	ResultType string             `json:"resultType"`
	Result     []PromSeriesResult `json:"result"`
	Hints      []string           `json:"hints,omitempty"`
}

type ListPrometheusLabelValuesRequest struct {
	Datasource *DatasourceRef `json:"datasource,omitempty" jsonschema:"description=Datasource reference (id/uid/name). Omit to use default Prometheus datasource"`
	LabelName  string         `json:"labelName" jsonschema:"required,description=Label name to query values for"`
	Start      string         `json:"start,omitempty" jsonschema:"description=Optional start time for filtering"`
	End        string         `json:"end,omitempty" jsonschema:"description=Optional end time for filtering"`
	Limit      int            `json:"limit,omitempty" jsonschema:"description=Max values to return (default 100)"`
}

type ListPrometheusLabelValuesResponse struct {
	LabelName string   `json:"labelName"`
	Values    []string `json:"values"`
	Total     int      `json:"total"`
	Truncated bool     `json:"truncated"`
}

type ListPrometheusMetricNamesRequest struct {
	Datasource *DatasourceRef `json:"datasource,omitempty" jsonschema:"description=Datasource reference (id/uid/name). Omit to use default Prometheus datasource"`
	Regex      string         `json:"regex,omitempty" jsonschema:"description=Optional regex filter on metric names"`
	Limit      int            `json:"limit,omitempty" jsonschema:"description=Max results (default 50)"`
	Page       int            `json:"page,omitempty" jsonschema:"description=Page number for pagination (default 1)"`
}

type ListPrometheusMetricNamesResponse struct {
	Metrics []string `json:"metrics"`
	Total   int      `json:"total"`
	Page    int      `json:"page"`
	HasMore bool     `json:"hasMore"`
}

type promQueryResponse struct {
	Status    string   `json:"status"`
	Data      promData `json:"data"`
	Error     string   `json:"error,omitempty"`
	ErrorType string   `json:"errorType,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

type promData struct {
	ResultType string          `json:"resultType"`
	Result     json.RawMessage `json:"result"`
}

type promSeries struct {
	Metric map[string]string `json:"metric"`
	Values [][]any           `json:"values,omitempty"`
	Value  []any             `json:"value,omitempty"`
}

type promLabelValuesResponse struct {
	Status    string   `json:"status"`
	Data      []string `json:"data"`
	Error     string   `json:"error,omitempty"`
	ErrorType string   `json:"errorType,omitempty"`
}

func queryPrometheus(ctx context.Context, args QueryPrometheusRequest) (*QueryPrometheusResponse, error) {
	if strings.TrimSpace(args.Expr) == "" {
		return nil, fmt.Errorf("expr is required")
	}
	if strings.TrimSpace(args.Start) == "" {
		return nil, fmt.Errorf("start is required")
	}

	start, err := parsePromTime(args.Start)
	if err != nil {
		return nil, fmt.Errorf("parse start: %w", err)
	}
	end := time.Now()
	if strings.TrimSpace(args.End) != "" {
		end, err = parsePromTime(args.End)
		if err != nil {
			return nil, fmt.Errorf("parse end: %w", err)
		}
	}

	queryType := strings.ToLower(strings.TrimSpace(args.QueryType))
	if queryType == "" {
		queryType = "range"
	}
	if queryType != "range" && queryType != "instant" {
		return nil, fmt.Errorf("queryType must be 'range' or 'instant'")
	}

	if end.Before(start) {
		return nil, fmt.Errorf("end must be after start")
	}

	dsID, err := resolvePrometheusDatasourceID(ctx, args.Datasource)
	if err != nil {
		return nil, fmt.Errorf("resolve datasource: %w", err)
	}

	query := url.Values{}
	query.Set("query", args.Expr)

	path := fmt.Sprintf("/datasources/proxy/%d/api/v1/query_range", dsID)
	if queryType == "instant" {
		path = fmt.Sprintf("/datasources/proxy/%d/api/v1/query", dsID)
		query.Set("time", formatPromTimestamp(start))
	} else {
		stepSeconds, err := parsePromStep(args.Step, start, end)
		if err != nil {
			return nil, err
		}
		query.Set("start", formatPromTimestamp(start))
		query.Set("end", formatPromTimestamp(end))
		query.Set("step", strconv.Itoa(stepSeconds))
	}

	respBody, statusCode, err := doAPIRequest(ctx, "GET", path, query, nil)
	if err != nil {
		return nil, fmt.Errorf("query prometheus: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	return parsePromQueryResult(respBody, queryType, args.Expr, start, end)
}

func listPrometheusLabelValues(ctx context.Context, args ListPrometheusLabelValuesRequest) (*ListPrometheusLabelValuesResponse, error) {
	if strings.TrimSpace(args.LabelName) == "" {
		return nil, fmt.Errorf("labelName is required")
	}

	dsID, err := resolvePrometheusDatasourceID(ctx, args.Datasource)
	if err != nil {
		return nil, fmt.Errorf("resolve datasource: %w", err)
	}

	var start, end *time.Time
	if strings.TrimSpace(args.Start) != "" {
		t, err := parsePromTime(args.Start)
		if err != nil {
			return nil, fmt.Errorf("parse start: %w", err)
		}
		start = &t
	}
	if strings.TrimSpace(args.End) != "" {
		t, err := parsePromTime(args.End)
		if err != nil {
			return nil, fmt.Errorf("parse end: %w", err)
		}
		end = &t
	}

	values, err := fetchPromLabelValues(ctx, dsID, args.LabelName, start, end)
	if err != nil {
		return nil, err
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 10000 {
		limit = 10000
	}

	total := len(values)
	truncated := false
	if len(values) > limit {
		values = values[:limit]
		truncated = true
	}

	return &ListPrometheusLabelValuesResponse{
		LabelName: args.LabelName,
		Values:    values,
		Total:     total,
		Truncated: truncated,
	}, nil
}

func listPrometheusMetricNames(ctx context.Context, args ListPrometheusMetricNamesRequest) (*ListPrometheusMetricNamesResponse, error) {
	dsID, err := resolvePrometheusDatasourceID(ctx, args.Datasource)
	if err != nil {
		return nil, fmt.Errorf("resolve datasource: %w", err)
	}

	values, err := fetchPromLabelValues(ctx, dsID, "__name__", nil, nil)
	if err != nil {
		return nil, err
	}

	filtered := values
	if strings.TrimSpace(args.Regex) != "" {
		re, err := regexp.Compile(args.Regex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
		filtered = make([]string, 0, len(values))
		for _, v := range values {
			if re.MatchString(v) {
				filtered = append(filtered, v)
			}
		}
	}
	sort.Strings(filtered)

	limit := args.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 10000 {
		limit = 10000
	}

	page := args.Page
	if page <= 0 {
		page = 1
	}

	total := len(filtered)
	start := (page - 1) * limit
	if start >= total {
		return &ListPrometheusMetricNamesResponse{
			Metrics: []string{},
			Total:   total,
			Page:    page,
			HasMore: false,
		}, nil
	}

	end := start + limit
	if end > total {
		end = total
	}

	return &ListPrometheusMetricNamesResponse{
		Metrics: filtered[start:end],
		Total:   total,
		Page:    page,
		HasMore: end < total,
	}, nil
}

func fetchPromLabelValues(ctx context.Context, dsID int64, labelName string, start, end *time.Time) ([]string, error) {
	path := fmt.Sprintf("/datasources/proxy/%d/api/v1/label/%s/values", dsID, url.PathEscape(labelName))
	query := url.Values{}
	if start != nil {
		query.Set("start", formatPromTimestamp(*start))
	}
	if end != nil {
		query.Set("end", formatPromTimestamp(*end))
	}

	respBody, statusCode, err := doAPIRequest(ctx, "GET", path, query, nil)
	if err != nil {
		return nil, fmt.Errorf("list label values: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	var payload promLabelValuesResponse
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("decode label values response: %w", err)
	}
	if payload.Status != "" && payload.Status != "success" {
		msg := payload.Error
		if msg == "" {
			msg = "prometheus label values query failed"
		}
		if payload.ErrorType != "" {
			msg = fmt.Sprintf("%s (%s)", msg, payload.ErrorType)
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return payload.Data, nil
}

func parsePromQueryResult(respBody []byte, queryType, expr string, start, end time.Time) (*QueryPrometheusResponse, error) {
	var payload promQueryResponse
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}
	if payload.Status != "" && payload.Status != "success" {
		msg := payload.Error
		if msg == "" {
			msg = "prometheus query failed"
		}
		if payload.ErrorType != "" {
			msg = fmt.Sprintf("%s (%s)", msg, payload.ErrorType)
		}
		return nil, fmt.Errorf("%s", msg)
	}

	out := &QueryPrometheusResponse{
		ResultType: payload.Data.ResultType,
		Result:     []PromSeriesResult{},
	}

	switch payload.Data.ResultType {
	case "matrix":
		var seriesList []promSeries
		if err := json.Unmarshal(payload.Data.Result, &seriesList); err != nil {
			return nil, fmt.Errorf("decode matrix result: %w", err)
		}
		out.Result = make([]PromSeriesResult, 0, len(seriesList))
		for _, series := range seriesList {
			item := PromSeriesResult{
				Metric: mapOrEmpty(series.Metric),
				Values: make([]PromSample, 0, len(series.Values)),
			}
			for _, raw := range series.Values {
				sample, err := convertPromSample(raw)
				if err != nil {
					return nil, fmt.Errorf("parse matrix sample: %w", err)
				}
				item.Values = append(item.Values, sample)
			}
			out.Result = append(out.Result, item)
		}
	case "vector":
		var seriesList []promSeries
		if err := json.Unmarshal(payload.Data.Result, &seriesList); err != nil {
			return nil, fmt.Errorf("decode vector result: %w", err)
		}
		out.Result = make([]PromSeriesResult, 0, len(seriesList))
		for _, series := range seriesList {
			item := PromSeriesResult{
				Metric: mapOrEmpty(series.Metric),
			}
			sample, err := convertPromSample(series.Value)
			if err != nil {
				return nil, fmt.Errorf("parse vector sample: %w", err)
			}
			item.Value = &sample
			out.Result = append(out.Result, item)
		}
	case "scalar", "string":
		var rawSample []any
		if err := json.Unmarshal(payload.Data.Result, &rawSample); err != nil {
			return nil, fmt.Errorf("decode %s result: %w", payload.Data.ResultType, err)
		}
		if len(rawSample) > 0 {
			sample, err := convertPromSample(rawSample)
			if err != nil {
				return nil, fmt.Errorf("parse %s sample: %w", payload.Data.ResultType, err)
			}
			out.Result = append(out.Result, PromSeriesResult{
				Metric: map[string]string{},
				Value:  &sample,
			})
		}
	default:
		return nil, fmt.Errorf("unsupported resultType: %s", payload.Data.ResultType)
	}

	if len(out.Result) == 0 {
		out.Hints = []string{
			"No data returned for the current query and time range.",
			"Try widening the time range or adjusting step resolution.",
			fmt.Sprintf("Verify PromQL expression: %s (%s query)", expr, queryType),
		}
		if !end.After(start) {
			out.Hints = append(out.Hints, "Ensure end time is after start time.")
		}
	}

	return out, nil
}

func parsePromStep(step string, start, end time.Time) (int, error) {
	if strings.TrimSpace(step) == "" {
		return defaultPromStep(start, end), nil
	}

	d, err := parseFlexibleDuration(step)
	if err != nil {
		return 0, fmt.Errorf("invalid step: %w", err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("step must be positive")
	}

	seconds := int(math.Round(d.Seconds()))
	if seconds <= 0 {
		seconds = 1
	}
	return seconds, nil
}

func defaultPromStep(start, end time.Time) int {
	d := end.Sub(start)
	switch {
	case d <= 30*time.Minute:
		return 30
	case d <= 3*time.Hour:
		return 60
	case d <= 24*time.Hour:
		return 300
	default:
		return 3600
	}
}

func resolvePrometheusDatasourceID(ctx context.Context, ref *DatasourceRef) (int64, error) {
	if ref != nil {
		resolved, err := resolveDatasourceRef(ctx, *ref)
		if err != nil {
			return 0, err
		}
		if !isPrometheusDatasourceType(resolved.Datasource.Type) {
			return 0, fmt.Errorf("datasource %q is type %q, expected prometheus", resolved.Datasource.Name, resolved.Datasource.Type)
		}
		return resolved.Datasource.ID, nil
	}

	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return 0, err
	}

	resp, err := gc.Datasources.GetDataSources()
	if err != nil {
		return 0, fmt.Errorf("list datasources: %w", wrapOpenAPIError(err))
	}

	var candidates []int64
	for _, ds := range resp.Payload {
		if ds == nil || !isPrometheusDatasourceType(ds.Type) {
			continue
		}
		if ds.IsDefault {
			return ds.ID, nil
		}
		candidates = append(candidates, ds.ID)
	}

	switch len(candidates) {
	case 0:
		return 0, fmt.Errorf("no prometheus datasource found; provide datasource id/uid/name")
	case 1:
		return candidates[0], nil
	default:
		return 0, fmt.Errorf("multiple prometheus datasources found; provide datasource id/uid/name")
	}
}

func isPrometheusDatasourceType(dsType string) bool {
	return strings.EqualFold(strings.TrimSpace(dsType), "prometheus")
}

func parsePromTime(value string) (time.Time, error) {
	return parsePromTimeAt(value, time.Now())
}

func parsePromTimeAt(value string, now time.Time) (time.Time, error) {
	s := strings.TrimSpace(value)
	if s == "" || s == "now" {
		return now, nil
	}

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	if strings.HasPrefix(s, "now") {
		suffix := strings.TrimSpace(s[3:])
		if suffix == "" {
			return now, nil
		}
		d, err := parseFlexibleDuration(suffix)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid time expression %q: %w", s, err)
		}
		return now.Add(d), nil
	}

	return time.Time{}, fmt.Errorf("unsupported time format %q", s)
}

var durationTokenPattern = regexp.MustCompile(`(\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h|d)`)

func parseFlexibleDuration(value string) (time.Duration, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	sign := 1.0
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	} else if strings.HasPrefix(s, "-") {
		sign = -1
		s = s[1:]
	}
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	matches := durationTokenPattern.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid duration %q", value)
	}

	joined := ""
	totalNanos := 0.0
	for _, m := range matches {
		token := m[0]
		joined += token
		numStr := m[1]
		unitStr := m[2]

		amount, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration number %q", numStr)
		}

		unit, ok := durationUnit(unitStr)
		if !ok {
			return 0, fmt.Errorf("invalid duration unit %q", unitStr)
		}

		totalNanos += amount * float64(unit)
	}

	if joined != s {
		return 0, fmt.Errorf("invalid duration %q", value)
	}

	totalNanos *= sign
	return time.Duration(math.Round(totalNanos)), nil
}

func durationUnit(unit string) (time.Duration, bool) {
	switch unit {
	case "ns":
		return time.Nanosecond, true
	case "us", "µs":
		return time.Microsecond, true
	case "ms":
		return time.Millisecond, true
	case "s":
		return time.Second, true
	case "m":
		return time.Minute, true
	case "h":
		return time.Hour, true
	case "d":
		return 24 * time.Hour, true
	default:
		return 0, false
	}
}

func formatPromTimestamp(t time.Time) string {
	seconds := float64(t.UnixNano()) / float64(time.Second)
	return strconv.FormatFloat(seconds, 'f', -1, 64)
}

func convertPromSample(raw []any) (PromSample, error) {
	if len(raw) != 2 {
		return PromSample{}, fmt.Errorf("sample must have exactly 2 fields")
	}

	ts, err := toFloat64(raw[0])
	if err != nil {
		return PromSample{}, fmt.Errorf("invalid timestamp: %w", err)
	}

	ns := int64(math.Round(ts * float64(time.Second)))
	t := time.Unix(0, ns).UTC()

	value := ""
	switch v := raw[1].(type) {
	case string:
		value = v
	default:
		value = fmt.Sprintf("%v", v)
	}

	return PromSample{
		Time:  t.Format(time.RFC3339Nano),
		Value: value,
	}, nil
}

func toFloat64(v any) (float64, error) {
	switch tv := v.(type) {
	case float64:
		return tv, nil
	case json.Number:
		return tv.Float64()
	case string:
		return strconv.ParseFloat(tv, 64)
	default:
		return 0, fmt.Errorf("unsupported number type %T", v)
	}
}

func mapOrEmpty(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

var QueryPrometheusTool = mcpgrafana.MustTool(
	"query_prometheus",
	"WORKFLOW: list_prometheus_metric_names -> list_prometheus_label_values -> query_prometheus. Execute a PromQL query against a Prometheus datasource via Grafana proxy. Returns time series with RFC3339 timestamps.",
	queryPrometheus,
	mcp.WithTitleAnnotation("Query Prometheus metrics"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

var ListPrometheusLabelValuesTool = mcpgrafana.MustTool(
	"list_prometheus_label_values",
	"Get values for a label in a Prometheus datasource. Useful for resolving template variables before building PromQL queries.",
	listPrometheusLabelValues,
	mcp.WithTitleAnnotation("List Prometheus label values"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

var ListPrometheusMetricNamesTool = mcpgrafana.MustTool(
	"list_prometheus_metric_names",
	"DISCOVERY: Call this first to find available metrics before querying. Lists metric names with optional regex filter and pagination.",
	listPrometheusMetricNames,
	mcp.WithTitleAnnotation("List Prometheus metric names"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
