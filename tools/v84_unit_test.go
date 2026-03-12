//go:build unit

package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newV84TestContext(server *httptest.Server) context.Context {
	grafanaCfg := mcpgrafana.GrafanaConfig{
		URL:    server.URL,
		APIKey: "test-token",
	}

	return mcpgrafana.WithGrafanaConfig(context.Background(), grafanaCfg)
}

func int64Ptr(v int64) *int64 {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

func TestNewAPIHTTPClientTimeout(t *testing.T) {
	t.Run("uses default timeout when config timeout is zero", func(t *testing.T) {
		ctx := mcpgrafana.WithGrafanaConfig(context.Background(), mcpgrafana.GrafanaConfig{
			URL: "http://localhost:3000",
		})

		client, _, err := newAPIHTTPClient(ctx)
		require.NoError(t, err)
		assert.Equal(t, mcpgrafana.DefaultGrafanaClientTimeout, client.Timeout)
	})

	t.Run("uses explicit timeout when configured", func(t *testing.T) {
		ctx := mcpgrafana.WithGrafanaConfig(context.Background(), mcpgrafana.GrafanaConfig{
			URL:     "http://localhost:3000",
			Timeout: 250 * time.Millisecond,
		})

		client, _, err := newAPIHTTPClient(ctx)
		require.NoError(t, err)
		assert.Equal(t, 250*time.Millisecond, client.Timeout)
	})
}

func TestResolveDatasourceRef(t *testing.T) {
	t.Run("id has priority over uid and name", func(t *testing.T) {
		var requestedID2 bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/datasources":
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"id": 1, "uid": "uid-1", "name": "DS One", "type": "prometheus"},
					{"id": 2, "uid": "uid-2", "name": "DS Two", "type": "loki"},
				})
			case "/api/datasources/2":
				requestedID2 = true
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":   2,
					"uid":  "uid-2",
					"name": "DS Two",
					"type": "loki",
					"url":  "http://loki",
				})
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		ctx := newV84TestContext(server)
		got, err := resolveDatasourceRef(ctx, DatasourceRef{
			ID:   int64Ptr(2),
			UID:  "uid-1",
			Name: "DS One",
		})
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, requestedID2)
		assert.Equal(t, "id", got.ResolvedBy)
		assert.EqualValues(t, 2, got.Datasource.ID)
		assert.Equal(t, "uid-2", got.Datasource.UID)
	})

	t.Run("resolve by uid when id is absent", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/datasources":
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"id": 11, "uid": "uid-11", "name": "Main DS", "type": "prometheus"},
				})
			case "/api/datasources/11":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":   11,
					"uid":  "uid-11",
					"name": "Main DS",
					"type": "prometheus",
				})
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		got, err := resolveDatasourceRef(newV84TestContext(server), DatasourceRef{UID: "uid-11"})
		require.NoError(t, err)
		assert.Equal(t, "uid", got.ResolvedBy)
		assert.EqualValues(t, 11, got.Datasource.ID)
	})

	t.Run("name matching is case insensitive", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/datasources":
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"id": 7, "uid": "uid-7", "name": "Prom Main", "type": "prometheus"},
				})
			case "/api/datasources/7":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":   7,
					"uid":  "uid-7",
					"name": "Prom Main",
					"type": "prometheus",
				})
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		got, err := resolveDatasourceRef(newV84TestContext(server), DatasourceRef{Name: "prom main"})
		require.NoError(t, err)
		assert.Equal(t, "name", got.ResolvedBy)
		assert.Equal(t, "uid-7", got.Datasource.UID)
	})

	t.Run("fallback to list item when get by id fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/datasources":
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{
						"id":        42,
						"uid":       "uid-42",
						"name":      "Fallback DS",
						"type":      "loki",
						"url":       "http://loki",
						"database":  "db",
						"isDefault": true,
					},
				})
			case "/api/datasources/42":
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"boom"}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		got, err := resolveDatasourceRef(newV84TestContext(server), DatasourceRef{ID: int64Ptr(42)})
		require.NoError(t, err)
		assert.EqualValues(t, 42, got.Datasource.ID)
		assert.Equal(t, "uid-42", got.Datasource.UID)
		assert.Equal(t, "Fallback DS", got.Datasource.Name)
		assert.Equal(t, "loki", got.Datasource.Type)
		assert.Equal(t, "http://loki", got.Datasource.URL)
	})

	t.Run("error when no identifier is provided", func(t *testing.T) {
		_, err := resolveDatasourceRef(context.Background(), DatasourceRef{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "one of id, uid, or name is required")
	})

	t.Run("error when datasource is not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/datasources" {
				_ = json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		_, err := resolveDatasourceRef(newV84TestContext(server), DatasourceRef{UID: "missing"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "datasource not found")
	})
}

func TestListDatasources_FilterAndPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "/api/datasources", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "uid": "prom-1", "name": "Prom 1", "type": "prometheus"},
			{"id": 2, "uid": "loki-1", "name": "Loki 1", "type": "loki"},
			{"id": 3, "uid": "prom-2", "name": "Prom 2", "type": "prometheus"},
			{"id": 4, "uid": "tempo-1", "name": "Tempo 1", "type": "tempo"},
		})
	}))
	defer server.Close()

	ctx := newV84TestContext(server)

	t.Run("filter by type and paginate", func(t *testing.T) {
		got, err := listDatasources(ctx, ListDatasourcesRequest{
			Type:   "prom",
			Limit:  int64Ptr(1),
			Offset: int64Ptr(1),
		})
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.EqualValues(t, 2, got.Total)
		assert.Len(t, got.Items, 1)
		assert.Equal(t, "prom-2", got.Items[0].UID)
		assert.False(t, got.HasMore)
	})

	t.Run("offset beyond range returns empty list", func(t *testing.T) {
		got, err := listDatasources(ctx, ListDatasourcesRequest{
			Limit:  int64Ptr(2),
			Offset: int64Ptr(100),
		})
		require.NoError(t, err)
		assert.Len(t, got.Items, 0)
		assert.EqualValues(t, 4, got.Total)
		assert.False(t, got.HasMore)
	})
}

func TestGetDatasourceAndResolveDatasourceReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/datasources":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 9, "uid": "uid-9", "name": "My DS", "type": "prometheus", "url": "http://prom"},
			})
		case "/api/datasources/9":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   9,
				"uid":  "uid-9",
				"name": "My DS",
				"type": "prometheus",
				"url":  "http://prom",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx := newV84TestContext(server)

	ds, err := getDatasource(ctx, GetDatasourceRequest{UID: "uid-9"})
	require.NoError(t, err)
	assert.Equal(t, "uid", ds.ResolvedBy)
	assert.EqualValues(t, 9, ds.Datasource.ID)

	ref, err := resolveDatasourceReference(ctx, ResolveDatasourceRefRequest{UID: "uid-9"})
	require.NoError(t, err)
	assert.EqualValues(t, 9, ref.ID)
	assert.Equal(t, "uid-9", ref.UID)
	assert.Equal(t, "My DS", ref.Name)
	assert.Equal(t, "prometheus", ref.Type)
}

func TestQueryDatasource(t *testing.T) {
	t.Run("validates required fields", func(t *testing.T) {
		_, err := queryDatasource(context.Background(), QueryDatasourceRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from and to are required")

		_, err = queryDatasource(context.Background(), QueryDatasourceRequest{
			From: "now-1h",
			To:   "now",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "queries is required")
	})

	t.Run("injects datasourceId when missing", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/datasources":
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"id": 100, "uid": "uid-100", "name": "Loki Main", "type": "loki"},
				})
			case "/api/datasources/100":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":   100,
					"uid":  "uid-100",
					"name": "Loki Main",
					"type": "loki",
				})
			case "/api/tsdb/query":
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, "now-1h", body["from"])
				assert.Equal(t, "now", body["to"])
				assert.Equal(t, true, body["debug"])

				queries, ok := body["queries"].([]any)
				require.True(t, ok)
				require.Len(t, queries, 1)
				query0, ok := queries[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "A", query0["refId"])
				assert.Equal(t, float64(100), query0["datasourceId"])

				_, _ = w.Write([]byte(`{"results":{"A":{"status":200}},"message":"query ok"}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		ctx := newV84TestContext(server)
		got, err := queryDatasource(ctx, QueryDatasourceRequest{
			From:  "now-1h",
			To:    "now",
			Debug: boolPtr(true),
			Datasource: &DatasourceRef{
				UID: "uid-100",
			},
			Queries: []map[string]any{
				{"refId": "A", "expr": "rate(http_requests_total[5m])"},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.NotEmpty(t, got.Raw)
		assert.Contains(t, got.Responses, "A")
		assert.Equal(t, []string{"query ok"}, got.Hints)
	})

	t.Run("keeps caller provided datasourceId", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/datasources":
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"id": 200, "uid": "uid-200", "name": "Prom Main", "type": "prometheus"},
				})
			case "/api/datasources/200":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":   200,
					"uid":  "uid-200",
					"name": "Prom Main",
					"type": "prometheus",
				})
			case "/api/tsdb/query":
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				queries, ok := body["queries"].([]any)
				require.True(t, ok)
				query0, ok := queries[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, float64(999), query0["datasourceId"])

				_, _ = w.Write([]byte(`{"results":{"A":{"status":200}}}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		got, err := queryDatasource(newV84TestContext(server), QueryDatasourceRequest{
			From: "now-1h",
			To:   "now",
			Datasource: &DatasourceRef{
				UID: "uid-200",
			},
			Queries: []map[string]any{
				{"refId": "A", "datasourceId": 999, "expr": "up"},
			},
		})
		require.NoError(t, err)
		assert.Contains(t, got.Responses, "A")
	})
}

func TestListLegacyAlertsAndNotifications(t *testing.T) {
	t.Run("list legacy alerts builds query and parses response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/alerts", r.URL.Path)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			q := r.URL.Query()
			assert.Equal(t, "11", q.Get("dashboardId"))
			assert.Equal(t, "22", q.Get("panelId"))
			assert.Equal(t, "cpu", q.Get("query"))
			assert.Equal(t, "alerting", q.Get("state"))
			assert.Equal(t, "50", q.Get("limit"))
			assert.ElementsMatch(t, []string{"team-a", "team-b"}, q["dashboardTag"])

			_, _ = w.Write([]byte(`[{"id":1,"name":"CPU High"}]`))
		}))
		defer server.Close()

		got, err := listLegacyAlerts(newV84TestContext(server), ListLegacyAlertsRequest{
			DashboardID:  int64Ptr(11),
			PanelID:      int64Ptr(22),
			Query:        "cpu",
			State:        "alerting",
			Limit:        int64Ptr(50),
			DashboardTag: []string{"team-a", "team-b"},
		})
		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "CPU High", got.Items[0].Name)
	})

	t.Run("invalid json from legacy alerts returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer server.Close()

		_, err := listLegacyAlerts(newV84TestContext(server), ListLegacyAlertsRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode legacy alerts response")
	})

	t.Run("list legacy notification channels uses name filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/alert-notifications", r.URL.Path)
			assert.Equal(t, "pagerduty", r.URL.Query().Get("name"))
			_, _ = w.Write([]byte(`[{"id":2,"name":"pagerduty-main"}]`))
		}))
		defer server.Close()

		got, err := listLegacyNotificationChannels(newV84TestContext(server), ListLegacyNotificationChannelsRequest{
			Name: "pagerduty",
		})
		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "pagerduty-main", got.Items[0].Name)
	})
}

func TestListLegacyAlertsReturnsHardErrorWhenGrafanaURLMissing(t *testing.T) {
	_, err := listLegacyAlerts(context.Background(), ListLegacyAlertsRequest{})
	require.Error(t, err)

	var hardErr *mcpgrafana.HardError
	require.ErrorAs(t, err, &hardErr)
	assert.Contains(t, hardErr.Error(), "grafana url is not configured")
}

// ----- Health / User / Org -----

func TestGetHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "/api/health", r.URL.Path)
		_, _ = w.Write([]byte(`{"commit":"abc123","database":"ok","version":"8.4.7"}`))
	}))
	defer server.Close()

	got, err := getHealth(newV84TestContext(server), GetHealthRequest{})
	require.NoError(t, err)
	assert.NotNil(t, got)
}

func TestGetCurrentUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "/api/user", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":             1,
			"login":          "admin",
			"email":          "admin@grafana.test",
			"name":           "Admin",
			"isGrafanaAdmin": true,
			"orgId":          1,
			"theme":          "dark",
		})
	}))
	defer server.Close()

	got, err := getCurrentUser(newV84TestContext(server), GetCurrentUserRequest{})
	require.NoError(t, err)
	assert.Equal(t, "admin", got.Login)
	assert.Equal(t, "admin@grafana.test", got.Email)
	assert.Equal(t, "Admin", got.Name)
	assert.True(t, got.IsGrafanaAdmin)
	assert.EqualValues(t, 1, got.OrgID)
	assert.Equal(t, "dark", got.Theme)
}

func TestGetCurrentOrg(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "/api/org", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   1,
			"name": "Main Org.",
		})
	}))
	defer server.Close()

	got, err := getCurrentOrg(newV84TestContext(server), GetCurrentOrgRequest{})
	require.NoError(t, err)
	assert.EqualValues(t, 1, got.ID)
	assert.Equal(t, "Main Org.", got.Name)
}

// ----- Search Dashboards -----

func TestSearchDashboards(t *testing.T) {
	t.Run("basic search with pagination", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/search", r.URL.Path)
			assert.Equal(t, "test", r.URL.Query().Get("query"))
			assert.Equal(t, "dash-db", r.URL.Query().Get("type"))
			assert.Equal(t, "2", r.URL.Query().Get("limit"))
			assert.Equal(t, "1", r.URL.Query().Get("page"))

			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":          1,
					"uid":         "abc",
					"title":       "Dashboard 1",
					"type":        "dash-db",
					"url":         "/d/abc/dashboard-1",
					"tags":        []string{"prod"},
					"folderId":    10,
					"folderUid":   "folder-x",
					"folderTitle": "Folder X",
				},
				{
					"id":    2,
					"uid":   "def",
					"title": "Dashboard 2",
					"type":  "dash-db",
					"url":   "/d/def/dashboard-2",
				},
			})
		}))
		defer server.Close()

		got, err := searchDashboards(newV84TestContext(server), SearchDashboardsRequest{
			Query: "test",
			Limit: int64Ptr(2),
		})
		require.NoError(t, err)
		assert.Len(t, got.Items, 2)
		assert.True(t, got.HasMore)
		assert.EqualValues(t, 2, got.Limit)
		assert.EqualValues(t, 1, got.Page)

		assert.Equal(t, "abc", got.Items[0].UID)
		assert.Equal(t, "Dashboard 1", got.Items[0].Title)
		assert.Equal(t, "dash-db", got.Items[0].Type)
		assert.Equal(t, "/d/abc/dashboard-1", got.Items[0].URL)
		assert.Equal(t, []string{"prod"}, got.Items[0].Tags)
		assert.EqualValues(t, 10, got.Items[0].FolderID)
		assert.Equal(t, "folder-x", got.Items[0].FolderUID)
		assert.Equal(t, "Folder X", got.Items[0].FolderTitle)
	})

	t.Run("empty result", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		}))
		defer server.Close()

		got, err := searchDashboards(newV84TestContext(server), SearchDashboardsRequest{})
		require.NoError(t, err)
		assert.Len(t, got.Items, 0)
		assert.False(t, got.HasMore)
	})
}

// ----- Dashboard CRUD -----

func TestGetDashboardByUID(t *testing.T) {
	t.Run("uid is required", func(t *testing.T) {
		_, err := getDashboardByUID(context.Background(), GetDashboardByUIDRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "uid is required")
	})

	t.Run("returns dashboard and meta", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/dashboards/uid/test-uid", r.URL.Path)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"dashboard": map[string]any{
					"id":    1,
					"title": "My Dashboard",
					"uid":   "test-uid",
				},
				"meta": map[string]any{
					"slug":    "my-dashboard",
					"created": "2023-01-01",
				},
			})
		}))
		defer server.Close()

		got, err := getDashboardByUID(newV84TestContext(server), GetDashboardByUIDRequest{UID: "test-uid"})
		require.NoError(t, err)
		assert.Equal(t, "My Dashboard", got.Dashboard["title"])
		assert.Equal(t, "my-dashboard", got.Meta["slug"])
	})
}

func TestUpsertDashboard(t *testing.T) {
	t.Run("dashboard is required", func(t *testing.T) {
		_, err := upsertDashboard(context.Background(), UpsertDashboardRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dashboard is required")
	})

	t.Run("creates dashboard and parses response correctly", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch r.URL.Path {
			case "/api/dashboards/db":
				assert.Equal(t, http.MethodPost, r.Method)
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.NotNil(t, body["dashboard"])
				assert.Equal(t, float64(10), body["folderId"])
				assert.Equal(t, true, body["overwrite"])
				assert.Equal(t, "test save", body["message"])

				_, _ = w.Write([]byte(`{"id":42,"uid":"new-uid","slug":"my-new-dashboard","url":"/d/new-uid/my-new-dashboard","status":"success","version":1}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		got, err := upsertDashboard(newV84TestContext(server), UpsertDashboardRequest{
			Dashboard: map[string]any{"title": "My New Dashboard"},
			FolderID:  int64Ptr(10),
			Overwrite: boolPtr(true),
			Message:   "test save",
		})
		require.NoError(t, err)
		assert.Equal(t, "success", got.Status)
		assert.Equal(t, "new-uid", got.UID)
		assert.Equal(t, "/d/new-uid/my-new-dashboard", got.URL)
		assert.Equal(t, "My New Dashboard", got.Title)
		assert.EqualValues(t, 1, got.Version)
	})

	t.Run("falls back to slug when dashboard title is absent", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/dashboards/db" {
				_, _ = w.Write([]byte(`{"id":1,"uid":"u","slug":"slug-val","url":"/d/u","status":"success","version":1}`))
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		got, err := upsertDashboard(newV84TestContext(server), UpsertDashboardRequest{
			Dashboard: map[string]any{"panels": []any{}},
		})
		require.NoError(t, err)
		assert.Equal(t, "slug-val", got.Title)
	})
}

// ----- Folders -----

func TestListFolders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "/api/folders", r.URL.Path)
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		assert.Equal(t, "2", r.URL.Query().Get("page"))

		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "uid": "f1", "title": "Folder 1"},
			{"id": 2, "uid": "f2", "title": "Folder 2"},
		})
	}))
	defer server.Close()

	got, err := listFolders(newV84TestContext(server), ListFoldersRequest{
		Limit: int64Ptr(10),
		Page:  int64Ptr(2),
	})
	require.NoError(t, err)
	require.Len(t, got.Items, 2)
	assert.Equal(t, "f1", got.Items[0].UID)
	assert.Equal(t, "Folder 1", got.Items[0].Title)
	assert.Equal(t, "f2", got.Items[1].UID)
}

func TestCreateFolder(t *testing.T) {
	t.Run("title is required", func(t *testing.T) {
		_, err := createFolder(context.Background(), CreateFolderRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "title is required")
	})

	t.Run("creates folder successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/folders", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)

			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "New Folder", body["title"])
			assert.Equal(t, "custom-uid", body["uid"])

			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":    100,
				"uid":   "custom-uid",
				"title": "New Folder",
				"url":   "/dashboards/f/custom-uid/new-folder",
			})
		}))
		defer server.Close()

		got, err := createFolder(newV84TestContext(server), CreateFolderRequest{
			Title: "New Folder",
			UID:   "custom-uid",
		})
		require.NoError(t, err)
		assert.EqualValues(t, 100, got.ID)
		assert.Equal(t, "custom-uid", got.UID)
		assert.Equal(t, "New Folder", got.Title)
	})
}

func TestUpdateFolder(t *testing.T) {
	t.Run("folderUid is required", func(t *testing.T) {
		_, err := updateFolder(context.Background(), UpdateFolderRequest{Title: "x"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "folderUid is required")
	})

	t.Run("title is required", func(t *testing.T) {
		_, err := updateFolder(context.Background(), UpdateFolderRequest{FolderUID: "x"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "title is required")
	})

	t.Run("updates folder", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/folders/f1", r.URL.Path)
			assert.Equal(t, http.MethodPut, r.Method)

			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":    1,
				"uid":   "f1",
				"title": "Updated Title",
			})
		}))
		defer server.Close()

		got, err := updateFolder(newV84TestContext(server), UpdateFolderRequest{
			FolderUID: "f1",
			Title:     "Updated Title",
		})
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", got.Title)
		assert.Equal(t, "f1", got.UID)
	})
}

// ----- Annotations -----

func TestGetAnnotations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "/api/annotations", r.URL.Path)
		assert.Equal(t, "1000", r.URL.Query().Get("from"))
		assert.Equal(t, "2000", r.URL.Query().Get("to"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))

		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":          1,
				"dashboardId": 10,
				"panelId":     5,
				"text":        "annotation text",
				"tags":        []string{"tag1"},
				"time":        1000,
				"timeEnd":     2000,
			},
		})
	}))
	defer server.Close()

	got, err := getAnnotations(newV84TestContext(server), GetAnnotationsRequest{
		From:  int64Ptr(1000),
		To:    int64Ptr(2000),
		Limit: int64Ptr(10),
	})
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.EqualValues(t, 1, got.Items[0].ID)
	assert.Equal(t, "annotation text", got.Items[0].Text)
	assert.EqualValues(t, 10, got.Items[0].DashboardID)
	assert.EqualValues(t, 5, got.Items[0].PanelID)
	assert.Equal(t, []string{"tag1"}, got.Items[0].Tags)
}

func TestCreateAnnotation(t *testing.T) {
	t.Run("text is required", func(t *testing.T) {
		_, err := createAnnotation(context.Background(), CreateAnnotationRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "text is required")
	})

	t.Run("creates annotation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/annotations", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)

			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": "Annotation added",
				"id":      42,
			})
		}))
		defer server.Close()

		got, err := createAnnotation(newV84TestContext(server), CreateAnnotationRequest{
			Text: "test annotation",
			Tags: []string{"test"},
		})
		require.NoError(t, err)
		assert.Equal(t, "Annotation added", got.Message)
		assert.EqualValues(t, 42, got.ID)
	})
}

func TestPatchAnnotation(t *testing.T) {
	t.Run("id is required", func(t *testing.T) {
		_, err := patchAnnotation(context.Background(), PatchAnnotationRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "id is required")
	})

	t.Run("patches annotation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/annotations/42", r.URL.Path)
			assert.Equal(t, http.MethodPatch, r.Method)

			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": "Annotation patched",
			})
		}))
		defer server.Close()

		text := "updated text"
		got, err := patchAnnotation(newV84TestContext(server), PatchAnnotationRequest{
			ID:   42,
			Text: &text,
		})
		require.NoError(t, err)
		assert.Equal(t, "Annotation patched", got.Message)
	})
}

// ----- Org Users & Teams -----

func TestListOrgUsers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "/api/org/users", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"userId":    1,
				"orgId":     1,
				"login":     "admin",
				"email":     "admin@test.com",
				"name":      "Admin",
				"role":      "Admin",
				"avatarUrl": "/avatar/abc",
			},
			{
				"userId": 2,
				"orgId":  1,
				"login":  "viewer1",
				"email":  "viewer@test.com",
				"name":   "Viewer",
				"role":   "Viewer",
			},
		})
	}))
	defer server.Close()

	got, err := listOrgUsers(newV84TestContext(server), ListOrgUsersRequest{})
	require.NoError(t, err)
	require.Len(t, got.Items, 2)
	assert.EqualValues(t, 1, got.Items[0].UserID)
	assert.Equal(t, "admin", got.Items[0].Login)
	assert.Equal(t, "Admin", got.Items[0].Role)
	assert.EqualValues(t, 2, got.Items[1].UserID)
	assert.Equal(t, "viewer1", got.Items[1].Login)
}

func TestListTeams(t *testing.T) {
	t.Run("lists teams with pagination", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/teams/search", r.URL.Path)
			assert.Equal(t, "ops", r.URL.Query().Get("query"))

			_ = json.NewEncoder(w).Encode(map[string]any{
				"totalCount": 1,
				"page":       1,
				"perPage":    50,
				"teams": []map[string]any{
					{
						"id":          10,
						"orgId":       1,
						"name":        "Ops Team",
						"email":       "ops@test.com",
						"memberCount": 5,
					},
				},
			})
		}))
		defer server.Close()

		got, err := listTeams(newV84TestContext(server), ListTeamsRequest{Query: "ops"})
		require.NoError(t, err)
		assert.EqualValues(t, 1, got.TotalCount)
		require.Len(t, got.Teams, 1)
		assert.EqualValues(t, 10, got.Teams[0].ID)
		assert.Equal(t, "Ops Team", got.Teams[0].Name)
		assert.EqualValues(t, 5, got.Teams[0].MemberCount)
	})

	t.Run("empty teams result", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"totalCount": 0,
				"page":       1,
				"perPage":    50,
				"teams":      []map[string]any{},
			})
		}))
		defer server.Close()

		got, err := listTeams(newV84TestContext(server), ListTeamsRequest{})
		require.NoError(t, err)
		assert.EqualValues(t, 0, got.TotalCount)
		assert.Len(t, got.Teams, 0)
	})
}

// ----- APIError -----

func TestAPIErrorModel(t *testing.T) {
	t.Run("Error() returns message", func(t *testing.T) {
		err := &APIError{StatusCode: 404, Message: "not found"}
		assert.Equal(t, "not found", err.Error())
	})

	t.Run("Error() returns message with detail", func(t *testing.T) {
		err := &APIError{StatusCode: 500, Message: "server error", Detail: "something went wrong"}
		assert.Equal(t, "server error: something went wrong", err.Error())
	})

	t.Run("wrapAPIError parses JSON body", func(t *testing.T) {
		body := []byte(`{"message":"resource not found","status":"error"}`)
		apiErr := wrapAPIError(404, body, "fallback")
		assert.Equal(t, 404, apiErr.StatusCode)
		assert.Equal(t, "resource not found", apiErr.Message)
		assert.Equal(t, "error", apiErr.Upstream["status"])
	})

	t.Run("wrapAPIError falls back on invalid JSON", func(t *testing.T) {
		apiErr := wrapAPIError(500, []byte(`not json`), "server error")
		assert.Equal(t, 500, apiErr.StatusCode)
		assert.Equal(t, "server error", apiErr.Message)
		assert.Nil(t, apiErr.Upstream)
	})

	t.Run("wrapAPIError falls back on empty body", func(t *testing.T) {
		apiErr := wrapAPIError(502, nil, "bad gateway")
		assert.Equal(t, 502, apiErr.StatusCode)
		assert.Equal(t, "bad gateway", apiErr.Message)
	})
}

func TestWrapOpenAPIError(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		assert.Nil(t, wrapOpenAPIError(nil))
	})

	t.Run("HardError passes through unchanged", func(t *testing.T) {
		hardErr := &mcpgrafana.HardError{Err: assert.AnError}
		result := wrapOpenAPIError(hardErr)
		var he *mcpgrafana.HardError
		require.ErrorAs(t, result, &he)
		assert.Equal(t, assert.AnError, he.Err)
	})

	t.Run("generic error becomes APIError", func(t *testing.T) {
		result := wrapOpenAPIError(assert.AnError)
		var apiErr *APIError
		require.ErrorAs(t, result, &apiErr)
		assert.Equal(t, 0, apiErr.StatusCode)
		assert.Contains(t, apiErr.Message, assert.AnError.Error())
	})
}

func TestWrapRawAPIError(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		assert.Nil(t, wrapRawAPIError(0, nil, nil))
	})

	t.Run("HardError passes through unchanged", func(t *testing.T) {
		hardErr := &mcpgrafana.HardError{Err: assert.AnError}
		result := wrapRawAPIError(0, nil, hardErr)
		var he *mcpgrafana.HardError
		require.ErrorAs(t, result, &he)
		assert.Equal(t, assert.AnError, he.Err)
	})

	t.Run("generic error becomes APIError with status code", func(t *testing.T) {
		body := []byte(`{"message":"forbidden"}`)
		result := wrapRawAPIError(403, body, assert.AnError)
		var apiErr *APIError
		require.ErrorAs(t, result, &apiErr)
		assert.Equal(t, 403, apiErr.StatusCode)
		assert.Equal(t, "forbidden", apiErr.Message)
	})
}

// ----- Annotation DashboardUID field -----

func TestGetAnnotationsWithDashboardUID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":           1,
				"dashboardId":  10,
				"dashboardUID": "dash-abc",
				"text":         "with uid",
			},
		})
	}))
	defer server.Close()

	got, err := getAnnotations(newV84TestContext(server), GetAnnotationsRequest{})
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.Equal(t, "dash-abc", got.Items[0].DashboardUID)
	assert.EqualValues(t, 10, got.Items[0].DashboardID)
}

// ----- Datasource JSONData mapping -----

func TestDatasourceJSONDataMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/datasources":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":   5,
					"uid":  "ds-5",
					"name": "Prom with JSONData",
					"type": "prometheus",
					"jsonData": map[string]any{
						"httpMethod":   "POST",
						"timeInterval": "15s",
					},
				},
			})
		case "/api/datasources/5":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   5,
				"uid":  "ds-5",
				"name": "Prom with JSONData",
				"type": "prometheus",
				"jsonData": map[string]any{
					"httpMethod":   "POST",
					"timeInterval": "15s",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	got, err := getDatasource(newV84TestContext(server), GetDatasourceRequest{UID: "ds-5"})
	require.NoError(t, err)
	require.NotNil(t, got.Datasource.JSONData)
	assert.Equal(t, "POST", got.Datasource.JSONData["httpMethod"])
	assert.Equal(t, "15s", got.Datasource.JSONData["timeInterval"])
}

// ----- Folder Permission filter -----

func TestListFoldersWithPermission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "/api/folders", r.URL.Path)
		assert.Equal(t, "Edit", r.URL.Query().Get("permission"))
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "uid": "f1", "title": "Editable Folder"},
		})
	}))
	defer server.Close()

	got, err := listFolders(newV84TestContext(server), ListFoldersRequest{
		Permission: "Edit",
	})
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.Equal(t, "Editable Folder", got.Items[0].Title)
}

// ----- Error paths for OpenAPI tools -----

func TestGetHealthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"internal error"}`))
	}))
	defer server.Close()

	_, err := getHealth(newV84TestContext(server), GetHealthRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get health")
}

func TestSearchDashboardsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer server.Close()

	_, err := searchDashboards(newV84TestContext(server), SearchDashboardsRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "search dashboards")
}

func TestGetDashboardByUIDError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"dashboard not found"}`))
	}))
	defer server.Close()

	_, err := getDashboardByUID(newV84TestContext(server), GetDashboardByUIDRequest{UID: "missing"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get dashboard by uid")
}

func TestListDatasourcesEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer server.Close()

	got, err := listDatasources(newV84TestContext(server), ListDatasourcesRequest{})
	require.NoError(t, err)
	assert.Len(t, got.Items, 0)
	assert.EqualValues(t, 0, got.Total)
}

func TestQueryDatasourceWithoutGrafanaURL(t *testing.T) {
	_, err := queryDatasource(context.Background(), QueryDatasourceRequest{
		From:    "now-1h",
		To:      "now",
		Queries: []map[string]any{{"refId": "A", "datasourceId": 1}},
	})
	require.Error(t, err)
	var hardErr *mcpgrafana.HardError
	assert.ErrorAs(t, err, &hardErr)
}

func TestQueryDatasourceWithoutDatasourceRef(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/tsdb/query" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			queries := body["queries"].([]any)
			q0 := queries[0].(map[string]any)
			assert.Equal(t, float64(42), q0["datasourceId"])
			_, _ = w.Write([]byte(`{"results":{"A":{"status":200}}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	got, err := queryDatasource(newV84TestContext(server), QueryDatasourceRequest{
		From:    "now-1h",
		To:      "now",
		Queries: []map[string]any{{"refId": "A", "datasourceId": 42}},
	})
	require.NoError(t, err)
	assert.Contains(t, got.Responses, "A")
}

func TestFlexibleIDUnmarshal(t *testing.T) {
	t.Run("integer value preserved as raw JSON", func(t *testing.T) {
		type testObj struct {
			ID FlexibleID `json:"id"`
		}
		var obj testObj
		require.NoError(t, json.Unmarshal([]byte(`{"id":42}`), &obj))
		assert.JSONEq(t, `42`, string(obj.ID))
	})

	t.Run("string value preserved as raw JSON", func(t *testing.T) {
		type testObj struct {
			ID FlexibleID `json:"id"`
		}
		var obj testObj
		require.NoError(t, json.Unmarshal([]byte(`{"id":"str-42"}`), &obj))
		assert.JSONEq(t, `"str-42"`, string(obj.ID))
	})

	t.Run("null value", func(t *testing.T) {
		type testObj struct {
			ID FlexibleID `json:"id"`
		}
		var obj testObj
		require.NoError(t, json.Unmarshal([]byte(`{"id":null}`), &obj))
		assert.Equal(t, "null", string(obj.ID))
	})
}
