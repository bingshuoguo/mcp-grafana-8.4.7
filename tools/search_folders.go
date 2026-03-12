package tools

import (
	"context"
	"fmt"

	"github.com/grafana/grafana-openapi-client-go/client/search"
	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

const folderSearchType = "dash-folder"

type SearchFoldersRequest struct {
	Query string `json:"query,omitempty" jsonschema:"description=Search text for folder titles"`
	Limit *int64 `json:"limit,omitempty" jsonschema:"description=Max results per page\\, default 50"`
	Page  *int64 `json:"page,omitempty" jsonschema:"description=Page number\\, 1-indexed"`
}

type SearchFoldersResponse struct {
	Items   []SearchHit `json:"items"`
	Page    int64       `json:"page"`
	Limit   int64       `json:"limit"`
	HasMore bool        `json:"hasMore"`
}

func searchFolders(ctx context.Context, args SearchFoldersRequest) (*SearchFoldersResponse, error) {
	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	params := search.NewSearchParamsWithContext(ctx)
	t := folderSearchType
	params.SetType(&t)

	if args.Query != "" {
		params.SetQuery(&args.Query)
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
		return nil, fmt.Errorf("search folders: %w", wrapOpenAPIError(err))
	}

	hits := make([]SearchHit, 0)
	if resp != nil && resp.Payload != nil {
		for _, h := range resp.Payload {
			if h == nil {
				continue
			}
			hits = append(hits, SearchHit{
				ID:    h.ID,
				UID:   h.UID,
				Title: h.Title,
				Type:  string(h.Type),
				URL:   h.URL,
				URI:   h.URI,
				Tags:  h.Tags,
			})
		}
	}

	return &SearchFoldersResponse{
		Items:   hits,
		Page:    page,
		Limit:   limit,
		HasMore: int64(len(hits)) == limit,
	}, nil
}

var SearchFoldersTool = mcpgrafana.MustTool(
	"search_folders",
	"Search Grafana folders by title.",
	searchFolders,
	mcp.WithTitleAnnotation("Search folders"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
