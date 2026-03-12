package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

type GetHealthRequest struct{}

type GetHealthResponse struct {
	Database string `json:"database,omitempty"`
	Version  string `json:"version,omitempty"`
	Commit   string `json:"commit,omitempty"`
}

func getHealth(ctx context.Context, _ GetHealthRequest) (*GetHealthResponse, error) {
	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := gc.Health.GetHealth()
	if err != nil {
		return nil, fmt.Errorf("get health: %w", wrapOpenAPIError(err))
	}

	if resp == nil || resp.Payload == nil {
		return &GetHealthResponse{}, nil
	}

	return &GetHealthResponse{
		Database: resp.Payload.Database,
		Version:  resp.Payload.Version,
		Commit:   resp.Payload.Commit,
	}, nil
}

var GetHealthTool = mcpgrafana.MustTool(
	"get_health",
	"Get Grafana health, version, and commit information.",
	getHealth,
	mcp.WithTitleAnnotation("Get health"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

type GetCurrentUserRequest struct{}

type GetCurrentUserResponse struct {
	ID             int64           `json:"id,omitempty"`
	Login          string          `json:"login,omitempty"`
	Email          string          `json:"email,omitempty"`
	Name           string          `json:"name,omitempty"`
	AvatarURL      string          `json:"avatarUrl,omitempty"`
	IsGrafanaAdmin bool            `json:"isGrafanaAdmin,omitempty"`
	IsExternal     bool            `json:"isExternal,omitempty"`
	IsDisabled     bool            `json:"isDisabled,omitempty"`
	OrgID          int64           `json:"orgId,omitempty"`
	Theme          string          `json:"theme,omitempty"`
	AuthLabels     []string        `json:"authLabels,omitempty"`
	AccessControl  map[string]bool `json:"accessControl,omitempty"`
}

func getCurrentUser(ctx context.Context, _ GetCurrentUserRequest) (*GetCurrentUserResponse, error) {
	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := gc.SignedInUser.GetSignedInUser()
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", wrapOpenAPIError(err))
	}
	if resp == nil || resp.Payload == nil {
		return &GetCurrentUserResponse{}, nil
	}
	p := resp.Payload
	return &GetCurrentUserResponse{
		ID:             p.ID,
		Login:          p.Login,
		Email:          p.Email,
		Name:           p.Name,
		AvatarURL:      p.AvatarURL,
		IsGrafanaAdmin: p.IsGrafanaAdmin,
		IsExternal:     p.IsExternal,
		IsDisabled:     p.IsDisabled,
		OrgID:          p.OrgID,
		Theme:          p.Theme,
		AuthLabels:     p.AuthLabels,
		AccessControl:  p.AccessControl,
	}, nil
}

var GetCurrentUserTool = mcpgrafana.MustTool(
	"get_current_user",
	"Get information about the current Grafana user.",
	getCurrentUser,
	mcp.WithTitleAnnotation("Get current user"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

type GetCurrentOrgRequest struct{}

type GetCurrentOrgResponse struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func getCurrentOrg(ctx context.Context, _ GetCurrentOrgRequest) (*GetCurrentOrgResponse, error) {
	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := gc.Org.GetCurrentOrg()
	if err != nil {
		return nil, fmt.Errorf("get current org: %w", wrapOpenAPIError(err))
	}
	if resp == nil || resp.Payload == nil {
		return &GetCurrentOrgResponse{}, nil
	}
	return &GetCurrentOrgResponse{
		ID:   resp.Payload.ID,
		Name: resp.Payload.Name,
	}, nil
}

var GetCurrentOrgTool = mcpgrafana.MustTool(
	"get_current_org",
	"Get the current Grafana organization.",
	getCurrentOrg,
	mcp.WithTitleAnnotation("Get current org"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
