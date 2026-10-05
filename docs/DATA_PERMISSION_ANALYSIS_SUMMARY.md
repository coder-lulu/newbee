# NewBee数据权限架构分析总结报告

> **分析日期**: 2025-10-19
> **分析范围**: CMDB CI权限 + Common数据权限中间件 + 统一输入输出服务设计

---

## 📊 执行摘要

本次分析深入研究了NewBee系统中现有的数据权限实现，包括CMDB服务的CI权限功能和Common包的数据权限中间件，并基于分析结果为统一输入输出服务设计了完整的数据权限实施方案。

### 核心发现

1. ✅ **CMDB CI权限实现非常完善** - 提供了多维度、细粒度的权限模型
2. ✅ **Common包数据权限架构成熟** - 提供了开箱即用的权限拦截能力
3. ✅ **两者可以完美结合** - 形成从API层到DB层的多层防护体系

---

## 1. CMDB CI权限实现分析

### 1.1 核心特性

| 特性 | 说明 | 优势 |
|------|------|------|
| **多维度权限范围** | global/ci_type/ci_instance/attribute | 支持从全局到字段级的细粒度控制 |
| **灵活的主体类型** | user/role/department | 支持基于用户、角色、部门的授权 |
| **位掩码操作权限** | 1=read, 2=write, 4=delete... | 高效的权限存储和检查（位运算） |
| **时间控制** | effective_from/effective_to | 支持临时权限和权限有效期 |
| **继承机制** | inheritable + parent_permission_id | 支持权限继承，简化配置 |
| **安全增强** | require_approval/require_mfa/risk_level | 支持审批流程和MFA认证 |

### 1.2 数据模型亮点

```go
type CiPermission struct {
    // 权限范围（4级粒度）
    ScopeType       string  // global/ci_type/ci_instance/attribute
    ScopeTargetType string  // ci_type_id/ci_id/attribute_id
    ScopeTargetID   uint64

    // 权限主体（3种类型）
    SubjectType string  // user/role/department
    SubjectID   uint64

    // 操作掩码（位运算优化）
    OperationsMask int  // 1=read,2=write,4=delete,8=approve,16=export

    // 安全控制
    RequireApproval bool
    RequireMfa      bool
    RiskLevel       string  // low/medium/high/critical

    // 时间控制
    IsTemporary   bool
    EffectiveFrom time.Time
    EffectiveTo   time.Time
}
```

**设计优势**:
- ✅ 使用位掩码存储操作权限，节省存储空间且检查高效
- ✅ 支持多级权限范围，满足不同粒度的权限需求
- ✅ 集成审批流程和MFA，符合企业安全规范
- ✅ 时间控制机制支持临时授权场景

---

## 2. Common包数据权限中间件分析

### 2.1 整体架构

```
┌────────────────────────────────────────────────────────┐
│          API层 (UnifiedDataPermPlugin)                 │
│  - Casbin权限检查                                      │
│  - 生成数据过滤规则                                    │
│  - 注入增强权限上下文                                  │
└────────────────┬───────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────┐
│          业务层 (Business Logic)                       │
│  - 透明获取权限上下文                                  │
│  - 无需手动添加过滤条件                                │
└────────────────┬───────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────┐
│          DB层 (EnhancedDataPermInterceptor)            │
│  - 自动应用SQL过滤                                     │
│  - 字段级权限检查                                      │
│  - 数据脱敏处理                                        │
└────────────────┬───────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────┐
│          数据库 (Database)                             │
└────────────────────────────────────────────────────────┘
```

### 2.2 五级数据权限实现

```go
type DataPermScope uint8

const (
    DataPermAll          DataPermScope = 1  // 全部数据
    DataPermCustomDept   DataPermScope = 2  // 自定义部门
    DataPermOwnDeptAndSub DataPermScope = 3  // 本部门及下属
    DataPermOwnDept      DataPermScope = 4  // 仅本部门
    DataPermOwn          DataPermScope = 5  // 仅本人
)
```

**实现机制**:

| 权限范围 | SQL过滤条件 | 应用场景 |
|---------|------------|---------|
| **全部数据** | 无过滤 | 系统管理员、租户管理员 |
| **自定义部门** | `department_id IN (1,2,3)` | 跨部门协作、虚拟团队 |
| **本部门及下属** | `department_id IN (用户部门+下属部门)` | 部门经理、团队负责人 |
| **仅本部门** | `department_id = 用户部门ID` | 部门成员 |
| **仅本人** | `created_by = 用户ID` | 普通员工 |

### 2.3 核心组件

#### 2.3.1 UnifiedDataPermPlugin (中间件)

```go
// 核心流程
func (p *UnifiedDataPermPlugin) ProcessRequest(ctx context.Context) error {
    // 1. Casbin权限检查
    result, err := p.casbinProvider.CheckPermissionWithRoles(...)

    // 2. 生成数据过滤规则
    dataRules, err := p.ruleEngine.GenerateDataRules(...)

    // 3. 注入增强权限上下文
    enhancedCtx := p.contextManager.SetEnhancedPermissions(...)

    return next.ProcessRequest(enhancedCtx)
}
```

**职责**:
- ✅ API层权限检查（是否有权访问资源）
- ✅ 生成数据过滤规则（根据角色、部门）
- ✅ 注入权限上下文（传递给下游）

#### 2.3.2 EnhancedDataPermInterceptor (拦截器)

```go
// 自动SQL过滤
func (i *EnhancedDataPermInterceptor) Intercept(next ent.Querier) ent.Querier {
    return ent.QuerierFunc(func(ctx context.Context, query ent.Query) (ent.Value, error) {
        dataScope := getDataScopeFromContext(ctx)

        switch dataScope {
        case DataPermCustomDept:
            applyCustomDeptFilter(ctx, query)
        case DataPermOwnDeptAndSub:
            applySubDeptFilter(ctx, query)
        case DataPermOwnDept:
            applyOwnDeptFilter(ctx, query)
        case DataPermOwn:
            applyUserDataFilter(ctx, query)
        }

        return next.Query(ctx, query)
    })
}
```

**职责**:
- ✅ 自动应用SQL过滤条件（基于数据权限范围）
- ✅ 字段级权限检查
- ✅ 数据脱敏处理

### 2.4 字段脱敏机制

```go
// 敏感字段配置
var SensitiveFieldsConfig = map[string]FieldMaskConfig{
    "data_source.password": {
        MaskType:    "full",      // 完全掩码: ********
        AccessRoles: []string{"admin", "system_admin"},
    },
    "user.email": {
        MaskType:    "email",     // 邮箱掩码: a****@example.com
        AccessRoles: []string{"admin", "hr", "self"},
    },
    "user.phone": {
        MaskType:    "phone",     // 手机掩码: 138****5678
        AccessRoles: []string{"admin", "hr", "self"},
    },
}
```

**掩码策略**:
- `full`: 完全掩码 `********`
- `partial`: 部分掩码 `{{prefix}}****{{suffix}}`
- `email`: 邮箱掩码 `a****@example.com`
- `phone`: 手机掩码 `138****5678`
- `hash`: 哈希值 `md5(value)`
- `encrypt`: 加密值（可解密）

---

## 3. 两者结合方式总结

### 3.1 分层防护架构

```
┌─────────────────────────────────────────────────────────────┐
│  API层 (UnifiedDataPermPlugin)                              │
│  - 检查: 用户是否有权访问该资源类型                          │
│  - 来源: CMDB风格的资源权限表 (io_resource_permissions)     │
│  - 示例: 用户是否有权访问 data_source 资源                  │
└─────────────────┬───────────────────────────────────────────┘
                  │ ✓ 有权访问
                  ▼
┌─────────────────────────────────────────────────────────────┐
│  RPC层 (PermissionService)                                  │
│  - 检查: 用户是否有权对特定资源执行特定操作                  │
│  - 来源: io_resource_permissions 表 (位掩码)                │
│  - 示例: 用户是否有权对 data_source(id=100) 执行 update     │
└─────────────────┬───────────────────────────────────────────┘
                  │ ✓ 有权操作
                  ▼
┌─────────────────────────────────────────────────────────────┐
│  DB层 (EnhancedDataPermInterceptor)                         │
│  - 检查: 用户数据权限范围                                    │
│  - 来源: Common包的五级数据权限                              │
│  - 示例: 自动过滤只返回用户有权查看的数据                    │
└─────────────────┬───────────────────────────────────────────┘
                  │ ✓ 已过滤
                  ▼
┌─────────────────────────────────────────────────────────────┐
│  字段层 (FieldMaskProcessor)                                │
│  - 检查: 字段级权限                                          │
│  - 来源: SensitiveFieldsConfig配置                          │
│  - 示例: 普通用户看到的密码字段是 ********                   │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 权限检查流程

```go
// 示例: 用户请求查看数据源列表

// 1. API层 - UnifiedDataPermPlugin
// 检查: 用户是否有 data_source:read 权限
casbinCheck("user123", "data_source", "read", "tenant1")
// ✓ 通过

// 2. RPC层 - 业务逻辑执行
func GetDataSourceList(req) {
    // 透明执行，无需手动添加过滤条件
    results := db.DataSource.Query().All(ctx)
}

// 3. DB层 - EnhancedDataPermInterceptor
// 自动应用SQL过滤
// 根据用户的数据权限范围:
// - 管理员: 无过滤 (看到所有数据源)
// - 部门经理: WHERE department_id IN (用户部门+下属部门)
// - 普通用户: WHERE created_by = 用户ID

// 4. 字段层 - FieldMaskProcessor
// 对返回结果中的敏感字段自动脱敏
// - password: ******** (管理员可见明文)
// - api_key: ******** (管理员可见明文)
// - access_token: abc****xyz (部分掩码)
```

### 3.3 关键集成点

| 集成点 | CMDB实现 | Common包实现 | 统一输入输出设计 |
|-------|---------|-------------|---------------|
| **资源权限表** | `cmdb_ci_permissions` | 无（使用Casbin） | `io_resource_permissions` |
| **权限检查** | 应用层手动检查 | `UnifiedDataPermPlugin` | 继承Common包实现 |
| **数据过滤** | 手动添加SQL条件 | `EnhancedDataPermInterceptor` | 自动应用 |
| **字段脱敏** | 应用层手动掩码 | `FieldMaskProcessor` | 声明式配置 |
| **Casbin集成** | 独立实现 | 深度集成 | 复用Core服务Casbin |

---

## 4. 统一输入输出服务数据权限设计要点

### 4.1 资源类型定义

```go
const (
    // 输入相关
    ResourceDataSource    = "data_source"
    ResourceProvider      = "provider"
    ResourceFieldMapping  = "field_mapping"
    ResourceDiscovery     = "discovery"

    // 输出相关
    ResourceDataTarget    = "data_target"
    ResourceSyncTask      = "sync_task"
    ResourceSyncTemplate  = "sync_template"

    // 工作流
    ResourceWorkflow      = "workflow"
    ResourceWorkflowTask  = "workflow_task"
)
```

### 4.2 权限检查示例

```go
// 创建数据源
func CreateDataSource(in *io.DataSourceInfo) (*io.BaseIDResp, error) {
    // 1. 权限检查 (RPC层)
    allowed, err := permissionService.CheckResourcePermission(
        ctx, userID, "data_source", nil, "create",
    )
    if !allowed {
        return nil, ErrPermissionDenied
    }

    // 2. 创建资源 (DB层自动应用租户隔离)
    ds, err := db.DataSource.Create().
        SetName(in.Name).
        Save(ctx)

    return &io.BaseIDResp{Id: ds.ID}, nil
}

// 查询数据源列表
func GetDataSourceList(in *io.DataSourceListReq) (*io.DataSourceListResp, error) {
    // 无需手动检查权限，也无需手动添加过滤条件
    // EnhancedDataPermInterceptor 会自动:
    // 1. 根据用户数据权限范围过滤
    // 2. 应用租户隔离
    // 3. 对敏感字段脱敏

    results, err := db.DataSource.Query().
        Order(ent.Desc(datasource.FieldCreatedAt)).
        All(ctx)

    return convertToResponse(results), nil
}
```

### 4.3 敏感字段配置

```go
var SensitiveFieldsConfig = map[string]FieldMaskConfig{
    "data_source.password": {
        MaskType:    "full",
        AccessRoles: []string{"admin", "system_admin"},
    },
    "data_source.api_key": {
        MaskType:    "full",
        AccessRoles: []string{"admin", "system_admin"},
    },
    "data_source.access_token": {
        MaskType:    "partial",
        AccessRoles: []string{"admin", "system_admin", "operator"},
        MaskPattern: "{{prefix4}}****{{suffix4}}",
    },
    "data_target.password": {
        MaskType:    "full",
        AccessRoles: []string{"admin", "system_admin"},
    },
}
```

### 4.4 开发步骤总结

| 阶段 | 时间 | 任务 | 交付物 |
|------|------|------|--------|
| **阶段1** | Week 1 | 数据模型与Schema | Migration, Ent Schema |
| **阶段2** | Week 1-2 | 权限Service实现 | PermissionService, 单元测试 |
| **阶段3** | Week 2 | 数据权限拦截器集成 | ServiceContext配置 |
| **阶段4** | Week 2-3 | API中间件集成 | 统一中间件配置 |
| **阶段5** | Week 3 | 字段级权限与脱敏 | 敏感字段配置 |
| **阶段6** | Week 3-4 | Casbin规则集成 | 默认策略初始化 |
| **阶段7** | Week 4 | 测试与验证 | 测试报告 |

---

## 5. 关键设计决策

### 5.1 为什么不完全复制CMDB的CI权限？

**原因**:
1. ✅ **避免重复造轮子** - Common包已提供成熟的数据权限中间件
2. ✅ **统一架构** - 与Core服务保持一致的权限模型
3. ✅ **简化实现** - 利用已有的Casbin集成和状态管理器
4. ✅ **更好的性能** - 自动SQL过滤比应用层过滤更高效

**借鉴的部分**:
- ✅ 资源权限表设计（io_resource_permissions）
- ✅ 位掩码操作权限（高效存储）
- ✅ 多维度权限范围（global/type/instance/field）
- ✅ 安全控制机制（MFA/审批/风险等级）

### 5.2 为什么使用Common包的数据权限中间件？

**优势**:
1. ✅ **开箱即用** - 无需重新实现数据过滤逻辑
2. ✅ **自动化** - EnhancedDataPermInterceptor自动应用SQL过滤
3. ✅ **透明化** - 业务代码无需关心权限逻辑
4. ✅ **统一管理** - 与Core服务使用相同的Casbin规则
5. ✅ **性能优化** - 内置Redis缓存和批量检查

### 5.3 为什么需要资源权限表？

**原因**:
1. ✅ **细粒度控制** - 支持资源级别的权限控制（不仅是数据范围）
2. ✅ **操作权限** - 区分read/create/update/delete/execute等操作
3. ✅ **字段级权限** - 支持字段级别的访问控制
4. ✅ **审计需求** - 记录权限授予历史和使用情况
5. ✅ **企业需求** - 支持审批流程、临时授权、MFA等企业级特性

---

## 6. 架构优势

### 6.1 多层防护

```
API层 ──→ 资源访问权限检查 (能否访问data_source?)
  │
  ↓
RPC层 ──→ 资源操作权限检查 (能否update data_source:100?)
  │
  ↓
DB层  ──→ 数据范围权限过滤 (只返回权限范围内的数据)
  │
  ↓
字段层 ──→ 敏感字段脱敏 (password显示为********)
```

### 6.2 性能优化

| 优化项 | 实现方式 | 效果 |
|-------|---------|------|
| **Redis缓存** | 权限检查结果缓存5分钟 | 缓存命中率>80% |
| **位掩码** | 使用位运算存储操作权限 | 节省存储空间，检查效率高 |
| **SQL过滤** | DB层直接过滤，减少数据传输 | 减少网络开销 |
| **状态管理器** | 优化上下文传递 | 减少重复查询 |

### 6.3 开发友好

| 特性 | 说明 | 优势 |
|------|------|------|
| **声明式配置** | 敏感字段配置、掩码策略配置 | 无需编写重复代码 |
| **自动化** | 数据过滤和字段脱敏自动应用 | 减少人为错误 |
| **透明化** | 业务代码无需关心权限逻辑 | 代码更简洁 |
| **可测试** | 完整的测试套件和Mock支持 | 易于测试 |

---

## 7. 验收标准

### 7.1 功能验收

- [ ] ✅ 用户只能访问其权限范围内的资源
- [ ] ✅ 五级数据权限正确生效
- [ ] ✅ 敏感字段根据角色正确脱敏
- [ ] ✅ 租户间数据完全隔离
- [ ] ✅ 位掩码操作权限检查正确
- [ ] ✅ 临时权限和有效期生效

### 7.2 性能验收

- [ ] ✅ 权限检查P95延迟 <50ms
- [ ] ✅ Redis缓存命中率 >80%
- [ ] ✅ 数据库查询性能影响 <10%
- [ ] ✅ 支持并发10k+ QPS

### 7.3 安全验收

- [ ] ✅ 租户隔离测试100%通过
- [ ] ✅ 越权访问尝试100%被拦截
- [ ] ✅ 敏感信息无泄露
- [ ] ✅ 审计日志完整

---

## 8. 结论

### 8.1 核心价值

1. **完全复用现有架构** - 充分利用CMDB和Common包的成熟实现
2. **多层防护体系** - 从API到DB层的全方位权限控制
3. **高性能设计** - Redis缓存、位掩码、SQL过滤等优化手段
4. **企业级特性** - MFA、审批流程、临时授权等企业需求
5. **开发友好** - 声明式配置、自动化、透明化

### 8.2 实施建议

1. **优先级**: P0 - 必须在统一输入输出服务上线前实施
2. **时间规划**: 4周完成全部开发和测试
3. **资源投入**: 2名后端工程师 + 1名测试工程师
4. **风险评估**: 低 - 基于成熟的架构和实现

### 8.3 后续优化方向

1. **可视化权限管理界面** - 提供管理后台
2. **权限变更审批流程** - 集成工作流引擎
3. **权限使用情况分析** - 提供权限审计报告
4. **基于标签的权限控制** - 支持更灵活的权限规则

---

**文档完成日期**: 2025-10-19
**分析人**: 架构师团队
**审核人**: CTO
**状态**: ✅ 已完成

---

## 附录: 相关文档

- [统一输入输出平台Kafka集成设计](./unified-io-kafka-queue-design.md)
- [统一输入输出服务数据权限设计](./UNIFIED_IO_DATA_PERMISSION_DESIGN.md)
- [Common包数据权限中间件README](../common/middleware/dataperm/README.md)
- [NewBee编码准则](./CLAUDE.md)
