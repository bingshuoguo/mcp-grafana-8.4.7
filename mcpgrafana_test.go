//go:build unit
// +build unit

package mcpgrafana

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-openapi/runtime/client"
	grafana_client "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractGrafanaInfoFromHeaders(t *testing.T) {
	t.Run("no headers, no env", func(t *testing.T) {
		// Explicitly clear environment variables to ensure test isolation
		t.Setenv("GRAFANA_URL", "")
		t.Setenv("GRAFANA_API_KEY", "")
		t.Setenv("GRAFANA_SERVICE_ACCOUNT_TOKEN", "")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, defaultGrafanaURL, config.URL)
		assert.Equal(t, "", config.APIKey)
		assert.Nil(t, config.BasicAuth)
	})

	t.Run("no headers, with env", func(t *testing.T) {
		t.Setenv("GRAFANA_URL", "http://my-test-url.grafana.com")
		t.Setenv("GRAFANA_API_KEY", "my-test-api-key")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, "http://my-test-url.grafana.com", config.URL)
		assert.Equal(t, "my-test-api-key", config.APIKey)
	})

	t.Run("no headers, with service account token", func(t *testing.T) {
		t.Setenv("GRAFANA_URL", "http://my-test-url.grafana.com")
		t.Setenv("GRAFANA_SERVICE_ACCOUNT_TOKEN", "my-service-account-token")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, "http://my-test-url.grafana.com", config.URL)
		assert.Equal(t, "my-service-account-token", config.APIKey)
	})

	t.Run("no headers, service account token takes precedence over api key", func(t *testing.T) {
		t.Setenv("GRAFANA_URL", "http://my-test-url.grafana.com")
		t.Setenv("GRAFANA_API_KEY", "my-deprecated-api-key")
		t.Setenv("GRAFANA_SERVICE_ACCOUNT_TOKEN", "my-service-account-token")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, "http://my-test-url.grafana.com", config.URL)
		assert.Equal(t, "my-service-account-token", config.APIKey)
	})

	t.Run("with headers, no env", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		req.Header.Set(grafanaURLHeader, "http://my-test-url.grafana.com")
		req.Header.Set(grafanaAPIKeyHeader, "my-test-api-key")
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, "http://my-test-url.grafana.com", config.URL)
		assert.Equal(t, "my-test-api-key", config.APIKey)
	})

	t.Run("with headers, with env", func(t *testing.T) {
		// Env vars should be ignored if headers are present.
		t.Setenv("GRAFANA_URL", "will-not-be-used")
		t.Setenv("GRAFANA_API_KEY", "will-not-be-used")
		t.Setenv("GRAFANA_SERVICE_ACCOUNT_TOKEN", "will-not-be-used")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		req.Header.Set(grafanaURLHeader, "http://my-test-url.grafana.com")
		req.Header.Set(grafanaAPIKeyHeader, "my-test-api-key")
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, "http://my-test-url.grafana.com", config.URL)
		assert.Equal(t, "my-test-api-key", config.APIKey)
	})

	t.Run("no headers, with env", func(t *testing.T) {
		t.Setenv("GRAFANA_USERNAME", "foo")
		t.Setenv("GRAFANA_PASSWORD", "bar")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, "foo", config.BasicAuth.Username())
		password, _ := config.BasicAuth.Password()
		assert.Equal(t, "bar", password)
	})

	t.Run("user auth with headers, no env", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://example.com", nil)
		req.SetBasicAuth("foo", "bar")
		require.NoError(t, err)
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, "foo", config.BasicAuth.Username())
		password, _ := config.BasicAuth.Password()
		assert.Equal(t, "bar", password)
	})

	t.Run("user auth with headers, with env", func(t *testing.T) {
		t.Setenv("GRAFANA_USERNAME", "will-not-be-used")
		t.Setenv("GRAFANA_PASSWORD", "will-not-be-used")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		req.SetBasicAuth("foo", "bar")
		require.NoError(t, err)
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, "foo", config.BasicAuth.Username())
		password, _ := config.BasicAuth.Password()
		assert.Equal(t, "bar", password)
	})

	t.Run("orgID from env", func(t *testing.T) {
		t.Setenv("GRAFANA_ORG_ID", "123")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, int64(123), config.OrgID)
	})

	t.Run("orgID from header", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		req.Header.Set("X-Grafana-Org-Id", "456")
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, int64(456), config.OrgID)
	})

	t.Run("orgID header takes precedence over env", func(t *testing.T) {
		t.Setenv("GRAFANA_ORG_ID", "123")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		req.Header.Set("X-Grafana-Org-Id", "456")
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, int64(456), config.OrgID)
	})

	t.Run("invalid orgID from env ignored", func(t *testing.T) {
		t.Setenv("GRAFANA_ORG_ID", "not-a-number")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, int64(0), config.OrgID)
	})

	t.Run("invalid orgID from header ignored", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		req.Header.Set("X-Grafana-Org-Id", "invalid")
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, int64(0), config.OrgID)
	})
}

func TestExtractGrafanaClientPath(t *testing.T) {
	t.Run("no custom path", func(t *testing.T) {
		t.Setenv("GRAFANA_URL", "http://my-test-url.grafana.com/")
		ctx := ExtractGrafanaClientFromEnv(context.Background())

		c := GrafanaClientFromContext(ctx)
		require.NotNil(t, c)
		rt := c.Transport.(*client.Runtime)
		assert.Equal(t, "/api", rt.BasePath)
	})

	t.Run("custom path", func(t *testing.T) {
		t.Setenv("GRAFANA_URL", "http://my-test-url.grafana.com/grafana")
		ctx := ExtractGrafanaClientFromEnv(context.Background())

		c := GrafanaClientFromContext(ctx)
		require.NotNil(t, c)
		rt := c.Transport.(*client.Runtime)
		assert.Equal(t, "/grafana/api", rt.BasePath)
	})

	t.Run("custom path, trailing slash", func(t *testing.T) {
		t.Setenv("GRAFANA_URL", "http://my-test-url.grafana.com/grafana/")
		ctx := ExtractGrafanaClientFromEnv(context.Background())

		c := GrafanaClientFromContext(ctx)
		require.NotNil(t, c)
		rt := c.Transport.(*client.Runtime)
		assert.Equal(t, "/grafana/api", rt.BasePath)
	})
}

// minURL is a helper struct representing what we can extract from a constructed
// Grafana client.
type minURL struct {
	host, basePath string
}

// minURLFromClient extracts some minimal amount of URL info from a Grafana client.
func minURLFromClient(c *grafana_client.GrafanaHTTPAPI) minURL {
	rt := c.Transport.(*client.Runtime)
	return minURL{rt.Host, rt.BasePath}
}

func TestExtractGrafanaClientFromHeaders(t *testing.T) {
	t.Run("no headers, no env", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		ctx := ExtractGrafanaClientFromHeaders(context.Background(), req)
		c := GrafanaClientFromContext(ctx)
		url := minURLFromClient(c)
		assert.Equal(t, "localhost:3000", url.host)
		assert.Equal(t, "/api", url.basePath)
	})

	t.Run("no headers, with env", func(t *testing.T) {
		t.Setenv("GRAFANA_URL", "http://my-test-url.grafana.com")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		ctx := ExtractGrafanaClientFromHeaders(context.Background(), req)
		c := GrafanaClientFromContext(ctx)
		url := minURLFromClient(c)
		assert.Equal(t, "my-test-url.grafana.com", url.host)
		assert.Equal(t, "/api", url.basePath)
	})

	t.Run("with headers, no env", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		req.Header.Set(grafanaURLHeader, "http://my-test-url.grafana.com")
		ctx := ExtractGrafanaClientFromHeaders(context.Background(), req)
		c := GrafanaClientFromContext(ctx)
		url := minURLFromClient(c)
		assert.Equal(t, "my-test-url.grafana.com", url.host)
		assert.Equal(t, "/api", url.basePath)
	})

	t.Run("with headers, with env", func(t *testing.T) {
		// Env vars should be ignored if headers are present.
		t.Setenv("GRAFANA_URL", "will-not-be-used")

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)
		req.Header.Set(grafanaURLHeader, "http://my-test-url.grafana.com")
		ctx := ExtractGrafanaClientFromHeaders(context.Background(), req)
		c := GrafanaClientFromContext(ctx)
		url := minURLFromClient(c)
		assert.Equal(t, "my-test-url.grafana.com", url.host)
		assert.Equal(t, "/api", url.basePath)
	})
}

func TestHTTPTracingConfiguration(t *testing.T) {
	t.Run("HTTP tracing always enabled for context propagation", func(t *testing.T) {
		// Create context (HTTP tracing always enabled)
		config := GrafanaConfig{}
		ctx := WithGrafanaConfig(context.Background(), config)

		// Create Grafana client
		client := NewGrafanaClient(ctx, "http://localhost:3000", "test-api-key", nil, 0)
		require.NotNil(t, client)

		// Verify the client was created successfully (should not panic)
		assert.NotNil(t, client.Transport)
	})

	t.Run("tracing works gracefully without OpenTelemetry configured", func(t *testing.T) {
		// No OpenTelemetry tracer provider configured

		// Create context (tracing always enabled for context propagation)
		config := GrafanaConfig{}
		ctx := WithGrafanaConfig(context.Background(), config)

		// Create Grafana client (should not panic even without OTEL configured)
		client := NewGrafanaClient(ctx, "http://localhost:3000", "test-api-key", nil, 0)
		require.NotNil(t, client)

		// Verify the client was created successfully
		assert.NotNil(t, client.Transport)
	})
}

func TestExtraHeadersFromEnv(t *testing.T) {
	t.Run("empty env returns nil", func(t *testing.T) {
		t.Setenv("GRAFANA_EXTRA_HEADERS", "")
		headers := extraHeadersFromEnv()
		assert.Nil(t, headers)
	})

	t.Run("valid JSON", func(t *testing.T) {
		t.Setenv("GRAFANA_EXTRA_HEADERS", `{"X-Custom-Header": "custom-value", "X-Another": "another-value"}`)
		headers := extraHeadersFromEnv()
		assert.Equal(t, map[string]string{
			"X-Custom-Header": "custom-value",
			"X-Another":       "another-value",
		}, headers)
	})

	t.Run("invalid JSON returns nil", func(t *testing.T) {
		t.Setenv("GRAFANA_EXTRA_HEADERS", "not-json")
		headers := extraHeadersFromEnv()
		assert.Nil(t, headers)
	})

	t.Run("empty object", func(t *testing.T) {
		t.Setenv("GRAFANA_EXTRA_HEADERS", "{}")
		headers := extraHeadersFromEnv()
		assert.Equal(t, map[string]string{}, headers)
	})
}

func TestExtraHeadersRoundTripper(t *testing.T) {
	t.Run("adds headers to request", func(t *testing.T) {
		var capturedReq *http.Request
		mockRT := &extraHeadersMockRT{
			fn: func(req *http.Request) (*http.Response, error) {
				capturedReq = req
				return &http.Response{StatusCode: 200}, nil
			},
		}

		rt := NewExtraHeadersRoundTripper(mockRT, map[string]string{
			"X-Custom":  "value1",
			"X-Another": "value2",
		})

		req, _ := http.NewRequest("GET", "http://example.com", nil)
		_, err := rt.RoundTrip(req)
		require.NoError(t, err)

		assert.Equal(t, "value1", capturedReq.Header.Get("X-Custom"))
		assert.Equal(t, "value2", capturedReq.Header.Get("X-Another"))
	})

	t.Run("does not modify original request", func(t *testing.T) {
		mockRT := &extraHeadersMockRT{
			fn: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200}, nil
			},
		}

		rt := NewExtraHeadersRoundTripper(mockRT, map[string]string{
			"X-Custom": "value",
		})

		req, _ := http.NewRequest("GET", "http://example.com", nil)
		_, err := rt.RoundTrip(req)
		require.NoError(t, err)

		assert.Equal(t, "", req.Header.Get("X-Custom"))
	})

	t.Run("nil transport uses default", func(t *testing.T) {
		rt := NewExtraHeadersRoundTripper(nil, map[string]string{})
		assert.NotNil(t, rt.underlying)
	})
}

type extraHeadersMockRT struct {
	fn func(*http.Request) (*http.Response, error)
}

func (m *extraHeadersMockRT) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

func TestExtractGrafanaInfoWithExtraHeaders(t *testing.T) {
	t.Run("extra headers from env in ExtractGrafanaInfoFromEnv", func(t *testing.T) {
		t.Setenv("GRAFANA_EXTRA_HEADERS", `{"X-Tenant-ID": "tenant-123"}`)
		ctx := ExtractGrafanaInfoFromEnv(context.Background())
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, map[string]string{"X-Tenant-ID": "tenant-123"}, config.ExtraHeaders)
	})

	t.Run("extra headers from env in ExtractGrafanaInfoFromHeaders", func(t *testing.T) {
		t.Setenv("GRAFANA_EXTRA_HEADERS", `{"X-Tenant-ID": "tenant-456"}`)
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		ctx := ExtractGrafanaInfoFromHeaders(context.Background(), req)
		config := GrafanaConfigFromContext(ctx)
		assert.Equal(t, map[string]string{"X-Tenant-ID": "tenant-456"}, config.ExtraHeaders)
	})
}
