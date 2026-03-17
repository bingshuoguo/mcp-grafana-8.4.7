//go:build unit

package tools

import (
	"bytes"
	"log/slog"
	"slices"
	"testing"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddV84ToolsDefaultBehavior(t *testing.T) {
	srv := server.NewMCPServer("test", "test")

	AddV84Tools(srv, RegisterOptions{
		EnableWriteTools:    true,
		EnableOptionalTools: false,
	})

	tools := srv.ListTools()
	require.NotNil(t, tools)
	assert.Contains(t, tools, GetHealthTool.Tool.Name)
	assert.Contains(t, tools, UpsertDashboardTool.Tool.Name)
	assert.Contains(t, tools, UpdateDashboardTool.Tool.Name)
	assert.Contains(t, tools, ListPrometheusLabelNamesTool.Tool.Name)
	assert.NotContains(t, tools, GetPanelImageTool.Tool.Name)
	assert.NotContains(t, tools, ListAlertRulesTool.Tool.Name)
}

func TestAddV84ToolsDisableToolsRemovesDefaults(t *testing.T) {
	srv := server.NewMCPServer("test", "test")

	AddV84Tools(srv, RegisterOptions{
		EnableWriteTools: true,
		DisableTools:     []string{GetHealthTool.Tool.Name, UpsertDashboardTool.Tool.Name},
		DisableToolsSet:  true,
	})

	tools := srv.ListTools()
	require.NotNil(t, tools)
	assert.NotContains(t, tools, GetHealthTool.Tool.Name)
	assert.NotContains(t, tools, UpsertDashboardTool.Tool.Name)
	assert.Contains(t, tools, UpdateDashboardTool.Tool.Name)
}

func TestAddV84ToolsEnableToolsIsExplicitAllowlist(t *testing.T) {
	srv := server.NewMCPServer("test", "test")

	AddV84Tools(srv, RegisterOptions{
		EnableWriteTools:    false,
		EnableOptionalTools: false,
		EnableTools:         []string{CreateFolderTool.Tool.Name, GetPanelImageTool.Tool.Name},
		EnableToolsSet:      true,
	})

	tools := srv.ListTools()
	require.Len(t, tools, 2)
	assert.Contains(t, tools, CreateFolderTool.Tool.Name)
	assert.Contains(t, tools, GetPanelImageTool.Tool.Name)
}

func TestAddV84ToolsDisableWinsOverEnable(t *testing.T) {
	srv := server.NewMCPServer("test", "test")

	AddV84Tools(srv, RegisterOptions{
		EnableTools:     []string{GetHealthTool.Tool.Name, SearchDashboardsTool.Tool.Name},
		EnableToolsSet:  true,
		DisableTools:    []string{GetHealthTool.Tool.Name},
		DisableToolsSet: true,
	})

	tools := srv.ListTools()
	require.Len(t, tools, 1)
	assert.NotContains(t, tools, GetHealthTool.Tool.Name)
	assert.Contains(t, tools, SearchDashboardsTool.Tool.Name)
}

func TestAddV84ToolsAliasFilteringIsIndependent(t *testing.T) {
	srv := server.NewMCPServer("test", "test")

	AddV84Tools(srv, RegisterOptions{
		EnableTools:    []string{UpdateDashboardTool.Tool.Name},
		EnableToolsSet: true,
	})

	tools := srv.ListTools()
	require.Len(t, tools, 1)
	assert.Contains(t, tools, UpdateDashboardTool.Tool.Name)
	assert.NotContains(t, tools, UpsertDashboardTool.Tool.Name)
}

func TestAddV84ToolsDisablingAliasLeavesCanonical(t *testing.T) {
	srv := server.NewMCPServer("test", "test")

	AddV84Tools(srv, RegisterOptions{
		EnableWriteTools: true,
		DisableTools:     []string{UpdateDashboardTool.Tool.Name},
		DisableToolsSet:  true,
	})

	tools := srv.ListTools()
	require.NotNil(t, tools)
	assert.Contains(t, tools, UpsertDashboardTool.Tool.Name, "canonical upsert_dashboard should remain when only alias update_dashboard is disabled")
	assert.NotContains(t, tools, UpdateDashboardTool.Tool.Name)
}

func TestFilterToolsPreservesCatalogOrder(t *testing.T) {
	all := allV84Tools()

	filtered, _ := filterTools(all, RegisterOptions{
		EnableTools:    []string{SearchDashboardsTool.Tool.Name, GetHealthTool.Tool.Name},
		EnableToolsSet: true,
	})

	require.Len(t, filtered, 2)
	assert.Equal(t, []string{
		GetHealthTool.Tool.Name,
		SearchDashboardsTool.Tool.Name,
	}, toolNames(filtered))
}

func TestFilterToolsWarnsOnUnknownToolsAndAllowsEmptySet(t *testing.T) {
	var logBuf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, nil)))
	defer slog.SetDefault(old)

	srv := server.NewMCPServer("test", "test")
	AddV84Tools(srv, RegisterOptions{
		EnableTools:     []string{"unknown_tool"},
		EnableToolsSet:  true,
		DisableTools:    []string{"another_unknown"},
		DisableToolsSet: true,
	})

	assert.Nil(t, srv.ListTools())
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "unknown tool in --enable-tools; ignoring")
	assert.Contains(t, logOutput, "unknown tool in --disable-tools; ignoring")
	assert.Contains(t, logOutput, "no tools registered after applying tool filters")
}

func TestFilterToolsWithOptionalAndWriteFlags(t *testing.T) {
	srv := server.NewMCPServer("test", "test")

	AddV84Tools(srv, RegisterOptions{
		EnableWriteTools:    false,
		EnableOptionalTools: true,
	})

	tools := srv.ListTools()
	require.NotNil(t, tools)
	assert.Contains(t, tools, ListAlertRulesTool.Tool.Name)
	assert.Contains(t, tools, GetPanelImageTool.Tool.Name)
	assert.NotContains(t, tools, CreateAlertRuleTool.Tool.Name)
}

func TestAddV84ToolsToolsetsActsAsAllowlist(t *testing.T) {
	srv := server.NewMCPServer("test", "test")

	AddV84Tools(srv, RegisterOptions{
		EnableWriteTools: false,
		Toolsets:         []string{"prometheus", "dashboards"},
		ToolsetsSet:      true,
	})

	tools := srv.ListTools()
	require.NotNil(t, tools)
	assert.Contains(t, tools, QueryPrometheusTool.Tool.Name)
	assert.Contains(t, tools, GetDashboardSummaryTool.Tool.Name)
	assert.Contains(t, tools, UpsertDashboardTool.Tool.Name, "explicit toolset selection should override the default write/optional profile boundaries")
	assert.NotContains(t, tools, QueryLokiLogsTool.Tool.Name)
}

func TestFilterToolsWarnsOnUnknownToolsets(t *testing.T) {
	var logBuf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, nil)))
	defer slog.SetDefault(old)

	srv := server.NewMCPServer("test", "test")
	AddV84Tools(srv, RegisterOptions{
		EnableWriteTools: true,
		Toolsets:         []string{"unknown_toolset"},
		ToolsetsSet:      true,
	})

	assert.Nil(t, srv.ListTools())
	assert.Contains(t, logBuf.String(), "unknown toolset in --toolsets; ignoring")
}

func toolNames(tools []mcpgrafana.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Tool.Name)
	}
	return names
}

func TestAllV84ToolsAreUnique(t *testing.T) {
	all := allV84Tools()
	names := toolNames(all)
	sorted := slices.Clone(names)
	slices.Sort(sorted)
	for i := 1; i < len(sorted); i++ {
		assert.NotEqual(t, sorted[i-1], sorted[i], "duplicate tool name: %s", sorted[i])
	}
}
