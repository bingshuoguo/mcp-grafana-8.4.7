package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

type ListLegacyAlertsRequest struct {
	DashboardID  *int64   `json:"dashboardId,omitempty" jsonschema:"description=Filter by dashboard ID"`
	PanelID      *int64   `json:"panelId,omitempty" jsonschema:"description=Filter by panel ID"`
	Query        string   `json:"query,omitempty" jsonschema:"description=Search query for alert names"`
	State        string   `json:"state,omitempty" jsonschema:"description=Filter by alert state"`
	Limit        *int64   `json:"limit,omitempty" jsonschema:"description=Max number of alerts to return"`
	DashboardTag []string `json:"dashboardTag,omitempty" jsonschema:"description=Filter by dashboard tags"`
}

type ListLegacyAlertsResponse struct {
	Items []LegacyAlertItem `json:"items"`
}

func listLegacyAlerts(ctx context.Context, args ListLegacyAlertsRequest) (*ListLegacyAlertsResponse, error) {
	query := url.Values{}
	if args.DashboardID != nil {
		query.Set("dashboardId", strconv.FormatInt(*args.DashboardID, 10))
	}
	if args.PanelID != nil {
		query.Set("panelId", strconv.FormatInt(*args.PanelID, 10))
	}
	if args.Query != "" {
		query.Set("query", args.Query)
	}
	if args.State != "" {
		query.Set("state", args.State)
	}
	if args.Limit != nil {
		query.Set("limit", strconv.FormatInt(*args.Limit, 10))
	}
	for _, tag := range args.DashboardTag {
		query.Add("dashboardTag", tag)
	}

	respBody, statusCode, err := doAPIRequest(ctx, "GET", "/alerts", query, nil)
	if err != nil {
		return nil, fmt.Errorf("list legacy alerts: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	var items []LegacyAlertItem
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &items); err != nil {
			return nil, fmt.Errorf("decode legacy alerts response: %w", err)
		}
	}

	if items == nil {
		items = []LegacyAlertItem{}
	}

	return &ListLegacyAlertsResponse{Items: items}, nil
}

var ListLegacyAlertsTool = mcpgrafana.MustTool(
	"list_legacy_alerts",
	"List legacy alert rules from /api/alerts.",
	listLegacyAlerts,
	mcp.WithTitleAnnotation("List legacy alerts"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

type ListLegacyNotificationChannelsRequest struct {
	Name string `json:"name,omitempty" jsonschema:"description=Filter by notification channel name"`
}

type ListLegacyNotificationChannelsResponse struct {
	Items []LegacyNotificationChannel `json:"items"`
}

func listLegacyNotificationChannels(ctx context.Context, args ListLegacyNotificationChannelsRequest) (*ListLegacyNotificationChannelsResponse, error) {
	query := url.Values{}
	if args.Name != "" {
		query.Set("name", args.Name)
	}

	respBody, statusCode, err := doAPIRequest(ctx, "GET", "/alert-notifications", query, nil)
	if err != nil {
		return nil, fmt.Errorf("list legacy notification channels: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	var items []LegacyNotificationChannel
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &items); err != nil {
			return nil, fmt.Errorf("decode legacy notification channels response: %w", err)
		}
	}
	if items == nil {
		items = []LegacyNotificationChannel{}
	}

	return &ListLegacyNotificationChannelsResponse{Items: items}, nil
}

var ListLegacyNotificationChannelsTool = mcpgrafana.MustTool(
	"list_legacy_notification_channels",
	"List legacy alert notification channels from /api/alert-notifications.",
	listLegacyNotificationChannels,
	mcp.WithTitleAnnotation("List legacy notification channels"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
