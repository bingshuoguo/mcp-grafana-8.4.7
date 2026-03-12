package tools

import (
	"context"
	"fmt"

	"github.com/grafana/grafana-openapi-client-go/client/search"
	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

const dashboardType = "dash-db"

type SearchDashboardsRequest struct {
	Query        string   `json:"query,omitempty" jsonschema:"description=Search text for dashboard titles and metadata"`
	Limit        *int64   `json:"limit,omitempty" jsonschema:"description=Max results per page\\, default 50"`
	Page         *int64   `json:"page,omitempty" jsonschema:"description=Page number\\, 1-indexed"`
	Tag          []string `json:"tag,omitempty" jsonschema:"description=Filter by tags"`
	DashboardIDs []int64  `json:"dashboardIds,omitempty" jsonschema:"description=Filter by dashboard IDs"`
	FolderIDs    []int64  `json:"folderIds,omitempty" jsonschema:"description=Filter by folder IDs"`
	Starred      *bool    `json:"starred,omitempty" jsonschema:"description=Only starred dashboards when true"`
}

type SearchDashboardsResponse struct {
	Items   []SearchHit `json:"items"`
	Page    int64       `json:"page"`
	Limit   int64       `json:"limit"`
	HasMore bool        `json:"hasMore"`
}

func searchDashboards(ctx context.Context, args SearchDashboardsRequest) (*SearchDashboardsResponse, error) {
	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	params := search.NewSearchParamsWithContext(ctx)
	params.SetType(func() *string { t := dashboardType; return &t }())

	if args.Query != "" {
		params.SetQuery(&args.Query)
	}
	if len(args.Tag) > 0 {
		params.SetTag(args.Tag)
	}
	if len(args.FolderIDs) > 0 {
		params.SetFolderIds(args.FolderIDs)
	}
	if len(args.DashboardIDs) > 0 {
		params.SetDashboardIds(args.DashboardIDs)
	}
	if args.Starred != nil {
		params.SetStarred(args.Starred)
	}

	limit := int64(50)
	if args.Limit != nil {
		limit = *args.Limit
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 5000 {
		limit = 5000
	}
	params.SetLimit(&limit)

	page := int64(1)
	if args.Page != nil {
		page = *args.Page
	}
	if page <= 0 {
		page = 1
	}
	params.SetPage(&page)

	resp, err := gc.Search.Search(params)
	if err != nil {
		return nil, fmt.Errorf("search dashboards: %w", wrapOpenAPIError(err))
	}

	hits := make([]SearchHit, 0)
	if resp != nil && resp.Payload != nil {
		for _, h := range resp.Payload {
			if h == nil {
				continue
			}
			hits = append(hits, SearchHit{
				ID:          h.ID,
				UID:         h.UID,
				Title:       h.Title,
				Type:        string(h.Type),
				URL:         h.URL,
				URI:         h.URI,
				Slug:        h.Slug,
				Tags:        h.Tags,
				FolderID:    h.FolderID,
				FolderUID:   h.FolderUID,
				FolderTitle: h.FolderTitle,
				FolderURL:   h.FolderURL,
				IsStarred:   h.IsStarred,
			})
		}
	}

	return &SearchDashboardsResponse{
		Items:   hits,
		Page:    page,
		Limit:   limit,
		HasMore: int64(len(hits)) == limit,
	}, nil
}

var SearchDashboardsTool = mcpgrafana.MustTool(
	"search_dashboards",
	"Search Grafana dashboards with pagination and filters.",
	searchDashboards,
	mcp.WithTitleAnnotation("Search dashboards"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
