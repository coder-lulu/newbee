# 统一权限配置方案设计

## 📋 设计目标

1. **单一数据源**：所有权限规则统一存储在 `sys_casbin_rules` 表
2. **配置一致性**：接口权限和数据权限使用相同的配置机制
3. **简化管理**：消除多处配置，降低维护成本
4. **向后兼容**：平滑迁移，不影响现有功能

## 🏗️ 架构设计

### 统一权限模型

```
┌─────────────────────────────────────────────────────────┐
│           sys_casbin_rules 统一权限规则表                │
│                                                          │
│  ┌─────────────────────────────────────────────────┐  │
│  │ 接口权限规则 (ptype=p, v3=allow/deny)           │  │
│  │ - 角色可访问哪些API接口                          │  │
│  │ - 支持租户隔离（tenant_id）                      │  │
│  └─────────────────────────────────────────────────┘  │
│                                                          │
│  ┌─────────────────────────────────────────────────┐  │
│  │ 数据权限规则 (ptype=d, v3=data_scope)           │  │
│  │ - 角色可访问哪些数据范围                         │  │
│  │ - v3: all/own_dept/own_dept_and_sub/own         │  │
│  │ - v4: custom_dept_ids (JSON数组)                │  │
│  └─────────────────────────────────────────────────┘  │
│                                                          │
│  ┌─────────────────────────────────────────────────┐  │
│  │ 角色继承规则 (ptype=g)                          │  │
│  │ - 用户拥有哪些角色                               │  │
│  │ - 角色继承关系                                   │  │
│  └─────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### 规则类型定义

#### 1. 接口权限规则（ptype=p）

**用途**：控制角色可以访问哪些API接口

**字段映射**：
```
ptype: p（策略规则）
v0:    角色代码（如：admin, manager）
v1:    租户ID（domain，支持多租户）
v2:    资源路径（如：/api/user/list）
v3:    HTTP方法（如：GET, POST）
v4:    保留（可用于条件表达式）
v5:    保留（可用于优先级）
```

**示例**：
```sql
-- admin角色可以访问租户1的用户列表接口
INSERT INTO sys_casbin_rules (ptype, v0, v1, v2, v3, tenant_id, service_name, status)
VALUES ('p', 'admin', '1', '/api/user/list', 'GET', 1, 'core', 1);

-- manager角色可以访问租户1的部门管理接口
INSERT INTO sys_casbin_rules (ptype, v0, v1, v2, v3, tenant_id, service_name, status)
VALUES ('p', 'manager', '1', '/api/department/*', 'GET', 1, 'core', 1);
```

#### 2. 数据权限规则（ptype=d）

**用途**：控制角色可以访问哪些数据范围

**字段映射**：
```
ptype: d（数据权限规则）
v0:    角色代码（如：admin, manager）
v1:    租户ID（domain，支持多租户）
v2:    资源类型（如：user, department, 或 * 表示所有）
v3:    数据权限范围（all/custom_dept/own_dept_and_sub/own_dept/own）
v4:    自定义部门ID列表（JSON数组，仅当v3=custom_dept时有效）
v5:    保留（可用于扩展）
```

**数据权限范围说明**：
- `all`: 全部数据（不添加任何过滤条件）
- `custom_dept`: 自定义部门数据（由v4指定部门列表）
- `own_dept_and_sub`: 本部门及子部门数据
- `own_dept`: 仅本部门数据
- `own`: 仅个人数据（user_id过滤）

**示例**：
```sql
-- admin角色可以访问租户1的全部用户数据
INSERT INTO sys_casbin_rules (ptype, v0, v1, v2, v3, v4, tenant_id, service_name, status)
VALUES ('d', 'admin', '1', '*', 'all', NULL, 1, 'core', 1);

-- manager角色可以访问租户1的本部门及子部门数据
INSERT INTO sys_casbin_rules (ptype, v0, v1, v2, v3, v4, tenant_id, service_name, status)
VALUES ('d', 'manager', '1', '*', 'own_dept_and_sub', NULL, 1, 'core', 1);

-- leader角色可以访问自定义部门的数据
INSERT INTO sys_casbin_rules (ptype, v0, v1, v2, v3, v4, tenant_id, service_name, status)
VALUES ('d', 'leader', '1', 'user', 'custom_dept', '[101, 102, 103]', 1, 'core', 1);

-- employee角色只能访问自己的数据
INSERT INTO sys_casbin_rules (ptype, v0, v1, v2, v3, v4, tenant_id, service_name, status)
VALUES ('d', 'employee', '1', '*', 'own', NULL, 1, 'core', 1);
```

#### 3. 角色继承规则（ptype=g）

**用途**：定义用户-角色关系和角色继承

**字段映射**：
```
ptype: g（分组/角色继承）
v0:    用户ID或子角色
v1:    角色代码或父角色
v2:    租户ID（domain）
```

**示例**：
```sql
-- 用户123拥有admin角色
INSERT INTO sys_casbin_rules (ptype, v0, v1, v2, tenant_id, status)
VALUES ('g', '123', 'admin', '1', 1, 1);

-- 用户456拥有manager角色
INSERT INTO sys_casbin_rules (ptype, v0, v1, v2, tenant_id, status)
VALUES ('g', '456', 'manager', '1', 1, 1);
```

## 🔄 迁移方案

### 从 sys_roles.data_scope 迁移

#### 步骤1：数据迁移脚本

```sql
-- 迁移脚本：将sys_roles.data_scope迁移到sys_casbin_rules

-- 1. 为每个角色创建数据权限规则
INSERT INTO sys_casbin_rules (
    ptype, v0, v1, v2, v3, v4,
    tenant_id, service_name, status,
    category, rule_name, description
)
SELECT
    'd' as ptype,
    r.code as v0,           -- 角色代码
    r.tenant_id as v1,      -- 租户ID
    '*' as v2,              -- 所有资源
    CASE r.data_scope
        WHEN 1 THEN 'all'
        WHEN 2 THEN 'custom_dept'
        WHEN 3 THEN 'own_dept_and_sub'
        WHEN 4 THEN 'own_dept'
        WHEN 5 THEN 'own'
    END as v3,              -- 数据权限范围
    CASE
        WHEN r.data_scope = 2 AND r.custom_dept_ids IS NOT NULL
        THEN r.custom_dept_ids
        ELSE NULL
    END as v4,              -- 自定义部门ID
    r.tenant_id,
    'core' as service_name,
    1 as status,
    'migrated' as category,
    CONCAT('DataPerm_', r.code) as rule_name,
    CONCAT('数据权限规则 - 从角色', r.name, '迁移') as description
FROM sys_roles r
WHERE r.status = 1
AND NOT EXISTS (
    -- 避免重复迁移
    SELECT 1 FROM sys_casbin_rules cr
    WHERE cr.ptype = 'd'
    AND cr.v0 = r.code
    AND cr.tenant_id = r.tenant_id
);

-- 2. 验证迁移结果
SELECT
    r.code as role_code,
    r.data_scope as old_data_scope,
    cr.v3 as new_data_scope,
    CASE WHEN cr.id IS NULL THEN 'Missing' ELSE 'OK' END as migration_status
FROM sys_roles r
LEFT JOIN sys_casbin_rules cr ON cr.ptype = 'd' AND cr.v0 = r.code AND cr.tenant_id = r.tenant_id
WHERE r.status = 1;
```

#### 步骤2：保留过渡期

**阶段1（当前）**：双轨运行
- sys_roles.data_scope 继续使用（旧版DataPerm）
- sys_casbin_rules 开始同步写入（新版UnifiedDataPerm）

**阶段2（v2.0）**：优先读取Casbin
- UnifiedDataPerm优先从 sys_casbin_rules 读取
- 如果没有找到，降级到 sys_roles.data_scope

**阶段3（v2.1）**：完全迁移
- 所有服务强制使用 UnifiedDataPerm
- 废弃 sys_roles.data_scope 字段

#### 步骤3：同步更新机制

在 `AssignRoleDataScope` RPC方法中同时更新两处：

```go
// core/rpc/internal/logic/role/assign_role_data_scope_logic.go
func (l *AssignRoleDataScopeLogic) AssignRoleDataScope(in *core.RoleDataScopeReq) (*core.BaseResp, error) {
    err := entx.WithTx(l.ctx, l.svcCtx.DB, func(tx *ent.Tx) error {
        // 1. 获取角色信息
        role, err := tx.Role.Get(l.ctx, in.Id)
        if err != nil {
            return err
        }

        // 2. 更新 sys_roles 表（向后兼容）
        err = tx.Role.UpdateOneID(in.Id).
            SetNotNilDataScope(pointy.GetStatusPointer(&in.DataScope)).
            SetNotNilCustomDeptIds(in.CustomDeptIds).
            Exec(l.ctx)
        if err != nil {
            return err
        }

        // 3. 🔥 同步更新 sys_casbin_rules 表
        dataScopeStr := convertDataScopeToString(in.DataScope)
        customDeptJson := convertDeptIdsToJson(in.CustomDeptIds)

        // 删除旧规则
        _, err = tx.CasbinRule.Delete().
            Where(
                casbinrule.PtypeEQ("d"),
                casbinrule.V0EQ(role.Code),
                casbinrule.TenantIDEQ(role.TenantID),
            ).
            Exec(l.ctx)
        if err != nil {
            return err
        }

        // 插入新规则
        err = tx.CasbinRule.Create().
            SetPtype("d").
            SetV0(role.Code).
            SetV1(fmt.Sprintf("%d", role.TenantID)).
            SetV2("*").
            SetV3(dataScopeStr).
            SetNotNilV4(customDeptJson).
            SetTenantID(role.TenantID).
            SetServiceName("core").
            SetCategory("data_permission").
            SetRuleName(fmt.Sprintf("DataPerm_%s", role.Code)).
            SetDescription(fmt.Sprintf("数据权限规则 - 角色%s", role.Name)).
            Exec(l.ctx)
        if err != nil {
            return err
        }

        return nil
    })

    if err != nil {
        return nil, dberrorhandler.DefaultEntError(l.Logger, err, in)
    }
    return &core.BaseResp{Msg: i18n.UpdateSuccess}, nil
}
```

## 🎯 实施计划

### Phase 1: 准备阶段（当前）

- [x] 分析现有权限配置
- [x] 设计统一权限方案
- [x] 废弃旧版DataPerm插件
- [ ] 编写迁移脚本

### Phase 2: 迁移阶段（v2.0）

- [ ] 执行数据迁移脚本
- [ ] 修改所有服务配置启用UnifiedDataPerm
- [ ] 修改AssignRoleDataScope同步更新两处
- [ ] 全面测试验证

### Phase 3: 清理阶段（v2.1）

- [ ] 移除旧版DataPerm插件代码
- [ ] 移除sys_roles.data_scope字段
- [ ] 更新所有文档

## ✅ 配置示例

### 服务配置（所有API服务统一）

```yaml
# core/api/etc/core.yaml
Middleware:
  # Permission中间件 - 接口权限检查
  permission:
    enabled: true
    skipPaths:
      - /core/init/database
      - /captcha

  # DataPerm中间件 - 数据权限过滤
  dataPerm:
    enabled: true
    casbinEnabled: true  # ✅ 启用Casbin集成
    skipPaths:
      - /core/init/database
      - /captcha
```

### 服务初始化代码（统一模式）

```go
// internal/svc/service_context.go
func NewServiceContext(c config.Config) *ServiceContext {
    // 1. 初始化Core RPC客户端
    coreRpc := coreclient.NewCore(...)

    // 2. 创建RPC Casbin查询器
    rpcQuerier := apicasbin.NewRpcCasbinRuleQuerier(coreRpc)

    // 3. 初始化Casbin Enforcer
    systemCtx := hooks.NewSystemContext(context.Background())
    adapter := commonadapter.NewEntAdapter(rpcQuerier, systemCtx)
    modelText := commoncasbin.GetDefaultRBACWithDomainsModel()
    m, _ := model.NewModelFromString(modelText)
    cbn, _ := casbin.NewEnforcer(m, adapter)
    cbn.LoadPolicy()

    svcCtx := &ServiceContext{
        CoreRpc: coreRpc,
        Casbin:  cbn,
    }

    // 4. 🔥 统一中间件集成
    result, _ := integration.Setup(&integration.Config{
        Redis:     rds,
        JWTSecret: jwtSecret,
        Mode:      integration.Production,

        // ✅ 传递RBAC提供者（用于Permission中间件）
        RbacProvider: svcCtx,

        // ✅ 传递DataPerm提供者（用于DataPerm中间件）
        // RpcCasbinRuleQuerier实现了CasbinProvider接口
        DataPermCasbinProvider: rpcQuerier,

        Middleware: &c.Middleware,
    })

    svcCtx.ContextManager = result.ContextManager
    svcCtx.Middlewares = result.Middlewares

    return svcCtx
}

// GetCasbinEnforcer 实现permission.EnforcerProvider接口
func (svc *ServiceContext) GetCasbinEnforcer() interface{} {
    return svc.Casbin
}
```

## 📊 权限检查流程

### 完整流程图

```
HTTP Request
    ↓
[Auth中间件] - 验证JWT Token，提取userId, tenantId, roleCodes
    ↓
[TenantCheck中间件] - 验证租户有效性
    ↓
[Permission中间件] - 🔥 Casbin接口权限检查
    ├─ enforcer.Enforce(roleCode, tenantId, path, method)
    ├─ 查询: ptype=p, v0=roleCode, v1=tenantId, v2=path, v3=method
    └─ 决策: 允许/拒绝接口访问
    ↓
[DataPerm中间件] - 🔥 Casbin数据权限提取
    ├─ casbinProvider.GetDataScope(roleCode, tenantId, resource)
    ├─ 查询: ptype=d, v0=roleCode, v1=tenantId
    ├─ 提取: v3=dataScope, v4=customDeptIds
    └─ 注入上下文: SetDataScope(ctx, dataScope, customDeptIds)
    ↓
[Business Logic] - 业务逻辑处理
    ↓
[Ent Query] - 数据库查询
    ├─ DataPermInterceptor自动拦截
    ├─ 从上下文获取dataScope
    └─ 自动添加SQL WHERE条件
    ↓
Filtered Response
```

## 🔍 优势总结

### 1. 配置简化

**之前**：
- Permission规则 → sys_casbin_rules
- DataPerm规则 → sys_roles.data_scope + Redis缓存
- 配置分散，容易不一致

**现在**：
- 所有规则 → sys_casbin_rules
- 单一数据源，配置一致

### 2. 维护简化

**之前**：
- 修改接口权限：更新 sys_casbin_rules
- 修改数据权限：更新 sys_roles.data_scope + 清理Redis缓存
- 两处维护，容易遗漏

**现在**：
- 修改任何权限：只更新 sys_casbin_rules
- Casbin自动刷新（Redis Watcher）
- 一处维护，自动同步

### 3. 功能增强

- ✅ 支持资源级数据权限（v2字段）
- ✅ 支持审批流程（require_approval字段）
- ✅ 支持临时权限（effective_from/to字段）
- ✅ 支持规则版本管理（version字段）
- ✅ 支持使用统计（usage_count字段）

### 4. 性能优化

- ✅ Casbin内存缓存 + Redis Watcher
- ✅ 优化的索引设计
- ✅ 批量权限检查支持

## 📝 注意事项

1. **迁移过程保持向后兼容**
   - 过渡期同时支持两种配置方式
   - 逐步迁移，降低风险

2. **确保数据一致性**
   - 使用事务同时更新两处
   - 定期验证数据一致性

3. **监控和日志**
   - 记录所有权限检查日志
   - 监控性能指标

4. **文档更新**
   - 更新API文档
   - 更新操作手册
   - 培训运维人员

---

**最后更新**: 2025-10-12
**版本**: v1.0
**作者**: NewBee Team
