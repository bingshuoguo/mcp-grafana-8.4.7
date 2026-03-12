package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

type GetDashboardByUIDRequest struct {
	UID string `json:"uid" jsonschema:"required,description=Dashboard UID"`
}

type GetDashboardByUIDResponse struct {
	Dashboard map[string]any `json:"dashboard"`
	Meta      map[string]any `json:"meta,omitempty"`
}

func getDashboardByUID(ctx context.Context, args GetDashboardByUIDRequest) (*GetDashboardByUIDResponse, error) {
	if args.UID == "" {
		return nil, fmt.Errorf("uid is required")
	}

	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := gc.Dashboards.GetDashboardByUID(args.UID)
	if err != nil {
		return nil, fmt.Errorf("get dashboard by uid: %w", wrapOpenAPIError(err))
	}
	if resp == nil || resp.Payload == nil {
		return &GetDashboardByUIDResponse{}, nil
	}

	dashboard := toStringAnyMap(resp.Payload.Dashboard)
	meta := toStringAnyMap(resp.Payload.Meta)

	return &GetDashboardByUIDResponse{
		Dashboard: dashboard,
		Meta:      meta,
	}, nil
}

func toStringAnyMap(v any) map[string]any {
	if v == nil {
		return nil
	}

	if m, ok := v.(map[string]any); ok {
		return m
	}

	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}

	return out
}

var GetDashboardByUIDTool = mcpgrafana.MustTool(
	"get_dashboard_by_uid",
	"Get the full dashboard definition by dashboard UID.",
	getDashboardByUID,
	mcp.WithTitleAnnotation("Get dashboard by UID"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

type UpsertDashboardRequest struct {
	Dashboard map[string]any `json:"dashboard" jsonschema:"required,description=Full dashboard JSON payload"`
	FolderID  *int64         `json:"folderId,omitempty" jsonschema:"description=Numeric folder ID"`
	FolderUID string         `json:"folderUid,omitempty" jsonschema:"description=Folder UID"`
	Overwrite *bool          `json:"overwrite,omitempty" jsonschema:"description=Overwrite existing dashboard"`
	Message   string         `json:"message,omitempty" jsonschema:"description=Version history message"`
}

type UpsertDashboardResponse struct {
	Status  string     `json:"status,omitempty"`
	ID      FlexibleID `json:"id,omitempty"`
	UID     string     `json:"uid,omitempty"`
	Title   string     `json:"title,omitempty"`
	URL     string     `json:"url,omitempty"`
	Version int64      `json:"version,omitempty"`
}

// upsertDashboardRawResponse is the actual JSON shape returned by Grafana 8.4.7
// POST /api/dashboards/db. We use a dedicated struct because the generated
// OpenAPI model has incorrect field mappings (e.g. Go "Slug" mapped to
// json:"title" while the real API returns json:"slug"; ID typed as *string
// while the API returns an integer).
type upsertDashboardRawResponse struct {
	Status  string     `json:"status"`
	ID      FlexibleID `json:"id"`
	UID     string     `json:"uid"`
	Slug    string     `json:"slug"`
	URL     string     `json:"url"`
	Version int64      `json:"version"`
}

func upsertDashboard(ctx context.Context, args UpsertDashboardRequest) (*UpsertDashboardResponse, error) {
	if args.Dashboard == nil {
		return nil, fmt.Errorf("dashboard is required")
	}

	requestBody := map[string]any{
		"dashboard": args.Dashboard,
	}
	if args.FolderID != nil {
		requestBody["folderId"] = *args.FolderID
	}
	if args.FolderUID != "" {
		requestBody["folderUid"] = args.FolderUID
	}
	if args.Overwrite != nil {
		requestBody["overwrite"] = *args.Overwrite
	}
	if args.Message != "" {
		requestBody["message"] = args.Message
	}

	respBody, statusCode, err := doAPIRequest(ctx, "POST", "/dashboards/db", nil, requestBody)
	if err != nil {
		return nil, fmt.Errorf("upsert dashboard: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	var raw upsertDashboardRawResponse
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &raw); err != nil {
			return nil, fmt.Errorf("decode upsert dashboard response: %w", err)
		}
	}

	title := raw.Slug
	if t, ok := args.Dashboard["title"].(string); ok && t != "" {
		title = t
	}

	return &UpsertDashboardResponse{
		Status:  raw.Status,
		ID:      raw.ID,
		UID:     raw.UID,
		Title:   title,
		URL:     raw.URL,
		Version: raw.Version,
	}, nil
}

var UpsertDashboardTool = mcpgrafana.MustTool(
	"upsert_dashboard",
	"Create or update a dashboard using the Grafana dashboard save API.",
	upsertDashboard,
	mcp.WithTitleAnnotation("Upsert dashboard"),
	mcp.WithDestructiveHintAnnotation(true),
)
