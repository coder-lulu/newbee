# 租户上下文传播问题与解决方案

## 问题描述

**现象**：
- 租户2的管理员在后台创建新用户时，新用户的 `tenant_id` 被设置为1，而不是2
- 所有在租户2中创建的数据，`tenant_id` 都默认为1

**影响**：
- 多租户隔离失效
- 数据安全问题
- 租户间数据泄漏风险

## 根因分析

### 数据流

1. **API层** (core/api)：
   - ✅ 中间件从JWT中提取租户ID，放入context
   - ✅ `SystemContextClientInterceptor` 将context中的租户ID添加到gRPC **outgoing** metadata
   - ✅ 代码位置：`/opt/code/newbee/core/api/internal/svc/service_context.go:363-365`

2. **gRPC传输层**：
   - ✅ 客户端的outgoing metadata自动转换为服务端的incoming metadata
   - ✅ 这是gRPC框架的标准行为

3. **RPC层** (core/rpc)：
   - ❌ **缺少服务端拦截器**将incoming metadata提取到context value中
   - ❌ RPC handler收到的context中只有incoming metadata，**没有context value**
   - ❌ 统一Hook系统的 `fromContext` 函数优先查找context value
   - ❌ 当context value不存在时，虽然会回退到查找incoming metadata，但存在问题

### 核心问题

`GetTenantIDFromCtx` 函数（`/opt/code/newbee/common/orm/ent/entctx/tenantctx/tenant_ctx.go:34-60`）的执行流程：

```go
func GetTenantIDFromCtx(ctx context.Context) uint64 {
    // 1. 首先从context value中查找
    if tenantId, ok = ctx.Value(keys.TenantIDKey).(string); !ok {
        // 2. 如果没有，从gRPC incoming metadata中查找
        if md, ok := metadata.FromIncomingContext(ctx); !ok {
            // 3. 如果还没有，返回默认值1
            return entenum.TenantDefaultId
        } else {
            if data := md.Get(keys.TenantIDKey.String()); len(data) > 0 {
                tenantId = data[0]
            } else {
                // 4. metadata中也没有，返回默认值1
                return entenum.TenantDefaultId
            }
        }
    }
    // ... 转换并返回
}
```

**问题点**：
- 虽然理论上步骤2应该能从incoming metadata中获取租户ID
- 但在实际运行中，incoming metadata可能为空或者key不匹配
- 最终返回 `entenum.TenantDefaultId` (值为1)

## 解决方案

### 方案1：服务端拦截器（推荐）⭐

**原理**：在RPC服务端添加拦截器，从incoming metadata中提取租户ID等信息，注入到context value中。

**实现**：

1. 创建服务端拦截器文件：`/opt/code/newbee/common/orm/ent/hooks/grpc_server_interceptor.go`

```go
// ContextPropagationServerInterceptor 从gRPC incoming metadata中提取上下文信息
func ContextPropagationServerInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
            return handler(ctx, req)
        }

        newCtx := ctx
        cm := keys.NewContextManager()

        // 提取并注入租户ID
        if tenantIDs := md.Get(keys.TenantIDKey.String()); len(tenantIDs) > 0 {
            tenantID := tenantIDs[0]
            if tenantID != "" && tenantID != "0" {
                newCtx = cm.SetTenantID(newCtx, tenantID)
            }
        }

        // 提取其他信息...
        return handler(newCtx, req)
    }
}
```

2. 在RPC服务启动时注册拦截器：

**问题**：go-zero的 `zrpc.MustNewServer` 不支持直接通过选项注册自定义拦截器。

**变通方案**：需要找到go-zero支持的拦截器注册方式。

可能的注册方式：
- 通过配置文件 `etc/core.yaml` 中的 `Middlewares` 字段
- 使用go-zero的中间件机制
- 直接修改 `grpc.Server` 初始化逻辑

### 方案2：中间件包装（待验证）

在每个RPC logic的构造函数中，手动从metadata提取租户ID并更新context。

**缺点**：
- 需要修改所有logic文件
- 代码重复
- 容易遗漏

### 方案3：修改GetTenantIDFromCtx（不推荐）

修改 `GetTenantIDFromCtx` 函数，增强从incoming metadata中提取租户ID的逻辑。

**缺点**：
- 治标不治本
- 没有解决context value缺失的根本问题
- 其他依赖context value的功能仍会失败

## 当前状态（2025-10 更新）

1. ✅ 已创建并注册服务端拦截器：
   - 拦截器实现位于 `/opt/code/newbee/common/orm/ent/hooks/grpc_server_interceptor.go`
   - 在 `core/rpc/core.go` 中通过 `s.AddUnaryInterceptors(hooks.ContextPropagationServerInterceptor())` 和 `s.AddStreamInterceptors(...)` 完成注册

2. ✅ API → RPC → Hook 全链路租户上下文链路已经闭环：
   - `AuthPlugin` 将 `tenantId`/`originalTenantId` 写入 context value + gRPC metadata
   - gRPC 服务端拦截器在进入 logic 之前恢复 context value，统一 Hook 与 TenantCheck 均能读到正确租户

3. ⚠️ 建议：后续新增 gRPC 服务必须重复以上注册动作，或抽象为统一的 `zrpc.ServerOption` 封装，避免遗漏

## 使用与扩展注意事项

1. **TenantCheck 中间件顺序**：必须注册在 `Auth` 之后、`Permission/DataPerm` 之前，确保请求在任何权限判断之前完成租户校验。
2. **继承/自定义租户信息提供器**：若业务需要额外校验项，实现 `framework.TenantInfoProvider` 并在 `CoreServices` 注入；`TenantCheckPlugin.initTenant()` 会优先调用该 Provider 获取租户状态。
3. **系统上下文调用**：仅在初始化脚本或跨租户管理流程中使用 `hooks.NewSystemContext`，业务逻辑禁止绕过租户隔离。若确有需要，务必添加审计日志。
4. **超级管理员租户切换**：前端/中间件会记录 `originalTenantId`；RPC 逻辑在查询时需优先使用 `originalTenantId` 读取超级租户数据，避免租户锁死。参见 `core/rpc/internal/logic/user/get_user_by_id_logic.go`。
5. **测试建议**：新增功能时至少覆盖以下场景：
   - 正常租户请求：`tenant_id` 保持在 context、hook 不回退默认值
   - 租户停用：TenantCheck 返回 `CodeTenantInactive`
   - 超管切换：`originalTenantId` 与活动租户不同，仍能读取到超级租户级配置

## 已完成事项

- [x] 研究 go-zero 拦截器注册机制并在 `core/rpc` 中落地
- [x] 验证租户上下文在 Hook 中被正确识别
- [x] 在文档中补充扩展指引以及测试用例建议

## 参考资料

- go-zero gRPC拦截器文档
- gRPC Go拦截器最佳实践
- 统一Hook系统文档：`/opt/code/newbee/common/docs/统一Hook系统使用指南.md`

---

**创建时间**：2025-10-09 04:30
**状态**：🔴 待解决
**优先级**：⭐⭐⭐⭐⭐ 高（影响多租户核心功能）
