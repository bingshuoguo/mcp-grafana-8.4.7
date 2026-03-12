package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

// --- get_dashboard_panel_queries ---

type GetDashboardPanelQueriesRequest struct {
	UID     string `json:"uid" jsonschema:"required,description=Dashboard UID"`
	PanelID *int64 `json:"panelId,omitempty" jsonschema:"description=Filter to a specific panel ID"`
}

type PanelTarget struct {
	RefID      string         `json:"refId,omitempty"`
	Datasource map[string]any `json:"datasource,omitempty"`
	RawQuery   map[string]any `json:"rawQuery,omitempty"`
}

type PanelQueryInfo struct {
	PanelID    int64          `json:"panelId"`
	Title      string         `json:"title,omitempty"`
	Type       string         `json:"type,omitempty"`
	Datasource map[string]any `json:"datasource,omitempty"`
	Targets    []PanelTarget  `json:"targets"`
}

type GetDashboardPanelQueriesResponse struct {
	UID    string           `json:"uid"`
	Panels []PanelQueryInfo `json:"panels"`
}

func getDashboardPanelQueries(ctx context.Context, args GetDashboardPanelQueriesRequest) (*GetDashboardPanelQueriesResponse, error) {
	if args.UID == "" {
		return nil, fmt.Errorf("uid is required")
	}

	dbResp, err := getDashboardByUID(ctx, GetDashboardByUIDRequest{UID: args.UID})
	if err != nil {
		return nil, err
	}

	panels := extractPanels(dbResp.Dashboard)
	result := make([]PanelQueryInfo, 0, len(panels))
	for _, p := range panels {
		info := panelToQueryInfo(p)
		if args.PanelID != nil && info.PanelID != *args.PanelID {
			continue
		}
		result = append(result, info)
	}

	return &GetDashboardPanelQueriesResponse{UID: args.UID, Panels: result}, nil
}

// extractPanels returns all panels from a dashboard JSON, including collapsed row sub-panels.
func extractPanels(dashboard map[string]any) []map[string]any {
	if dashboard == nil {
		return nil
	}
	raw, ok := dashboard["panels"]
	if !ok {
		return nil
	}
	rawSlice, ok := raw.([]any)
	if !ok {
		return nil
	}

	panels := make([]map[string]any, 0, len(rawSlice))
	for _, item := range rawSlice {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		panels = append(panels, m)
		// If panel is a row, also include any collapsed sub-panels.
		if t, _ := m["type"].(string); t == "row" {
			if sub, ok := m["panels"].([]any); ok {
				for _, s := range sub {
					if sm, ok := s.(map[string]any); ok {
						panels = append(panels, sm)
					}
				}
			}
		}
	}
	return panels
}

// anyToInt64 converts a JSON-decoded numeric value to int64, handling float64,
// int64, and json.Number (the latter is used by go-openapi with UseNumber).
func anyToInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case int64:
		return x, true
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i, true
		}
	}
	return 0, false
}

func panelToQueryInfo(p map[string]any) PanelQueryInfo {
	info := PanelQueryInfo{Targets: []PanelTarget{}}

	if id, ok := p["id"]; ok {
		if v, ok := anyToInt64(id); ok {
			info.PanelID = v
		}
	}
	info.Title, _ = p["title"].(string)
	info.Type, _ = p["type"].(string)
	if ds, ok := p["datasource"].(map[string]any); ok {
		info.Datasource = ds
	}

	targets, _ := p["targets"].([]any)
	for _, t := range targets {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}
		target := PanelTarget{}
		target.RefID, _ = tm["refId"].(string)
		if ds, ok := tm["datasource"].(map[string]any); ok {
			target.Datasource = ds
		}
		// Capture remaining fields as rawQuery.
		raw := make(map[string]any, len(tm))
		for k, v := range tm {
			if k != "datasource" && k != "refId" {
				raw[k] = v
			}
		}
		if len(raw) > 0 {
			target.RawQuery = raw
		}
		info.Targets = append(info.Targets, target)
	}

	return info
}

var GetDashboardPanelQueriesTool = mcpgrafana.MustTool(
	"get_dashboard_panel_queries",
	"Get the queries (targets) for all panels in a dashboard, optionally filtered by panel ID.",
	getDashboardPanelQueries,
	mcp.WithTitleAnnotation("Get dashboard panel queries"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// --- get_dashboard_property ---

type GetDashboardPropertyRequest struct {
	UID  string `json:"uid" jsonschema:"required,description=Dashboard UID"`
	Path string `json:"path" jsonschema:"required,description=Dot-separated property path (e.g. 'title'\\, 'panels'\\, 'templating.list')"`
}

type GetDashboardPropertyResponse struct {
	UID   string `json:"uid"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

func getDashboardProperty(ctx context.Context, args GetDashboardPropertyRequest) (*GetDashboardPropertyResponse, error) {
	if args.UID == "" {
		return nil, fmt.Errorf("uid is required")
	}
	if args.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	dbResp, err := getDashboardByUID(ctx, GetDashboardByUIDRequest{UID: args.UID})
	if err != nil {
		return nil, err
	}

	value, err := getNestedValue(dbResp.Dashboard, args.Path)
	if err != nil {
		return nil, err
	}

	return &GetDashboardPropertyResponse{UID: args.UID, Path: args.Path, Value: value}, nil
}

// getNestedValue traverses obj using a dot-separated path.
// Array elements can be accessed by numeric index (e.g. "panels.0.title").
func getNestedValue(obj map[string]any, path string) (any, error) {
	parts := strings.Split(path, ".")
	var current any = obj
	for i, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			next, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("key %q not found at path segment %d", part, i)
			}
			current = next
		case []any:
			var idx int
			if _, err := fmt.Sscanf(part, "%d", &idx); err != nil {
				return nil, fmt.Errorf("expected array index at path segment %d, got %q", i, part)
			}
			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("array index %d out of range at path segment %d", idx, i)
			}
			current = v[idx]
		default:
			return nil, fmt.Errorf("cannot traverse into %T at path segment %d", current, i)
		}
	}
	return current, nil
}

var GetDashboardPropertyTool = mcpgrafana.MustTool(
	"get_dashboard_property",
	"Get a specific property from a dashboard by dot-separated path (e.g. 'title', 'panels', 'templating.list').",
	getDashboardProperty,
	mcp.WithTitleAnnotation("Get dashboard property"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// --- get_dashboard_summary ---

type GetDashboardSummaryRequest struct {
	UID string `json:"uid" jsonschema:"required,description=Dashboard UID"`
}

type VariableSummary struct {
	Name  string `json:"name,omitempty"`
	Type  string `json:"type,omitempty"`
	Label string `json:"label,omitempty"`
}

type GetDashboardSummaryResponse struct {
	UID        string            `json:"uid"`
	Title      string            `json:"title,omitempty"`
	Tags       []string          `json:"tags"`
	PanelCount int               `json:"panelCount"`
	PanelTypes map[string]int    `json:"panelTypes"`
	Variables  []VariableSummary `json:"variables"`
	URL        string            `json:"url,omitempty"`
	FolderUID  string            `json:"folderUid,omitempty"`
}

func getDashboardSummary(ctx context.Context, args GetDashboardSummaryRequest) (*GetDashboardSummaryResponse, error) {
	if args.UID == "" {
		return nil, fmt.Errorf("uid is required")
	}

	dbResp, err := getDashboardByUID(ctx, GetDashboardByUIDRequest{UID: args.UID})
	if err != nil {
		return nil, err
	}

	db := dbResp.Dashboard
	meta := dbResp.Meta

	resp := &GetDashboardSummaryResponse{
		UID:        args.UID,
		Tags:       []string{},
		PanelTypes: make(map[string]int),
		Variables:  []VariableSummary{},
	}

	resp.Title, _ = db["title"].(string)

	if rawTags, ok := db["tags"].([]any); ok {
		for _, t := range rawTags {
			if s, ok := t.(string); ok {
				resp.Tags = append(resp.Tags, s)
			}
		}
	}

	panels := extractPanels(db)
	resp.PanelCount = len(panels)
	for _, p := range panels {
		if t, ok := p["type"].(string); ok && t != "" {
			resp.PanelTypes[t]++
		}
	}

	if tmpl, ok := db["templating"].(map[string]any); ok {
		if list, ok := tmpl["list"].([]any); ok {
			for _, item := range list {
				if m, ok := item.(map[string]any); ok {
					v := VariableSummary{}
					v.Name, _ = m["name"].(string)
					v.Type, _ = m["type"].(string)
					v.Label, _ = m["label"].(string)
					resp.Variables = append(resp.Variables, v)
				}
			}
		}
	}

	if meta != nil {
		if u, ok := meta["url"].(string); ok {
			resp.URL = u
		}
		if folderUID, ok := meta["folderUid"].(string); ok {
			resp.FolderUID = folderUID
		}
	}

	return resp, nil
}

var GetDashboardSummaryTool = mcpgrafana.MustTool(
	"get_dashboard_summary",
	"Get a summary of a dashboard including title, panel count, panel types, variables, and tags.",
	getDashboardSummary,
	mcp.WithTitleAnnotation("Get dashboard summary"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// --- get_dashboard_versions ---

type GetDashboardVersionsRequest struct {
	UID   string `json:"uid" jsonschema:"required,description=Dashboard UID"`
	Limit *int   `json:"limit,omitempty" jsonschema:"description=Max versions to return (default: 10\\, max: 500)"`
	Start *int   `json:"start,omitempty" jsonschema:"description=Version offset for pagination"`
}

type DashboardVersionInfo struct {
	ID            int    `json:"id"`
	DashboardID   int    `json:"dashboardId"`
	Version       int    `json:"version"`
	ParentVersion int    `json:"parentVersion"`
	RestoredFrom  int    `json:"restoredFrom"`
	Created       string `json:"created"`
	CreatedBy     string `json:"createdBy"`
	Message       string `json:"message,omitempty"`
}

type GetDashboardVersionsResponse struct {
	UID      string                 `json:"uid"`
	Versions []DashboardVersionInfo `json:"versions"`
}

func getDashboardVersions(ctx context.Context, args GetDashboardVersionsRequest) (*GetDashboardVersionsResponse, error) {
	if args.UID == "" {
		return nil, fmt.Errorf("uid is required")
	}

	dbResp, err := getDashboardByUID(ctx, GetDashboardByUIDRequest{UID: args.UID})
	if err != nil {
		return nil, err
	}

	// Grafana 8.4.7 returns numeric dashboard id under dashboard.id.
	// Keep meta.id as a compatibility fallback for non-standard payloads.
	dashID, ok := anyToInt64(dbResp.Dashboard["id"])
	if !ok || dashID == 0 {
		dashID, ok = anyToInt64(dbResp.Meta["id"])
	}
	if !ok || dashID == 0 {
		return nil, fmt.Errorf("could not determine numeric dashboard ID for uid %q", args.UID)
	}

	query := url.Values{}
	limit := 10
	if args.Limit != nil && *args.Limit > 0 {
		limit = *args.Limit
	}
	query.Set("limit", strconv.Itoa(limit))
	if args.Start != nil && *args.Start > 0 {
		query.Set("start", strconv.Itoa(*args.Start))
	}

	path := fmt.Sprintf("/dashboards/id/%d/versions", dashID)
	respBody, statusCode, err := doAPIRequest(ctx, "GET", path, query, nil)
	if err != nil {
		return nil, fmt.Errorf("get dashboard versions: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	var versions []DashboardVersionInfo
	if err := json.Unmarshal(respBody, &versions); err != nil {
		return nil, fmt.Errorf("decode dashboard versions: %w", err)
	}
	if versions == nil {
		versions = []DashboardVersionInfo{}
	}

	return &GetDashboardVersionsResponse{UID: args.UID, Versions: versions}, nil
}

var GetDashboardVersionsTool = mcpgrafana.MustTool(
	"get_dashboard_versions",
	"Get the version history of a dashboard, sorted newest first. Useful for auditing changes and understanding when a dashboard was last modified.",
	getDashboardVersions,
	mcp.WithTitleAnnotation("Get dashboard versions"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
