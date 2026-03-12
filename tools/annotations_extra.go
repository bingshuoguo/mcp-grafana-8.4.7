package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

// --- create_graphite_annotation ---

type CreateGraphiteAnnotationRequest struct {
	What string   `json:"what" jsonschema:"required,description=Short description of the event"`
	Tags []string `json:"tags,omitempty" jsonschema:"description=Tags to attach to the annotation"`
	When *int64   `json:"when,omitempty" jsonschema:"description=Epoch second timestamp (default: now)"`
	Data string   `json:"data,omitempty" jsonschema:"description=Long description of the event"`
}

type CreateGraphiteAnnotationResponse struct {
	Message string `json:"message,omitempty"`
	ID      int64  `json:"id,omitempty"`
}

func createGraphiteAnnotation(ctx context.Context, args CreateGraphiteAnnotationRequest) (*CreateGraphiteAnnotationResponse, error) {
	if args.What == "" {
		return nil, fmt.Errorf("what is required")
	}

	body := map[string]any{
		"what": args.What,
		"tags": args.Tags,
		"data": args.Data,
	}
	if args.When != nil {
		body["when"] = *args.When
	}

	respBody, statusCode, err := doAPIRequest(ctx, "POST", "/annotations/graphite", nil, body)
	if err != nil {
		return nil, fmt.Errorf("create graphite annotation: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	var raw struct {
		Message string `json:"message"`
		ID      int64  `json:"id"`
	}
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &raw)
	}

	return &CreateGraphiteAnnotationResponse{Message: raw.Message, ID: raw.ID}, nil
}

var CreateGraphiteAnnotationTool = mcpgrafana.MustTool(
	"create_graphite_annotation",
	"Create a Graphite-compatible event annotation.",
	createGraphiteAnnotation,
	mcp.WithTitleAnnotation("Create Graphite annotation"),
	mcp.WithDestructiveHintAnnotation(true),
)

// --- update_annotation ---

type UpdateAnnotationRequest struct {
	ID      int64    `json:"id" jsonschema:"required,description=Annotation ID"`
	Time    *int64   `json:"time,omitempty" jsonschema:"description=Epoch millisecond start time"`
	TimeEnd *int64   `json:"timeEnd,omitempty" jsonschema:"description=Epoch millisecond end time"`
	Tags    []string `json:"tags,omitempty" jsonschema:"description=Updated annotation tags"`
	Text    string   `json:"text,omitempty" jsonschema:"description=Updated annotation text"`
}

type UpdateAnnotationResponse struct {
	Message string `json:"message,omitempty"`
}

func updateAnnotation(ctx context.Context, args UpdateAnnotationRequest) (*UpdateAnnotationResponse, error) {
	if args.ID <= 0 {
		return nil, fmt.Errorf("id is required")
	}

	body := map[string]any{
		"text": args.Text,
		"tags": args.Tags,
	}
	if args.Time != nil {
		body["time"] = *args.Time
	}
	if args.TimeEnd != nil {
		body["timeEnd"] = *args.TimeEnd
	}

	path := "/annotations/" + strconv.FormatInt(args.ID, 10)
	respBody, statusCode, err := doAPIRequest(ctx, "PUT", path, nil, body)
	if err != nil {
		return nil, fmt.Errorf("update annotation: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	var raw struct {
		Message string `json:"message"`
	}
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &raw)
	}

	return &UpdateAnnotationResponse{Message: raw.Message}, nil
}

var UpdateAnnotationTool = mcpgrafana.MustTool(
	"update_annotation",
	"Update (replace) all fields on an existing annotation by ID.",
	updateAnnotation,
	mcp.WithTitleAnnotation("Update annotation"),
	mcp.WithDestructiveHintAnnotation(true),
)

// --- get_annotation_tags ---

type GetAnnotationTagsRequest struct {
	Tag   string `json:"tag,omitempty" jsonschema:"description=Filter by tag prefix"`
	Limit *int64 `json:"limit,omitempty" jsonschema:"description=Max number of tags to return\\, default 100"`
}

type AnnotationTagItem struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

type GetAnnotationTagsResponse struct {
	Items []AnnotationTagItem `json:"items"`
}

type annotationTagsRaw struct {
	Result struct {
		Tags []struct {
			Tag   string `json:"tag"`
			Count int64  `json:"count"`
		} `json:"tags"`
	} `json:"result"`
}

func getAnnotationTags(ctx context.Context, args GetAnnotationTagsRequest) (*GetAnnotationTagsResponse, error) {
	query := url.Values{}
	if args.Tag != "" {
		query.Set("tag", args.Tag)
	}

	limit := int64(100)
	if args.Limit != nil {
		limit = *args.Limit
	}
	if limit <= 0 {
		limit = 100
	}
	query.Set("limit", strconv.FormatInt(limit, 10))

	// Tag aggregation can be slow on large Grafana instances, so use a larger timeout and one retry on timeout.
	reqCtx := withGrafanaTimeout(ctx, 30*time.Second)
	respBody, statusCode, err := doAPIRequest(reqCtx, "GET", "/annotations/tags", query, nil)
	if err != nil && isTimeoutError(err) {
		retryCtx := withGrafanaTimeout(ctx, 60*time.Second)
		respBody, statusCode, err = doAPIRequest(retryCtx, "GET", "/annotations/tags", query, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("get annotation tags: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	var raw annotationTagsRaw
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &raw); err != nil {
			return nil, fmt.Errorf("decode annotation tags: %w", err)
		}
	}

	items := make([]AnnotationTagItem, 0, len(raw.Result.Tags))
	for _, t := range raw.Result.Tags {
		items = append(items, AnnotationTagItem{Tag: t.Tag, Count: t.Count})
	}

	return &GetAnnotationTagsResponse{Items: items}, nil
}

var GetAnnotationTagsTool = mcpgrafana.MustTool(
	"get_annotation_tags",
	"List annotation tags with optional prefix filter and occurrence count.",
	getAnnotationTags,
	mcp.WithTitleAnnotation("Get annotation tags"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// --- delete_annotation ---

type DeleteAnnotationRequest struct {
	ID int64 `json:"id" jsonschema:"required,description=Annotation ID to delete"`
}

type DeleteAnnotationResponse struct {
	Message string `json:"message,omitempty"`
	ID      int64  `json:"id"`
}

func deleteAnnotation(ctx context.Context, args DeleteAnnotationRequest) (*DeleteAnnotationResponse, error) {
	if args.ID <= 0 {
		return nil, fmt.Errorf("id is required")
	}

	path := "/annotations/" + strconv.FormatInt(args.ID, 10)
	respBody, statusCode, err := doAPIRequest(ctx, "DELETE", path, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("delete annotation: %w", wrapRawAPIError(statusCode, respBody, err))
	}

	var raw struct {
		Message string `json:"message"`
	}
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &raw)
	}

	return &DeleteAnnotationResponse{Message: raw.Message, ID: args.ID}, nil
}

var DeleteAnnotationTool = mcpgrafana.MustTool(
	"delete_annotation",
	"Delete an annotation by ID.",
	deleteAnnotation,
	mcp.WithTitleAnnotation("Delete annotation"),
	mcp.WithDestructiveHintAnnotation(true),
)
