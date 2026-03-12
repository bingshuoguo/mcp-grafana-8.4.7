package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

// ─── Ruler API types ──────────────────────────────────────────────────────────

// RulerRule is a single Grafana-managed alert rule in the Ruler API format.
type RulerRule struct {
	UID          string            `json:"uid,omitempty"`
	Title        string            `json:"title"`
	Condition    string            `json:"condition"`
	Data         []json.RawMessage `json:"data"`
	For          string            `json:"for,omitempty"`
	NoDataState  string            `json:"noDataState,omitempty"`
	ExecErrState string            `json:"execErrState,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	IsPaused     bool              `json:"isPaused,omitempty"`
}

// RulerRuleGroup is a named group of alert rules within a namespace (folder).
type RulerRuleGroup struct {
	Name     string      `json:"name"`
	Interval string      `json:"interval,omitempty"`
	Rules    []RulerRule `json:"rules"`
}

// rulerRulesResponse is the response from GET /api/ruler/grafana/api/v1/rules:
// map[namespace (folder title)][]RulerRuleGroup
type rulerRulesResponse map[string][]RulerRuleGroup

// AlertRuleSummary is a flat summary of a single alert rule with its context.
type AlertRuleSummary struct {
	UID          string            `json:"uid"`
	Title        string            `json:"title"`
	Namespace    string            `json:"namespace"`
	Group        string            `json:"group"`
	For          string            `json:"for,omitempty"`
	NoDataState  string            `json:"noDataState,omitempty"`
	ExecErrState string            `json:"execErrState,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	IsPaused     bool              `json:"isPaused,omitempty"`
}

// flattenRulerRules converts the nested Ruler API response to a flat list.
func flattenRulerRules(resp rulerRulesResponse) []AlertRuleSummary {
	var out []AlertRuleSummary
	for ns, groups := range resp {
		for _, g := range groups {
			for _, r := range g.Rules {
				out = append(out, AlertRuleSummary{
					UID:          r.UID,
					Title:        r.Title,
					Namespace:    ns,
					Group:        g.Name,
					For:          r.For,
					NoDataState:  r.NoDataState,
					ExecErrState: r.ExecErrState,
					Labels:       r.Labels,
					Annotations:  r.Annotations,
					IsPaused:     r.IsPaused,
				})
			}
		}
	}
	return out
}

// getRulerRules fetches all rules or rules filtered by namespace.
func getRulerRules(ctx context.Context, namespace string) (rulerRulesResponse, error) {
	path := "/ruler/grafana/api/v1/rules"
	if namespace != "" {
		path += "/" + url.PathEscape(namespace)
	}
	body, statusCode, err := doAPIRequest(ctx, "GET", path, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("ruler API: %w", wrapRawAPIError(statusCode, body, err))
	}
	var resp rulerRulesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode ruler rules: %w", err)
	}
	return resp, nil
}

// ─── list_alert_rules ─────────────────────────────────────────────────────────

type ListAlertRulesRequest struct {
	Namespace string `json:"namespace,omitempty" jsonschema:"description=Filter by folder/namespace (optional; returns all if omitted)"`
}

func listAlertRules(ctx context.Context, args ListAlertRulesRequest) ([]AlertRuleSummary, error) {
	resp, err := getRulerRules(ctx, args.Namespace)
	if err != nil {
		return nil, err
	}
	rules := flattenRulerRules(resp)
	if rules == nil {
		rules = []AlertRuleSummary{}
	}
	return rules, nil
}

var ListAlertRulesTool = mcpgrafana.MustTool(
	"list_alert_rules",
	`List Unified Alerting rules from the Ruler API (Grafana 8.4.7+).

Returns a flat list of alert rules with their namespace (folder) and group context.
Filter by namespace to narrow results. Requires Unified Alerting to be enabled.`,
	listAlertRules,
	mcp.WithTitleAnnotation("List alert rules"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// ─── get_alert_rule_by_uid ────────────────────────────────────────────────────

type GetAlertRuleByUIDRequest struct {
	UID string `json:"uid" jsonschema:"required,description=UID of the alert rule to retrieve"`
}

type AlertRuleDetail struct {
	AlertRuleSummary
	Condition string            `json:"condition"`
	Data      []json.RawMessage `json:"data"`
}

func getAlertRuleByUID(ctx context.Context, args GetAlertRuleByUIDRequest) (*AlertRuleDetail, error) {
	if strings.TrimSpace(args.UID) == "" {
		return nil, fmt.Errorf("uid is required")
	}
	resp, err := getRulerRules(ctx, "")
	if err != nil {
		return nil, err
	}
	for ns, groups := range resp {
		for _, g := range groups {
			for _, r := range g.Rules {
				if r.UID == args.UID {
					return &AlertRuleDetail{
						AlertRuleSummary: AlertRuleSummary{
							UID:          r.UID,
							Title:        r.Title,
							Namespace:    ns,
							Group:        g.Name,
							For:          r.For,
							NoDataState:  r.NoDataState,
							ExecErrState: r.ExecErrState,
							Labels:       r.Labels,
							Annotations:  r.Annotations,
							IsPaused:     r.IsPaused,
						},
						Condition: r.Condition,
						Data:      r.Data,
					}, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("alert rule with uid %q not found", args.UID)
}

var GetAlertRuleByUIDTool = mcpgrafana.MustTool(
	"get_alert_rule_by_uid",
	"Get a single Unified Alerting rule by its UID, including full query data and conditions.",
	getAlertRuleByUID,
	mcp.WithTitleAnnotation("Get alert rule by UID"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// ─── create_alert_rule ────────────────────────────────────────────────────────

type CreateAlertRuleRequest struct {
	Namespace string         `json:"namespace" jsonschema:"required,description=Folder title (namespace) to create the rule group in"`
	Group     RulerRuleGroup `json:"group" jsonschema:"required,description=Rule group definition (name\\, optional interval\\, and list of rules)"`
}

func createAlertRule(ctx context.Context, args CreateAlertRuleRequest) (*RulerRuleGroup, error) {
	if strings.TrimSpace(args.Namespace) == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if strings.TrimSpace(args.Group.Name) == "" {
		return nil, fmt.Errorf("group.name is required")
	}
	path := "/ruler/grafana/api/v1/rules/" + url.PathEscape(args.Namespace)
	body, statusCode, err := doAPIRequest(ctx, "POST", path, nil, args.Group)
	if err != nil {
		return nil, fmt.Errorf("create alert rule: %w", wrapRawAPIError(statusCode, body, err))
	}
	return &args.Group, nil
}

var CreateAlertRuleTool = mcpgrafana.MustTool(
	"create_alert_rule",
	`Create or replace a Unified Alerting rule group via the Ruler API.

POST replaces the entire group if a group with the same name already exists in the namespace.
Use get_alert_rule_by_uid + update_alert_rule to modify an existing rule without replacing others in the group.`,
	createAlertRule,
	mcp.WithTitleAnnotation("Create alert rule group"),
)

// ─── update_alert_rule ────────────────────────────────────────────────────────

type UpdateAlertRuleRequest struct {
	Namespace string         `json:"namespace" jsonschema:"required,description=Folder title (namespace) containing the rule group"`
	Group     RulerRuleGroup `json:"group" jsonschema:"required,description=Updated rule group definition (must include all rules; replaces the group entirely)"`
}

func updateAlertRule(ctx context.Context, args UpdateAlertRuleRequest) (*RulerRuleGroup, error) {
	if strings.TrimSpace(args.Namespace) == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if strings.TrimSpace(args.Group.Name) == "" {
		return nil, fmt.Errorf("group.name is required")
	}
	path := "/ruler/grafana/api/v1/rules/" + url.PathEscape(args.Namespace)
	body, statusCode, err := doAPIRequest(ctx, "POST", path, nil, args.Group)
	if err != nil {
		return nil, fmt.Errorf("update alert rule: %w", wrapRawAPIError(statusCode, body, err))
	}
	return &args.Group, nil
}

var UpdateAlertRuleTool = mcpgrafana.MustTool(
	"update_alert_rule",
	`Replace a Unified Alerting rule group (full update via Ruler API).

Replaces the entire named group. Workflow: get_alert_rule_by_uid → modify → update_alert_rule.`,
	updateAlertRule,
	mcp.WithTitleAnnotation("Update alert rule group"),
)

// ─── delete_alert_rule ────────────────────────────────────────────────────────

type DeleteAlertRuleRequest struct {
	Namespace string `json:"namespace" jsonschema:"required,description=Folder title (namespace) containing the rule group"`
	Group     string `json:"group" jsonschema:"required,description=Rule group name to delete"`
}

type DeleteAlertRuleResult struct {
	Deleted   bool   `json:"deleted"`
	Namespace string `json:"namespace"`
	Group     string `json:"group"`
}

func deleteAlertRule(ctx context.Context, args DeleteAlertRuleRequest) (*DeleteAlertRuleResult, error) {
	if strings.TrimSpace(args.Namespace) == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if strings.TrimSpace(args.Group) == "" {
		return nil, fmt.Errorf("group is required")
	}
	path := fmt.Sprintf("/ruler/grafana/api/v1/rules/%s/%s",
		url.PathEscape(args.Namespace), url.PathEscape(args.Group))
	body, statusCode, err := doAPIRequest(ctx, "DELETE", path, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("delete alert rule: %w", wrapRawAPIError(statusCode, body, err))
	}
	return &DeleteAlertRuleResult{Deleted: true, Namespace: args.Namespace, Group: args.Group}, nil
}

var DeleteAlertRuleTool = mcpgrafana.MustTool(
	"delete_alert_rule",
	"Delete an entire Unified Alerting rule group from a namespace (folder). All rules in the group are removed.",
	deleteAlertRule,
	mcp.WithTitleAnnotation("Delete alert rule group"),
)

// ─── list_contact_points ──────────────────────────────────────────────────────

type ContactPointReceiver struct {
	Name    string              `json:"name"`
	Configs []ContactPointEntry `json:"configs,omitempty"`
}

type ContactPointEntry struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Settings map[string]any `json:"settings,omitempty"`
}

// alertmanagerReceiver is the raw Alertmanager API response shape.
type alertmanagerReceiver struct {
	Name    string `json:"name"`
	Configs []struct {
		Name     string         `json:"name"`
		Type     string         `json:"type"`
		Settings map[string]any `json:"settings,omitempty"`
	} `json:"grafana_managed_receiver_configs,omitempty"`
}

type ListContactPointsRequest struct{}

func listContactPoints(ctx context.Context, _ ListContactPointsRequest) ([]ContactPointReceiver, error) {
	body, statusCode, err := doAPIRequest(ctx, "GET", "/alertmanager/grafana/api/v2/receivers", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("list contact points: %w", wrapRawAPIError(statusCode, body, err))
	}
	var raw []alertmanagerReceiver
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode contact points: %w", err)
	}
	out := make([]ContactPointReceiver, 0, len(raw))
	for _, r := range raw {
		cp := ContactPointReceiver{Name: r.Name}
		for _, c := range r.Configs {
			cp.Configs = append(cp.Configs, ContactPointEntry{
				Name:     c.Name,
				Type:     c.Type,
				Settings: c.Settings,
			})
		}
		out = append(out, cp)
	}
	return out, nil
}

var ListContactPointsTool = mcpgrafana.MustTool(
	"list_contact_points",
	"List Alertmanager contact points (notification receivers). Requires Unified Alerting to be enabled.",
	listContactPoints,
	mcp.WithTitleAnnotation("List contact points"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// ─── get_firing_alerts ────────────────────────────────────────────────────────

type GetFiringAlertsRequest struct {
	Filter    []string `json:"filter,omitempty" jsonschema:"description=Label matchers (e.g. [\\\"alertname=HighCPU\\\", \\\"severity=critical\\\"])"`
	Silenced  *bool    `json:"silenced,omitempty" jsonschema:"description=Include silenced alerts"`
	Inhibited *bool    `json:"inhibited,omitempty" jsonschema:"description=Include inhibited alerts"`
	Active    *bool    `json:"active,omitempty" jsonschema:"description=Include active (unfired) alerts"`
}

type FiringAlertStatus struct {
	State       string   `json:"state"`
	SilencedBy  []string `json:"silencedBy"`
	InhibitedBy []string `json:"inhibitedBy"`
}

type FiringAlert struct {
	Labels       map[string]string  `json:"labels"`
	Annotations  map[string]string  `json:"annotations"`
	StartsAt     string             `json:"startsAt"`
	EndsAt       string             `json:"endsAt,omitempty"`
	GeneratorURL string             `json:"generatorURL,omitempty"`
	Fingerprint  string             `json:"fingerprint,omitempty"`
	Status       *FiringAlertStatus `json:"status,omitempty"`
}

func getFiringAlerts(ctx context.Context, args GetFiringAlertsRequest) ([]FiringAlert, error) {
	query := url.Values{}
	for _, f := range args.Filter {
		query.Add("filter", f)
	}
	if args.Silenced != nil {
		query.Set("silenced", strconv.FormatBool(*args.Silenced))
	}
	if args.Inhibited != nil {
		query.Set("inhibited", strconv.FormatBool(*args.Inhibited))
	}
	if args.Active != nil {
		query.Set("active", strconv.FormatBool(*args.Active))
	}

	body, statusCode, err := doAPIRequest(ctx, "GET", "/alertmanager/grafana/api/v2/alerts", query, nil)
	if err != nil {
		return nil, fmt.Errorf("get firing alerts: %w", wrapRawAPIError(statusCode, body, err))
	}

	var alerts []FiringAlert
	if err := json.Unmarshal(body, &alerts); err != nil {
		return nil, fmt.Errorf("decode firing alerts: %w", err)
	}
	if alerts == nil {
		alerts = []FiringAlert{}
	}
	return alerts, nil
}

var GetFiringAlertsTool = mcpgrafana.MustTool(
	"get_firing_alerts",
	`Get currently firing alert instances from Alertmanager.

Returns alerts that have transitioned from Pending to Firing state.
Use filter to narrow by label (e.g. ["alertname=HighCPU", "severity=critical"]).
Requires Unified Alerting to be enabled.`,
	getFiringAlerts,
	mcp.WithTitleAnnotation("Get firing alerts"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)

// ─── get_alert_rules_with_state ───────────────────────────────────────────────

type GetAlertRulesWithStateRequest struct {
	State    string `json:"state,omitempty" jsonschema:"description=Filter by rule state: firing\\, pending\\, or inactive"`
	RuleName string `json:"ruleName,omitempty" jsonschema:"description=Filter by rule name (partial match)"`
}

type AlertInstanceWithState struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations,omitempty"`
	State       string            `json:"state"`
	ActiveAt    string            `json:"activeAt,omitempty"`
	Value       string            `json:"value,omitempty"`
}

type AlertRuleWithState struct {
	Name           string                   `json:"name"`
	State          string                   `json:"state"`
	Health         string                   `json:"health,omitempty"`
	LastEvaluation string                   `json:"lastEvaluation,omitempty"`
	EvaluationTime float64                  `json:"evaluationTime,omitempty"`
	Labels         map[string]string        `json:"labels,omitempty"`
	Annotations    map[string]string        `json:"annotations,omitempty"`
	Alerts         []AlertInstanceWithState `json:"alerts,omitempty"`
}

type AlertRuleGroupWithState struct {
	Name      string               `json:"name"`
	Namespace string               `json:"namespace"`
	Rules     []AlertRuleWithState `json:"rules"`
}

type GetAlertRulesWithStateResponse struct {
	Groups []AlertRuleGroupWithState `json:"groups"`
}

// prometheusRulesResponse is the raw Prometheus-compat API response shape.
type prometheusRulesResponse struct {
	Status string `json:"status"`
	Data   struct {
		Groups []struct {
			Name     string  `json:"name"`
			File     string  `json:"file"`
			Interval float64 `json:"interval"`
			Rules    []struct {
				Name           string                   `json:"name"`
				State          string                   `json:"state"`
				Health         string                   `json:"health"`
				LastEvaluation string                   `json:"lastEvaluation"`
				EvaluationTime float64                  `json:"evaluationTime"`
				Labels         map[string]string        `json:"labels"`
				Annotations    map[string]string        `json:"annotations"`
				Alerts         []AlertInstanceWithState `json:"alerts"`
				Type           string                   `json:"type"`
			} `json:"rules"`
		} `json:"groups"`
	} `json:"data"`
}

func getAlertRulesWithState(ctx context.Context, args GetAlertRulesWithStateRequest) (*GetAlertRulesWithStateResponse, error) {
	query := url.Values{}
	query.Set("type", "alert")

	body, statusCode, err := doAPIRequest(ctx, "GET", "/prometheus/grafana/api/v1/rules", query, nil)
	if err != nil {
		return nil, fmt.Errorf("get alert rules with state: %w", wrapRawAPIError(statusCode, body, err))
	}

	var raw prometheusRulesResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode alert rules response: %w", err)
	}

	resp := &GetAlertRulesWithStateResponse{Groups: []AlertRuleGroupWithState{}}
	for _, g := range raw.Data.Groups {
		group := AlertRuleGroupWithState{
			Name:      g.Name,
			Namespace: g.File,
			Rules:     []AlertRuleWithState{},
		}
		for _, r := range g.Rules {
			if args.State != "" && r.State != args.State {
				continue
			}
			if args.RuleName != "" && !strings.Contains(r.Name, args.RuleName) {
				continue
			}
			group.Rules = append(group.Rules, AlertRuleWithState{
				Name:           r.Name,
				State:          r.State,
				Health:         r.Health,
				LastEvaluation: r.LastEvaluation,
				EvaluationTime: r.EvaluationTime,
				Labels:         r.Labels,
				Annotations:    r.Annotations,
				Alerts:         r.Alerts,
			})
		}
		if len(group.Rules) > 0 {
			resp.Groups = append(resp.Groups, group)
		}
	}
	return resp, nil
}

var GetAlertRulesWithStateTool = mcpgrafana.MustTool(
	"get_alert_rules_with_state",
	`Get all alert rules with their current evaluation state via the Prometheus-compat API.

Returns rules organized by group/namespace including state (firing/pending/inactive),
health, last evaluation time, and currently firing alert instances.
Use state or ruleName to filter results. Requires Unified Alerting to be enabled.`,
	getAlertRulesWithState,
	mcp.WithTitleAnnotation("Get alert rules with state"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
)
