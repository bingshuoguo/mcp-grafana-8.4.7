# VS Code and GitHub Copilot

This guide helps you set up the `mcp-grafana` server for VS Code with GitHub Copilot agent mode.

## Prerequisites

- VS Code with GitHub Copilot extension
- Grafana 8.4.7 (or another 8.x release with the same API surface)
- `mcp-grafana` binary installed locally, or Docker available

## Important

GitHub Copilot in VS Code is one of the few clients here that uses **SSE transport**, not local `stdio`. You still install the same `mcp-grafana` binary, but Copilot connects to a running HTTP endpoint instead of launching the process directly.

## Install the binary

```bash
go install github.com/bingshuoguo/grafana-v8-mcp/cmd/mcp-grafana@latest
```

Or download the archive for your platform from [GitHub Releases](https://github.com/bingshuoguo/grafana-v8-mcp/releases).

## Setup

### 1. Start the MCP server

```bash
export GRAFANA_URL="http://localhost:3000"
export GRAFANA_SERVICE_ACCOUNT_TOKEN="<your-token>"
mcp-grafana --transport sse --address localhost:8000
```

Or with Docker:

```bash
docker run --rm -p 8000:8000 \
  -e GRAFANA_URL=http://host.docker.internal:3000 \
  -e GRAFANA_SERVICE_ACCOUNT_TOKEN=<your-token> \
  bingshuoguo/grafana-v8-mcp:latest --transport sse --address :8000
```

### 2. Configure VS Code

Add to your VS Code settings (`settings.json`):

```json
{
  "github.copilot.chat.mcpServers": {
    "grafana": {
      "url": "http://localhost:8000/sse"
    }
  }
}
```

Or use workspace settings (`.vscode/settings.json`) for project-specific config.

## Debug mode

Start the server with debug logging:

```bash
mcp-grafana --transport sse --address localhost:8000 --debug
```

## Verify configuration

1. Restart VS Code after configuration changes
2. Open Copilot Chat (Ctrl+Shift+I)
3. Type: `@grafana list dashboards`
4. If tools are available, Copilot will query Grafana

## Troubleshooting

**Server not connecting:**

- Verify server is running: `curl http://localhost:8000/sse`
- Check firewall allows port 8000
- Restart VS Code after configuration changes

**Tools not appearing:**

- GitHub Copilot agent mode required (may need Copilot Chat enabled)
- Check VS Code output panel for MCP errors

## Running as a service

For persistent server, create a systemd unit or launchd plist.

**Linux systemd** (`~/.config/systemd/user/mcp-grafana.service`):

```ini
[Unit]
Description=Grafana MCP Server
After=network.target

[Service]
ExecStart=/path/to/mcp-grafana --transport sse --address localhost:8000
Environment=GRAFANA_URL=http://localhost:3000
Environment=GRAFANA_SERVICE_ACCOUNT_TOKEN=<your-token>
Restart=always

[Install]
WantedBy=default.target
```

Enable with:

```bash
systemctl --user enable --now mcp-grafana
```

## Read-only mode

```bash
mcp-grafana --transport sse --address localhost:8000 --disable-write
```
