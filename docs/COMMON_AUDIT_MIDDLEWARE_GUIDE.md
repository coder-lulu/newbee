# 审计中间件集成与最佳实践

本文档总结 `common/middleware/audit` 的关键能力、集成步骤以及新服务落地时需要关注的要点。

## 1. 中间件能力总览

- **统一插件入口**：通过 `common/middleware/integration.Setup` 自动挂载 `AuditPlugin`，配合其他租户、认证中间件形成完整链路。
- **高性能写入**：`AuditPlugin` 首选实现 `AuditRPCClient` 的写入器，避免反射；默认内置 `BuiltinAuditWriter` 负责将审计数据转换为 Core RPC 的 `AuditLogInfo` 并持久化。
- **异步可靠队列**：内置 `AsyncAuditQueue` 支持多协程写入、失败重试与死信文件降级（`/tmp/audit_dead_letter.log`）。
- **敏感数据防护**：对请求/响应体进行尺⼨限制和敏感字段过滤，确保落库数据符合安全要求。
- **可配置的网络信息**：支持自定义真实 IP 头（`realIpHeader`），自动拆分 `X-Forwarded-For` 等头部并回退到 `RemoteAddr`。
- **审计字段完善**：标准化 `ResourceType`、`ResourceID`、`Metadata`、`UserName` 等字段，方便下游统计分析。

## 2. 新服务集成步骤

1. **实现 `AuditSvcProvider`**：服务上下文必须提供 `GetCoreRpcClient()`，返回 Core RPC 客户端或自定义实现。
2. **调用统一入口**：

   ```go
   result, err := integration.Setup(&integration.Config{
       Redis:               svc.Redis,
       JWTSecret:           jwtSecret,
       Mode:                integration.Production,
       ApiResourceProvider: svc.NewApiProvider(),
       AuditWriter:         audit.NewBuiltinAuditWriter(svc), // 或自定义 writer
       Middleware:          &svc.Config.Middleware,
   })
   ```

3. **保存集成结果**：将 `ContextManager` 与 `Middlewares` 回填到服务上下文，在启动时通过 `integration.ApplyToServer` 注入链路。
4. **配置清单**：
   - 在 `configs/<service>.yaml` 中启用 `middleware.audit.enabled` 并维护 `skipPaths`。
   - 如部署在反向代理后，设置 `middleware.audit.realIpHeader`（例如 `X-Forwarded-For`）。
   - 大流量或文件流接口可关闭响应捕获：`middleware.audit.captureResponseBody: false`。
   - 异步参数可根据服务 QPS 调整 `asyncWorkers`、`asyncBufferSize`。

## 3. 审计数据映射与附加信息

- **ResourceType/ResourceID**：
  - `ResourceType` 默认使用资源名称，若缺失则回退到规范化路径。
  - `ResourceID` 默认使用规范化路径，可由业务在 `AuditLogData` 上覆写。
- **UserName**：插件会自动读取上下文中的 `username`；若缺失则回退到 `userId`，建议认证链路在 `ContextManager` 中设置用户名。
- **Metadata**：中间件默认追加 `request_id`、`trace_id` 等上下文字段，业务可自行扩展 `AuditLogData.Metadata`。
- **真实 IP**：调用 `resolveClientIP` 顺序：`realIpHeader` → `RemoteAddr` 主机段 → `RemoteAddr` 原值。

## 4. 注意事项

- **RPC 健康性**：内置写入器在 Core RPC 不可用时会将失败记录写入死信文件，请配置监控及时处理。
- **循环调用保护**：`HighPerformanceCoreAuditWriter` 自动跳过审计相关 API，避免递归写入；扩展新审计接口时需同步维护路径前缀列表。
- **请求/响应体**：默认截取 1MB 数据并进行敏感字段过滤，如不希望持久化响应体可通过配置禁用。
- **跨服务复用**：非 Core 服务建议沿用 `audit.NewBuiltinAuditWriter`；如需直连第三方审计系统，可实现 `AuditRPCClient` 并传入自定义 `AuditWriter`。

## 5. 常见问题排查

| 现象 | 可能原因 | 建议处理 |
| ---- | -------- | -------- |
| 无法写入审计表 | Core RPC 不可用或 `GetCoreRpcClient` 返回 `nil` | 检查健康探测、实现 `AuditSvcProvider` 降级逻辑 |
| 审计日志缺失用户名 | 上下文未注入 `username` | 确保认证中间件调用 `ContextManager.SetFullAuthContext` |
| IP 地址显示为 `IP:port` | 未配置 `realIpHeader` | 在配置中声明可信头部 |
| 响应体过大被截断 | 达到 `maxBodySize` 限制 | 调整配置或关闭响应捕获 |

更多示例可参考 `common/middleware/integration/UNIFIED_EXAMPLES.md` 中的审计部分。
