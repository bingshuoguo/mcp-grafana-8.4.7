package tools

import (
	"log/slog"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
	"github.com/mark3labs/mcp-go/server"
)

type RegisterOptions struct {
	EnableWriteTools    bool
	EnableOptionalTools bool
	Toolsets            []string
	ToolsetsSet         bool
	EnableTools         []string
	EnableToolsSet      bool
	DisableTools        []string
	DisableToolsSet     bool
}

// AddV84Tools registers Grafana 8.4.7 tool contracts.
func AddV84Tools(m *server.MCPServer, opts RegisterOptions) {
	tools, finalNames := ResolveV84Tools(opts)
	for _, tool := range tools {
		tool.Register(m)
	}
	slog.Info("tool filter applied",
		"disable_write", !opts.EnableWriteTools,
		"enable_optional_tools", opts.EnableOptionalTools,
		"toolsets_count", len(opts.Toolsets),
		"enable_tools_count", len(opts.EnableTools),
		"disable_tools_count", len(opts.DisableTools),
		"final_tools_count", len(finalNames))
	if len(finalNames) == 0 {
		slog.Warn("no tools registered after applying tool filters")
	}
}

// ResolveV84Tools returns the final tool list after applying write/optional,
// toolset, and explicit tool filters.
func ResolveV84Tools(opts RegisterOptions) ([]mcpgrafana.Tool, map[string]struct{}) {
	return filterTools(allV84Tools(), opts)
}

func defaultV84Tools(opts RegisterOptions) []mcpgrafana.Tool {
	tools := append([]mcpgrafana.Tool{}, v84CoreReadTools()...)
	tools = append(tools, v84CoreReadOnlyExtensions()...)
	tools = append(tools, v84P1DataTools()...)

	if opts.EnableWriteTools {
		tools = append(tools, v84CoreWriteTools()...)
	}

	if opts.EnableOptionalTools {
		tools = append(tools, v84P2OptionalReadTools()...)
		if opts.EnableWriteTools {
			tools = append(tools, v84P2OptionalWriteTools()...)
		}
	}

	return tools
}

func allV84Tools() []mcpgrafana.Tool {
	tools := append([]mcpgrafana.Tool{}, v84CoreReadTools()...)
	tools = append(tools, v84CoreReadOnlyExtensions()...)
	tools = append(tools, v84P1DataTools()...)
	tools = append(tools, v84CoreWriteTools()...)
	tools = append(tools, v84P2OptionalReadTools()...)
	tools = append(tools, v84P2OptionalWriteTools()...)
	return tools
}

func filterTools(all []mcpgrafana.Tool, opts RegisterOptions) ([]mcpgrafana.Tool, map[string]struct{}) {
	allByName := toolSetFromSlice(all)
	warnUnknown("--enable-tools", uniqueNames(opts.EnableTools), allByName)
	warnUnknown("--disable-tools", uniqueNames(opts.DisableTools), allByName)
	warnUnknownToolsets("--toolsets", uniqueNames(opts.Toolsets))

	candidate := defaultV84Tools(opts)
	if opts.ToolsetsSet {
		toolsetSet := uniqueNames(opts.Toolsets)
		candidate = filterSliceBySet(all, toolNamesForToolsets(toolsetSet))
	}
	if opts.EnableToolsSet {
		enableSet := uniqueNames(opts.EnableTools)
		candidate = filterSliceBySet(all, enableSet)
	}

	disableSet := uniqueNames(opts.DisableTools)
	filtered := make([]mcpgrafana.Tool, 0, len(candidate))
	finalNames := make(map[string]struct{}, len(candidate))
	for _, tool := range candidate {
		name := tool.Tool.Name
		if _, disabled := disableSet[name]; disabled {
			continue
		}
		if _, seen := finalNames[name]; seen {
			continue
		}
		filtered = append(filtered, tool)
		finalNames[name] = struct{}{}
	}

	return filtered, finalNames
}

func warnUnknownToolsets(flagName string, names map[string]struct{}) {
	for name := range names {
		if _, ok := ToolsetByName(name); ok {
			continue
		}
		slog.Warn("unknown toolset in "+flagName+"; ignoring", "toolset", name)
	}
}

func warnUnknown(flagName string, names map[string]struct{}, known map[string]mcpgrafana.Tool) {
	for name := range names {
		if _, ok := known[name]; ok {
			continue
		}
		slog.Warn("unknown tool in "+flagName+"; ignoring", "tool", name)
	}
}

func filterSliceBySet(all []mcpgrafana.Tool, allow map[string]struct{}) []mcpgrafana.Tool {
	filtered := make([]mcpgrafana.Tool, 0, len(allow))
	seen := make(map[string]struct{}, len(allow))
	for _, tool := range all {
		name := tool.Tool.Name
		if _, ok := allow[name]; !ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		filtered = append(filtered, tool)
		seen[name] = struct{}{}
	}
	return filtered
}

func uniqueNames(names []string) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}
	unique := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		unique[name] = struct{}{}
	}
	return unique
}

func toolSetFromSlice(tools []mcpgrafana.Tool) map[string]mcpgrafana.Tool {
	byName := make(map[string]mcpgrafana.Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Tool.Name] = tool
	}
	return byName
}

func v84CoreReadTools() []mcpgrafana.Tool {
	return []mcpgrafana.Tool{
		GetHealthTool,
		GetCurrentUserTool,
		GetCurrentOrgTool,
		SearchDashboardsTool,
		GetDashboardByUIDTool,
		ListFoldersTool,
		ListDatasourcesTool,
		GetDatasourceTool,
		ResolveDatasourceRefTool,
		QueryDatasourceTool,
		QueryDatasourceExpressionsTool,
		QueryPrometheusTool,
		ListPrometheusLabelValuesTool,
		ListPrometheusMetricNamesTool,
		GetAnnotationsTool,
		ListLegacyAlertsTool,
		ListLegacyNotificationChannelsTool,
		ListOrgUsersTool,
		ListTeamsTool,
	}
}

func v84CoreReadOnlyExtensions() []mcpgrafana.Tool {
	return []mcpgrafana.Tool{
		GetDatasourceByUIDTool,
		GetDatasourceByNameTool,
		ListUsersByOrgTool,
		SearchFoldersTool,
		GetDashboardPanelQueriesTool,
		GetDashboardPropertyTool,
		GetDashboardSummaryTool,
		GetDashboardVersionsTool,
		GetAnnotationTagsTool,
		GenerateDeeplinkTool,
		GetQueryExamplesTool,
	}
}

func v84P1DataTools() []mcpgrafana.Tool {
	return []mcpgrafana.Tool{
		ListPrometheusLabelNamesTool,
		ListPrometheusMetricMetadataTool,
		QueryPrometheusHistogramTool,
		ListLokiLabelNamesTool,
		ListLokiLabelValuesTool,
		QueryLokiLogsTool,
		QueryLokiStatsTool,
		QueryLokiPatternsTool,
		QueryClickHouseTool,
		ListClickHouseTablesTool,
		DescribeClickHouseTableTool,
		SearchLogsTool,
	}
}

func v84P2OptionalReadTools() []mcpgrafana.Tool {
	return []mcpgrafana.Tool{
		ListAlertRulesTool,
		GetAlertRuleByUIDTool,
		ListContactPointsTool,
		GetFiringAlertsTool,
		GetAlertRulesWithStateTool,
		GetPanelImageTool,
	}
}

func v84P2OptionalWriteTools() []mcpgrafana.Tool {
	return []mcpgrafana.Tool{
		CreateAlertRuleTool,
		UpdateAlertRuleTool,
		DeleteAlertRuleTool,
	}
}

func v84CoreWriteTools() []mcpgrafana.Tool {
	return []mcpgrafana.Tool{
		UpsertDashboardTool,
		UpdateDashboardTool,
		CreateFolderTool,
		UpdateFolderTool,
		CreateAnnotationTool,
		PatchAnnotationTool,
		UpdateAnnotationTool,
		CreateGraphiteAnnotationTool,
		DeleteAnnotationTool,
	}
}
