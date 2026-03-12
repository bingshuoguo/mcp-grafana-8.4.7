package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/grafana-openapi-client-go/client/org"
	"github.com/grafana/grafana-openapi-client-go/client/teams"
	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

type ListOrgUsersRequest struct{}

type ListOrgUsersResponse struct {
	Items []OrgUserItem `json:"items"`
}

func listOrgUsers(ctx context.Context, _ ListOrgUsersRequest) (*ListOrgUsersResponse, error) {
	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := gc.Org.GetOrgUsersForCurrentOrg(org.NewGetOrgUsersForCurrentOrgParamsWithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("list org users: %w", wrapOpenAPIError(err))
	}

	items := make([]OrgUserItem, 0)
	if resp != nil && resp.Payload != nil {
		for _, u := range resp.Payload {
			if u == nil {
				continue
			}
			items = append(items, OrgUserItem{
				UserID:        u.UserID,
				OrgID:         u.OrgID,
				Login:         u.Login,
				Name:          u.Name,
				Email:         u.Email,
				AvatarURL:     u.AvatarURL,
				Role:          u.Role,
				LastSeenAt:    time.Time(u.LastSeenAt),
				LastSeenAtAge: u.LastSeenAtAge,
				AccessControl: u.AccessControl,
			})
		}
	}

	return &ListOrgUsersResponse{Items: items}, nil
}

var ListOrgUsersTool = mcpgrafana.MustTool(
	"list_org_users",
	"List users in the current organization.",
	listOrgUsers,
	mcp.WithTitleAnnotation("List org users"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

type ListTeamsRequest struct {
	Query   string `json:"query,omitempty" jsonschema:"description=Search query for team names"`
	Page    *int64 `json:"page,omitempty" jsonschema:"description=Page number\\, default 1"`
	PerPage *int64 `json:"perPage,omitempty" jsonschema:"description=Items per page\\, default 100"`
}

type ListTeamsResponse struct {
	TotalCount int64      `json:"totalCount,omitempty"`
	Page       int64      `json:"page,omitempty"`
	PerPage    int64      `json:"perPage,omitempty"`
	Teams      []TeamItem `json:"teams"`
}

func listTeams(ctx context.Context, args ListTeamsRequest) (*ListTeamsResponse, error) {
	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	params := teams.NewSearchTeamsParamsWithContext(ctx)
	if args.Query != "" {
		params.SetQuery(&args.Query)
	}
	if args.Page != nil {
		params.SetPage(args.Page)
	}
	if args.PerPage != nil {
		params.SetPerpage(args.PerPage)
	}

	resp, err := gc.Teams.SearchTeams(params)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", wrapOpenAPIError(err))
	}

	result := &ListTeamsResponse{Teams: []TeamItem{}}
	if resp == nil || resp.Payload == nil {
		return result, nil
	}

	result.TotalCount = resp.Payload.TotalCount
	result.Page = resp.Payload.Page
	result.PerPage = resp.Payload.PerPage
	if resp.Payload.Teams != nil {
		for _, t := range resp.Payload.Teams {
			if t == nil {
				continue
			}
			result.Teams = append(result.Teams, TeamItem{
				ID:            int64Value(t.ID),
				OrgID:         int64Value(t.OrgID),
				Name:          stringValue(t.Name),
				Email:         t.Email,
				AvatarURL:     t.AvatarURL,
				MemberCount:   int64Value(t.MemberCount),
				Permission:    int64(t.Permission),
				AccessControl: t.AccessControl,
			})
		}
	}
	return result, nil
}

var ListTeamsTool = mcpgrafana.MustTool(
	"list_teams",
	"List teams in the current organization.",
	listTeams,
	mcp.WithTitleAnnotation("List teams"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

func int64Value(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
