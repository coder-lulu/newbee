# NewBee 仓库全景总览

> **更新时间**：2026-01-17  
> **定位**：统一的多租户运维/CMDB 平台，涵盖核心身份与权限服务（Core）、CI 自动发现（CMDB）、统一数据处理（Unified IO）、运维编排中心（Ops Center）、远程代理（NewBee Proxy/nb-agent）、前端控制台（UI）以及跨项目公共库（common）等。

## 1. 仓库结构速览

- 根目录由 `go.work` 统一协调 Go 1.25.1 工作区（`core`、`cmdb/api|rpc`、`common`、`newbee-proxy`、`ops-center/api|rpc`、`unified-io/api|rpc`）。
- 关键服务均基于 go-zero + ent + `github.com/coder-lulu/newbee-common` 中间件体系；前端统一落在 `ui/` pnpm workspace。
- 运维脚本与模板位于 `scripts/`、`templates/`，配置与架构文档集中在 `configs/`、`docs/`。

### 1.1 工作区模块矩阵

| 模块 | 角色 | 技术栈/依赖 | 主要命令 |
| --- | --- | --- | --- |
| `core/` | 多租户后台（REST+RPC） | go-zero REST & zrpc、ent、newbee-common、中间件 | `cd core && make test`、`make gen-rpc` |
| `cmdb/api` & `cmdb/rpc` | CIType 自动发现服务 | go-zero、ent、Redis、PostgreSQL | `go test ./...`、`make` 任务（按 README 执行） |
| `common/` | 公共 Go 库 | newbee-common（认证/租户/数据权限/审计等） | `cd common && go test ./...` |
| `newbee-proxy/` | 远程连接代理/插件框架 | go-zero、WebSocket、Guacamole 插件、MySQL | `go run ./cmd/proxy -f etc/proxy.yaml`、`./start-proxy.sh` |
| `ops-center/api` & `ops-center/rpc` | 运维编排中心 | go-zero、ent、common 中间件、Ops Worker/Proxy 注册 | `go run ops-center/api/ops.go -f etc/ops.yaml` |
| `unified-io/api` & `unified-io/rpc` | 统一数据处理平台 | go-zero、gRPC、Consul、Prometheus | `cd unified-io && make gen-rpc`、`go run main.go` |

> 其余独立模块（`nb-agent/`、`ui/`、`job/`、`packages/asset-selector-sdk/`）不在 go.work 中，但有独立 go.mod / package.json。

## 2. 核心后端服务

### 2.1 Core（`core/README.md`）
- **职责**：平台级 JWT 认证、租户/数据权限、审计、OAuth、第三方任务调度封装，REST API (`core/api`) 与 gRPC (`core/rpc`) 双组件。
- **关键目录**：`api/internal/svc/service_context.go`（中间件集成）、`api/etc/core.yaml` & `rpc/etc/core.yaml`（配置模板）、`rpc/ent/schema`（多租户实体）。
- **工具链**：`make fmt|test|lint|gen-api|gen-rpc|gen-ent|docker`；Go 1.25 + goctl + ent + Prometheus/Zipkin。
- **部署**：`deploy/docker-compose/*`、`deploy/k8s` 提供 all-in-one / core-only / K8s 模板。

### 2.2 CMDB 自动发现（`cmdb/PROJECT_COMPLETION_SUMMARY.md`、`IMPLEMENTATION_GUIDE.md`）
- **功能**：CIType Auto-Discovery 系统，覆盖 Provider 注册、发现引擎、属性映射、冲突解决、增量更新、执行监控、React+TS 前端。
- **后端结构**：`rpc/internal/discovery/engine/`（执行/转换/冲突/增量模块）、`rpc/ent/schema`（CiTypeDiscoveryConfig 等 26+ 字段）、`rpc/logic`（15 个 gRPC 逻辑）。
- **前端结构**：`frontend/src/components/*`（DiscoveryConfigManagement、AttributeMappingEditor、MonitoringDashboard 等 1000+ 行组件）。
- **交付情况**：项目版本 v2.0.0，全部 Phase 完结，性能指标（P95<50ms、并发 50+）达标。
- **命令**：按文档运行 ent 迁移、go-zero API/RPC 启动，数据库 PostgreSQL14+，Redis/MQ 可选。

### 2.3 Unified IO 平台（`unified-io/docs/README.md`）
- **定位**：统一数据处理 / 流式任务平台，具备自适应限流、背压控制、Consul 服务发现、Prometheus 监控、K8s 适配。
- **目录**：`rpc/internal/processing/*`（engine/worker_pool/task_scheduler）、`ratelimit/`、`monitoring/`、`cloud/`、`optimization/`、`testing/`；`docs/` 提供架构/部署/监控/权限/Outbox/SSH Provider 等 30+ 篇指南。
- **技术栈**：Go 1.21+、go-zero gRPC、Ent ORM、MySQL/PostgreSQL、Redis、Consul、Prometheus/Grafana/Jaeger、Docker/K8s/Helm。
- **运行**：`go mod download` → `make gen-rpc` → `go run main.go`；Docker/K8s 模板及基准测试脚本（`go test -bench`）。

### 2.4 Ops Center（`docs/ops-center-服务设计.md`, `ops-center/WORKER_PROJECT_SUMMARY.md`）
- **职责**：运维编排与治理，统一会话/任务、Proxy/Worker 选路、访问画像、短期凭证与审计聚合。
- **模型**：Session、Task、AccessProfile、ProxyNode、CredentialRef、AuditEvent（均含 `TenantMixin` 与 JSON 灵活字段）。
- **API**：`/ops/session|task|proxy|profile|credential|audit`（11+ handler，PSK 认证用于 worker 注册/心跳，其余需 JWT）。
- **Worker 管理**：`ops-center/rpc` 负责 WorkerRegistry+WorkerManager 双层存储、7 种调度策略、健康检查、指标快照（`ops_workers` & `ops_worker_metrics`）。
- **脚手架**：`scripts/ops-center-generate-and-build.sh` 一键生成/构建 API+RPC，`docs/ops-center-快速开始.md` 演示 Token→Proxy→WS 会话最小闭环。

### 2.5 NewBee Proxy（`newbee-proxy/ROOT_README.md`, `README_QUICKSTART.md`）
- **角色**：远程连接代理服务，提供多协议（SSH/Telnet/RDP/VNC/DB/IPMI/SNMP）会话隧道、插件化协议管理、WebSocket 桥接、连接池与监控。
- **目录**：`cmd/proxy` 主入口、`plugins/`（协议插件）、`internal/`（连接/监控）、`provider/`（第三方集成）、`docs/`（架构、插件、安全、API、运维）。
- **运行**：`./start-proxy.sh` 或 `go run ./cmd/proxy -f etc/proxy.yaml`；`curl /status`、`/health`、`/plugins` 验证；Ops Center 集成需配置 `OpsCenter.Endpoints` 与 PSK。
- **告警**：已知风险（并发安全、内存、日志/配置校验）在 `ROOT_README.md` 中按优先级列出。

### 2.6 nb-agent（`nb-agent/README.md` & `docs/center-design.md`/`proxy-design.md`）
- **定位**：Agent Server + CLI，Go 语言实现的智能运维面板，强调极低占用、低侵入、离线运行、全开源。
- **目录**：`cmd/nb`（服务端入口）、`cmd/cli`（命令行）、`internal/`（核心逻辑）、`panel/` `web/` `storage/`（控制面 UI/数据）、`server/`（HTTP/API）、`pkg/`（通用库）、`nb-agent`/`nb-cli`（可执行产物）。
- **命令**：`make agent`（服务端二进制）、`make cli`、`make test`；配置样例 `config.example.yml`。
- **特性**：支持 amd64/arm64、离线运行、完全开源；UI 截图及合作伙伴列表见 README。

### 2.7 simple-admin-job（`job/README.md`）
- **作用**：Simple Admin 在线定时任务扩展，基于 asynq 的 RPC 模块。
- **结构**：`ent/`（Schema）、`job.proto`+`types/`（RPC 定义）、`jobclient/`（客户端）、`internal/`（逻辑），`Makefile` 内置构建/生成命令。
- **用法**：配合 Core / Unified IO 等服务调用，提供 RPC 触发/调度能力。

## 3. 前端与 SDK

### 3.1 UI Monorepo（`ui/README.md`）
- **定位**：NewBee Admin UI，基于 Vue 3 + TypeScript + Vite + Ant Design Vue + Pinia + Turbo（pnpm workspace 管理）。
- **结构**：`apps/web-antd`（主应用）、`packages/@core|effects|locales|stores|styles|types|utils`、`internal/*`（lint/vite/tsconfig 工具）、`scripts/`（构建辅助）、`docs/`、`images/`。
- **命令**：`pnpm install` → `pnpm dev` / `pnpm dev:antd`（端口 5555）、`pnpm build:antd`、`pnpm lint`、`pnpm format`、`pnpm check:type`、`pnpm check`（循环依赖/依赖版本/类型/拼写）。
- **功能**：RBAC & 数据权限、租户管理、监控中心、生成器、表单设计器；安全性（JWT、请求加密、XSS/CSRF 防护）与性能（懒加载、Gzip、CDN）。
- **Monaco 相关**：`MONACO_*.md` 系列记录 Monaco 编辑器引入/修复过程。

### 3.2 Asset Selector SDK（`packages/asset-selector-sdk/README.md`）
- **定位**：`@newbee/asset-selector-sdk` 通用资产选择器，虚拟滚动+智能分页+插件化+主题系统，支持 CMDB/Kubernetes/Docker 等数据源。
- **内容**：`src/`（组件/插件/服务适配器）、`example/`、`dist/`、`README.md`（Vue 插件/工厂函数示例）、`IMPLEMENTATION_SUMMARY.md`（上线报告）。
- **使用**：`npm|yarn|pnpm add @newbee/asset-selector-sdk`，提供 ConfigPresets、Builder、CMDBAdapter、自定义插件、权限/审计插件等。

## 4. 基础能力与工具

### 4.1 公共库 `common/`（`common/README.md`）
- **模块**：`middleware/*`（Auth/Tenant/DataPerm/Permission/Audit/Encryption/Integration）、`orm/ent|gorm` Hook、`tenant/`、`audit/`、`casbin/`、`config/`、`msg/`、`utils/`（captcha/crypto/jwt/validator 等）。
- **重点**：统一中间件优先级、Ent 多租户 Hook & 数据权限拦截器、JWT/加密工具、Casbin 适配器，Go >=1.24。
- **文档**：`docs/` 目录含认证/租户/数据权限/Hook 使用指南、配置示例、快速参考。

### 4.2 模板与脚本
- **`templates/`**：`newbee-service` 引导脚手架（CLI 交互式创建 REST+RPC 双服务、自动配置端口/DB/Redis、填充 go-zero/ent 代码），`service-template/` 基础模板。`README.md` 详述流程、命名/端口规则、最佳实践。
- **`scripts/ops-center-generate-and-build.sh`**：封装 goctl/ent 生成与构建流程，确保 ops-center API/RPC 一致。
- **根脚本**：`start_service.sh`、`init-databases.sh` 等辅助初始化。

### 4.3 配置与监控
- **`configs/monitoring.yaml`**：统一监控开关（Prometheus、OpenTelemetry、Alert 阈值、缓存命中率目标、邮件/Slack/PagerDuty 通道）。
- **`docs/performance_*`, `middleware_performance_analysis_report.md`**：性能与监控策略参考。

## 5. 文档地图（`docs/` 目录精选）

- **架构与状态**：`ARCHITECTURE_REVIEW_REPORT.md`、`PROJECT_STATUS_AND_NEXT_STEPS.md`、`IMPLEMENTATION_STATUS_AND_PRIORITY.md`。
- **CMDB & 资源发现**：`CMDB_ANALYSIS.md`、`CIType_Auto_Discovery_Design.md`、`CIType_Discovery_Development_Plan.md`、`CMDB统计分析系统设计文档.md`、`cmdb-访问画像-集成设计.md`。
- **数据权限**：`DataPerm_v2.1_Release_Notes.md`、`DATAPERM_ROADMAP.md`、`UNIFIED_IO_DATA_PERMISSION_DESIGN.md`、`dataperm-unification-summary.md`。
- **Ops Center / Proxy**：`ops-center-服务设计.md`、`ops-center-生成与构建脚本.md`、`ops-center-快速开始.md`、`proxy-worker-安全联调指南.md`、`proxy-worker-安全冒烟清单.md`。
- **Kafka/Unified IO**：`KAFKA_PROJECT_SUMMARY.md`、`unified-io-kafka-queue-design.md`、`KAFKA_DESIGN_CHANGELOG.md`、`KAFKA_PRODUCER_IMPLEMENTATION_REPORT.md`。
- **UI 菜单与权限**：`ui-运维中心-菜单与权限设计.md`。
- **其他**：`编码准则.md`（编码规范）、`CLAUDE.md`（ent 更新流程）、`AGENTS.md`（自动化说明）等。

## 6. 典型开发/验证流程

1. **公共库更新**：在 `common/` 扩展中间件/Hooks → `go test ./...` → 确保 `core`/`cmdb` 等模块 `go get` 到最新版本。
2. **服务开发**：使用 `templates/newbee-service` 创建新服务骨架或依托既有 `core/cmdb/...` 目录；严格遵循 `common/middleware/integration` 接入 Auth/Tenant/DataPerm/Audit。
3. **Schema 变更**：修改 `rpc/ent/schema` → `make gen-ent`（参见 `CLAUDE.md`）→ `make gen-rpc` → 更新 API/Logic。
4. **测试**：各 Go 模块执行 `make test` 或 `go test ./...`；前端执行 `pnpm test:unit` / `pnpm test:e2e`（如需），nb-agent/Proxy/Ops Center 根据 README 的验证脚本检查心跳/注册/WS 建链。
5. **部署**：优先使用目录内 `deploy`（Core）、`Dockerfile`（cmdb/newbee-proxy/nb-agent）、`docs/DEPLOYMENT_GUIDE.md`（unified-io）等提供的模板；监控配置参考 `configs/monitoring.yaml`。

## 7. 关联生态与下一步

- **服务关系**：Core 提供统一认证/租户钩子；Ops Center 借助 nb-agent/NewBee Proxy 执行任务与隧道；CMDB/Unified IO 输出资产/数据供 UI 与 SDK（Asset Selector）消费；simple-admin-job 负责异步调度；`common` 贯穿所有服务。
- **前端集成**：`ui/apps/web-antd` 的运维中心菜单已对接 Ops Center Worker 管理 (`docs/ui-运维中心-菜单与权限设计.md`)，配合 Asset Selector SDK 实现跨服务资产选择体验。
- **自动化脚本**：`init-databases.sh`、`start_service.sh` 可与 `ops-center`/`newbee-proxy` README 的一键脚本结合，实现本地演示环境。

> 本文档旨在作为项目入口指南，如需深入请跳转各子目录 README 或 `docs/` 内的专题文档。
