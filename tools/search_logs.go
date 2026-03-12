package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

const (
	v84DefaultSearchLogsLimit = 100
	v84MaxSearchLogsLimit     = 1000
)

// SearchLogsRequest is the input for search_logs.
type SearchLogsRequest struct {
	DatasourceUID string `json:"datasourceUid" jsonschema:"required,description=The UID of a Loki or ClickHouse datasource"`
	Pattern       string `json:"pattern" jsonschema:"required,description=Text pattern or regex to search for in log messages"`
	Table         string `json:"table,omitempty" jsonschema:"description=Table name for ClickHouse queries (default: otel_logs). Ignored for Loki."`
	Start         string `json:"start,omitempty" jsonschema:"description=Start time ('now-1h'\\, RFC3339). Default: 1 hour ago"`
	End           string `json:"end,omitempty" jsonschema:"description=End time ('now'\\, RFC3339). Default: now"`
	Limit         int    `json:"limit,omitempty" jsonschema:"description=Max log entries (default: 100\\, max: 1000)"`
}

// SearchLogsResult is the output for search_logs.
type SearchLogsResult struct {
	Logs           []SearchLogEntry `json:"logs"`
	DatasourceType string           `json:"datasourceType"`
	Query          string           `json:"query"`
	TotalFound     int              `json:"totalFound"`
	Hints          []string         `json:"hints,omitempty"`
}

// SearchLogEntry is a normalized log line.
type SearchLogEntry struct {
	Timestamp string            `json:"timestamp"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
}

func searchLogs(ctx context.Context, args SearchLogsRequest) (*SearchLogsResult, error) {
	resolved, err := resolveDatasourceRef(ctx, DatasourceRef{UID: args.DatasourceUID})
	if err != nil {
		return nil, fmt.Errorf("resolve datasource: %w", err)
	}

	limit := args.Limit
	if limit <= 0 {
		limit = v84DefaultSearchLogsLimit
	}
	if limit > v84MaxSearchLogsLimit {
		limit = v84MaxSearchLogsLimit
	}

	// Parse time bounds (used for Loki; ClickHouse re-parses internally via queryClickHouse).
	now := time.Now()
	startTime := now.Add(-time.Hour)
	endTime := now

	if strings.TrimSpace(args.Start) != "" {
		t, err := parsePromTime(args.Start)
		if err != nil {
			return nil, fmt.Errorf("parse start: %w", err)
		}
		startTime = t
	}
	if strings.TrimSpace(args.End) != "" {
		t, err := parsePromTime(args.End)
		if err != nil {
			return nil, fmt.Errorf("parse end: %w", err)
		}
		endTime = t
	}

	dsType := strings.ToLower(resolved.Datasource.Type)
	switch {
	case strings.Contains(dsType, "loki"):
		return searchLogsInLoki(ctx, args, limit,
			startTime.Format(time.RFC3339),
			endTime.Format(time.RFC3339))
	case strings.Contains(dsType, "clickhouse"):
		return searchLogsInClickHouse(ctx, args, limit)
	default:
		return nil, fmt.Errorf("unsupported datasource type %q: search_logs supports loki and grafana-clickhouse-datasource", resolved.Datasource.Type)
	}
}

// isRegexLike returns true if pattern contains common regex metacharacters.
func isRegexLike(pattern string) bool {
	for _, c := range []string{".", "*", "+", "?", "^", "$", "[", "]", "(", ")", "{", "}", "|", "\\"} {
		if strings.Contains(pattern, c) {
			return true
		}
	}
	return false
}

func searchLogsInLoki(ctx context.Context, args SearchLogsRequest, limit int, startRFC3339, endRFC3339 string) (*SearchLogsResult, error) {
	var query string
	escaped := strings.ReplaceAll(args.Pattern, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	if isRegexLike(args.Pattern) {
		query = fmt.Sprintf(`{} |~ "%s"`, escaped)
	} else {
		query = fmt.Sprintf(`{} |= "%s"`, escaped)
	}

	result, err := queryLokiLogs(ctx, QueryLokiLogsRequest{
		DatasourceUID: args.DatasourceUID,
		LogQL:         query,
		StartRFC3339:  startRFC3339,
		EndRFC3339:    endRFC3339,
		Limit:         limit,
		Direction:     "backward",
	})
	if err != nil {
		return nil, fmt.Errorf("search loki: %w", err)
	}

	logs := make([]SearchLogEntry, 0, len(result.Entries))
	for _, e := range result.Entries {
		logs = append(logs, SearchLogEntry{
			Timestamp: e.Timestamp,
			Message:   e.Line,
			Labels:    e.Labels,
		})
	}

	sr := &SearchLogsResult{
		Logs:           logs,
		DatasourceType: lokiDatasourceType,
		Query:          query,
		TotalFound:     len(logs),
	}
	if len(logs) == 0 {
		sr.Hints = []string{
			"No logs found. Try a simpler pattern or a wider time range.",
			"Use list_loki_label_names to discover available labels.",
			"Use query_loki_stats to check if data exists in the time range.",
		}
	}
	return sr, nil
}

func searchLogsInClickHouse(ctx context.Context, args SearchLogsRequest, limit int) (*SearchLogsResult, error) {
	table := args.Table
	if table == "" {
		table = "otel_logs"
	}

	var whereClause string
	if isRegexLike(args.Pattern) {
		escaped := strings.ReplaceAll(args.Pattern, `'`, `''`)
		whereClause = fmt.Sprintf("match(Body, '%s')", escaped)
	} else {
		escaped := strings.ReplaceAll(args.Pattern, `'`, `''`)
		escaped = strings.ReplaceAll(escaped, `%`, `\%`)
		escaped = strings.ReplaceAll(escaped, `_`, `\_`)
		whereClause = fmt.Sprintf("Body ILIKE '%%%s%%'", escaped)
	}

	sql := fmt.Sprintf(
		`SELECT Timestamp, Body, ServiceName, SeverityText FROM %s WHERE %s AND $__timeFilter(Timestamp) ORDER BY Timestamp DESC LIMIT %d`,
		table, whereClause, limit,
	)

	result, err := queryClickHouse(ctx, QueryClickHouseRequest{
		DatasourceUID: args.DatasourceUID,
		Query:         sql,
		Start:         args.Start,
		End:           args.End,
		Limit:         limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search clickhouse: %w", err)
	}

	logs := make([]SearchLogEntry, 0, len(result.Rows))
	for _, row := range result.Rows {
		entry := SearchLogEntry{Labels: make(map[string]string)}
		if ts, ok := row["Timestamp"].(string); ok {
			entry.Timestamp = ts
		}
		if body, ok := row["Body"].(string); ok {
			entry.Message = body
		}
		if svc, ok := row["ServiceName"].(string); ok && svc != "" {
			entry.Labels["service"] = svc
		}
		if sev, ok := row["SeverityText"].(string); ok && sev != "" {
			entry.Labels["level"] = sev
		}
		logs = append(logs, entry)
	}

	sr := &SearchLogsResult{
		Logs:           logs,
		DatasourceType: v84CHDatasourceType,
		Query:          sql,
		TotalFound:     len(logs),
	}
	if len(logs) == 0 {
		sr.Hints = []string{
			"No logs found. IMPORTANT: Run list_clickhouse_tables first to verify the table exists.",
			"Default table 'otel_logs' may not exist — use the 'table' parameter to specify the correct table.",
			"Use describe_clickhouse_table to verify column names.",
		}
	}
	return sr, nil
}

var SearchLogsTool = mcpgrafana.MustTool(
	"search_logs",
	`Search for log entries matching a text pattern across Loki and ClickHouse datasources.

For ClickHouse: Run list_clickhouse_tables FIRST. Default table 'otel_logs' may not exist.

Examples:
- Pattern "error"          → case-insensitive substring match
- Pattern "timeout|refused" → regex match`,
	searchLogs,
	mcp.WithTitleAnnotation("Search logs"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
