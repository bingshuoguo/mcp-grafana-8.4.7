# Grafana MCP Server (Grafana 8.4.7 Profile)

A [Model Context Protocol][mcp] (MCP) server for **Grafana 8.4.7**, forked from [grafana/mcp-grafana](https://github.com/grafana/mcp-grafana).

This variant provides a dedicated tool profile (`v84`) that targets the Grafana 8.4.7 REST API, using **ID-first datasource resolution** and **raw HTTP fallbacks** where the upstream OpenAPI client models do not match the 8.4.7 API behaviour.

## Quick Start

The recommended installation and connection path is:

1. Install the `mcp-grafana` binary from [GitHub Releases](https://github.com/bingshuoguo/grafana-v8-mcp/releases) or `go install`
2. Verify the binary locally
3. Configure your MCP client to launch `mcp-grafana` over `stdio`

```bash
go install github.com/bingshuoguo/grafana-v8-mcp/cmd/mcp-grafana@latest
mcp-grafana --version
```

Minimal MCP client configuration:

```json
{
  "mcpServers": {
    "grafana": {
      "command": "mcp-grafana",
      "args": [],
      "env": {
        "GRAFANA_URL": "http://localhost:3000",
        "GRAFANA_SERVICE_ACCOUNT_TOKEN": "<your-token>"
      }
    }
  }
}
```

If your client cannot find `mcp-grafana` in `PATH`, use the absolute path to the binary instead.

## Requirements

- **Grafana 8.4.7** (or any 8.x release with the same API surface)
- Go 1.24+ (only needed for `go install` or building from source)

## Features

### Tool Profile: `v84`

The `v84` profile registers **20 MVP tools** (15 read-only + 5 write) that cover the most common Grafana operations.

#### Health / Identity

| Tool | API | Description |
|------|-----|-------------|
| `get_health` | `GET /api/health` | Database status, Grafana version and commit hash |
| `get_current_user` | `GET /api/user` | Signed-in user profile |
| `get_current_org` | `GET /api/org` | Current organization info |

#### Dashboard

| Tool | API | Description |
|------|-----|-------------|
| `search_dashboards` | `GET /api/search` | Search dashboards by title, tags, folder, starred status; paginated |
| `get_dashboard_by_uid` | `GET /api/dashboards/uid/{uid}` | Full dashboard JSON + meta |
| `upsert_dashboard`* | `POST /api/dashboards/db` | Create or update a dashboard (raw HTTP) |

#### Folders

| Tool | API | Description |
|------|-----|-------------|
| `list_folders` | `GET /api/folders` | List folders with permission filter |
| `create_folder`* | `POST /api/folders` | Create a new folder |
| `update_folder`* | `PUT /api/folders/{uid}` | Update an existing folder |

#### Datasources

| Tool | API | Description |
|------|-----|-------------|
| `list_datasources` | `GET /api/datasources` | List all datasources with type filter and pagination |
| `get_datasource` | Composite | Get datasource by ID, UID, or name (ID-first resolution) |
| `resolve_datasource_ref` | Composite | Resolve a datasource reference to its canonical `{id, uid, name, type, url}` |

#### Query

| Tool | API | Description |
|------|-----|-------------|
| `query_datasource` | `POST /api/tsdb/query` | Execute datasource queries with automatic `datasourceId` injection |

#### Annotations

| Tool | API | Description |
|------|-----|-------------|
| `get_annotations` | `GET /api/annotations` | Query annotations with time range, dashboard, tag, and type filters |
| `create_annotation`* | `POST /api/annotations` | Create a new annotation |
| `patch_annotation`* | `PATCH /api/annotations/{id}` | Partially update an annotation |

#### Legacy Alerting

| Tool | API | Description |
|------|-----|-------------|
| `list_legacy_alerts` | `GET /api/alerts` | List legacy alert rules with dashboard/panel/state filters |
| `list_legacy_notification_channels` | `GET /api/alert-notifications` | List legacy notification channels |

#### Organization / Admin

| Tool | API | Description |
|------|-----|-------------|
| `list_org_users` | `GET /api/org/users` | List users in the current organization |
| `list_teams` | `GET /api/teams/search` | Search teams with pagination |

_\* Write tools. Disabled when `--disable-write` is set._

### Architecture Highlights

- **ID-first datasource resolution**: `id > uid > name` priority with list-API fallback when `GET /api/datasources/{id}` fails.
- **Raw HTTP for incompatible endpoints**: `upsert_dashboard`, `query_datasource`, and legacy alerting endpoints use `doAPIRequest` instead of the OpenAPI client when the generated models are incorrect for 8.4.7.
- **Unified `APIError` model**: All tool error paths normalize errors into `APIError{statusCode, message, status, detail, upstream}`. Configuration/auth errors propagate as `HardError` for JSON-RPC protocol-level failures.
- **`FlexibleID`**: Dashboard upsert response `id` is typed as `json.RawMessage` to handle both integer and string values.
- **`jsonschema` struct tags**: All request structs carry `jsonschema` tags for MCP input schema generation. Commas in descriptions are escaped as `\\,` per linter rule.

## CLI Flags Reference

### Transport Options

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --transport` | `stdio` | Transport type: `stdio`, `sse`, or `streamable-http` |
| `--address` | `localhost:8000` | Host and port for SSE / streamable-http |
| `--base-path` | | Base path for SSE server |
| `--endpoint-path` | `/mcp` | Endpoint path for streamable-http |

### Tool Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `--enabled-tools` | `v84` | Comma-separated list of enabled tool profiles |
| `--disable-v84` | `false` | Disable the Grafana 8.4.7 tool profile |
| `--disable-write` | `false` | Disable write tools (upsert_dashboard, create/update folder, create/patch annotation) |
| `--enable-v84-optional-tools` | `false` | Enable optional phase-2 tools |

### Debug and Logging

| Flag | Default | Description |
|------|---------|-------------|
| `--debug` | `false` | Enable detailed HTTP request/response logging |
| `--log-level` | `info` | Log level: `debug`, `info`, `warn`, `error` |

### Client TLS (for Grafana connections)

| Flag | Description |
|------|-------------|
| `--tls-cert-file` | Path to TLS certificate file for client authentication |
| `--tls-key-file` | Path to TLS private key file |
| `--tls-ca-file` | Path to TLS CA certificate file for server verification |
| `--tls-skip-verify` | Skip TLS certificate verification (insecure) |

### Server TLS (streamable-http only)

| Flag | Description |
|------|-------------|
| `--server.tls-cert-file` | Path to TLS certificate file for server HTTPS |
| `--server.tls-key-file` | Path to TLS private key file for server HTTPS |

### Observability

| Flag | Default | Description |
|------|---------|-------------|
| `--metrics` | `false` | Enable Prometheus metrics endpoint at `/metrics` |
| `--metrics-address` | | Separate address for metrics server (e.g., `:9090`) |

## Installation

### Prerequisites

- **Grafana 8.4.7** instance, accessible via HTTP/HTTPS
- A Grafana **Service Account Token** or **username/password** for authentication
- **Go 1.24+** only if you install via `go install` or build from source

### Step 1: Prepare Grafana Credentials

You need one of the following authentication methods:

**Option A: Service Account Token (recommended)**

1. Log in to your Grafana 8.4.7 instance.
2. Go to **Configuration > Service Accounts** (or **Administration > Service Accounts** depending on your Grafana version).
3. Click **Add service account**, give it a name (e.g., `mcp-server`), and assign the **Editor** role.
4. Click **Add token**, copy the generated token and save it -- you will use it as `GRAFANA_SERVICE_ACCOUNT_TOKEN`.

**Option B: Username / Password**

Use the admin or any other user's credentials. These will be set as `GRAFANA_USERNAME` and `GRAFANA_PASSWORD`.

### Step 2: Install the Binary

Choose one of the following methods. For most users, GitHub Releases or `go install` is the simplest path.

#### Method 1: GitHub Releases (recommended)

Download the archive for your platform from [GitHub Releases](https://github.com/bingshuoguo/grafana-v8-mcp/releases), extract it, and place `mcp-grafana` somewhere in your `PATH`.

The release page includes platform-specific archives and a checksum file for verification.

#### Method 2: Go Install

If you have a Go toolchain, install directly into `$GOBIN`:

```bash
GOBIN="$HOME/go/bin" go install github.com/bingshuoguo/grafana-v8-mcp/cmd/mcp-grafana@latest
```

Make sure `$HOME/go/bin` is in your `PATH`.

#### Method 3: Build from Source

```bash
# Clone the repository
git clone <repo-url> mcp-grafana-8.4.7
cd mcp-grafana-8.4.7

# Build the binary (output: dist/mcp-grafana)
make build
```

#### Method 4: Docker Image

```bash
# Build the Docker image locally
make build-image
# This creates: mcp-grafana:latest
```

### Step 3: Verify Installation

```bash
# Check the binary version
mcp-grafana --version

# Quick health check (should connect to your Grafana and return version info)
GRAFANA_URL=http://localhost:3000 \
GRAFANA_SERVICE_ACCOUNT_TOKEN=<your-token> \
mcp-grafana --version
```

### Step 4: Configure Your MCP Client

The default and recommended transport is `stdio`. Most MCP clients can launch the binary directly:

- `command = "mcp-grafana"` when the binary is in `PATH`
- `command = "/absolute/path/to/mcp-grafana"` if your client cannot resolve the binary

Docker and HTTP-based transports are available, but they are secondary options for isolated or remote deployments.

Choose the client guide that matches your AI assistant / IDE:

- [Claude Desktop](docs/clients/claude-desktop.md)
- [Claude Code](docs/clients/claude-code.md)
- [Codex CLI](docs/clients/codex.md)
- [Cursor](docs/clients/cursor.md)
- [Gemini CLI](docs/clients/gemini-cli.md)
- [Windsurf](docs/clients/windsurf.md)
- [Zed](docs/clients/zed.md)
- [VS Code and GitHub Copilot](docs/clients/vscode-copilot.md)

#### Cursor IDE

Create or edit the file `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "grafana": {
      "command": "/absolute/path/to/mcp-grafana",
      "args": [],
      "env": {
        "GRAFANA_URL": "http://localhost:3000",
        "GRAFANA_SERVICE_ACCOUNT_TOKEN": "<your service account token>"
      }
    }
  }
}
```

> **Tip:** You must use the **absolute path** to the `mcp-grafana` binary. Relative paths or just the binary name may cause `ENOENT` errors.

If you built from source, the path is typically:

```json
"command": "/Users/<you>/path/to/mcp-grafana-8.4.7/dist/mcp-grafana"
```

Or if you used `go install`:

```json
"command": "/Users/<you>/go/bin/mcp-grafana"
```

#### Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or the equivalent path on your OS:

```json
{
  "mcpServers": {
    "grafana": {
      "command": "/absolute/path/to/mcp-grafana",
      "args": [],
      "env": {
        "GRAFANA_URL": "http://localhost:3000",
        "GRAFANA_SERVICE_ACCOUNT_TOKEN": "<your service account token>"
      }
    }
  }
}
```

#### Using Username/Password Instead of Token

```json
{
  "mcpServers": {
    "grafana": {
      "command": "/absolute/path/to/mcp-grafana",
      "args": [],
      "env": {
        "GRAFANA_URL": "http://localhost:3000",
        "GRAFANA_USERNAME": "admin",
        "GRAFANA_PASSWORD": "admin"
      }
    }
  }
}
```

#### Using Docker (stdio mode)

```json
{
  "mcpServers": {
    "grafana": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-e", "GRAFANA_URL",
        "-e", "GRAFANA_SERVICE_ACCOUNT_TOKEN",
        "bingshuoguo/grafana-v8-mcp:latest",
        "-t", "stdio"
      ],
      "env": {
        "GRAFANA_URL": "http://host.docker.internal:3000",
        "GRAFANA_SERVICE_ACCOUNT_TOKEN": "<your service account token>"
      }
    }
  }
}
```

> **Note:** Use `host.docker.internal` instead of `localhost` when Grafana is running on the host machine.

#### Read-Only Mode

Add `--disable-write` to `args` to prevent any write operations:

```json
{
  "mcpServers": {
    "grafana": {
      "command": "/absolute/path/to/mcp-grafana",
      "args": ["--disable-write"],
      "env": {
        "GRAFANA_URL": "http://localhost:3000",
        "GRAFANA_SERVICE_ACCOUNT_TOKEN": "<your service account token>"
      }
    }
  }
}
```

This disables 5 write tools (`upsert_dashboard`, `create_folder`, `update_folder`, `create_annotation`, `patch_annotation`) while keeping all 15 read tools available.

#### Debug Mode

Add `--debug` to `args` for detailed HTTP request/response logging (sent to stderr):

```json
"args": ["--debug", "--log-level", "debug"]
```

### Step 5: Verify the MCP Connection

After configuring your MCP client:

1. **Restart** your IDE / Claude Desktop to pick up the new configuration.
2. Open a new chat and ask the assistant: _"What is the Grafana health status?"_
3. The assistant should invoke the `get_health` tool and return your Grafana version, database status, and commit hash.

If something goes wrong, check the following:

| Symptom | Cause | Fix |
|---------|-------|-----|
| `ENOENT` error | Binary path is wrong or relative | Use the **absolute path** in `command` |
| `connection refused` | `GRAFANA_URL` is wrong or Grafana is down | Verify the URL and that Grafana is running |
| `401 Unauthorized` | Invalid token or password | Regenerate the service account token |
| `403 Forbidden` | Insufficient permissions | Assign **Editor** role to the service account |
| Tools not showing | Config file syntax error | Validate JSON with `jq . < mcp.json` |

## Environment Variables Reference

| Variable | Required | Description |
|----------|----------|-------------|
| `GRAFANA_URL` | Yes | Grafana instance URL (e.g., `http://localhost:3000`) |
| `GRAFANA_SERVICE_ACCOUNT_TOKEN` | One of token or user/pass | Service account token for authentication |
| `GRAFANA_USERNAME` | One of token or user/pass | Username for basic authentication |
| `GRAFANA_PASSWORD` | One of token or user/pass | Password for basic authentication |
| `GRAFANA_ORG_ID` | No | Organization ID for multi-org support |
| `GRAFANA_EXTRA_HEADERS` | No | JSON object of extra HTTP headers (e.g., `{"X-Custom":"value"}`) |

## Recommended Connection Modes

- **Binary + stdio (recommended):** best default for local use, the shortest execution path, and the least moving parts.
- **Docker + stdio:** useful when you want isolation or do not want to install the binary on the host.
- **SSE / streamable-http:** useful for remote deployments or when multiple clients need to share one running MCP server.
- **`npx mcp-remote`:** a bridge to a remote SSE server, not the primary installation path for this repository.

## Advanced Usage

### SSE / Streamable HTTP Mode

For multi-client or remote access scenarios, you can run the server as an HTTP service. This is a secondary option compared with running the binary locally over `stdio`.

```bash
# SSE mode (port 8000)
GRAFANA_URL=http://localhost:3000 \
GRAFANA_SERVICE_ACCOUNT_TOKEN=<token> \
./dist/mcp-grafana -t sse

# Streamable HTTP mode (port 8000)
GRAFANA_URL=http://localhost:3000 \
GRAFANA_SERVICE_ACCOUNT_TOKEN=<token> \
./dist/mcp-grafana -t streamable-http
```

Both modes expose a health endpoint at `GET /healthz`.

VSCode / Cursor can connect to the SSE endpoint:

```json
{
  "mcp": {
    "servers": {
      "grafana": {
        "type": "sse",
        "url": "http://localhost:8000/sse"
      }
    }
  }
}
```

### Docker (SSE Mode)

```bash
docker run --rm -p 8000:8000 \
  -e GRAFANA_URL=http://host.docker.internal:3000 \
  -e GRAFANA_SERVICE_ACCOUNT_TOKEN=<token> \
  bingshuoguo/grafana-v8-mcp:latest
```

The Docker image defaults to SSE mode on port 8000. Override with `-t stdio` for stdio mode.

## Development

### Build

```bash
make build          # Build dist/mcp-grafana
make build-image    # Build Docker image
```

### Test

```bash
# Unit tests (no external dependencies)
make test-unit

# Or equivalently
GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn go test -tags unit ./tools/ -v

# Integration tests (requires Docker services)
make run-test-services   # Start Grafana + dependencies
make test-integration

# Cloud tests (requires Grafana Cloud token)
make test-cloud
```

### Lint

```bash
make lint              # golangci-lint
make lint-jsonschema   # Custom jsonschema comma-escape linter
```

### Project Structure

```
cmd/mcp-grafana/          # Server entry point
tools/                    # Grafana 8.4.7 tool implementations
  common.go               #   OpenAPI client factory, doAPIRequest, error helpers
  types.go                #   Contract types (SearchHit, DatasourceModel, AnnotationItem, etc.)
  datasource_resolver.go  #   ID-first datasource resolution logic
  registry.go             #   Tool registration (AddV84Tools)
  health_user_org.go      #   get_health, get_current_user, get_current_org
  dashboard.go            #   get_dashboard_by_uid, upsert_dashboard
  search.go               #   search_dashboards
  folder.go               #   list_folders, create_folder, update_folder
  datasource.go           #   list_datasources, get_datasource, resolve_datasource_ref
  query.go                #   query_datasource
  annotations.go          #   get_annotations, create_annotation, patch_annotation
  legacy_alerting.go      #   list_legacy_alerts, list_legacy_notification_channels
  org_admin.go            #   list_org_users, list_teams
  v84_unit_test.go        #   Unit tests (43 tests)
docs/
  grafana-8.4.7-mcp-tool-spec.md           # Tool contract specification (JSON)
  grafana-8.4.7-mcp-go-struct-mapping.md   # Go struct definitions
  grafana-8.4.7-mcp-implementation-blueprint.md  # Architecture blueprint
```

## License

This project is licensed under the [Apache License, Version 2.0](LICENSE).

[mcp]: https://modelcontextprotocol.io/
