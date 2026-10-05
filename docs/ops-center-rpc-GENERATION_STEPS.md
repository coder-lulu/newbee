# Ops Center RPC 代码生成与迁移指引（遵循 CLAUDE.md）

本指引确保所有业务代码为真实实现，且与其它微服务保持一致。

## 1. 准备工作
- Schema 已定义：`ops-center/rpc/ent/schema/accessprofile.go`（包含 TenantMixin）
- Proto 已定义：`ops-center/rpc/desc/ops.proto`
- 遵循 CLAUDE.md：Ent 生成 → RPC 生成 → 注册 Hook/拦截器 → 接线。

## 2. 生成代码（本地执行）
```bash
cd ops-center/rpc
# 生成 Ent 代码
go run entgo.io/ent/cmd/ent generate ./ent/schema --feature sql/execquery,intercept,sql/modifier

# 生成 RPC 代码（使用 goctls）
make gen-rpc
```

生成后将出现：
- `internal/svc` 标准 ServiceContext
- `internal/server` RPC Server
- `internal/logic` 逻辑实现骨架
- `types/ops` Proto 映射类型

## 3. 注册多租户与数据权限（必选）
在 `internal/svc/service_context.go` 中：
```go
// 初始化 Ent Client 后
// db := ent.NewClient(...)
db.Use(hooks.TenantMutationHook())
db.Intercept(hooks.TenantQueryInterceptor())
// 如启用数据权限
hooks.RegisterDataPermissionInterceptorsWithTenant(db, "users", "departments", "positions", "roles")
```

## 4. 迁移业务逻辑（避免生成覆盖）
- 业务代码放在非生成目录，例如：`internal/domain/`、`pkg/`。
- 生成的 `internal/logic` 中仅调用 `domain` 层，避免生成覆盖对业务的影响。
- 现有 API 业务保持在 `ops-center/api`，RPC 仅提供存取操作。

## 5. API 接入 RPC 客户端
- 在 `ops-center/api/internal/svc/service_context.go` 中创建 RPC 客户端：
```go
cli := zrpc.MustNewClient(c.OpsRpc)
opsCli := opsclient.NewOps(cli) // 生成代码中的 NewOps
svcCtx.OpsClient = opsCli
```
- `ops-center/api/internal/handler/profile/*` 将自动使用 RPC 分支；未配置时回退内存实现。

## 6. 新增 Session 持久化（本次补充）
- 已添加 Ent Schema：`ent/schema/session.go`
- 已在 Proto 添加 Session 消息与列表请求/响应（待生成服务接口）：`desc/ops.proto`
- 生成后将获得 `Session` 的 CRUD 能力；API 层可切换至 RPC 持久化，替换当前内存版 `SessionStore`。

生成步骤示例：
```bash
cd ops-center/rpc
go run entgo.io/ent/cmd/ent generate ./ent/schema --feature sql/execquery,intercept,sql/modifier
make gen-rpc
```

## 7. 验证
- 启动 RPC 服务：`go run ops.go -f etc/ops.yaml`
- 启动 API 服务：`go run ops-center/api/ops.go -f ops-center/api/etc/ops.yaml`
- 验证画像 CRUD、会话创建、Proxy 注册/心跳与选路均可用。

## 8. 注意事项
- 不要在生成目录（internal/server、internal/logic、types）中写业务代码。
- Schema 更新必须走 CLAUDE 流程；禁止原生 SQL。
- 如需自定义初始化逻辑，可在 `internal/bootstrap` 新增并由 main 调用。

## 9. AccessProfile 列表排序下沉到 RPC（本次补充）
- 在 `desc/ops.proto` 的 `AccessProfileListReq` 增加排序字段：
  - `optional string sort_by = 13; // created_at|updated_at`
  - `optional string order = 14;   // asc|desc`
- 在 `internal/logic/access_profile/get_access_profile_list_logic.go` 中按字段进行排序：
  - `created_at` → `accessprofile.ByCreatedAt`
  - `updated_at` → `accessprofile.ByUpdatedAt`
- 生成步骤与校验：
  - 跑 `make gen-rpc` 生成最新 pb/types 与 server/logic 框架
  - API 层直接透传 `sortBy/order`，不再做本地排序

## 10. 下一步：CredentialRef / AuditEvent 的 Proto 与 RPC 生成
- 在 `desc/ops.proto` 增加以下消息与方法（示例）：
  - `CredentialRefInfo`/`CredentialRefListReq`/`CredentialRefListResp` + `create/update/getById/getList/delete`
  - `AuditEventInfo`/`AuditEventListReq`/`AuditEventListResp` + `create/update/getById/getList/delete`
- 执行生成：
  - `make gen-rpc`（或运行 `scripts/ops-center-generate-and-build.sh`）
- 生成后：
  - 在 `internal/server/newbee_ops_rpc_server.go` 注册 CredentialRef/AuditEvent 方法
  - 在 `internal/logic/credentialref` 与 `internal/logic/auditevent` 中填充真实 JSON 解析与分页（已提供模板逻辑，可直接对接）
  - 在 API 侧扩展 `internal/rpcclient/ops_client.go` 并将 Credential/Audit 的 handler 优先切换走 RPC
