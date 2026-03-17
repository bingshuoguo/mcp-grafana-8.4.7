# Grafana v8 MCP 项目介绍

## 1. 文档概述

本文档用于介绍 `Grafana v8 MCP` 项目的建设背景、项目定位、核心能力、适用边界及内部落地情况，帮助普通研发快速理解该项目的价值与适用方式。

本文侧重回答以下问题：

- 为什么需要单独建设 Grafana v8 MCP
- 该项目解决了什么问题
- 当前提供了哪些核心工具能力
- 适合哪些研发场景使用
- 项目的使用边界和配套文档在哪里

关于客户端配置、接入步骤和使用示例，请参考：

- [公司内部 AI 客户端接入 Grafana MCP 说明](https://confluence.shopee.io/pages/viewpage.action?pageId=3127159550)

## 2. 项目背景

随着公司内部 AI 客户端逐步应用于研发提效，越来越多研发同学希望能够通过自然语言直接访问 Grafana 中的监控资源，例如 dashboard、指标、日志、annotations 和告警信息，以降低监控排障的使用门槛，缩短问题定位时间。

Grafana 官方已开源 MCP Server，但其能力设计主要面向 **Grafana v9.0 及以上版本**。而公司当前使用的 Grafana 版本为 **8.4.7**，与官方 MCP 的目标版本存在明显差异。在实际验证过程中，官方开源 MCP 在当前环境下存在以下问题：

- 部分 tools 依赖的 OpenAPI client 与 Grafana v8.4.7 的 API 行为不兼容
- 某些接口在 v8 环境下返回结构与高版本不一致，导致 tools 无法正常工作
- 官方 MCP 的能力边界与当前内部监控排障场景并不完全匹配

基于以上问题，针对公司当前 Grafana 版本现状，需要一套专门面向 **Grafana v8.4.7** 的 MCP 方案，以保障工具能力的可用性、稳定性和推广价值。

## 3. 项目定位

`Grafana v8 MCP` 是一个专门面向 **Grafana v8** 场景构建的 MCP Server，重点适配 **Grafana 8.4.7**，用于支撑普通研发在 AI 客户端中以自然语言方式完成监控查询和排障辅助分析。

项目的定位可概括为以下几点：

- 为公司内部 Grafana 8.4.7 环境提供可落地的 MCP 能力
- 弥补官方 Grafana MCP 在 v8 场景下的大量 tool 不可用问题
- 围绕实际监控排障需求增强指标、日志和导航类能力
- 通过只读优先的接入方式降低误操作风险
- 以开源项目形式沉淀 Grafana v8 的 MCP 适配方案

开源仓库地址如下：

- `https://github.com/bingshuoguo/grafana-v8-mcp`

## 4. 核心设计思路

为了让 MCP 在 Grafana 8.4.7 环境中稳定运行，项目并未简单复用官方实现，而是针对 v8 API 特征进行了工程化适配。核心设计思路包括以下几个方面。

### 4.1 兼容 Grafana v8 API 行为

项目针对官方 OpenAPI client 与 Grafana v8.4.7 实际行为不一致的问题进行了修复与绕过处理。对于兼容性较差的接口，采用更贴近实际 API 行为的实现方式，以降低因版本差异导致的调用失败风险。

### 4.2 保留查询能力优先

本项目优先保障普通研发最常用的只读查询能力，包括 dashboard 检索、datasource 查询、Prometheus 查询、Loki 日志查询、ClickHouse 查询和 deeplink 生成等。整体上遵循“先可查、再扩展”的建设原则。

### 4.3 以排障场景为中心扩展工具

项目不仅关注基础 Grafana 元数据读取能力，也补充了 Prometheus、Loki、ClickHouse 和统一日志搜索等对研发排障最有价值的工具能力，降低使用者在多个平台之间频繁切换的成本。

### 4.4 支持只读接入模式

项目支持通过 `--disable-write` 参数关闭写工具，适合作为公司内部默认接入方式。这种模式更符合普通研发日常使用场景，也更容易满足安全和权限控制要求。

## 5. 核心能力概览

当前 `Grafana v8 MCP` 已具备一套面向研发排障场景的核心工具集合，覆盖 Grafana 元数据读取、监控查询、日志检索和辅助导航等能力。

### 5.1 核心工具能力表

| Tool | 能力说明 | 关键参数 |
|------|----------|----------|
| `get_health` | 获取 Grafana 健康状态、版本与构建信息 | 无 |
| `get_current_user` | 获取当前用户信息 | 无 |
| `get_current_org` | 获取当前组织信息 | 无 |
| `search_dashboards` | 按标题、标签、目录搜索 dashboard | `query`, `tag`, `folderIds`, `limit`, `page` |
| `get_dashboard_by_uid` | 获取指定 dashboard 的完整定义 | `uid` |
| `get_dashboard_summary` | 获取 dashboard 摘要信息 | `uid` |
| `get_dashboard_panel_queries` | 获取 dashboard 下 panel 的查询语句 | `uid`, `panelId` |
| `list_folders` | 查询目录列表 | `limit`, `page`, `permission` |
| `search_folders` | 按名称搜索目录 | `query`, `limit` |
| `list_datasources` | 查询可用 datasource | `type`, `limit`, `offset` |
| `get_datasource` | 按 `id / uid / name` 查询 datasource | `id`, `uid`, `name` |
| `resolve_datasource_ref` | 将 datasource 引用解析为标准信息 | `id`, `uid`, `name` |
| `query_datasource` | 通过 Grafana `/api/tsdb/query` 执行通用查询 | `from`, `to`, `queries`, `datasource` |
| `query_datasource_expressions` | 执行 Grafana `/api/ds/query` 查询，必要时自动回退 | `from`, `to`, `queries`, `datasource` |
| `query_prometheus` | 执行 PromQL 查询 | `expr`, `start`, `end`, `step`, `queryType` |
| `list_prometheus_metric_names` | 检索 Prometheus 指标名 | `regex`, `limit`, `page`, `datasource` |
| `list_prometheus_label_names` | 查询 Prometheus label 名称 | `datasource`, `start`, `end` |
| `list_prometheus_label_values` | 查询指定 label 的可选值 | `labelName`, `start`, `end`, `limit`, `datasource` |
| `list_prometheus_metric_metadata` | 获取指标元数据 | `metric`, `limit`, `datasource` |
| `query_prometheus_histogram` | 计算直方图分位值，如 p95/p99 | `metric`, `quantile`, `selector`, `start`, `end`, `step` |
| `list_loki_label_names` | 查询 Loki label 名称 | `datasourceUid`, `startRfc3339`, `endRfc3339` |
| `list_loki_label_values` | 查询 Loki label 值 | `datasourceUid`, `labelName`, `startRfc3339`, `endRfc3339` |
| `query_loki_logs` | 查询 Loki 日志 | `datasourceUid`, `logql`, `startRfc3339`, `endRfc3339`, `limit`, `direction` |
| `query_loki_stats` | 获取 Loki 数据统计信息 | `datasourceUid`, `logql`, `startRfc3339`, `endRfc3339` |
| `query_loki_patterns` | 提取 Loki 日志模式 | `datasourceUid`, `logql`, `startRfc3339`, `endRfc3339`, `step` |
| `list_clickhouse_tables` | 查询 ClickHouse 可用表 | `datasourceUid`, `database` |
| `describe_clickhouse_table` | 查询 ClickHouse 表结构 | `datasourceUid`, `database`, `table` |
| `query_clickhouse` | 执行 ClickHouse SQL 查询 | `datasourceUid`, `query`, `start`, `end`, `limit`, `variables` |
| `search_logs` | 在 Loki 或 ClickHouse 中按关键词或模式搜索日志 | `datasourceUid`, `pattern`, `start`, `end`, `limit`, `table` |
| `generate_deeplink` | 生成 dashboard、panel 或 Explore 的 Grafana 链接 | `resourceType`, `dashboardUid`, `panelId`, `datasourceUid`, `timeRange` |
| `get_annotations` | 查询 annotations | `from`, `to`, `dashboardUid`, `dashboardId`, `panelId`, `tags`, `type` |
| `get_annotation_tags` | 查询 annotation tags | `tag`, `limit` |
| `list_legacy_alerts` | 查询 legacy alert rules | `dashboardId`, `panelId`, `state`, `query`, `limit` |
| `list_legacy_notification_channels` | 查询 legacy notification channels | `name` |
| `list_org_users` | 查询当前组织成员 | 无 |
| `list_teams` | 查询团队信息 | `page`, `perPage`, `query` |

### 5.2 常用参数说明

为了便于普通研发理解和使用，以下参数类型最值得关注。

#### 时间范围参数

查询类 tools 普遍支持时间范围控制，常见形式包括：

- 相对时间，例如 `now-1h`、`now-6h`、`now-1d`
- 绝对时间，例如 RFC3339 格式时间
- Prometheus 与通用查询常见参数为 `start`、`end`、`from`、`to`
- Loki 查询常见参数为 `startRfc3339`、`endRfc3339`

#### Datasource 参数

项目支持通过多种方式解析 datasource，常见写法包括：

- `id`
- `uid`
- `name`

这使得 AI 客户端可以更稳定地定位数据源，避免单一标识方式在 v8 环境下带来的兼容性问题。

#### 查询语句参数

不同数据源对应不同查询语法：

- Prometheus 使用 `expr`
- Loki 使用 `logql`
- ClickHouse 使用 `query`
- 通用 Grafana datasource query 使用 `queries`

#### 结果控制参数

部分工具支持 `limit`、`page`、`offset` 等参数，用于控制结果集规模，避免返回过多内容影响 AI 客户端上下文质量。

## 6. 典型使用场景

`Grafana v8 MCP` 主要服务于普通研发在日常开发、测试与问题定位过程中的监控检索需求，典型场景包括：

- 按服务名称或关键字搜索 dashboard
- 查询某个指标在过去一段时间内的变化趋势
- 搜索 Loki 中包含指定错误关键字的日志
- 查询 ClickHouse 中某张日志表的结构和样例数据
- 生成 dashboard 或 panel 的直达链接，便于协作排查
- 辅助查看 annotations、legacy alerts、组织用户等信息

对应的自然语言请求示例如下：

- “帮我搜索标题里包含 error rate 的 dashboard”
- “查询过去 1 小时 payment 服务的错误率趋势”
- “搜索最近 30 分钟 Loki 中包含 timeout 的日志”
- “列出某个 ClickHouse datasource 下有哪些表”
- “帮我生成这个 dashboard 的 Grafana deeplink”

## 7. 内部落地情况

当前 `Grafana v8 MCP` 已在公司内部实际接入使用，并对监控排障效率提升产生了正向价值。

目前已接入团队包括：

- MAP Search 团队
- QA 团队

通过将 Grafana 能力以 MCP 的形式接入 AI 客户端，研发同学可以在更少上下文切换的情况下完成 dashboard 检索、指标查询、日志搜索和问题辅助分析，从而缩短监控排障路径，提高问题定位效率。

## 8. 使用边界与限制

为避免误用，本文明确说明本项目的适用边界如下：

- 本项目主要面向 **Grafana v8**
- 当前重点验证版本为 **Grafana 8.4.7**
- 不以 Grafana v9 及以上版本为主要兼容目标
- 更适合读场景、查询场景和辅助分析场景
- 写操作工具不建议作为普通研发默认能力开放

因此，`Grafana v8 MCP` 应理解为一套针对当前内部 Grafana 版本环境所建设的适配方案，而非面向所有 Grafana 版本的通用 MCP 实现。

## 9. 文档与仓库链接

为方便后续推广与维护，相关资料统一如下：

- 项目开源仓库：`https://github.com/bingshuoguo/grafana-v8-mcp`
- 公司内部接入文档：[公司内部 AI 客户端接入 Grafana MCP 说明](https://confluence.shopee.io/pages/viewpage.action?pageId=3127159550)

建议文档分工如下：

- 本文用于介绍项目背景、价值、能力和边界
- 接入说明用于介绍客户端配置方式、接入步骤和使用建议

## 10. 总结

`Grafana v8 MCP` 的建设目标并不是简单复用官方 Grafana MCP，而是针对公司当前 **Grafana 8.4.7** 环境的现实问题，提供一套可用、稳定、适合推广的 MCP 解决方案。

该项目一方面解决了官方 MCP 在 v8 环境下大量 tool 不可用的问题，另一方面围绕研发实际排障需求增强了 Prometheus、Loki、ClickHouse 和日志搜索等核心能力，并通过只读优先的接入模式降低了使用风险。

对于普通研发而言，`Grafana v8 MCP` 提供了一条更低门槛、更高效率的监控查询入口，也为公司内部 AI 辅助研发场景提供了更贴合现状的基础能力支撑。
