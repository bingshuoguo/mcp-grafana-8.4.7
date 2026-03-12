package tools

import (
	"context"
	"fmt"

	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

type ListFoldersRequest struct {
	Limit      *int64 `json:"limit,omitempty" jsonschema:"description=Max items\\, default 1000"`
	Page       *int64 `json:"page,omitempty" jsonschema:"description=Page number\\, default 1"`
	Permission string `json:"permission,omitempty" jsonschema:"description=View or Edit"`
}

type ListFoldersResponse struct {
	Items []FolderItem `json:"items"`
}

func listFolders(ctx context.Context, args ListFoldersRequest) (*ListFoldersResponse, error) {
	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	params := folders.NewGetFoldersParamsWithContext(ctx)
	if args.Limit != nil {
		params.SetLimit(args.Limit)
	}
	if args.Page != nil {
		params.SetPage(args.Page)
	}
	if args.Permission != "" {
		params.SetPermission(&args.Permission)
	}
	resp, err := gc.Folders.GetFolders(params)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", wrapOpenAPIError(err))
	}

	items := make([]FolderItem, 0)
	if resp != nil && resp.Payload != nil {
		for _, f := range resp.Payload {
			if f == nil {
				continue
			}
			items = append(items, FolderItem{
				ID:    f.ID,
				UID:   f.UID,
				Title: f.Title,
			})
		}
	}

	return &ListFoldersResponse{Items: items}, nil
}

var ListFoldersTool = mcpgrafana.MustTool(
	"list_folders",
	"List folders available in Grafana.",
	listFolders,
	mcp.WithTitleAnnotation("List folders"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

type CreateFolderRequest struct {
	Title     string `json:"title" jsonschema:"required,description=Folder title"`
	UID       string `json:"uid,omitempty" jsonschema:"description=Custom folder UID"`
	ParentUID string `json:"parentUid,omitempty" jsonschema:"description=Parent folder UID"`
}

type CreateFolderResponse struct {
	ID    int64  `json:"id,omitempty"`
	UID   string `json:"uid,omitempty"`
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
}

func folderToCreateResponse(f *models.Folder) *CreateFolderResponse {
	if f == nil {
		return &CreateFolderResponse{}
	}
	return &CreateFolderResponse{
		ID:    f.ID,
		UID:   f.UID,
		Title: f.Title,
		URL:   f.URL,
	}
}

func createFolder(ctx context.Context, args CreateFolderRequest) (*CreateFolderResponse, error) {
	if args.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	cmd := &models.CreateFolderCommand{
		Title:     args.Title,
		UID:       args.UID,
		ParentUID: args.ParentUID,
	}
	resp, err := gc.Folders.CreateFolder(cmd)
	if err != nil {
		return nil, fmt.Errorf("create folder: %w", wrapOpenAPIError(err))
	}

	if resp == nil {
		return &CreateFolderResponse{}, nil
	}
	return folderToCreateResponse(resp.Payload), nil
}

var CreateFolderTool = mcpgrafana.MustTool(
	"create_folder",
	"Create a new Grafana folder.",
	createFolder,
	mcp.WithTitleAnnotation("Create folder"),
	mcp.WithDestructiveHintAnnotation(true),
)

type UpdateFolderRequest struct {
	FolderUID   string `json:"folderUid" jsonschema:"required,description=Folder UID"`
	Title       string `json:"title" jsonschema:"required,description=New folder title"`
	Description string `json:"description,omitempty" jsonschema:"description=New folder description"`
	Version     *int64 `json:"version,omitempty" jsonschema:"description=Legacy version guard"`
	Overwrite   *bool  `json:"overwrite,omitempty" jsonschema:"description=Legacy overwrite option"`
}

type UpdateFolderResponse struct {
	ID    int64  `json:"id,omitempty"`
	UID   string `json:"uid,omitempty"`
	Title string `json:"title,omitempty"`
}

func updateFolder(ctx context.Context, args UpdateFolderRequest) (*UpdateFolderResponse, error) {
	if args.FolderUID == "" {
		return nil, fmt.Errorf("folderUid is required")
	}
	if args.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	gc, err := getGrafanaClient(ctx)
	if err != nil {
		return nil, err
	}

	cmd := &models.UpdateFolderCommand{
		Title: args.Title,
	}
	if args.Version != nil {
		cmd.Version = *args.Version
	}
	if args.Overwrite != nil {
		cmd.Overwrite = *args.Overwrite
	}

	resp, err := gc.Folders.UpdateFolder(args.FolderUID, cmd)
	if err != nil {
		return nil, fmt.Errorf("update folder: %w", wrapOpenAPIError(err))
	}
	if resp == nil || resp.Payload == nil {
		return &UpdateFolderResponse{}, nil
	}
	return &UpdateFolderResponse{
		ID:    resp.Payload.ID,
		UID:   resp.Payload.UID,
		Title: resp.Payload.Title,
	}, nil
}

var UpdateFolderTool = mcpgrafana.MustTool(
	"update_folder",
	"Update an existing Grafana folder.",
	updateFolder,
	mcp.WithTitleAnnotation("Update folder"),
	mcp.WithDestructiveHintAnnotation(true),
)
