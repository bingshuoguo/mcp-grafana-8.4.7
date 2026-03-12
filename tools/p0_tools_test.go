//go:build unit

package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Compat aliases ----

func TestCompatAliases(t *testing.T) {
	dsServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/datasources":
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"id": 5, "uid": "ds-uid", "name": "MyDS", "type": "prometheus"},
				})
			case "/api/datasources/5":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":   5,
					"uid":  "ds-uid",
					"name": "MyDS",
					"type": "prometheus",
				})
			default:
				http.NotFound(w, r)
			}
		}))
	}

	t.Run("get_datasource_by_uid delegates to getDatasource", func(t *testing.T) {
		srv := dsServer()
		defer srv.Close()

		got, err := getDatasourceByUID(newV84TestContext(srv), GetDatasourceByUIDRequest{UID: "ds-uid"})
		require.NoError(t, err)
		assert.Equal(t, "uid", got.ResolvedBy)
		assert.EqualValues(t, 5, got.Datasource.ID)
		assert.Equal(t, "MyDS", got.Datasource.Name)
	})

	t.Run("get_datasource_by_uid missing uid returns error", func(t *testing.T) {
		_, err := getDatasourceByUID(context.Background(), GetDatasourceByUIDRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "one of id, uid, or name is required")
	})

	t.Run("get_datasource_by_name delegates to getDatasource", func(t *testing.T) {
		srv := dsServer()
		defer srv.Close()

		got, err := getDatasourceByName(newV84TestContext(srv), GetDatasourceByNameRequest{Name: "MyDS"})
		require.NoError(t, err)
		assert.Equal(t, "name", got.ResolvedBy)
		assert.Equal(t, "ds-uid", got.Datasource.UID)
	})

	t.Run("get_datasource_by_name missing name returns error", func(t *testing.T) {
		_, err := getDatasourceByName(context.Background(), GetDatasourceByNameRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "one of id, uid, or name is required")
	})

	t.Run("list_users_by_org delegates to listOrgUsers", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"userId": 1, "login": "admin", "role": "Admin"},
			})
		}))
		defer srv.Close()

		got, err := listUsersByOrg(newV84TestContext(srv), ListOrgUsersRequest{})
		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "admin", got.Items[0].Login)
	})

	t.Run("update_dashboard delegates to upsertDashboard", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/dashboards/db" {
				_, _ = w.Write([]byte(`{"status":"success","uid":"u1","slug":"test","url":"/d/u1","version":2}`))
				return
			}
			http.NotFound(w, r)
		}))
		defer srv.Close()

		got, err := updateDashboard(newV84TestContext(srv), UpsertDashboardRequest{
			Dashboard: map[string]any{"title": "Test"},
		})
		require.NoError(t, err)
		assert.Equal(t, "success", got.Status)
		assert.Equal(t, "u1", got.UID)
	})
}

// ---- search_folders ----

func TestSearchFolders(t *testing.T) {
	t.Run("searches folders and returns results", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/search", r.URL.Path)
			assert.Equal(t, "dash-folder", r.URL.Query().Get("type"))
			assert.Equal(t, "ops", r.URL.Query().Get("query"))

			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 1, "uid": "f1", "title": "Ops Folder", "type": "dash-folder", "url": "/dashboards/f/f1"},
			})
		}))
		defer srv.Close()

		got, err := searchFolders(newV84TestContext(srv), SearchFoldersRequest{
			Query: "ops",
			Limit: int64Ptr(10),
		})
		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "f1", got.Items[0].UID)
		assert.Equal(t, "Ops Folder", got.Items[0].Title)
		assert.Equal(t, "dash-folder", got.Items[0].Type)
	})

	t.Run("empty result returns empty list", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		}))
		defer srv.Close()

		got, err := searchFolders(newV84TestContext(srv), SearchFoldersRequest{})
		require.NoError(t, err)
		assert.Empty(t, got.Items)
		assert.False(t, got.HasMore)
	})

	t.Run("upstream error returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"server error"}`))
		}))
		defer srv.Close()

		_, err := searchFolders(newV84TestContext(srv), SearchFoldersRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "search folders")
	})
}

// ---- get_dashboard_panel_queries ----

func TestGetDashboardPanelQueries(t *testing.T) {
	dashboardJSON := map[string]any{
		"title": "Test Dashboard",
		"panels": []any{
			map[string]any{
				"id":    float64(1),
				"title": "Panel One",
				"type":  "timeseries",
				"datasource": map[string]any{
					"type": "prometheus",
					"uid":  "prom-uid",
				},
				"targets": []any{
					map[string]any{
						"refId": "A",
						"expr":  "up",
						"datasource": map[string]any{
							"uid": "prom-uid",
						},
					},
				},
			},
			map[string]any{
				"id":    float64(2),
				"title": "Panel Two",
				"type":  "gauge",
				"targets": []any{
					map[string]any{
						"refId": "B",
						"expr":  "rate(http_requests_total[5m])",
					},
				},
			},
		},
	}

	makeServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/dashboards/uid/test-uid" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"dashboard": dashboardJSON,
				})
				return
			}
			http.NotFound(w, r)
		}))
	}

	t.Run("returns all panel queries", func(t *testing.T) {
		srv := makeServer()
		defer srv.Close()

		got, err := getDashboardPanelQueries(newV84TestContext(srv), GetDashboardPanelQueriesRequest{UID: "test-uid"})
		require.NoError(t, err)
		assert.Equal(t, "test-uid", got.UID)
		require.Len(t, got.Panels, 2)

		assert.EqualValues(t, 1, got.Panels[0].PanelID)
		assert.Equal(t, "Panel One", got.Panels[0].Title)
		assert.Equal(t, "timeseries", got.Panels[0].Type)
		require.Len(t, got.Panels[0].Targets, 1)
		assert.Equal(t, "A", got.Panels[0].Targets[0].RefID)

		assert.EqualValues(t, 2, got.Panels[1].PanelID)
		require.Len(t, got.Panels[1].Targets, 1)
		assert.Equal(t, "B", got.Panels[1].Targets[0].RefID)
	})

	t.Run("filters by panelId", func(t *testing.T) {
		srv := makeServer()
		defer srv.Close()

		got, err := getDashboardPanelQueries(newV84TestContext(srv), GetDashboardPanelQueriesRequest{
			UID:     "test-uid",
			PanelID: int64Ptr(2),
		})
		require.NoError(t, err)
		require.Len(t, got.Panels, 1)
		assert.EqualValues(t, 2, got.Panels[0].PanelID)
	})

	t.Run("uid is required", func(t *testing.T) {
		_, err := getDashboardPanelQueries(context.Background(), GetDashboardPanelQueriesRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "uid is required")
	})

	t.Run("upstream error returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		}))
		defer srv.Close()

		_, err := getDashboardPanelQueries(newV84TestContext(srv), GetDashboardPanelQueriesRequest{UID: "missing"})
		require.Error(t, err)
	})
}

// ---- get_dashboard_property ----

func TestGetDashboardProperty(t *testing.T) {
	makeServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/dashboards/uid/prop-uid" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"dashboard": map[string]any{
						"title": "Prop Dashboard",
						"tags":  []any{"prod", "infra"},
						"panels": []any{
							map[string]any{"id": float64(1), "title": "First Panel"},
						},
						"templating": map[string]any{
							"list": []any{
								map[string]any{"name": "env", "type": "query"},
							},
						},
					},
				})
				return
			}
			http.NotFound(w, r)
		}))
	}

	t.Run("gets top-level string property", func(t *testing.T) {
		srv := makeServer()
		defer srv.Close()

		got, err := getDashboardProperty(newV84TestContext(srv), GetDashboardPropertyRequest{
			UID:  "prop-uid",
			Path: "title",
		})
		require.NoError(t, err)
		assert.Equal(t, "Prop Dashboard", got.Value)
	})

	t.Run("gets nested property via dot path", func(t *testing.T) {
		srv := makeServer()
		defer srv.Close()

		got, err := getDashboardProperty(newV84TestContext(srv), GetDashboardPropertyRequest{
			UID:  "prop-uid",
			Path: "templating.list",
		})
		require.NoError(t, err)
		list, ok := got.Value.([]any)
		require.True(t, ok)
		assert.Len(t, list, 1)
	})

	t.Run("gets array element by index", func(t *testing.T) {
		srv := makeServer()
		defer srv.Close()

		got, err := getDashboardProperty(newV84TestContext(srv), GetDashboardPropertyRequest{
			UID:  "prop-uid",
			Path: "panels.0.title",
		})
		require.NoError(t, err)
		assert.Equal(t, "First Panel", got.Value)
	})

	t.Run("uid is required", func(t *testing.T) {
		_, err := getDashboardProperty(context.Background(), GetDashboardPropertyRequest{Path: "title"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "uid is required")
	})

	t.Run("path is required", func(t *testing.T) {
		_, err := getDashboardProperty(context.Background(), GetDashboardPropertyRequest{UID: "x"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path is required")
	})

	t.Run("missing key returns error", func(t *testing.T) {
		srv := makeServer()
		defer srv.Close()

		_, err := getDashboardProperty(newV84TestContext(srv), GetDashboardPropertyRequest{
			UID:  "prop-uid",
			Path: "nonexistent",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ---- get_dashboard_summary ----

func TestGetDashboardSummary(t *testing.T) {
	t.Run("returns full summary", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/dashboards/uid/sum-uid" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"dashboard": map[string]any{
						"title": "Summary Dashboard",
						"tags":  []any{"prod", "k8s"},
						"panels": []any{
							map[string]any{"id": 1, "type": "timeseries"},
							map[string]any{"id": 2, "type": "timeseries"},
							map[string]any{"id": 3, "type": "gauge"},
						},
						"templating": map[string]any{
							"list": []any{
								map[string]any{"name": "namespace", "type": "query", "label": "Namespace"},
							},
						},
					},
					"meta": map[string]any{
						"url":       "/d/sum-uid/summary-dashboard",
						"folderUid": "folder-abc",
					},
				})
				return
			}
			http.NotFound(w, r)
		}))
		defer srv.Close()

		got, err := getDashboardSummary(newV84TestContext(srv), GetDashboardSummaryRequest{UID: "sum-uid"})
		require.NoError(t, err)
		assert.Equal(t, "sum-uid", got.UID)
		assert.Equal(t, "Summary Dashboard", got.Title)
		assert.ElementsMatch(t, []string{"prod", "k8s"}, got.Tags)
		assert.Equal(t, 3, got.PanelCount)
		assert.Equal(t, 2, got.PanelTypes["timeseries"])
		assert.Equal(t, 1, got.PanelTypes["gauge"])
		require.Len(t, got.Variables, 1)
		assert.Equal(t, "namespace", got.Variables[0].Name)
		assert.Equal(t, "query", got.Variables[0].Type)
		assert.Equal(t, "Namespace", got.Variables[0].Label)
		assert.Equal(t, "/d/sum-uid/summary-dashboard", got.URL)
		assert.Equal(t, "folder-abc", got.FolderUID)
	})

	t.Run("uid is required", func(t *testing.T) {
		_, err := getDashboardSummary(context.Background(), GetDashboardSummaryRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "uid is required")
	})

	t.Run("upstream error returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		}))
		defer srv.Close()

		_, err := getDashboardSummary(newV84TestContext(srv), GetDashboardSummaryRequest{UID: "missing"})
		require.Error(t, err)
	})
}

// ---- create_graphite_annotation ----

func TestCreateGraphiteAnnotation(t *testing.T) {
	t.Run("creates graphite annotation", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/annotations/graphite", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)

			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "deploy", body["what"])
			assert.Equal(t, "production", body["data"])

			_, _ = w.Write([]byte(`{"message":"Graphite annotation added","id":100}`))
		}))
		defer srv.Close()

		got, err := createGraphiteAnnotation(newV84TestContext(srv), CreateGraphiteAnnotationRequest{
			What: "deploy",
			Tags: []string{"prod"},
			Data: "production",
		})
		require.NoError(t, err)
		assert.Equal(t, "Graphite annotation added", got.Message)
		assert.EqualValues(t, 100, got.ID)
	})

	t.Run("what is required", func(t *testing.T) {
		_, err := createGraphiteAnnotation(context.Background(), CreateGraphiteAnnotationRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "what is required")
	})

	t.Run("upstream error returns wrapped error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"permission denied"}`))
		}))
		defer srv.Close()

		_, err := createGraphiteAnnotation(newV84TestContext(srv), CreateGraphiteAnnotationRequest{What: "event"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "create graphite annotation")
	})
}

// ---- update_annotation ----

func TestUpdateAnnotation(t *testing.T) {
	t.Run("updates annotation via PUT", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/annotations/42", r.URL.Path)
			assert.Equal(t, http.MethodPut, r.Method)

			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "updated text", body["text"])

			_, _ = w.Write([]byte(`{"message":"Annotation updated"}`))
		}))
		defer srv.Close()

		got, err := updateAnnotation(newV84TestContext(srv), UpdateAnnotationRequest{
			ID:   42,
			Text: "updated text",
			Tags: []string{"changed"},
		})
		require.NoError(t, err)
		assert.Equal(t, "Annotation updated", got.Message)
	})

	t.Run("id is required", func(t *testing.T) {
		_, err := updateAnnotation(context.Background(), UpdateAnnotationRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "id is required")
	})

	t.Run("upstream error returns wrapped error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"annotation not found"}`))
		}))
		defer srv.Close()

		_, err := updateAnnotation(newV84TestContext(srv), UpdateAnnotationRequest{ID: 99})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "update annotation")
	})
}

// ---- get_annotation_tags ----

func TestGetAnnotationTags(t *testing.T) {
	t.Run("returns annotation tags", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "/api/annotations/tags", r.URL.Path)
			assert.Equal(t, "prod", r.URL.Query().Get("tag"))
			assert.Equal(t, "50", r.URL.Query().Get("limit"))

			_, _ = w.Write([]byte(`{"result":{"tags":[{"tag":"prod","count":5},{"tag":"prod-eu","count":2}]}}`))
		}))
		defer srv.Close()

		got, err := getAnnotationTags(newV84TestContext(srv), GetAnnotationTagsRequest{
			Tag:   "prod",
			Limit: int64Ptr(50),
		})
		require.NoError(t, err)
		require.Len(t, got.Items, 2)
		assert.Equal(t, "prod", got.Items[0].Tag)
		assert.EqualValues(t, 5, got.Items[0].Count)
		assert.Equal(t, "prod-eu", got.Items[1].Tag)
	})

	t.Run("empty tags returns empty list", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"tags":[]}}`))
		}))
		defer srv.Close()

		got, err := getAnnotationTags(newV84TestContext(srv), GetAnnotationTagsRequest{})
		require.NoError(t, err)
		assert.Empty(t, got.Items)
	})

	t.Run("upstream error returns wrapped error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"server error"}`))
		}))
		defer srv.Close()

		_, err := getAnnotationTags(newV84TestContext(srv), GetAnnotationTagsRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get annotation tags")
	})
}

// ---- generate_deeplink ----

func TestGenerateDeeplink(t *testing.T) {
	makeCtx := func(baseURL string) context.Context {
		return mcpgrafana.WithGrafanaConfig(context.Background(), mcpgrafana.GrafanaConfig{
			URL:    baseURL,
			APIKey: "token",
		})
	}

	t.Run("generates dashboard link", func(t *testing.T) {
		uid := "dash-abc"
		got, err := generateDeeplink(makeCtx("http://grafana.local"), GenerateDeeplinkRequest{
			ResourceType: "dashboard",
			DashboardUID: &uid,
		})
		require.NoError(t, err)
		assert.Equal(t, "http://grafana.local/d/dash-abc", got.URL)
	})

	t.Run("generates panel link with viewPanel param", func(t *testing.T) {
		uid := "dash-abc"
		panelID := 5
		got, err := generateDeeplink(makeCtx("http://grafana.local"), GenerateDeeplinkRequest{
			ResourceType: "panel",
			DashboardUID: &uid,
			PanelID:      &panelID,
		})
		require.NoError(t, err)
		assert.Contains(t, got.URL, "/d/dash-abc")
		assert.Contains(t, got.URL, "viewPanel=5")
	})

	t.Run("generates explore link with datasource", func(t *testing.T) {
		dsUID := "prom-uid"
		got, err := generateDeeplink(makeCtx("http://grafana.local"), GenerateDeeplinkRequest{
			ResourceType:  "explore",
			DatasourceUID: &dsUID,
		})
		require.NoError(t, err)
		assert.Contains(t, got.URL, "/explore")
		assert.Contains(t, got.URL, "prom-uid")
	})

	t.Run("appends time range params", func(t *testing.T) {
		uid := "dash-abc"
		got, err := generateDeeplink(makeCtx("http://grafana.local"), GenerateDeeplinkRequest{
			ResourceType: "dashboard",
			DashboardUID: &uid,
			TimeRange:    &V84TimeRange{From: "now-1h", To: "now"},
		})
		require.NoError(t, err)
		assert.Contains(t, got.URL, "from=now-1h")
		assert.Contains(t, got.URL, "to=now")
	})

	t.Run("dashboard link missing uid returns error", func(t *testing.T) {
		_, err := generateDeeplink(makeCtx("http://grafana.local"), GenerateDeeplinkRequest{
			ResourceType: "dashboard",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dashboardUid is required")
	})

	t.Run("unsupported resource type returns error", func(t *testing.T) {
		_, err := generateDeeplink(makeCtx("http://grafana.local"), GenerateDeeplinkRequest{
			ResourceType: "unknown",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported resource type")
	})

	t.Run("missing grafana URL returns hard error", func(t *testing.T) {
		_, err := generateDeeplink(context.Background(), GenerateDeeplinkRequest{ResourceType: "dashboard"})
		require.Error(t, err)
		var hardErr *mcpgrafana.HardError
		assert.ErrorAs(t, err, &hardErr)
	})
}

// ---- get_query_examples ----

func TestGetQueryExamples(t *testing.T) {
	t.Run("returns prometheus examples", func(t *testing.T) {
		got, err := getQueryExamples(context.Background(), GetQueryExamplesRequest{DatasourceType: "prometheus"})
		require.NoError(t, err)
		assert.Equal(t, "prometheus", got.DatasourceType)
		assert.NotEmpty(t, got.Examples)
		for _, ex := range got.Examples {
			assert.NotEmpty(t, ex.Name)
			assert.NotEmpty(t, ex.Query)
		}
	})

	t.Run("returns loki examples", func(t *testing.T) {
		got, err := getQueryExamples(context.Background(), GetQueryExamplesRequest{DatasourceType: "loki"})
		require.NoError(t, err)
		assert.Equal(t, "loki", got.DatasourceType)
		assert.NotEmpty(t, got.Examples)
	})

	t.Run("returns clickhouse examples", func(t *testing.T) {
		got, err := getQueryExamples(context.Background(), GetQueryExamplesRequest{DatasourceType: "clickhouse"})
		require.NoError(t, err)
		assert.Equal(t, "clickhouse", got.DatasourceType)
		assert.NotEmpty(t, got.Examples)
	})

	t.Run("returns cloudwatch examples with metric config", func(t *testing.T) {
		got, err := getQueryExamples(context.Background(), GetQueryExamplesRequest{DatasourceType: "cloudwatch"})
		require.NoError(t, err)
		assert.Equal(t, "cloudwatch", got.DatasourceType)
		assert.NotEmpty(t, got.Examples)
		for _, ex := range got.Examples {
			assert.NotEmpty(t, ex.Namespace)
			assert.NotEmpty(t, ex.MetricName)
		}
	})

	t.Run("case insensitive type matching", func(t *testing.T) {
		got, err := getQueryExamples(context.Background(), GetQueryExamplesRequest{DatasourceType: "Prometheus"})
		require.NoError(t, err)
		assert.Equal(t, "prometheus", got.DatasourceType)
	})

	t.Run("unsupported datasource type returns error", func(t *testing.T) {
		_, err := getQueryExamples(context.Background(), GetQueryExamplesRequest{DatasourceType: "mysql"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported datasource type")
		assert.Contains(t, err.Error(), "mysql")
	})
}

// ---- extractPanels helper ----

func TestExtractPanels(t *testing.T) {
	t.Run("extracts top-level panels", func(t *testing.T) {
		db := map[string]any{
			"panels": []any{
				map[string]any{"id": 1, "type": "timeseries"},
				map[string]any{"id": 2, "type": "gauge"},
			},
		}
		panels := extractPanels(db)
		assert.Len(t, panels, 2)
	})

	t.Run("extracts collapsed row sub-panels", func(t *testing.T) {
		db := map[string]any{
			"panels": []any{
				map[string]any{
					"id":   1,
					"type": "row",
					"panels": []any{
						map[string]any{"id": 2, "type": "graph"},
						map[string]any{"id": 3, "type": "stat"},
					},
				},
			},
		}
		panels := extractPanels(db)
		// row itself + 2 sub-panels
		assert.Len(t, panels, 3)
	})

	t.Run("nil dashboard returns nil", func(t *testing.T) {
		assert.Nil(t, extractPanels(nil))
	})
}
