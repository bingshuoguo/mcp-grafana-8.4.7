package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	grafanatools "github.com/bingshuoguo/grafana-v8-mcp/tools"
)

const (
	serverOverviewURI  = "grafana://server/overview"
	serverToolsetsURI  = "grafana://server/toolsets"
	serverWorkflowsURI = "grafana://server/workflows"
	toolsetURIBase     = "grafana://toolsets/"
)

type serverCatalog struct {
	Tools         []grafanatools.ToolSummary `json:"tools"`
	Toolsets      []grafanatools.Toolset     `json:"toolsets"`
	SelectedFlags selectedFlags              `json:"selectedFlags"`
}

type selectedFlags struct {
	DisableWrite        bool     `json:"disableWrite"`
	EnableOptionalTools bool     `json:"enableOptionalTools"`
	Toolsets            []string `json:"toolsets,omitempty"`
	EnableTools         []string `json:"enableTools,omitempty"`
	DisableTools        []string `json:"disableTools,omitempty"`
}

func addServerAssets(s *server.MCPServer, opts grafanatools.RegisterOptions) {
	catalog := buildServerCatalog(opts)

	s.AddResource(
		mcp.NewResource(
			serverOverviewURI,
			"Server Overview",
			mcp.WithResourceDescription("Overview of the Grafana MCP server, active tool filters, and enabled tools."),
			mcp.WithMIMEType("text/markdown"),
		),
		func(ctx context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      serverOverviewURI,
					MIMEType: "text/markdown",
					Text:     renderOverviewMarkdown(catalog),
				},
			}, nil
		},
	)

	s.AddResource(
		mcp.NewResource(
			serverToolsetsURI,
			"Toolset Catalog",
			mcp.WithResourceDescription("JSON catalog of built-in toolsets and their tool membership."),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      serverToolsetsURI,
					MIMEType: "application/json",
					Text:     mustJSON(catalog),
				},
			}, nil
		},
	)

	s.AddResource(
		mcp.NewResource(
			serverWorkflowsURI,
			"Recommended Workflows",
			mcp.WithResourceDescription("Suggested investigation workflows for dashboards, datasources, logs, and alerts."),
			mcp.WithMIMEType("text/markdown"),
		),
		func(ctx context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      serverWorkflowsURI,
					MIMEType: "text/markdown",
					Text:     renderWorkflowMarkdown(),
				},
			}, nil
		},
	)

	s.AddResourceTemplate(
		mcp.NewResourceTemplate(
			toolsetURIBase+"{name}",
			"Toolset Details",
			mcp.WithTemplateDescription("Details for a specific built-in toolset."),
			mcp.WithTemplateMIMEType("application/json"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			name := strings.TrimPrefix(request.Params.URI, toolsetURIBase)
			toolset, ok := grafanatools.ToolsetByName(name)
			if !ok {
				return nil, fmt.Errorf("unknown toolset: %s", name)
			}
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      request.Params.URI,
					MIMEType: "application/json",
					Text:     mustJSON(toolset),
				},
			}, nil
		},
	)

	s.AddPrompt(
		mcp.NewPrompt(
			"grafana_dashboard_triage",
			mcp.WithPromptDescription("Guide the model through dashboard inspection and root-cause analysis."),
			mcp.WithArgument("dashboard_uid", mcp.ArgumentDescription("Grafana dashboard UID to inspect"), mcp.RequiredArgument()),
			mcp.WithArgument("question", mcp.ArgumentDescription("What the user wants to learn from the dashboard")),
		),
		func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			dashboardUID := strings.TrimSpace(request.Params.Arguments["dashboard_uid"])
			if dashboardUID == "" {
				return nil, fmt.Errorf("dashboard_uid is required")
			}
			question := strings.TrimSpace(request.Params.Arguments["question"])
			if question == "" {
				question = "Inspect the dashboard, summarize the important panels, and identify likely next queries."
			}
			return mcp.NewGetPromptResult(
				"Dashboard triage workflow",
				[]mcp.PromptMessage{
					mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(fmt.Sprintf(
						"Investigate dashboard `%s`. %s Start by reading the server overview and then inspect the dashboard summary, panel queries, and deeplinks before making conclusions.",
						dashboardUID, question,
					))),
					mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewEmbeddedResource(mcp.TextResourceContents{
						URI:      serverOverviewURI,
						MIMEType: "text/markdown",
						Text:     renderOverviewMarkdown(catalog),
					})),
					mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewEmbeddedResource(mcp.TextResourceContents{
						URI:      toolsetURIBase + "dashboards",
						MIMEType: "application/json",
						Text:     mustJSON(mustToolset("dashboards")),
					})),
				},
			), nil
		},
	)

	s.AddPrompt(
		mcp.NewPrompt(
			"grafana_datasource_query_review",
			mcp.WithPromptDescription("Guide the model through datasource discovery and safe query planning."),
			mcp.WithArgument("datasource_hint", mcp.ArgumentDescription("Datasource name, UID, ID, or type to start from"), mcp.RequiredArgument()),
			mcp.WithArgument("goal", mcp.ArgumentDescription("What signal or answer is needed from the datasource")),
		),
		func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			datasourceHint := strings.TrimSpace(request.Params.Arguments["datasource_hint"])
			if datasourceHint == "" {
				return nil, fmt.Errorf("datasource_hint is required")
			}
			goal := strings.TrimSpace(request.Params.Arguments["goal"])
			if goal == "" {
				goal = "Resolve the datasource first, then choose the smallest discovery query that can answer the question."
			}
			return mcp.NewGetPromptResult(
				"Datasource query review",
				[]mcp.PromptMessage{
					mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(fmt.Sprintf(
						"Investigate datasource `%s`. %s Prefer read-only exploration before broad queries. Use datasource resolution, datasource-type-specific discovery tools, and only then run the final query.",
						datasourceHint, goal,
					))),
					mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewEmbeddedResource(mcp.TextResourceContents{
						URI:      serverWorkflowsURI,
						MIMEType: "text/markdown",
						Text:     renderWorkflowMarkdown(),
					})),
					mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewEmbeddedResource(mcp.TextResourceContents{
						URI:      toolsetURIBase + "datasources",
						MIMEType: "application/json",
						Text:     mustJSON(mustToolset("datasources")),
					})),
				},
			), nil
		},
	)

	s.AddPrompt(
		mcp.NewPrompt(
			"grafana_alert_triage",
			mcp.WithPromptDescription("Guide the model through alert inspection across legacy and unified alerting paths."),
			mcp.WithArgument("alert_identifier", mcp.ArgumentDescription("Alert UID, alert name, or dashboard/panel context"), mcp.RequiredArgument()),
			mcp.WithArgument("focus", mcp.ArgumentDescription("What to verify: rule state, firing instances, history, or routing")),
		),
		func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			alertIdentifier := strings.TrimSpace(request.Params.Arguments["alert_identifier"])
			if alertIdentifier == "" {
				return nil, fmt.Errorf("alert_identifier is required")
			}
			focus := strings.TrimSpace(request.Params.Arguments["focus"])
			if focus == "" {
				focus = "Determine whether the alert exists, its current state, and the smallest next query to validate the signal."
			}
			return mcp.NewGetPromptResult(
				"Alert triage workflow",
				[]mcp.PromptMessage{
					mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(fmt.Sprintf(
						"Investigate alert context `%s`. %s Check both legacy and unified alerting capabilities when available, and explain which path is supported by the server before querying deeper.",
						alertIdentifier, focus,
					))),
					mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewEmbeddedResource(mcp.TextResourceContents{
						URI:      serverOverviewURI,
						MIMEType: "text/markdown",
						Text:     renderOverviewMarkdown(catalog),
					})),
					mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewEmbeddedResource(mcp.TextResourceContents{
						URI:      toolsetURIBase + "alerting",
						MIMEType: "application/json",
						Text:     mustJSON(mustToolset("alerting")),
					})),
				},
			), nil
		},
	)
}

func buildServerCatalog(opts grafanatools.RegisterOptions) serverCatalog {
	tools, _ := grafanatools.ResolveV84Tools(opts)
	return serverCatalog{
		Tools:    grafanatools.SummariesForTools(tools),
		Toolsets: grafanatools.BuiltInToolsets(),
		SelectedFlags: selectedFlags{
			DisableWrite:        !opts.EnableWriteTools,
			EnableOptionalTools: opts.EnableOptionalTools,
			Toolsets:            append([]string(nil), opts.Toolsets...),
			EnableTools:         append([]string(nil), opts.EnableTools...),
			DisableTools:        append([]string(nil), opts.DisableTools...),
		},
	}
}

func renderOverviewMarkdown(catalog serverCatalog) string {
	var b strings.Builder
	b.WriteString("# Grafana MCP Server Overview\n\n")
	b.WriteString("This server targets Grafana v8.4.7 and exposes grouped MCP capabilities for dashboards, datasources, logs, alerts, and organization visibility.\n\n")
	b.WriteString("## Active Tool Selection\n")
	fmt.Fprintf(&b, "- disableWrite: `%t`\n", catalog.SelectedFlags.DisableWrite)
	fmt.Fprintf(&b, "- enableOptionalTools: `%t`\n", catalog.SelectedFlags.EnableOptionalTools)
	if len(catalog.SelectedFlags.Toolsets) > 0 {
		fmt.Fprintf(&b, "- toolsets: `%s`\n", strings.Join(catalog.SelectedFlags.Toolsets, ", "))
	}
	if len(catalog.SelectedFlags.EnableTools) > 0 {
		fmt.Fprintf(&b, "- enableTools: `%s`\n", strings.Join(catalog.SelectedFlags.EnableTools, ", "))
	}
	if len(catalog.SelectedFlags.DisableTools) > 0 {
		fmt.Fprintf(&b, "- disableTools: `%s`\n", strings.Join(catalog.SelectedFlags.DisableTools, ", "))
	}
	b.WriteString("\n## Enabled Tools\n")
	for _, tool := range catalog.Tools {
		fmt.Fprintf(&b, "- `%s`: %s\n", tool.Name, tool.Description)
	}
	b.WriteString("\n## Built-in Toolsets\n")
	for _, toolset := range catalog.Toolsets {
		fmt.Fprintf(&b, "- `%s`: %s\n", toolset.Name, toolset.Description)
	}
	return b.String()
}

func renderWorkflowMarkdown() string {
	return `# Recommended Workflows

## Dashboard investigation
- Start with ` + "`get_dashboard_summary`" + ` or ` + "`get_dashboard_by_uid`" + `.
- Read panel queries before interpreting charts.
- Generate a deeplink when the user needs a direct dashboard or panel URL.

## Datasource investigation
- Resolve the datasource first.
- For Prometheus, list metrics or labels before writing PromQL.
- For Loki, inspect label names or stats before broad log pulls.
- For ClickHouse, list tables and describe schema before writing SQL.

## Alert investigation
- Determine whether legacy or unified alerting is available.
- Check current rule state before reading history.
- Use firing-alert or state-history tools only after confirming the rule namespace and identity.

## Log investigation
- Use datasource-specific discovery first.
- Prefer narrow time ranges and focused selectors.
- Only escalate to cross-backend search when the datasource is unclear.`
}

func mustJSON(v any) string {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(out)
}

func mustToolset(name string) grafanatools.Toolset {
	toolset, ok := grafanatools.ToolsetByName(name)
	if !ok {
		panic("unknown toolset: " + name)
	}
	return toolset
}
