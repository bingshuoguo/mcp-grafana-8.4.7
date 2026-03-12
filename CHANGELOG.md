# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-03-12

### Added

- `v84` tool profile targeting Grafana 8.4.7 REST API with 20 MVP tools (15 read-only + 5 write)
- ID-first datasource resolution for compatibility with Grafana 8.x API behaviour
- Raw HTTP fallbacks (`POST /api/tsdb/query`, `POST /api/dashboards/db`) where the upstream OpenAPI client models diverge from 8.4.7
- Legacy alerting tools: `list_legacy_alerts`, `list_legacy_notification_channels`
- `resolve_datasource_ref` composite tool for canonical datasource resolution by ID, UID, or name
- Layered tool registry with P0/P1/P2 priority tiers; `--disable-write` flag disables all write tools
- Comprehensive test suite with unit tests and failure reports for v84 API surface

### Changed

- Removed upstream tool profiles that target Grafana 9+ APIs (Unified Alerting, Incident, OnCall, SLO, etc.)
- Removed proxied runtime and unused tool stubs not applicable to Grafana 8.x

### Fixed

- Query compatibility: inject `datasourceId` into `POST /api/tsdb/query` request body
- Dashboard upsert: use raw HTTP path to avoid 8.x OpenAPI model mismatches

[1.0.0]: https://github.com/bingshuoguo/grafana-v8-mcp/releases/tag/v1.0.0
