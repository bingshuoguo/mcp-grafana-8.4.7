package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	v84annotations "github.com/grafana/grafana-openapi-client-go/client/annotations"
	v84dashboards "github.com/grafana/grafana-openapi-client-go/client/dashboards"
	v84datasources "github.com/grafana/grafana-openapi-client-go/client/datasources"
	v84folders "github.com/grafana/grafana-openapi-client-go/client/folders"
	v84health "github.com/grafana/grafana-openapi-client-go/client/health"
	v84org "github.com/grafana/grafana-openapi-client-go/client/org"
	v84search "github.com/grafana/grafana-openapi-client-go/client/search"
	v84signedin "github.com/grafana/grafana-openapi-client-go/client/signed_in_user"
	v84teams "github.com/grafana/grafana-openapi-client-go/client/teams"
	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
)

const orgIDHeader = "X-Grafana-Org-Id"

// DatasourceRef identifies a datasource using ID-first semantics.
type DatasourceRef struct {
	ID   *int64 `json:"id,omitempty" jsonschema:"description=Datasource numeric ID"`
	UID  string `json:"uid,omitempty" jsonschema:"description=Datasource UID"`
	Name string `json:"name,omitempty" jsonschema:"description=Datasource name"`
}

type grafanaClient struct {
	Annotations  v84annotations.ClientService
	Dashboards   v84dashboards.ClientService
	Datasources  v84datasources.ClientService
	Folders      v84folders.ClientService
	Health       v84health.ClientService
	Org          v84org.ClientService
	Search       v84search.ClientService
	SignedInUser v84signedin.ClientService
	Teams        v84teams.ClientService
}

func getGrafanaClient(ctx context.Context) (*grafanaClient, error) {
	httpClient, cfg, err := newAPIHTTPClient(ctx)
	if err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse grafana url: %w", err)
	}

	schemes := []string{"https"}
	if parsedURL.Scheme == "http" {
		schemes = []string{"http"}
	}

	transport := httptransport.NewWithClient(parsedURL.Host, makeAPIBasePath(parsedURL.Path), schemes, httpClient)
	transport.DefaultAuthentication = newAuthInfoWriter(cfg)

	return &grafanaClient{
		Annotations:  v84annotations.New(transport, strfmt.Default),
		Dashboards:   v84dashboards.New(transport, strfmt.Default),
		Datasources:  v84datasources.New(transport, strfmt.Default),
		Folders:      v84folders.New(transport, strfmt.Default),
		Health:       v84health.New(transport, strfmt.Default),
		Org:          v84org.New(transport, strfmt.Default),
		Search:       v84search.New(transport, strfmt.Default),
		SignedInUser: v84signedin.New(transport, strfmt.Default),
		Teams:        v84teams.New(transport, strfmt.Default),
	}, nil
}

func makeAPIBasePath(path string) string {
	if path == "" || path == "/" {
		return "/api"
	}
	return strings.TrimRight(path, "/") + "/api"
}

func newAuthInfoWriter(cfg mcpgrafana.GrafanaConfig) runtime.ClientAuthInfoWriter {
	return runtime.ClientAuthInfoWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		if cfg.AccessToken != "" && cfg.IDToken != "" {
			if err := req.SetHeaderParam("X-Access-Token", cfg.AccessToken); err != nil {
				return err
			}
			if err := req.SetHeaderParam("X-Grafana-Id", cfg.IDToken); err != nil {
				return err
			}
		} else if cfg.APIKey != "" {
			if err := req.SetHeaderParam("Authorization", "Bearer "+cfg.APIKey); err != nil {
				return err
			}
		} else if cfg.BasicAuth != nil {
			password, _ := cfg.BasicAuth.Password()
			token := base64.StdEncoding.EncodeToString([]byte(cfg.BasicAuth.Username() + ":" + password))
			if err := req.SetHeaderParam("Authorization", "Basic "+token); err != nil {
				return err
			}
		}

		if cfg.OrgID > 0 {
			if err := req.SetHeaderParam(orgIDHeader, fmt.Sprintf("%d", cfg.OrgID)); err != nil {
				return err
			}
		}
		return nil
	})
}

func getGrafanaConfig(ctx context.Context) (mcpgrafana.GrafanaConfig, error) {
	cfg := mcpgrafana.GrafanaConfigFromContext(ctx)
	if strings.TrimSpace(cfg.URL) == "" {
		return cfg, &mcpgrafana.HardError{Err: fmt.Errorf("grafana url is not configured")}
	}
	return cfg, nil
}

func newAPIHTTPClient(ctx context.Context) (*http.Client, mcpgrafana.GrafanaConfig, error) {
	cfg, err := getGrafanaConfig(ctx)
	if err != nil {
		return nil, cfg, err
	}

	rt, err := mcpgrafana.BuildTransport(&cfg, nil)
	if err != nil {
		return nil, cfg, fmt.Errorf("build HTTP transport: %w", err)
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = mcpgrafana.DefaultGrafanaClientTimeout
	}

	client := &http.Client{
		Transport: mcpgrafana.NewUserAgentTransport(rt),
		Timeout:   timeout,
	}

	return client, cfg, nil
}

func makeAPIURL(cfg mcpgrafana.GrafanaConfig, path string, query url.Values) (string, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	fullURL, err := url.Parse(strings.TrimRight(cfg.URL, "/") + "/api" + path)
	if err != nil {
		return "", fmt.Errorf("parse API url: %w", err)
	}
	if len(query) > 0 {
		fullURL.RawQuery = query.Encode()
	}
	return fullURL.String(), nil
}

func applyAuthHeaders(req *http.Request, cfg mcpgrafana.GrafanaConfig) {
	if cfg.AccessToken != "" && cfg.IDToken != "" {
		req.Header.Set("X-Access-Token", cfg.AccessToken)
		req.Header.Set("X-Grafana-Id", cfg.IDToken)
	} else if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	} else if cfg.BasicAuth != nil {
		password, _ := cfg.BasicAuth.Password()
		req.SetBasicAuth(cfg.BasicAuth.Username(), password)
	}

	if cfg.OrgID > 0 {
		req.Header.Set(orgIDHeader, fmt.Sprintf("%d", cfg.OrgID))
	}
}

func doAPIRequest(ctx context.Context, method, path string, query url.Values, body any) ([]byte, int, error) {
	httpClient, cfg, err := newAPIHTTPClient(ctx)
	if err != nil {
		return nil, 0, err
	}

	fullURL, err := makeAPIURL(cfg, path, query)
	if err != nil {
		return nil, 0, err
	}

	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	applyAuthHeaders(req, cfg)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return respBody, resp.StatusCode, fmt.Errorf("%s %s failed with status %d", method, path, resp.StatusCode)
	}

	return respBody, resp.StatusCode, nil
}

func withGrafanaTimeout(ctx context.Context, timeout time.Duration) context.Context {
	if timeout <= 0 {
		return ctx
	}
	cfg := mcpgrafana.GrafanaConfigFromContext(ctx)
	if cfg.Timeout >= timeout {
		return ctx
	}
	cfg.Timeout = timeout
	return mcpgrafana.WithGrafanaConfig(ctx, cfg)
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}

func decodeAPIResponse[T any](ctx context.Context, method, path string, query url.Values, body any) (T, error) {
	var out T
	respBody, _, err := doAPIRequest(ctx, method, path, query, body)
	if err != nil {
		return out, err
	}

	if len(respBody) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return out, fmt.Errorf("decode API response: %w", err)
	}
	return out, nil
}

// newAPIError creates a normalized APIError from an HTTP error response.
func newAPIError(statusCode int, message string, upstream map[string]any) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Message:    message,
		Upstream:   upstream,
	}
}

// wrapAPIError attempts to parse an HTTP error body into APIError.
// If parsing fails, it creates a generic APIError with the raw message.
func wrapAPIError(statusCode int, respBody []byte, fallbackMsg string) *APIError {
	if len(respBody) > 0 {
		var upstream map[string]any
		if err := json.Unmarshal(respBody, &upstream); err == nil {
			msg := fallbackMsg
			if m, ok := upstream["message"].(string); ok && m != "" {
				msg = m
			}
			return newAPIError(statusCode, msg, upstream)
		}
	}
	return newAPIError(statusCode, fallbackMsg, nil)
}

// openAPICodeError is the interface for OpenAPI client error responses.
type openAPICodeError interface {
	Code() int
	Error() string
}

// wrapOpenAPIError converts an OpenAPI client error into a normalized APIError.
// If the error is not from the OpenAPI client, it wraps it with a 0 status code.
// HardErrors (config/auth) are returned as-is to preserve their semantics.
func wrapOpenAPIError(err error) error {
	if err == nil {
		return nil
	}
	var hardErr *mcpgrafana.HardError
	if errors.As(err, &hardErr) {
		return err
	}
	var codeErr openAPICodeError
	if errors.As(err, &codeErr) {
		return newAPIError(codeErr.Code(), codeErr.Error(), nil)
	}
	return newAPIError(0, err.Error(), nil)
}

// wrapRawAPIError wraps an error from doAPIRequest into an APIError.
// HardErrors (config/auth) are returned as-is to preserve their semantics.
func wrapRawAPIError(statusCode int, respBody []byte, err error) error {
	if err == nil {
		return nil
	}
	var hardErr *mcpgrafana.HardError
	if errors.As(err, &hardErr) {
		return err
	}
	return wrapAPIError(statusCode, respBody, err.Error())
}
