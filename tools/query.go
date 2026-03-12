package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

type QueryDatasourceRequest struct {
	From       string           `json:"from" jsonschema:"required,description=From time\\, e.g. now-1h"`
	To         string           `json:"to" jsonschema:"required,description=To time\\, e.g. now"`
	Debug      *bool            `json:"debug,omitempty" jsonschema:"description=Enable query debug mode"`
	Datasource *DatasourceRef   `json:"datasource,omitempty" jsonschema:"description=Optional datasource reference"`
	Queries    []map[string]any `json:"queries" jsonschema:"required,description=Datasource query payload list"`
}

type QueryDatasourceResponse struct {
	Raw       json.RawMessage `json:"raw,omitempty"`
	Responses map[string]any  `json:"responses,omitempty"`
	Hints     []string        `json:"hints,omitempty"`
}

func queryDatasource(ctx context.Context, args QueryDatasourceRequest) (*QueryDatasourceResponse, error) {
	if args.From == "" || args.To == "" {
		return nil, fmt.Errorf("from and to are required")
	}
	if len(args.Queries) == 0 {
		return nil, fmt.Errorf("queries is required")
	}

	var resolvedID *int64
	if args.Datasource != nil {
		resolved, err := resolveDatasourceRef(ctx, *args.Datasource)
		if err != nil {
			return nil, fmt.Errorf("resolve datasource: %w", err)
		}
		resolvedID = &resolved.Datasource.ID
	}

	normalizedQueries := make([]map[string]any, 0, len(args.Queries))
	for _, q := range args.Queries {
		copyQ := make(map[string]any, len(q)+1)
		for k, v := range q {
			copyQ[k] = v
		}
		if resolvedID != nil {
			if _, exists := copyQ["datasourceId"]; !exists {
				copyQ["datasourceId"] = *resolvedID
			}
		}
		normalizedQueries = append(normalizedQueries, copyQ)
	}

	requestBody := map[string]any{
		"from":    args.From,
		"to":      args.To,
		"queries": normalizedQueries,
	}
	if args.Debug != nil {
		requestBody["debug"] = *args.Debug
	}

	respBody, statusCode, err := doAPIRequest(ctx, "POST", "/tsdb/query", nil, requestBody)
	if err != nil {
		return nil, fmt.Errorf("query datasource: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	response := &QueryDatasourceResponse{Raw: json.RawMessage(respBody)}

	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err == nil {
		if results, ok := parsed["results"].(map[string]any); ok {
			response.Responses = results
		} else {
			response.Responses = parsed
		}

		hints := make([]string, 0)
		if message, ok := parsed["message"].(string); ok && message != "" {
			hints = append(hints, message)
		}
		if len(hints) > 0 {
			response.Hints = hints
		}
	}

	return response, nil
}

var QueryDatasourceTool = mcpgrafana.MustTool(
	"query_datasource",
	"Query datasource metrics via Grafana /api/tsdb/query.",
	queryDatasource,
	mcp.WithTitleAnnotation("Query datasource"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// --- query_datasource_expressions ---

type QueryDatasourceExpressionsRequest struct {
	From       string           `json:"from" jsonschema:"required,description=From time\\, e.g. now-1h"`
	To         string           `json:"to" jsonschema:"required,description=To time\\, e.g. now"`
	Queries    []map[string]any `json:"queries" jsonschema:"required,description=Query list; each should include datasource:{uid\\,type} and refId"`
	Datasource *DatasourceRef   `json:"datasource,omitempty" jsonschema:"description=Optional datasource reference; auto-injected into queries that lack a datasource field"`
}

type QueryDatasourceExpressionsResponse struct {
	Raw     json.RawMessage            `json:"raw,omitempty"`
	Results map[string]json.RawMessage `json:"results,omitempty"`
}

func queryDatasourceExpressions(ctx context.Context, args QueryDatasourceExpressionsRequest) (*QueryDatasourceExpressionsResponse, error) {
	if args.From == "" || args.To == "" {
		return nil, fmt.Errorf("from and to are required")
	}
	if len(args.Queries) == 0 {
		return nil, fmt.Errorf("queries is required")
	}

	var dsRef map[string]string
	if args.Datasource != nil {
		resolved, err := resolveDatasourceRef(ctx, *args.Datasource)
		if err != nil {
			return nil, fmt.Errorf("resolve datasource: %w", err)
		}
		dsRef = map[string]string{
			"uid":  resolved.Datasource.UID,
			"type": resolved.Datasource.Type,
		}
	}

	normalizedQueries := make([]map[string]any, 0, len(args.Queries))
	for _, q := range args.Queries {
		copyQ := make(map[string]any, len(q))
		for k, v := range q {
			copyQ[k] = v
		}
		if dsRef != nil {
			if _, exists := copyQ["datasource"]; !exists {
				copyQ["datasource"] = dsRef
			}
		}
		normalizedQueries = append(normalizedQueries, copyQ)
	}

	requestBody := map[string]any{
		"queries": normalizedQueries,
		"from":    args.From,
		"to":      args.To,
	}

	respBody, statusCode, err := doAPIRequest(ctx, "POST", "/ds/query", nil, requestBody)
	if err != nil {
		// Grafana 8.4.x deployments often reject /api/ds/query for non-expression payloads.
		// Fall back to legacy /api/tsdb/query for compatibility.
		if shouldFallbackToTSDB(statusCode) {
			fallbackResp, fallbackErr := fallbackExpressionsToTSDB(ctx, args, normalizedQueries)
			if fallbackErr == nil {
				return fallbackResp, nil
			}
			return nil, fmt.Errorf("query datasource expressions: %w (fallback /tsdb/query failed: %v)", wrapRawAPIError(statusCode, respBody, err), fallbackErr)
		}
		return nil, fmt.Errorf("query datasource expressions: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	response := &QueryDatasourceExpressionsResponse{Raw: json.RawMessage(respBody)}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(respBody, &parsed); err == nil {
		if rawResults, ok := parsed["results"]; ok {
			var resultsMap map[string]json.RawMessage
			if err := json.Unmarshal(rawResults, &resultsMap); err == nil {
				response.Results = resultsMap
			}
		}
	}

	return response, nil
}

func shouldFallbackToTSDB(statusCode int) bool {
	switch statusCode {
	case 400, 404, 405, 501:
		return true
	default:
		return false
	}
}

func fallbackExpressionsToTSDB(ctx context.Context, args QueryDatasourceExpressionsRequest, normalizedQueries []map[string]any) (*QueryDatasourceExpressionsResponse, error) {
	tsdbQueries, err := normalizeQueriesForTSDB(ctx, normalizedQueries)
	if err != nil {
		return nil, err
	}

	tsdbResp, err := queryDatasource(ctx, QueryDatasourceRequest{
		From:       args.From,
		To:         args.To,
		Datasource: args.Datasource,
		Queries:    tsdbQueries,
	})
	if err != nil {
		return nil, err
	}

	resp := &QueryDatasourceExpressionsResponse{Raw: tsdbResp.Raw}
	if len(tsdbResp.Responses) > 0 {
		resp.Results = make(map[string]json.RawMessage, len(tsdbResp.Responses))
		for refID, v := range tsdbResp.Responses {
			b, marshalErr := json.Marshal(v)
			if marshalErr != nil {
				continue
			}
			resp.Results[refID] = json.RawMessage(b)
		}
	}
	return resp, nil
}

func normalizeQueriesForTSDB(ctx context.Context, queries []map[string]any) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(queries))
	for _, q := range queries {
		copyQ := make(map[string]any, len(q))
		for k, v := range q {
			copyQ[k] = v
		}

		if _, exists := copyQ["datasourceId"]; !exists {
			if dsRaw, hasDS := copyQ["datasource"]; hasDS {
				dsRef, ok := datasourceRefFromAny(dsRaw)
				if !ok {
					return nil, fmt.Errorf("invalid query datasource field for refId %v", copyQ["refId"])
				}
				resolved, err := resolveDatasourceRef(ctx, dsRef)
				if err != nil {
					return nil, fmt.Errorf("resolve query datasource for refId %v: %w", copyQ["refId"], err)
				}
				copyQ["datasourceId"] = resolved.Datasource.ID
			}
		}
		delete(copyQ, "datasource")
		out = append(out, copyQ)
	}
	return out, nil
}

func datasourceRefFromAny(v any) (DatasourceRef, bool) {
	switch ds := v.(type) {
	case map[string]any:
		ref := DatasourceRef{}
		if id, ok := anyToInt64(ds["id"]); ok && id > 0 {
			ref.ID = &id
		}
		if uid, ok := ds["uid"].(string); ok {
			ref.UID = strings.TrimSpace(uid)
		}
		if name, ok := ds["name"].(string); ok {
			ref.Name = strings.TrimSpace(name)
		}
		if ref.ID != nil || ref.UID != "" || ref.Name != "" {
			return ref, true
		}
	case map[string]string:
		ref := DatasourceRef{
			UID:  strings.TrimSpace(ds["uid"]),
			Name: strings.TrimSpace(ds["name"]),
		}
		if ref.UID != "" || ref.Name != "" {
			return ref, true
		}
	}
	return DatasourceRef{}, false
}

var QueryDatasourceExpressionsTool = mcpgrafana.MustTool(
	"query_datasource_expressions",
	`Query datasource using Grafana's unified /api/ds/query endpoint (supports expressions and data frames).

Each query in the queries array should include "datasource": {"uid": "...", "type": "..."} and "refId".
Alternatively provide a datasource parameter to auto-inject into all queries that lack a datasource field.
Returns raw data frames response; use query_datasource for the legacy tsdb format.`,
	queryDatasourceExpressions,
	mcp.WithTitleAnnotation("Query datasource expressions"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
