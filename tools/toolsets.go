package tools

import (
	"strings"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

// Toolset describes a named, user-facing bundle of related MCP capabilities.
// It is used both for CLI filtering and for MCP resources/prompts discovery.
type Toolset struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	ToolNames   []string `json:"toolNames"`
}

// ToolSummary is a lightweight catalog view of a registered tool.
type ToolSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Toolsets    []string `json:"toolsets,omitempty"`
}

func BuiltInToolsets() []Toolset {
	return []Toolset{
		newToolset("core", "Core", "Health, identity, dashboard search, folder management, datasource resolution, annotations, and organization reads.", appendTools(
			v84CoreReadTools(),
			v84CoreReadOnlyExtensions(),
		)),
		newToolset("dashboards", "Dashboards", "Dashboard search, inspection, summaries, versions, and dashboard write operations.", []mcpgrafana.Tool{
			SearchDashboardsTool,
			GetDashboardByUIDTool,
			GetDashboardPanelQueriesTool,
			GetDashboardPropertyTool,
			GetDashboardSummaryTool,
			GetDashboardVersionsTool,
			UpsertDashboardTool,
			UpdateDashboardTool,
		}),
		newToolset("datasources", "Datasources", "Datasource discovery and generic datasource queries.", []mcpgrafana.Tool{
			ListDatasourcesTool,
			GetDatasourceTool,
			GetDatasourceByUIDTool,
			GetDatasourceByNameTool,
			ResolveDatasourceRefTool,
			QueryDatasourceTool,
			QueryDatasourceExpressionsTool,
			GetQueryExamplesTool,
		}),
		newToolset("prometheus", "Prometheus", "Prometheus metric discovery, label exploration, and PromQL execution.", []mcpgrafana.Tool{
			ListPrometheusLabelNamesTool,
			ListPrometheusLabelValuesTool,
			ListPrometheusMetricNamesTool,
			ListPrometheusMetricMetadataTool,
			QueryPrometheusTool,
			QueryPrometheusHistogramTool,
		}),
		newToolset("loki", "Loki", "Loki label discovery, log queries, stats, and pattern extraction.", []mcpgrafana.Tool{
			ListLokiLabelNamesTool,
			ListLokiLabelValuesTool,
			QueryLokiLogsTool,
			QueryLokiStatsTool,
			QueryLokiPatternsTool,
		}),
		newToolset("clickhouse", "ClickHouse", "ClickHouse table discovery, schema inspection, and SQL queries.", []mcpgrafana.Tool{
			ListClickHouseTablesTool,
			DescribeClickHouseTableTool,
			QueryClickHouseTool,
		}),
		newToolset("logs", "Logs", "Cross-backend log search and navigation helpers.", []mcpgrafana.Tool{
			SearchLogsTool,
			GenerateDeeplinkTool,
		}),
		newToolset("annotations", "Annotations", "Annotation reads and annotation write operations.", []mcpgrafana.Tool{
			GetAnnotationsTool,
			GetAnnotationTagsTool,
			CreateAnnotationTool,
			PatchAnnotationTool,
			UpdateAnnotationTool,
			CreateGraphiteAnnotationTool,
			DeleteAnnotationTool,
		}),
		newToolset("alerting", "Alerting", "Legacy and unified alerting tools, plus alert images.", appendTools(
			[]mcpgrafana.Tool{ListLegacyAlertsTool, ListLegacyNotificationChannelsTool},
			v84P2OptionalReadTools(),
			v84P2OptionalWriteTools(),
		)),
		newToolset("admin", "Admin", "Organization and team visibility helpers.", []mcpgrafana.Tool{
			GetCurrentUserTool,
			GetCurrentOrgTool,
			ListOrgUsersTool,
			ListUsersByOrgTool,
			ListTeamsTool,
			ListFoldersTool,
			SearchFoldersTool,
			CreateFolderTool,
			UpdateFolderTool,
		}),
		newToolset("write", "Write", "All mutating operations exposed by this server.", appendTools(
			v84CoreWriteTools(),
			v84P2OptionalWriteTools(),
		)),
	}
}

func ToolsetByName(name string) (Toolset, bool) {
	for _, toolset := range BuiltInToolsets() {
		if toolset.Name == name {
			return toolset, true
		}
	}
	return Toolset{}, false
}

func toolNamesForToolsets(names map[string]struct{}) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}
	toolNames := make(map[string]struct{})
	for name := range names {
		toolset, ok := ToolsetByName(name)
		if !ok {
			continue
		}
		for _, toolName := range toolset.ToolNames {
			toolNames[toolName] = struct{}{}
		}
	}
	return toolNames
}

func SummariesForTools(tools []mcpgrafana.Tool) []ToolSummary {
	if len(tools) == 0 {
		return nil
	}
	toolsetIndex := toolsetsByToolName()
	summaries := make([]ToolSummary, 0, len(tools))
	for _, tool := range tools {
		name := tool.Tool.Name
		summaries = append(summaries, ToolSummary{
			Name:        name,
			Description: strings.TrimSpace(tool.Tool.Description),
			Toolsets:    toolsetIndex[name],
		})
	}
	return summaries
}

func toolsetsByToolName() map[string][]string {
	index := make(map[string][]string)
	for _, toolset := range BuiltInToolsets() {
		for _, toolName := range toolset.ToolNames {
			index[toolName] = append(index[toolName], toolset.Name)
		}
	}
	return index
}

func newToolset(name, title, description string, tools []mcpgrafana.Tool) Toolset {
	return Toolset{
		Name:        name,
		Title:       title,
		Description: description,
		ToolNames:   toolNamesFromSlice(tools),
	}
}

func appendTools(groups ...[]mcpgrafana.Tool) []mcpgrafana.Tool {
	var out []mcpgrafana.Tool
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}

func toolNamesFromSlice(tools []mcpgrafana.Tool) []string {
	names := make([]string, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		name := tool.Tool.Name
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}
