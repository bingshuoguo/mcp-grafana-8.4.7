package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

type V84TimeRange struct {
	From string `json:"from" jsonschema:"description=Start time (e.g. 'now-1h')"`
	To   string `json:"to" jsonschema:"description=End time (e.g. 'now')"`
}

type GenerateDeeplinkRequest struct {
	ResourceType  string            `json:"resourceType" jsonschema:"required,description=Type of resource: dashboard\\, panel\\, or explore"`
	DashboardUID  *string           `json:"dashboardUid,omitempty" jsonschema:"description=Dashboard UID (required for dashboard and panel types)"`
	DatasourceUID *string           `json:"datasourceUid,omitempty" jsonschema:"description=Datasource UID (required for explore type)"`
	PanelID       *int              `json:"panelId,omitempty" jsonschema:"description=Panel ID (required for panel type)"`
	QueryParams   map[string]string `json:"queryParams,omitempty" jsonschema:"description=Additional query parameters"`
	TimeRange     *V84TimeRange     `json:"timeRange,omitempty" jsonschema:"description=Time range for the link"`
}

type GenerateDeeplinkResponse struct {
	URL string `json:"url"`
}

func generateDeeplink(ctx context.Context, args GenerateDeeplinkRequest) (*GenerateDeeplinkResponse, error) {
	cfg, err := getGrafanaConfig(ctx)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(cfg.URL, "/")

	var deeplink string

	switch strings.ToLower(args.ResourceType) {
	case "dashboard":
		if args.DashboardUID == nil {
			return nil, fmt.Errorf("dashboardUid is required for dashboard links")
		}
		deeplink = fmt.Sprintf("%s/d/%s", baseURL, *args.DashboardUID)
	case "panel":
		if args.DashboardUID == nil {
			return nil, fmt.Errorf("dashboardUid is required for panel links")
		}
		if args.PanelID == nil {
			return nil, fmt.Errorf("panelId is required for panel links")
		}
		deeplink = fmt.Sprintf("%s/d/%s?viewPanel=%d", baseURL, *args.DashboardUID, *args.PanelID)
	case "explore":
		if args.DatasourceUID == nil {
			return nil, fmt.Errorf("datasourceUid is required for explore links")
		}
		params := url.Values{}
		exploreState := fmt.Sprintf(`{"datasource":"%s"}`, *args.DatasourceUID)
		params.Set("left", exploreState)
		deeplink = fmt.Sprintf("%s/explore?%s", baseURL, params.Encode())
	default:
		return nil, fmt.Errorf("unsupported resource type: %s. Supported types are: dashboard, panel, explore", args.ResourceType)
	}

	if args.TimeRange != nil {
		sep := "?"
		if strings.Contains(deeplink, "?") {
			sep = "&"
		}
		timeParams := url.Values{}
		if args.TimeRange.From != "" {
			timeParams.Set("from", args.TimeRange.From)
		}
		if args.TimeRange.To != "" {
			timeParams.Set("to", args.TimeRange.To)
		}
		if len(timeParams) > 0 {
			deeplink += sep + timeParams.Encode()
		}
	}

	if len(args.QueryParams) > 0 {
		sep := "?"
		if strings.Contains(deeplink, "?") {
			sep = "&"
		}
		additional := url.Values{}
		for k, v := range args.QueryParams {
			additional.Set(k, v)
		}
		deeplink += sep + additional.Encode()
	}

	return &GenerateDeeplinkResponse{URL: deeplink}, nil
}

var GenerateDeeplinkTool = mcpgrafana.MustTool(
	"generate_deeplink",
	"Generate deeplink URLs for Grafana resources. Supports dashboards (requires dashboardUid), panels (requires dashboardUid and panelId), and Explore queries (requires datasourceUid). Optionally accepts time range and additional query parameters.",
	generateDeeplink,
	mcp.WithTitleAnnotation("Generate navigation deeplink"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)
