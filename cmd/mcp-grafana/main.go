package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	mcpgrafana "github.com/bingshuoguo/grafana-v8-mcp"
	"github.com/bingshuoguo/grafana-v8-mcp/observability"
	grafanatools "github.com/bingshuoguo/grafana-v8-mcp/tools"
	"go.opentelemetry.io/otel/semconv/v1.39.0/mcpconv"
)

// toolConfig controls which tool categories are registered.
type toolConfig struct {
	disableWrite  bool
	optionalTools bool
	toolsets      toolNameList
	enableTools   toolNameList
	disableTools  toolNameList
}

// Configuration for the Grafana client.
type grafanaConfig struct {
	// Whether to enable debug mode for the Grafana transport.
	debug bool

	// TLS configuration
	tlsCertFile   string
	tlsKeyFile    string
	tlsCAFile     string
	tlsSkipVerify bool
}

func (tc *toolConfig) addFlags() {
	flag.BoolVar(&tc.disableWrite, "disable-write", false, "Disable write tools (create/update operations)")
	flag.BoolVar(&tc.optionalTools, "enable-optional-tools", false, "Enable optional tools (unified alerting, rendering)")
	flag.Var(&tc.toolsets, "toolsets", "Enable only the specified built-in toolsets. Accepts repeated flags or comma-separated values")
	flag.Var(&tc.enableTools, "enable-tools", "Enable only the specified exact public tool names. Accepts repeated flags or comma-separated values")
	flag.Var(&tc.disableTools, "disable-tools", "Disable the specified exact public tool names. Accepts repeated flags or comma-separated values")
}

func (gc *grafanaConfig) addFlags() {
	flag.BoolVar(&gc.debug, "debug", false, "Enable debug mode for the Grafana transport")

	// TLS configuration flags
	flag.StringVar(&gc.tlsCertFile, "tls-cert-file", "", "Path to TLS certificate file for client authentication")
	flag.StringVar(&gc.tlsKeyFile, "tls-key-file", "", "Path to TLS private key file for client authentication")
	flag.StringVar(&gc.tlsCAFile, "tls-ca-file", "", "Path to TLS CA certificate file for server verification")
	flag.BoolVar(&gc.tlsSkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification (insecure)")
}

func (tc *toolConfig) addTools(s *server.MCPServer) {
	grafanatools.AddV84Tools(s, tc.registerOptions())
}

func (tc *toolConfig) registerOptions() grafanatools.RegisterOptions {
	return grafanatools.RegisterOptions{
		EnableWriteTools:    !tc.disableWrite,
		EnableOptionalTools: tc.optionalTools,
		Toolsets:            tc.toolsets.Values(),
		ToolsetsSet:         tc.toolsets.IsSet(),
		EnableTools:         tc.enableTools.Values(),
		EnableToolsSet:      tc.enableTools.IsSet(),
		DisableTools:        tc.disableTools.Values(),
		DisableToolsSet:     tc.disableTools.IsSet(),
	}
}

func newServer(tc toolConfig, obs *observability.Observability) *server.MCPServer {
	hooks := observability.MergeHooks(&server.Hooks{}, obs.MCPHooks())

	s := server.NewMCPServer("mcp-grafana", mcpgrafana.Version(),
		server.WithToolCapabilities(true),
		server.WithPromptCapabilities(true),
		server.WithResourceCapabilities(false, true),
		server.WithRecovery(),
		server.WithResourceRecovery(),
		server.WithInstructions(`
This server exposes Grafana 8.4.7 MCP tools.

Default capabilities:
- Health, current user, current org.
- Dashboard search and dashboard CRUD via upsert.
- Folder list/create/update.
- Datasource list/get/resolve and generic datasource querying via /api/tsdb/query.
- Annotation list/create/patch.
- Legacy alerting reads: /api/alerts and /api/alert-notifications.
- Organization reads: list org users and teams.

Additional MCP capabilities:
- Toolsets for grouped discovery and selective enablement.
- Read-only resources documenting available toolsets and recommended workflows.
- Reusable prompts for dashboard, datasource, and alert investigations.

Use only tools returned by list_tools. Some tools can be disabled via flags.
`),
		server.WithHooks(hooks),
	)

	tc.addTools(s)
	addServerAssets(s, tc.registerOptions())
	return s
}

type tlsConfig struct {
	certFile, keyFile string
}

func (tc *tlsConfig) addFlags() {
	flag.StringVar(&tc.certFile, "server.tls-cert-file", "", "Path to TLS certificate file for server HTTPS (required for TLS)")
	flag.StringVar(&tc.keyFile, "server.tls-key-file", "", "Path to TLS private key file for server HTTPS (required for TLS)")
}

// httpServer represents a server with Start and Shutdown methods
type httpServer interface {
	Start(addr string) error
	Shutdown(ctx context.Context) error
}

// runHTTPServer handles the common logic for running HTTP-based servers
func runHTTPServer(ctx context.Context, srv httpServer, addr, transportName string) error {
	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Start(addr); err != nil {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Wait for either server error or shutdown signal
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		slog.Info(fmt.Sprintf("%s server shutting down...", transportName))

		// Create a timeout context for shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown error: %v", err)
		}
		slog.Debug("Shutdown called, waiting for connections to close...")

		// Wait for server to finish
		select {
		case err := <-serverErr:
			// http.ErrServerClosed is expected when shutting down
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("server error during shutdown: %v", err)
			}
		case <-shutdownCtx.Done():
			slog.Warn(fmt.Sprintf("%s server did not stop gracefully within timeout", transportName))
		}
	}

	return nil
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// runMetricsServer starts a separate HTTP server for metrics.
func runMetricsServer(addr string, o *observability.Observability) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", o.MetricsHandler())
	slog.Info("Starting metrics server", "address", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("metrics server error", "error", err)
	}
}

func run(transport, addr, basePath, endpointPath string, logLevel slog.Level, tc toolConfig, gc mcpgrafana.GrafanaConfig, tls tlsConfig, obs observability.Config) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	// Set up observability (metrics and tracing)
	o, err := observability.Setup(obs)
	if err != nil {
		return fmt.Errorf("failed to setup observability: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := o.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shutdown observability", "error", err)
		}
	}()

	s := newServer(tc, o)

	// Create a context that will be cancelled on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Handle shutdown signals
	go func() {
		<-sigChan
		slog.Info("Received shutdown signal")
		cancel()

		// For stdio, close stdin to unblock the Listen call
		if transport == "stdio" {
			_ = os.Stdin.Close()
		}
	}()

	// Start the appropriate server based on transport
	switch transport {
	case "stdio":
		srv := server.NewStdioServer(s)
		cf := mcpgrafana.ComposedStdioContextFunc(gc)
		srv.SetContextFunc(cf)

		slog.Info("Starting Grafana MCP server using stdio transport", "version", mcpgrafana.Version())

		err := srv.Listen(ctx, os.Stdin, os.Stdout)
		if err != nil && err != context.Canceled {
			return fmt.Errorf("server error: %v", err)
		}
		return nil

	case "sse":
		httpSrv := &http.Server{Addr: addr}
		srv := server.NewSSEServer(s,
			server.WithSSEContextFunc(mcpgrafana.ComposedSSEContextFunc(gc)),
			server.WithStaticBasePath(basePath),
			server.WithHTTPServer(httpSrv),
		)
		mux := http.NewServeMux()
		if basePath == "" {
			basePath = "/"
		}
		mux.Handle(basePath, observability.WrapHandler(srv, basePath))
		mux.HandleFunc("/healthz", handleHealthz)
		if obs.MetricsEnabled {
			if obs.MetricsAddress == "" {
				mux.Handle("/metrics", o.MetricsHandler())
			} else {
				go runMetricsServer(obs.MetricsAddress, o)
			}
		}
		httpSrv.Handler = mux
		slog.Info("Starting Grafana MCP server using SSE transport",
			"version", mcpgrafana.Version(), "address", addr, "basePath", basePath, "metrics", obs.MetricsEnabled)
		return runHTTPServer(ctx, srv, addr, "SSE")
	case "streamable-http":
		httpSrv := &http.Server{Addr: addr}
		opts := []server.StreamableHTTPOption{
			server.WithHTTPContextFunc(mcpgrafana.ComposedHTTPContextFunc(gc)),
			server.WithStateLess(true),
			server.WithEndpointPath(endpointPath),
			server.WithStreamableHTTPServer(httpSrv),
		}
		if tls.certFile != "" || tls.keyFile != "" {
			opts = append(opts, server.WithTLSCert(tls.certFile, tls.keyFile))
		}
		srv := server.NewStreamableHTTPServer(s, opts...)
		mux := http.NewServeMux()
		mux.Handle(endpointPath, observability.WrapHandler(srv, endpointPath))
		mux.HandleFunc("/healthz", handleHealthz)
		if obs.MetricsEnabled {
			if obs.MetricsAddress == "" {
				mux.Handle("/metrics", o.MetricsHandler())
			} else {
				go runMetricsServer(obs.MetricsAddress, o)
			}
		}
		httpSrv.Handler = mux
		slog.Info("Starting Grafana MCP server using StreamableHTTP transport",
			"version", mcpgrafana.Version(), "address", addr, "endpointPath", endpointPath, "metrics", obs.MetricsEnabled)
		return runHTTPServer(ctx, srv, addr, "StreamableHTTP")
	default:
		return fmt.Errorf("invalid transport type: %s. Must be 'stdio', 'sse' or 'streamable-http'", transport)
	}
}

func main() {
	var transport string
	flag.StringVar(&transport, "t", "stdio", "Transport type (stdio, sse or streamable-http)")
	flag.StringVar(
		&transport,
		"transport",
		"stdio",
		"Transport type (stdio, sse or streamable-http)",
	)
	addr := flag.String("address", "localhost:8000", "The host and port to start the sse server on")
	basePath := flag.String("base-path", "", "Base path for the sse server")
	endpointPath := flag.String("endpoint-path", "/mcp", "Endpoint path for the streamable-http server")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	showVersion := flag.Bool("version", false, "Print the version and exit")
	var tc toolConfig
	tc.addFlags()
	var gc grafanaConfig
	gc.addFlags()
	var tls tlsConfig
	tls.addFlags()
	var obs observability.Config
	flag.BoolVar(&obs.MetricsEnabled, "metrics", false, "Enable Prometheus metrics endpoint")
	flag.StringVar(&obs.MetricsAddress, "metrics-address", "", "Separate address for metrics server (e.g., :9090). If empty, metrics are served on the main server at /metrics")
	flag.Parse()

	if *showVersion {
		fmt.Println(mcpgrafana.Version())
		os.Exit(0)
	}

	// Convert local grafanaConfig to mcpgrafana.GrafanaConfig
	grafanaConfig := mcpgrafana.GrafanaConfig{Debug: gc.debug}
	if gc.tlsCertFile != "" || gc.tlsKeyFile != "" || gc.tlsCAFile != "" || gc.tlsSkipVerify {
		grafanaConfig.TLSConfig = &mcpgrafana.TLSConfig{
			CertFile:   gc.tlsCertFile,
			KeyFile:    gc.tlsKeyFile,
			CAFile:     gc.tlsCAFile,
			SkipVerify: gc.tlsSkipVerify,
		}
	}

	// Set OTel resource identity
	obs.ServerName = "mcp-grafana"
	obs.ServerVersion = mcpgrafana.Version()

	// Map transport flag to semconv network.transport values
	switch transport {
	case "stdio":
		obs.NetworkTransport = mcpconv.NetworkTransportPipe
	case "sse", "streamable-http":
		obs.NetworkTransport = mcpconv.NetworkTransportTCP
	}

	if err := run(transport, *addr, *basePath, *endpointPath, parseLevel(*logLevel), tc, grafanaConfig, tls, obs); err != nil {
		panic(err)
	}
}

func parseLevel(level string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return slog.LevelInfo
	}
	return l
}
