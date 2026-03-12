package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

const (
	v84CHDatasourceType = "grafana-clickhouse-datasource"
	v84CHDefaultLimit   = 100
	v84CHMaxLimit       = 1000
	v84CHFormatTable    = 1
)

// resolveClickHouseUID validates the datasource type and returns its UID.
func resolveClickHouseUID(ctx context.Context, datasourceUID string) (string, error) {
	resolved, err := resolveDatasourceRef(ctx, DatasourceRef{UID: datasourceUID})
	if err != nil {
		return "", err
	}
	if resolved.Datasource.Type != v84CHDatasourceType {
		return "", fmt.Errorf("datasource %q is type %q, expected %s",
			resolved.Datasource.Name, resolved.Datasource.Type, v84CHDatasourceType)
	}
	return resolved.Datasource.UID, nil
}

// chQueryResponse is the response from Grafana's /api/ds/query for ClickHouse.
type chQueryResponse struct {
	Results map[string]struct {
		Status int `json:"status,omitempty"`
		Frames []struct {
			Schema struct {
				Fields []struct {
					Name string `json:"name"`
					Type string `json:"type"`
				} `json:"fields"`
			} `json:"schema"`
			Data struct {
				Values [][]any `json:"values"`
			} `json:"data"`
		} `json:"frames,omitempty"`
		Error string `json:"error,omitempty"`
	} `json:"results"`
}

// execCHSQL posts a SQL query to Grafana's /api/ds/query endpoint.
func execCHSQL(ctx context.Context, uid, sql string, fromTime, toTime time.Time) (*chQueryResponse, error) {
	payload := map[string]any{
		"queries": []map[string]any{
			{
				"datasource": map[string]string{
					"uid":  uid,
					"type": v84CHDatasourceType,
				},
				"rawSql": sql,
				"refId":  "A",
				"format": v84CHFormatTable,
			},
		},
		"from": strconv.FormatInt(fromTime.UnixMilli(), 10),
		"to":   strconv.FormatInt(toTime.UnixMilli(), 10),
	}

	respBody, statusCode, err := doAPIRequest(ctx, "POST", "/ds/query", nil, payload)
	if err != nil {
		return nil, fmt.Errorf("clickhouse query: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	var result chQueryResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode clickhouse response: %w", err)
	}
	return &result, nil
}

// processCHResponse extracts columns and rows from the columnar API response.
// ClickHouse queries always return a single result keyed by refId "A".
func processCHResponse(resp *chQueryResponse) ([]string, []map[string]any, error) {
	for refID, r := range resp.Results {
		if r.Error != "" {
			return nil, nil, fmt.Errorf("query error (refId=%s): %s", refID, r.Error)
		}
		if len(r.Frames) == 0 {
			return []string{}, []map[string]any{}, nil
		}
		frame := r.Frames[0]
		columns := make([]string, len(frame.Schema.Fields))
		for i, f := range frame.Schema.Fields {
			columns[i] = f.Name
		}
		if len(frame.Data.Values) == 0 {
			return columns, []map[string]any{}, nil
		}
		rowCount := len(frame.Data.Values[0])
		rows := make([]map[string]any, rowCount)
		for i := range rows {
			rows[i] = make(map[string]any)
			for colIdx, colName := range columns {
				if colIdx < len(frame.Data.Values) && i < len(frame.Data.Values[colIdx]) {
					rows[i][colName] = frame.Data.Values[colIdx][i]
				}
			}
		}
		return columns, rows, nil
	}
	return []string{}, []map[string]any{}, nil
}

// chEnforceLimit ensures the SQL has a LIMIT within the allowed maximum.
func chEnforceLimit(query string, requested int) string {
	limit := requested
	if limit <= 0 {
		limit = v84CHDefaultLimit
	}
	if limit > v84CHMaxLimit {
		limit = v84CHMaxLimit
	}

	limitRe := regexp.MustCompile(`(?i)\bLIMIT\s+\d+`)
	if limitRe.MatchString(query) {
		return limitRe.ReplaceAllStringFunc(query, func(match string) string {
			numStr := regexp.MustCompile(`\d+`).FindString(match)
			existing, _ := strconv.Atoi(numStr)
			if existing > v84CHMaxLimit {
				return fmt.Sprintf("LIMIT %d", v84CHMaxLimit)
			}
			return match
		})
	}
	query = strings.TrimSuffix(strings.TrimSpace(query), ";")
	return fmt.Sprintf("%s LIMIT %d", query, limit)
}

// chSubstituteMacros replaces ClickHouse Grafana macros in the query.
func chSubstituteMacros(query string, from, to time.Time) string {
	fromSecs := from.Unix()
	toSecs := to.Unix()
	fromMillis := from.UnixMilli()
	toMillis := to.UnixMilli()

	intervalSeconds := max((toSecs-fromSecs)/1000, 1)

	timeFilterRe := regexp.MustCompile(`\$__timeFilter\(([^)]+)\)`)
	query = timeFilterRe.ReplaceAllStringFunc(query, func(match string) string {
		sub := timeFilterRe.FindStringSubmatch(match)
		if len(sub) > 1 {
			col := strings.TrimSpace(sub[1])
			return fmt.Sprintf("%s >= toDateTime(%d) AND %s <= toDateTime(%d)", col, fromSecs, col, toSecs)
		}
		return match
	})

	query = strings.ReplaceAll(query, "$__from", strconv.FormatInt(fromMillis, 10))
	query = strings.ReplaceAll(query, "$__to", strconv.FormatInt(toMillis, 10))
	query = strings.ReplaceAll(query, "$__interval_ms", strconv.FormatInt(intervalSeconds*1000, 10))
	query = strings.ReplaceAll(query, "$__interval", fmt.Sprintf("%ds", intervalSeconds))
	return query
}

// ─── query_clickhouse ─────────────────────────────────────────────────────────

type QueryClickHouseRequest struct {
	DatasourceUID string            `json:"datasourceUid" jsonschema:"required,description=The UID of the ClickHouse datasource"`
	Query         string            `json:"query" jsonschema:"required,description=Raw SQL query. Supports macros: $__timeFilter(col)\\, $__from\\, $__to\\, $__interval"`
	Start         string            `json:"start,omitempty" jsonschema:"description=Start time ('now-1h'\\, RFC3339). Default: 1 hour ago"`
	End           string            `json:"end,omitempty" jsonschema:"description=End time ('now'\\, RFC3339). Default: now"`
	Limit         int               `json:"limit,omitempty" jsonschema:"description=Max rows (default: 100\\, max: 1000)"`
	Variables     map[string]string `json:"variables,omitempty" jsonschema:"description=Template variable substitutions ({varname: value})"`
}

type QueryClickHouseResult struct {
	Columns        []string         `json:"columns"`
	Rows           []map[string]any `json:"rows"`
	RowCount       int              `json:"rowCount"`
	ProcessedQuery string           `json:"processedQuery,omitempty"`
}

func queryClickHouse(ctx context.Context, args QueryClickHouseRequest) (*QueryClickHouseResult, error) {
	uid, err := resolveClickHouseUID(ctx, args.DatasourceUID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	fromTime := now.Add(-time.Hour)
	toTime := now

	if strings.TrimSpace(args.Start) != "" {
		t, err := parsePromTime(args.Start)
		if err != nil {
			return nil, fmt.Errorf("parse start: %w", err)
		}
		fromTime = t
	}
	if strings.TrimSpace(args.End) != "" {
		t, err := parsePromTime(args.End)
		if err != nil {
			return nil, fmt.Errorf("parse end: %w", err)
		}
		toTime = t
	}

	processed := chSubstituteMacros(args.Query, fromTime, toTime)
	for name, value := range args.Variables {
		processed = strings.ReplaceAll(processed, fmt.Sprintf("${%s}", name), value)
		varRe := regexp.MustCompile(fmt.Sprintf(`\$%s\b`, regexp.QuoteMeta(name)))
		processed = varRe.ReplaceAllString(processed, value)
	}
	processed = chEnforceLimit(processed, args.Limit)

	resp, err := execCHSQL(ctx, uid, processed, fromTime, toTime)
	if err != nil {
		return nil, enhanceClickHouseError(err, args.DatasourceUID)
	}

	columns, rows, err := processCHResponse(resp)
	if err != nil {
		return nil, err
	}
	return &QueryClickHouseResult{
		Columns:        columns,
		Rows:           rows,
		RowCount:       len(rows),
		ProcessedQuery: processed,
	}, nil
}

func enhanceClickHouseError(err error, datasourceUID string) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "query data error") {
		return fmt.Errorf("%w. Hint: datasourceUid=%s backend query failed; verify ClickHouse datasource permissions, plugin health, and upstream ClickHouse connectivity", err, datasourceUID)
	}
	return err
}

var QueryClickHouseTool = mcpgrafana.MustTool(
	"query_clickhouse",
	`Query ClickHouse via Grafana. Use list_clickhouse_tables first, then describe_clickhouse_table to see schemas.

Supports macros: $__timeFilter(column), $__from, $__to, $__interval

Example: SELECT Timestamp, Body FROM otel_logs WHERE $__timeFilter(Timestamp)`,
	queryClickHouse,
	mcp.WithTitleAnnotation("Query ClickHouse"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// ─── list_clickhouse_tables ───────────────────────────────────────────────────

type ListClickHouseTablesRequest struct {
	DatasourceUID string `json:"datasourceUid" jsonschema:"required,description=The UID of the ClickHouse datasource"`
	Database      string `json:"database,omitempty" jsonschema:"description=Database name filter (lists all non-system databases if omitted)"`
}

type ClickHouseTableInfo struct {
	Database   string `json:"database"`
	Name       string `json:"name"`
	Engine     string `json:"engine"`
	TotalRows  int64  `json:"totalRows"`
	TotalBytes int64  `json:"totalBytes"`
}

func listClickHouseTables(ctx context.Context, args ListClickHouseTablesRequest) ([]ClickHouseTableInfo, error) {
	sql := `SELECT database, name, engine, total_rows, total_bytes
FROM system.tables
WHERE database NOT IN ('system', 'INFORMATION_SCHEMA', 'information_schema')`
	if args.Database != "" {
		sql += fmt.Sprintf(" AND database = '%s'", args.Database)
	}
	sql += " ORDER BY database, name LIMIT 500"

	result, err := queryClickHouse(ctx, QueryClickHouseRequest{
		DatasourceUID: args.DatasourceUID,
		Query:         sql,
		Limit:         500,
	})
	if err != nil {
		return nil, err
	}

	tables := make([]ClickHouseTableInfo, 0, len(result.Rows))
	for _, row := range result.Rows {
		t := ClickHouseTableInfo{}
		if v, ok := row["database"].(string); ok {
			t.Database = v
		}
		if v, ok := row["name"].(string); ok {
			t.Name = v
		}
		if v, ok := row["engine"].(string); ok {
			t.Engine = v
		}
		if v, ok := row["total_rows"].(float64); ok {
			t.TotalRows = int64(v)
		}
		if v, ok := row["total_bytes"].(float64); ok {
			t.TotalBytes = int64(v)
		}
		tables = append(tables, t)
	}
	return tables, nil
}

var ListClickHouseTablesTool = mcpgrafana.MustTool(
	"list_clickhouse_tables",
	"START HERE for ClickHouse: List available tables (name, database, engine, row count, size). NEXT: Use describe_clickhouse_table to see column schemas.",
	listClickHouseTables,
	mcp.WithTitleAnnotation("List ClickHouse tables"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// ─── describe_clickhouse_table ────────────────────────────────────────────────

type DescribeClickHouseTableRequest struct {
	DatasourceUID string `json:"datasourceUid" jsonschema:"required,description=The UID of the ClickHouse datasource"`
	Table         string `json:"table" jsonschema:"required,description=Table name to describe"`
	Database      string `json:"database,omitempty" jsonschema:"description=Database name (defaults to 'default')"`
}

type ClickHouseColumnInfo struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	DefaultType       string `json:"defaultType,omitempty"`
	DefaultExpression string `json:"defaultExpression,omitempty"`
	Comment           string `json:"comment,omitempty"`
}

func describeClickHouseTable(ctx context.Context, args DescribeClickHouseTableRequest) ([]ClickHouseColumnInfo, error) {
	if strings.TrimSpace(args.Table) == "" {
		return nil, fmt.Errorf("table is required")
	}
	database := args.Database
	if database == "" {
		database = "default"
	}

	sql := fmt.Sprintf(`SELECT name, type, default_kind as default_type, default_expression, comment
FROM system.columns
WHERE database = '%s' AND table = '%s'
ORDER BY position`, database, args.Table)

	result, err := queryClickHouse(ctx, QueryClickHouseRequest{
		DatasourceUID: args.DatasourceUID,
		Query:         sql,
		Limit:         1000,
	})
	if err != nil {
		return nil, err
	}

	cols := make([]ClickHouseColumnInfo, 0, len(result.Rows))
	for _, row := range result.Rows {
		col := ClickHouseColumnInfo{}
		if v, ok := row["name"].(string); ok {
			col.Name = v
		}
		if v, ok := row["type"].(string); ok {
			col.Type = v
		}
		if v, ok := row["default_type"].(string); ok {
			col.DefaultType = v
		}
		if v, ok := row["default_expression"].(string); ok {
			col.DefaultExpression = v
		}
		if v, ok := row["comment"].(string); ok {
			col.Comment = v
		}
		cols = append(cols, col)
	}
	return cols, nil
}

var DescribeClickHouseTableTool = mcpgrafana.MustTool(
	"describe_clickhouse_table",
	"Get column schema for a ClickHouse table. Pass the database from list_clickhouse_tables results. NEXT: Use query_clickhouse with discovered column names.",
	describeClickHouseTable,
	mcp.WithTitleAnnotation("Describe ClickHouse table"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
