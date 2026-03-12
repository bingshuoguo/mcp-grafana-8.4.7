package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

// ─── get_panel_image ──────────────────────────────────────────────────────────

type GetPanelImageRequest struct {
	DashboardUID string            `json:"dashboardUid" jsonschema:"required,description=UID of the dashboard to render"`
	PanelID      *int              `json:"panelId,omitempty" jsonschema:"description=Panel ID to render (renders whole dashboard if omitted)"`
	Width        *int              `json:"width,omitempty" jsonschema:"description=Image width in pixels (default: 1000)"`
	Height       *int              `json:"height,omitempty" jsonschema:"description=Image height in pixels (default: 500)"`
	From         string            `json:"from,omitempty" jsonschema:"description=Start time ('now-1h'\\, RFC3339). Default: now-1h"`
	To           string            `json:"to,omitempty" jsonschema:"description=End time ('now'\\, RFC3339). Default: now"`
	Theme        string            `json:"theme,omitempty" jsonschema:"description=Render theme: light or dark (default: dark)"`
	Variables    map[string]string `json:"variables,omitempty" jsonschema:"description=Dashboard template variables ({key: value})"`
	Timeout      *int              `json:"timeout,omitempty" jsonschema:"description=Render timeout in seconds (default: 60)"`
}

// buildV84RenderURL constructs the /render/d/{uid} URL (not under /api/).
func buildV84RenderURL(baseURL string, args GetPanelImageRequest) string {
	params := url.Values{}

	width := 1000
	if args.Width != nil && *args.Width > 0 {
		width = *args.Width
	}
	height := 500
	if args.Height != nil && *args.Height > 0 {
		height = *args.Height
	}
	params.Set("width", strconv.Itoa(width))
	params.Set("height", strconv.Itoa(height))

	if args.PanelID != nil {
		params.Set("viewPanel", strconv.Itoa(*args.PanelID))
	}
	if args.From != "" {
		params.Set("from", args.From)
	}
	if args.To != "" {
		params.Set("to", args.To)
	}
	if args.Theme != "" {
		params.Set("theme", args.Theme)
	}
	for k, v := range args.Variables {
		params.Set(k, v)
	}
	params.Set("kiosk", "true")

	return fmt.Sprintf("%s/render/d/%s?%s",
		strings.TrimRight(baseURL, "/"), args.DashboardUID, params.Encode())
}

func getPanelImage(ctx context.Context, args GetPanelImageRequest) (*mcp.CallToolResult, error) {
	cfg, err := getGrafanaConfig(ctx)
	if err != nil {
		return nil, err
	}

	renderURL := buildV84RenderURL(cfg.URL, args)

	timeout := 60 * time.Second
	if args.Timeout != nil && *args.Timeout > 0 {
		timeout = time.Duration(*args.Timeout) * time.Second
	}

	rt, err := mcpgrafana.BuildTransport(&cfg, nil)
	if err != nil {
		return nil, fmt.Errorf("build HTTP transport: %w", err)
	}
	httpClient := &http.Client{
		Transport: mcpgrafana.NewUserAgentTransport(rt),
		Timeout:   timeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, renderURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create render request: %w", err)
	}
	applyAuthHeaders(req, cfg)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("render request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("image renderer not available: ensure grafana-image-renderer is installed and enabled")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("render failed with HTTP %d: %s", resp.StatusCode, string(body))
	}

	imageData, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read render response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.ImageContent{
				Type:     "image",
				Data:     base64.StdEncoding.EncodeToString(imageData),
				MIMEType: "image/png",
			},
		},
	}, nil
}

var GetPanelImageTool = mcpgrafana.MustTool(
	"get_panel_image",
	`Render a Grafana dashboard panel or full dashboard as a PNG image.

Returns base64-encoded PNG. Requires Grafana Image Renderer plugin to be installed.
Use dashboardUid from search_dashboards and panelId from get_dashboard_summary.`,
	getPanelImage,
	mcp.WithTitleAnnotation("Get panel or dashboard image"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
