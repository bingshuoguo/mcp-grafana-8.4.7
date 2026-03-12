# Repository Guidelines

## Project Structure & Module Organization
`grafana-v8-mcp` is a Go project with a small Python E2E suite.
- `cmd/mcp-grafana/`: main server entrypoint.
- Root `*.go` files: core server/session/proxy wiring (`mcpgrafana.go`, `session.go`, `tools.go`, `proxied_*.go`).
- `tools/`: tool implementations and most Go test coverage.
- `internal/linter/jsonschema/`: custom linter for `jsonschema` struct tags.
- `tests/`: Python E2E tests (managed with `uv`).
- `testdata/`: dashboards/provisioning fixtures used by tests.
- `docs/`, `examples/`, `observability/`, `assets/`: documentation, examples, telemetry helpers, static assets.

## Build, Test, and Development Commands
- `make build`: build `dist/mcp-grafana` (binary name kept for compatibility).
- `make build-image`: build local Docker image.
- `make run`: run locally in stdio mode.
- `make run-sse`: run with SSE transport, debug logs, and metrics.
- `make run-streamable-http`: run with streamable HTTP transport.
- `make run-test-services`: start local Docker dependencies.
- `make test-unit`: run Go unit tests (`-tags unit`).
- `make test-integration`: run Docker-backed integration tests (`-tags integration`).
- `make test-cloud`: run cloud tests (`-tags cloud`, requires Grafana token).
- `make test-python-e2e`: run Python E2E tests from `tests/`.
- `make lint` / `make lint-jsonschema`: run `golangci-lint` and custom schema checks.

## Coding Style & Naming Conventions
- Follow standard Go formatting (`gofmt`) and keep code `golangci-lint` clean.
- Use clear package boundaries; keep tool logic in `tools/` unless it is shared infrastructure.
- Test file naming follows scope suffixes such as `*_unit_test.go`, `*_integration_test.go`, and `*_cloud_test.go`.
- In `jsonschema` tags, escape commas in descriptions as `\\,` (enforced by custom linter).

## Testing Guidelines
- Always run Go tests with: `GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn`.
- Preferred commands:
  - `GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn spkit test`
  - `GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn spkit run go test ./...`
- Start dependencies with `make run-test-services` before integration/E2E runs.
- Add or update tests with behavior changes, especially for new/changed tools.

## Commit & Pull Request Guidelines
- Follow Conventional Commits seen in history, e.g. `feat(dashboard): ...`, `fix(tests): ...`, `docs: ...`.
- Use concise, imperative commit summaries and include PR references when available (e.g. `(#578)`).
- PRs should include:
  - what changed and why,
  - test evidence (commands run),
  - linked issue/PR context,
  - doc updates (`README.md`/`DEVELOPING.md`) when behavior or workflows change.
