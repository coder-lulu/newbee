# Ops Center 服务设计

> 角色：运维编排与治理面。统一会话/任务、选路、凭证、审计、与 CMDB 的访问画像绑定。
> 框架：go-zero + ent + common/middleware/integration（Authority, TenantCheck, DataPerm, Audit）。

## 1. 模块职责
- 会话管理：创建/关闭/查询会话，生成短期 Session Token，选择 Proxy WS 入口。
- 任务编排：批量命令/文件分发/巡检，executor 选择（nb-agent 优先，SSH/Telnet 兜底）。
- 访问画像：CI 访问能力与偏好（协议/端口/凭证/跳板/首选 Proxy）维护。
- Proxy 管理：Proxy 节点注册/心跳/健康/容量与选路。
- 凭证引用：对接密管（后续），当前以短期令牌为主。
- 审计收敛：会话与任务审计索引（命令回放/录屏索引）。

## 2. 技术选型与中间件
- go-zero rest 服务，统一通过 `integration.ApplyToServer(server, svc.IntegrationResult)` 挂载中间件。
- ent + Tenant/DataPerm Hook：所有实体包含 `TenantMixin`，禁止原生 SQL。
- 审计：common/middleware/audit 的 BuiltinAuditWriter，通过 Core RPC 写入审计索引。

## 3. 领域模型（Ent）
- Session：tenant_id, user_id, ci_id, protocol, proxy_id, endpoint, status, started_at, ended_at, audit_ref
- Task：tenant_id, creator_id, task_id, name, ci_ids(json), executor, command_content, command_timeout, command_env(json), command_workdir, status_str, result_output, error_msg, exit_code, remote_task_id, start_time, end_time, tags(json)
- AccessProfile：ci_id, capabilities, ports, credential_ref, prefer_proxy, jump_chain, tags
- ProxyNode：proxy_id, region, az, endpoints, capabilities, labels, status, load, capacity, last_heartbeat
- CredentialRef：provider, ref, scope, created_by
- AuditEvent：event_id, type(session|task), subject(user/tenant/ci), timestamps, meta

说明：所有实体含审计字段与 `TenantMixin`；对高变字段采用 JSON。

## 4. API 设计
- /ops/session
  - POST /create → { sessionId, wsUrl, token, proxyId, expiresAt }
  - POST /close → { ok }
  - GET /{id}, GET /?page=&status=&ciId=
- /ops/task
  - POST /create → { taskId }
  - POST /{id}/start | /cancel
  - GET /{id} | /list | /logs | /result
- /ops/proxy
  - POST /register（Proxy→Center）
  - POST /heartbeat（Proxy→Center）
  - GET /pick（Center/前端）
- /ops/profile：CRUD CI 访问画像
- /ops/credential：/bind、/issue-short-token
- /ops/audit：审计检索/导出

说明：所有 API 通过 Authority, TenantCheck, DataPerm 保护；系统级操作用 SystemContext 拦截器。

## 5. 令牌与安全
- Session Token/Task Token（JWT 短期）：claims 包含 tenantId/userId/ciId/roleCodes/permScope/expiry/nonce。
- JWKS 轮转：提供 /ops/.well-known/jwks.json（后续），Proxy 定期刷新。
- 速率与配额：按租户/用户/CI 维度做限流（中间件层）。

## 6. 选路策略
- agent 在线优先 → nb-agent 执行；否则 Proxy 协议直连。
- Proxy 选择：region/az > 可达性/ACL > 能力标签 > 当前负载/配额 > 故障域避让。

## 7. CMDB 集成
- 优先在 Ops Center 存储 AccessProfile，引用 CMDB CI；仅拉取必要字段。
- 画像与 CI 变更同步：通过轮询/订阅（后续）更新本地缓存。

## 8. 原子任务（含关键点与测试）
1) 服务骨架与中间件
   - 关键点：统一中间件、租户/数据权限 Hook、生效校验
   - 测试：租户隔离/DataPerm、API 冒烟
2) 模型与迁移（Session/Task/Profile/ProxyNode/CredentialRef/Audit）
   - 关键点：TenantMixin、审计字段
   - 测试：Ent CRUD + Hook、迁移脚本
3) 会话 API + Token 签发
   - 关键点：claims 正确性、过期与权限裁剪
   - 测试：创建→worker WS 鉴权联调、过期/伪造拒绝
4) 任务 API（nb-agent 优先）
   - 关键点：executor 选择与幂等；错误回溯
   - 测试：多目标批量执行、结果拉取
5) Proxy 注册/心跳/选路
   - 关键点：状态一致性、过期淘汰、熔断
   - 测试：多 Proxy 选路、心跳乱序/抖动
6) 访问画像 CRUD + CI 绑定
   - 关键点：校验与兼容 CMDB 字段
   - 测试：画像驱动会话参数正确
7) 短期凭证/引用
   - 关键点：最小权限与脱敏
   - 测试：签发/过期/轮转
8) 审计聚合
   - 关键点：写入性能与索引
   - 测试：高并发写入与查询

## 9. 验收标准
- 全链路租户与权限隔离有效；
- 会话/任务端到端成功；
- Proxy 选路稳定、心跳健康；
- 基本审计可检索；
- 压测通过既定阈值。

## 变更记录
- 2025-10-03
  - AccessProfile 列表支持 JSON 过滤（MySQL JSON_CONTAINS/JSON_EXTRACT 与 PostgreSQL jsonb @>），包含字段：capabilities、ports、jump_chain、tags。
  - Session RPC/API 返回补充 created_at/updated_at（API 层改为透传 RPC 返回的 CreatedAt）。
  - AccessProfile 列表排序/分页正式下沉至 RPC：`AccessProfileListReq` 新增 `sort_by`、`order` 字段，RPC 端按 `created_at/updated_at` 排序；API 侧仅透传参数。
  - 收敛依赖：去除 simple-admin-common，统一替换为 `github.com/coder-lulu/newbee-common`（i18n/pointy）。
  - 修复 RPC 逻辑别名与生成残留：统一使用 `newbee_ops_rpc` 别名，消除 `newbee-ops-rpc` 非法标识符；修正 Task/Session 逻辑，移除 goctl 生成风格的 `SetNotNil*`/`.Page()` 用法，改为真实 JSON 解析与手工分页（Count + Limit/Offset）。
  - 现状：ops-center/rpc 已可独立编译；ops-center/api 受网络限制暂未 tidy/编译（CI/本地可运行 `go mod download && go build`）。后续将补充 CredentialRef/AuditEvent 的 proto 与 RPC 生成，并在 API 端切换优先走 RPC。
