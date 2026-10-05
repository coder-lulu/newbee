# Ops Center API 代码生成与迁移指引（遵循 CLAUDE.md）

本指南将现有手写 handler 逐步迁移到基于 .api 定义的 goctl 生成骨架，确保“先定义 API，再生成，再迁移逻辑”。

## 1. API 定义
- 文件：`ops-center/api/desc/ops.api`
- 覆盖：Session（create/close/get/list）、Proxy（register/heartbeat/pick）、AccessProfile（CRUD）

## 2. 生成命令
```
cd ops-center/api
make gen-api
# 注：命令打印在控制台；按需替换 goctl 路径与 --home
```

生成后目录包含：
- `internal/handler` 与 `internal/logic` 的骨架代码
- `ops.go` 启动入口（已存在，可对比合并）

## 3. 迁移策略
- 迁移业务逻辑：
  - 将当前 `internal/handler/*` 中的逻辑移动至生成后的 `internal/logic/*`，handler 仅做 `logic.NewXxxLogic(ctx, svcCtx).Xxx(req)` 调用与参数/响应绑定。
  - Session 相关：保留 `SessionStore` 作为过渡实现，后续替换为 ent 持久化（遵循 CLAUDE 流程）。
- 中间件与 ServiceContext：
  - 继续保留 `integration.ApplyToServer` 与 `ServiceContext` 的初始化；若生成入口覆盖，请按现有版本合并。

## 4. 测试与验证
- 运行已有单测或编写最小单测校验路由绑定（create/close/get/list）。
- 手工冒烟：
  - `POST /ops/session/create` → 拿到 `sessionId`
  - `GET /ops/session/:id` → 返回详情
  - `GET /ops/session/list?status=active` → 列表
  - `POST /ops/session/close` → ok=true

## 5. 后续演进
- 定义 `Session` 的 ent schema 与 RPC，替换内存版。
- 接入 `RegisterDataPermissionInterceptorsWithTenant`（如会话需要数据权限）。
- 将 Worker 鉴权与审计进一步与中台统一（可保留网关特性）。

