//go:build unit

package main

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bingshuoguo/grafana-v8-mcp/observability"
	grafanatools "github.com/bingshuoguo/grafana-v8-mcp/tools"
)

func TestBuildServerCatalogReflectsResolvedTools(t *testing.T) {
	catalog := buildServerCatalog(grafanatools.RegisterOptions{
		EnableWriteTools: false,
		Toolsets:         []string{"dashboards"},
		ToolsetsSet:      true,
	})

	require.NotEmpty(t, catalog.Tools)
	assert.Contains(t, renderOverviewMarkdown(catalog), "dashboards")
	assert.Contains(t, renderOverviewMarkdown(catalog), "upsert_dashboard")
}

func TestNewServerExposesPromptsAndResources(t *testing.T) {
	srv := newServer(toolConfig{}, &observability.Observability{})

	cli, err := client.NewInProcessClient(srv)
	require.NoError(t, err)
	defer cli.Close()

	ctx := context.Background()
	require.NoError(t, cli.Start(ctx))
	_, err = cli.Initialize(ctx, testInitializeRequest())
	require.NoError(t, err)

	resources, err := cli.ListResources(ctx, mcp.ListResourcesRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, resources.Resources)

	var foundOverview bool
	for _, resource := range resources.Resources {
		if resource.URI == serverOverviewURI {
			foundOverview = true
			break
		}
	}
	assert.True(t, foundOverview)

	readResult, err := cli.ReadResource(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: serverOverviewURI},
	})
	require.NoError(t, err)
	require.NotEmpty(t, readResult.Contents)
	textContent, ok := readResult.Contents[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "Enabled Tools")

	prompts, err := cli.ListPrompts(ctx, mcp.ListPromptsRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, prompts.Prompts)

	promptResult, err := cli.GetPrompt(ctx, mcp.GetPromptRequest{
		Params: mcp.GetPromptParams{
			Name: "grafana_dashboard_triage",
			Arguments: map[string]string{
				"dashboard_uid": "abc123",
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, promptResult)
	assert.Equal(t, "Dashboard triage workflow", promptResult.Description)
	assert.Len(t, promptResult.Messages, 3)
}

func TestNewServerEnablesPromptAndResourceCapabilities(t *testing.T) {
	srv := newServer(toolConfig{}, &observability.Observability{})

	cli, err := client.NewInProcessClient(srv)
	require.NoError(t, err)
	defer cli.Close()

	ctx := context.Background()
	require.NoError(t, cli.Start(ctx))
	result, err := cli.Initialize(ctx, testInitializeRequest())
	require.NoError(t, err)
	require.NotNil(t, result.Capabilities.Prompts)
	require.NotNil(t, result.Capabilities.Resources)
	require.NotNil(t, result.Capabilities.Tools)
}

func TestAddServerAssetsSupportsToolsetTemplate(t *testing.T) {
	srv := server.NewMCPServer("test", "1.0.0", server.WithResourceCapabilities(false, true), server.WithPromptCapabilities(true))
	addServerAssets(srv, grafanatools.RegisterOptions{EnableWriteTools: true})

	cli, err := client.NewInProcessClient(srv)
	require.NoError(t, err)
	defer cli.Close()
	ctx := context.Background()
	require.NoError(t, cli.Start(ctx))
	_, err = cli.Initialize(ctx, testInitializeRequest())
	require.NoError(t, err)

	readResult, err := cli.ReadResource(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: toolsetURIBase + "prometheus"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, readResult.Contents)
	textContent, ok := readResult.Contents[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "\"name\": \"prometheus\"")
}

func testInitializeRequest() mcp.InitializeRequest {
	req := mcp.InitializeRequest{}
	req.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	req.Params.ClientInfo = mcp.Implementation{
		Name:    "unit-test-client",
		Version: "1.0.0",
	}
	return req
}
