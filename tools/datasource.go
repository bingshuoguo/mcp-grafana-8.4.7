package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

type ListDatasourcesRequest struct {
	Type   string `json:"type,omitempty" jsonschema:"description=Datasource type filter"`
	Limit  *int64 `json:"limit,omitempty" jsonschema:"description=Max result size\\, default 100"`
	Offset *int64 `json:"offset,omitempty" jsonschema:"description=Pagination offset\\, default 0"`
}

type ListDatasourcesResponse struct {
	Items   []DatasourceModel `json:"items"`
	Total   int64             `json:"total"`
	HasMore bool              `json:"hasMore"`
}

func listDatasources(ctx context.Context, args ListDatasourcesRequest) (*ListDatasourcesResponse, error) {
	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := gc.Datasources.GetDataSources()
	if err != nil {
		return nil, fmt.Errorf("list datasources: %w", wrapOpenAPIError(err))
	}

	all := make([]*models.DataSourceListItemDTO, 0)
	if resp != nil && resp.Payload != nil {
		all = resp.Payload
	}

	filtered := filterDatasourceItems(all, args.Type)
	total := int64(len(filtered))

	offset := int64(0)
	if args.Offset != nil {
		offset = *args.Offset
	}
	if offset < 0 {
		offset = 0
	}

	limit := int64(100)
	if args.Limit != nil {
		limit = *args.Limit
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 5000 {
		limit = 5000
	}

	if offset >= int64(len(filtered)) {
		return &ListDatasourcesResponse{Items: []DatasourceModel{}, Total: total, HasMore: false}, nil
	}

	end := offset + limit
	if end > int64(len(filtered)) {
		end = int64(len(filtered))
	}

	items := make([]DatasourceModel, 0, end-offset)
	for _, ds := range filtered[offset:end] {
		items = append(items, listItemToModel(ds))
	}
	return &ListDatasourcesResponse{
		Items:   items,
		Total:   total,
		HasMore: end < total,
	}, nil
}

func filterDatasourceItems(items []*models.DataSourceListItemDTO, t string) []*models.DataSourceListItemDTO {
	if t == "" {
		return items
	}
	filtered := make([]*models.DataSourceListItemDTO, 0, len(items))
	want := strings.ToLower(t)
	for _, ds := range items {
		if ds == nil {
			continue
		}
		if strings.Contains(strings.ToLower(ds.Type), want) {
			filtered = append(filtered, ds)
		}
	}
	return filtered
}

var ListDatasourcesTool = mcpgrafana.MustTool(
	"list_datasources",
	"List Grafana datasources with optional type filtering and pagination.",
	listDatasources,
	mcp.WithTitleAnnotation("List datasources"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

type GetDatasourceRequest struct {
	ID   *int64 `json:"id,omitempty" jsonschema:"description=Datasource numeric ID"`
	UID  string `json:"uid,omitempty" jsonschema:"description=Datasource UID"`
	Name string `json:"name,omitempty" jsonschema:"description=Datasource name"`
}

type GetDatasourceResponse struct {
	ResolvedBy string          `json:"resolvedBy,omitempty"`
	Datasource DatasourceModel `json:"datasource"`
}

func getDatasource(ctx context.Context, args GetDatasourceRequest) (*GetDatasourceResponse, error) {
	resolved, err := resolveDatasourceRef(ctx, DatasourceRef{ID: args.ID, UID: args.UID, Name: args.Name})
	if err != nil {
		return nil, fmt.Errorf("get datasource: %w", err)
	}
	return &GetDatasourceResponse{
		ResolvedBy: resolved.ResolvedBy,
		Datasource: resolved.Datasource,
	}, nil
}

var GetDatasourceTool = mcpgrafana.MustTool(
	"get_datasource",
	"Get a datasource by id, uid, or name. Resolution order is id > uid > name.",
	getDatasource,
	mcp.WithTitleAnnotation("Get datasource"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

type ResolveDatasourceRefRequest struct {
	ID   *int64 `json:"id,omitempty" jsonschema:"description=Datasource numeric ID"`
	UID  string `json:"uid,omitempty" jsonschema:"description=Datasource UID"`
	Name string `json:"name,omitempty" jsonschema:"description=Datasource name"`
}

type ResolveDatasourceRefResponse struct {
	ID   int64  `json:"id"`
	UID  string `json:"uid,omitempty"`
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
}

func resolveDatasourceReference(ctx context.Context, args ResolveDatasourceRefRequest) (*ResolveDatasourceRefResponse, error) {
	resolved, err := resolveDatasourceRef(ctx, DatasourceRef{ID: args.ID, UID: args.UID, Name: args.Name})
	if err != nil {
		return nil, fmt.Errorf("resolve datasource ref: %w", err)
	}

	ds := resolved.Datasource
	return &ResolveDatasourceRefResponse{
		ID:   ds.ID,
		UID:  ds.UID,
		Name: ds.Name,
		Type: ds.Type,
		URL:  ds.URL,
	}, nil
}

var ResolveDatasourceRefTool = mcpgrafana.MustTool(
	"resolve_datasource_ref",
	"Resolve datasource reference using id > uid > name semantics.",
	resolveDatasourceReference,
	mcp.WithTitleAnnotation("Resolve datasource reference"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
