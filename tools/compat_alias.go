package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

// update_dashboard is a compat alias for upsert_dashboard.
func updateDashboard(ctx context.Context, args UpsertDashboardRequest) (*UpsertDashboardResponse, error) {
	return upsertDashboard(ctx, args)
}

var UpdateDashboardTool = mcpgrafana.MustTool(
	"update_dashboard",
	"Update an existing dashboard. Alias for upsert_dashboard.",
	updateDashboard,
	mcp.WithTitleAnnotation("Update dashboard"),
	mcp.WithDestructiveHintAnnotation(true),
)

// list_users_by_org is a compat alias for list_org_users.
func listUsersByOrg(ctx context.Context, args ListOrgUsersRequest) (*ListOrgUsersResponse, error) {
	return listOrgUsers(ctx, args)
}

var ListUsersByOrgTool = mcpgrafana.MustTool(
	"list_users_by_org",
	"List users in the current organization. Alias for list_org_users.",
	listUsersByOrg,
	mcp.WithTitleAnnotation("List users by org"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// GetDatasourceByUIDRequest is the request type for get_datasource_by_uid.
type GetDatasourceByUIDRequest struct {
	UID string `json:"uid" jsonschema:"required,description=Datasource UID"`
}

func getDatasourceByUID(ctx context.Context, args GetDatasourceByUIDRequest) (*GetDatasourceResponse, error) {
	return getDatasource(ctx, GetDatasourceRequest{UID: args.UID})
}

var GetDatasourceByUIDTool = mcpgrafana.MustTool(
	"get_datasource_by_uid",
	"Get a datasource by UID.",
	getDatasourceByUID,
	mcp.WithTitleAnnotation("Get datasource by UID"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// GetDatasourceByNameRequest is the request type for get_datasource_by_name.
type GetDatasourceByNameRequest struct {
	Name string `json:"name" jsonschema:"required,description=Datasource name"`
}

func getDatasourceByName(ctx context.Context, args GetDatasourceByNameRequest) (*GetDatasourceResponse, error) {
	return getDatasource(ctx, GetDatasourceRequest{Name: args.Name})
}

var GetDatasourceByNameTool = mcpgrafana.MustTool(
	"get_datasource_by_name",
	"Get a datasource by name.",
	getDatasourceByName,
	mcp.WithTitleAnnotation("Get datasource by name"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
