package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/grafana/grafana-openapi-client-go/client/annotations"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

type GetAnnotationsRequest struct {
	From         *int64   `json:"from,omitempty" jsonschema:"description=Epoch millisecond start time"`
	To           *int64   `json:"to,omitempty" jsonschema:"description=Epoch millisecond end time"`
	UserID       *int64   `json:"userId,omitempty" jsonschema:"description=Filter by creator user ID"`
	AlertID      *int64   `json:"alertId,omitempty" jsonschema:"description=Filter by alert ID"`
	AlertUID     *string  `json:"alertUid,omitempty" jsonschema:"description=Filter by alert UID"`
	DashboardID  *int64   `json:"dashboardId,omitempty" jsonschema:"description=Filter by dashboard ID"`
	DashboardUID *string  `json:"dashboardUid,omitempty" jsonschema:"description=Filter by dashboard UID"`
	PanelID      *int64   `json:"panelId,omitempty" jsonschema:"description=Filter by panel ID"`
	Limit        *int64   `json:"limit,omitempty" jsonschema:"description=Max number of annotations to return"`
	Tags         []string `json:"tags,omitempty" jsonschema:"description=Filter by annotation tags"`
	Type         string   `json:"type,omitempty" jsonschema:"description=Filter by type: alert or annotation"`
	MatchAny     *bool    `json:"matchAny,omitempty" jsonschema:"description=Match any tag (OR) instead of all (AND)"`
}

type GetAnnotationsResponse struct {
	Items []AnnotationItem `json:"items"`
}

func getAnnotations(ctx context.Context, args GetAnnotationsRequest) (*GetAnnotationsResponse, error) {
	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	params := &annotations.GetAnnotationsParams{
		Context:      ctx,
		From:         args.From,
		To:           args.To,
		UserID:       args.UserID,
		AlertID:      args.AlertID,
		AlertUID:     args.AlertUID,
		DashboardID:  args.DashboardID,
		DashboardUID: args.DashboardUID,
		PanelID:      args.PanelID,
		Limit:        args.Limit,
		Tags:         args.Tags,
		MatchAny:     args.MatchAny,
	}
	if args.Type != "" {
		params.Type = &args.Type
	}

	resp, err := gc.Annotations.GetAnnotations(params)
	if err != nil {
		return nil, fmt.Errorf("get annotations: %w", wrapOpenAPIError(err))
	}

	items := make([]AnnotationItem, 0)
	if resp != nil && resp.Payload != nil {
		for _, a := range resp.Payload {
			items = append(items, itemDTOToAnnotation(a))
		}
	}
	return &GetAnnotationsResponse{Items: items}, nil
}

func itemDTOToAnnotation(a *models.Annotation) AnnotationItem {
	if a == nil {
		return AnnotationItem{}
	}
	return AnnotationItem{
		ID:           a.ID,
		AlertID:      a.AlertID,
		AlertName:    a.AlertName,
		DashboardID:  a.DashboardID,
		DashboardUID: a.DashboardUID,
		PanelID:      a.PanelID,
		UserID:       a.UserID,
		Login:        a.Login,
		Email:        a.Email,
		AvatarURL:    a.AvatarURL,
		NewState:     a.NewState,
		PrevState:    a.PrevState,
		Text:         a.Text,
		Tags:         a.Tags,
		Time:         a.Time,
		TimeEnd:      a.TimeEnd,
		Created:      a.Created,
		Updated:      a.Updated,
	}
}

var GetAnnotationsTool = mcpgrafana.MustTool(
	"get_annotations",
	"List annotations with optional filters such as time range, dashboard, tags, and type.",
	getAnnotations,
	mcp.WithTitleAnnotation("Get annotations"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

type CreateAnnotationRequest struct {
	DashboardID  *int64         `json:"dashboardId,omitempty" jsonschema:"description=Dashboard numeric ID"`
	DashboardUID string         `json:"dashboardUid,omitempty" jsonschema:"description=Dashboard UID"`
	PanelID      *int64         `json:"panelId,omitempty" jsonschema:"description=Panel ID to attach annotation to"`
	Time         *int64         `json:"time,omitempty" jsonschema:"description=Epoch millisecond start time"`
	TimeEnd      *int64         `json:"timeEnd,omitempty" jsonschema:"description=Epoch millisecond end time (for region annotations)"`
	Tags         []string       `json:"tags,omitempty" jsonschema:"description=Annotation tags"`
	Text         string         `json:"text" jsonschema:"required,description=Annotation text"`
	Data         map[string]any `json:"data,omitempty" jsonschema:"description=Arbitrary annotation data"`
}

type CreateAnnotationResponse struct {
	Message string `json:"message,omitempty"`
	ID      int64  `json:"id,omitempty"`
}

func createAnnotation(ctx context.Context, args CreateAnnotationRequest) (*CreateAnnotationResponse, error) {
	if args.Text == "" {
		return nil, fmt.Errorf("text is required")
	}

	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	cmd := &models.PostAnnotationsCmd{
		Tags: args.Tags,
		Text: &args.Text,
		Data: args.Data,
	}
	if args.DashboardID != nil {
		cmd.DashboardID = *args.DashboardID
	}
	if args.DashboardUID != "" {
		cmd.DashboardUID = args.DashboardUID
	}
	if args.PanelID != nil {
		cmd.PanelID = *args.PanelID
	}
	if args.Time != nil {
		cmd.Time = *args.Time
	}
	if args.TimeEnd != nil {
		cmd.TimeEnd = *args.TimeEnd
	}

	resp, err := gc.Annotations.PostAnnotation(cmd)
	if err != nil {
		return nil, fmt.Errorf("create annotation: %w", wrapOpenAPIError(err))
	}
	if resp == nil || resp.Payload == nil {
		return &CreateAnnotationResponse{}, nil
	}

	result := &CreateAnnotationResponse{}
	if resp.Payload.Message != nil {
		result.Message = *resp.Payload.Message
	}
	if resp.Payload.ID != nil {
		result.ID = *resp.Payload.ID
	}
	return result, nil
}

var CreateAnnotationTool = mcpgrafana.MustTool(
	"create_annotation",
	"Create a new annotation.",
	createAnnotation,
	mcp.WithTitleAnnotation("Create annotation"),
	mcp.WithDestructiveHintAnnotation(true),
)

type PatchAnnotationRequest struct {
	ID      int64          `json:"id" jsonschema:"required,description=Annotation ID"`
	Time    *int64         `json:"time,omitempty" jsonschema:"description=Epoch millisecond start time"`
	TimeEnd *int64         `json:"timeEnd,omitempty" jsonschema:"description=Epoch millisecond end time"`
	Tags    []string       `json:"tags,omitempty" jsonschema:"description=Updated annotation tags"`
	Text    *string        `json:"text,omitempty" jsonschema:"description=Updated annotation text"`
	Data    map[string]any `json:"data,omitempty" jsonschema:"description=Updated arbitrary annotation data"`
}

type PatchAnnotationResponse struct {
	Message string `json:"message,omitempty"`
}

func patchAnnotation(ctx context.Context, args PatchAnnotationRequest) (*PatchAnnotationResponse, error) {
	if args.ID <= 0 {
		return nil, fmt.Errorf("id is required")
	}

	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	cmd := &models.PatchAnnotationsCmd{ID: args.ID}
	if args.Time != nil {
		cmd.Time = *args.Time
	}
	if args.TimeEnd != nil {
		cmd.TimeEnd = *args.TimeEnd
	}
	if args.Tags != nil {
		cmd.Tags = args.Tags
	}
	if args.Text != nil {
		cmd.Text = *args.Text
	}
	if args.Data != nil {
		cmd.Data = args.Data
	}

	resp, err := gc.Annotations.PatchAnnotation(strconv.FormatInt(args.ID, 10), cmd)
	if err != nil {
		return nil, fmt.Errorf("patch annotation: %w", wrapOpenAPIError(err))
	}
	if resp == nil || resp.Payload == nil {
		return &PatchAnnotationResponse{}, nil
	}

	return &PatchAnnotationResponse{Message: resp.Payload.Message}, nil
}

var PatchAnnotationTool = mcpgrafana.MustTool(
	"patch_annotation",
	"Patch selected fields on an annotation.",
	patchAnnotation,
	mcp.WithTitleAnnotation("Patch annotation"),
	mcp.WithDestructiveHintAnnotation(true),
)
