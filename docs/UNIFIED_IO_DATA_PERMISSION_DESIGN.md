# 统一输入输出服务数据权限设计与实施方案

> **文档版本**: v1.0
> **创建日期**: 2025-10-19
> **作者**: 架构师团队

## 目录

- [1. 背景分析](#1-背景分析)
- [2. CMDB CI权限实现分析](#2-cmdb-ci权限实现分析)
- [3. Common包数据权限架构](#3-common包数据权限架构)
- [4. 统一输入输出服务数据权限设计](#4-统一输入输出服务数据权限设计)
- [5. 开发实施步骤](#5-开发实施步骤)
- [6. 测试与验收](#6-测试与验收)

---

## 1. 背景分析

### 1.1 业务需求

统一输入输出平台需要实现细粒度的数据权限控制，包括：

1. **行级权限**: 用户只能访问其权限范围内的任务、连接器、目标等资源
2. **字段级权限**: 敏感字段（如密码、密钥）需要根据角色进行脱敏
3. **多租户隔离**: 租户间数据完全隔离
4. **部门级权限**: 支持五级数据权限（全部/自定义部门/本部门及下属/本部门/仅本人）

### 1.2 参考架构

我们将基于以下现有实现进行设计：

1. **CMDB服务的CI权限实现** - 提供资源级权限控制参考
2. **Common包的数据权限中间件** - 提供底层权限拦截能力
3. **Core服务的Casbin集成** - 提供统一的权限规则管理

---

## 2. CMDB CI权限实现分析

### 2.1 CI权限数据模型

CMDB的CI权限采用了非常完善的权限模型：

```go
type CiPermission struct {
    // 基础字段
    ID           uint64
    PermissionID string  // 权限唯一标识符
    DepartmentID uint64  // 部门ID

    // 权限范围
    ScopeType       string  // 权限范围类型: global/ci_type/ci_instance/attribute
    ScopeTargetType string  // 目标类型: ci_type_id/ci_id/attribute_id
    ScopeTargetID   uint64  // 目标ID
    ScopeFieldName  string  // 字段名称（字段级权限）

    // 权限主体
    SubjectType string  // 主体类型: user/role/department
    SubjectID   uint64  // 主体ID
    SubjectName string  // 主体名称
    SubjectCode string  // 主体代码

    // 权限类型和级别
    PermissionType  string  // read/write/delete/approve/export
    PermissionLevel string  // view/edit/manage/admin
    Priority        int     // 优先级（冲突时）

    // 操作掩码（位运算）
    OperationsMask int  // 1=read, 2=write, 4=delete, 8=approve, 16=export

    // 安全控制
    RequireApproval bool    // 是否需要审批
    RequireMfa      bool    // 是否需要MFA
    RiskLevel       string  // low/medium/high/critical

    // 时间控制
    IsTemporary   bool
    EffectiveFrom time.Time
    EffectiveTo   time.Time

    // 继承控制
    Inheritable        bool
    ParentPermissionID uint64

    // 使用统计
    UsageCount int
    LastUsedAt time.Time

    // 状态
    Status       string  // active/inactive/suspended/revoked/expired
    StatusReason string
}
```

### 2.2 关键设计亮点

#### 2.2.1 多维度权限范围

```go
// 支持四种权限范围粒度
type ScopeType string

const (
    ScopeGlobal      ScopeType = "global"       // 全局
    ScopeCiType      ScopeType = "ci_type"      // CI类型
    ScopeCiInstance  ScopeType = "ci_instance"  // CI实例
    ScopeAttribute   ScopeType = "attribute"    // 属性（字段级）
)
```

#### 2.2.2 灵活的主体类型

```go
type SubjectType string

const (
    SubjectUser       SubjectType = "user"       // 用户
    SubjectRole       SubjectType = "role"       // 角色
    SubjectDepartment SubjectType = "department" // 部门
)
```

#### 2.2.3 位掩码操作权限

```go
// 使用位运算高效存储和检查操作权限
const (
    OperationRead    = 1 << 0  // 1
    OperationWrite   = 1 << 1  // 2
    OperationDelete  = 1 << 2  // 4
    OperationApprove = 1 << 3  // 8
    OperationExport  = 1 << 4  // 16
)

// 检查是否有读权限
hasReadPerm := (permission.OperationsMask & OperationRead) != 0
```

---

## 3. Common包数据权限架构

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                    HTTP Request                             │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│          UnifiedDataPermPlugin (中间件)                     │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  1. 解析资源和操作类型                               │   │
│  │  2. Casbin权限检查                                  │   │
│  │  3. 生成数据过滤规则                                │   │
│  │  4. 注入增强权限上下文                              │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                 Business Logic                              │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│      EnhancedDataPermInterceptor (Ent拦截器)                │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  1. 获取权限上下文                                   │   │
│  │  2. 自动应用SQL过滤条件                             │   │
│  │  3. 字段级权限检查                                   │   │
│  │  4. 数据脱敏处理                                     │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                   Database                                  │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 核心组件

#### 3.2.1 UnifiedDataPermPlugin

**职责**: 中间件入口，负责权限检查和上下文注入

```go
// 核心流程
func (p *UnifiedDataPermPlugin) ProcessRequest(ctx context.Context, req *Request) error {
    // 1. 提取用户信息
    userID := getUserIDFromContext(ctx)
    tenantID := getTenantIDFromContext(ctx)

    // 2. Casbin权限检查
    resource := parseResource(req)
    action := parseAction(req)

    result, err := p.casbinProvider.CheckPermissionWithRoles(
        ctx, userID, resource, action, p.serviceName,
    )

    if err != nil || !result.Allowed {
        return ErrPermissionDenied
    }

    // 3. 生成数据过滤规则
    dataRules, err := p.ruleEngine.GenerateDataRules(
        ctx, userID, resource, action, result.AppliedRules,
    )

    // 4. 注入增强权限上下文
    enhancedCtx := p.contextManager.SetEnhancedPermissions(
        ctx, userID, tenantID, dataRules,
    )

    return p.next.ProcessRequest(enhancedCtx, req)
}
```

#### 3.2.2 EnhancedDataPermInterceptor

**职责**: Ent查询拦截器，自动应用SQL过滤和字段权限

```go
func (i *EnhancedDataPermInterceptor) Intercept(
    next ent.Querier,
) ent.Querier {
    return ent.QuerierFunc(func(ctx context.Context, query ent.Query) (ent.Value, error) {
        // 1. 系统上下文跳过检查
        if IsSystemContext(ctx) {
            return next.Query(ctx, query)
        }

        // 2. 获取数据权限范围
        dataScope := getDataScopeFromContext(ctx)

        // 3. 应用SQL过滤条件
        switch dataScope {
        case DataPermAll:
            // 不过滤
        case DataPermCustomDept:
            applyCustomDeptFilter(ctx, query)
        case DataPermOwnDeptAndSub:
            applySubDeptFilter(ctx, query)
        case DataPermOwnDept:
            applyOwnDeptFilter(ctx, query)
        case DataPermOwn:
            applyUserDataFilter(ctx, query)
        }

        // 4. 执行查询
        return next.Query(ctx, query)
    })
}
```

### 3.3 五级数据权限实现

```go
// 数据权限范围枚举
type DataPermScope uint8

const (
    DataPermAll          DataPermScope = 1  // 全部数据
    DataPermCustomDept   DataPermScope = 2  // 自定义部门
    DataPermOwnDeptAndSub DataPermScope = 3  // 本部门及下属
    DataPermOwnDept      DataPermScope = 4  // 仅本部门
    DataPermOwn          DataPermScope = 5  // 仅本人
)

// 应用自定义部门过滤
func applyCustomDeptFilter(ctx context.Context, query ent.Query) error {
    customDeptIds := getCustomDeptIdsFromContext(ctx)
    if len(customDeptIds) == 0 {
        return applyUserDataFilter(ctx, query)  // 降级为个人权限
    }

    query.(*sql.Selector).Where(sql.In("department_id", customDeptIds...))
    return nil
}

// 应用子部门过滤
func applySubDeptFilter(ctx context.Context, query ent.Query) error {
    userDeptId := getUserDeptIdFromContext(ctx)
    subDeptIds := getSubDeptIdsFromContext(ctx)

    allDeptIds := append(subDeptIds, userDeptId)
    query.(*sql.Selector).Where(sql.In("department_id", allDeptIds...))
    return nil
}

// 应用本部门过滤
func applyOwnDeptFilter(ctx context.Context, query ent.Query) error {
    userDeptId := getUserDeptIdFromContext(ctx)
    query.(*sql.Selector).Where(sql.EQ("department_id", userDeptId))
    return nil
}

// 应用用户数据过滤
func applyUserDataFilter(ctx context.Context, query ent.Query) error {
    userId := getUserIdFromContext(ctx)
    query.(*sql.Selector).Where(sql.EQ("created_by", userId))
    return nil
}
```

---

## 4. 统一输入输出服务数据权限设计

### 4.1 权限模型设计

#### 4.1.1 资源类型定义

```go
// unified-io/rpc/internal/permission/resource.go

package permission

// 资源类型
type ResourceType string

const (
    // 输入相关资源
    ResourceDataSource    ResourceType = "data_source"     // 数据源
    ResourceProvider      ResourceType = "provider"        // Provider
    ResourceFieldMapping  ResourceType = "field_mapping"   // 字段映射
    ResourceDiscovery     ResourceType = "discovery"       // 发现任务
    ResourceDiscoveryRule ResourceType = "discovery_rule"  // 发现规则

    // 输出相关资源
    ResourceDataTarget    ResourceType = "data_target"     // 数据目标
    ResourceSyncTask      ResourceType = "sync_task"       // 同步任务
    ResourceSyncTemplate  ResourceType = "sync_template"   // 同步模板

    // 工作流资源
    ResourceWorkflow      ResourceType = "workflow"        // 工作流
    ResourceWorkflowTask  ResourceType = "workflow_task"   // 工作流任务
)

// 操作类型
type ActionType string

const (
    ActionRead    ActionType = "read"     // 查看
    ActionCreate  ActionType = "create"   // 创建
    ActionUpdate  ActionType = "update"   // 更新
    ActionDelete  ActionType = "delete"   // 删除
    ActionExecute ActionType = "execute"  // 执行
    ActionApprove ActionType = "approve"  // 审批
    ActionExport  ActionType = "export"   // 导出
)
```

#### 4.1.2 权限表设计

```sql
-- unified-io/rpc/migrations/004_create_io_permissions.sql

CREATE TABLE io_resource_permissions (
    id                  BIGSERIAL PRIMARY KEY,
    tenant_id           BIGINT       NOT NULL,
    department_id       BIGINT       NOT NULL,

    -- 权限标识
    permission_id       VARCHAR(100) NOT NULL UNIQUE,

    -- 资源范围
    resource_type       VARCHAR(50)  NOT NULL,  -- data_source/provider/field_mapping等
    resource_id         BIGINT,                  -- 具体资源ID（NULL表示全局）
    field_name          VARCHAR(100),            -- 字段名称（字段级权限）

    -- 权限主体
    subject_type        VARCHAR(20)  NOT NULL,  -- user/role/department
    subject_id          BIGINT       NOT NULL,
    subject_name        VARCHAR(255) NOT NULL,
    subject_code        VARCHAR(100),

    -- 权限类型
    permission_type     VARCHAR(20)  NOT NULL,  -- read/write/delete/execute/approve/export
    permission_level    VARCHAR(20)  NOT NULL,  -- view/edit/manage/admin
    priority            INT          DEFAULT 50,

    -- 操作掩码 (位运算优化)
    operations_mask     INT          DEFAULT 0,  -- 1=read,2=create,4=update,8=delete,16=execute,32=approve,64=export

    -- 数据过滤
    data_scope          VARCHAR(50),             -- all/custom_dept/own_dept_and_sub/own_dept/own
    custom_dept_ids     BIGINT[],                -- 自定义部门列表

    -- 字段权限
    field_masks         JSONB,                   -- 字段掩码配置
    allowed_values      JSONB,                   -- 允许的字段值

    -- 安全控制
    require_approval    BOOLEAN      DEFAULT FALSE,
    require_mfa         BOOLEAN      DEFAULT FALSE,
    risk_level          VARCHAR(20)  DEFAULT 'low',  -- low/medium/high/critical

    -- 时间控制
    is_temporary        BOOLEAN      DEFAULT FALSE,
    effective_from      TIMESTAMPTZ,
    effective_to        TIMESTAMPTZ,

    -- 继承
    inheritable         BOOLEAN      DEFAULT FALSE,
    parent_permission_id BIGINT,

    -- 使用统计
    usage_count         INT          DEFAULT 0,
    last_used_at        TIMESTAMPTZ,

    -- 状态
    status              VARCHAR(20)  NOT NULL DEFAULT 'active',  -- active/inactive/suspended/revoked/expired
    status_reason       TEXT,

    -- 审计
    description         TEXT,
    comments            TEXT,
    created_by          BIGINT       NOT NULL,
    updated_by          BIGINT,
    last_reviewed_at    TIMESTAMPTZ,
    last_reviewed_by    BIGINT,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_parent_permission FOREIGN KEY (parent_permission_id) REFERENCES io_resource_permissions(id)
);

-- 索引
CREATE INDEX idx_io_resource_permissions_tenant ON io_resource_permissions(tenant_id);
CREATE INDEX idx_io_resource_permissions_dept ON io_resource_permissions(department_id);
CREATE INDEX idx_io_resource_permissions_subject ON io_resource_permissions(subject_type, subject_id);
CREATE INDEX idx_io_resource_permissions_resource ON io_resource_permissions(resource_type, resource_id);
CREATE INDEX idx_io_resource_permissions_status ON io_resource_permissions(status) WHERE status = 'active';
CREATE INDEX idx_io_resource_permissions_effective ON io_resource_permissions(effective_from, effective_to)
    WHERE is_temporary = TRUE;
```

### 4.2 服务集成设计

#### 4.2.1 RPC服务集成

```go
// unified-io/rpc/internal/svc/service_context.go

package svc

import (
    "github.com/coder-lulu/newbee-common/v2/middleware/dataperm"
    "github.com/coder-lulu/newbee-common/v2/orm/ent/hooks"
)

type ServiceContext struct {
    Config config.Config
    DB     *ent.Client
    Redis  redis.UniversalClient

    // 数据权限相关
    DataPermPlugin      *dataperm.UnifiedDataPermPlugin
    DataPermInterceptor ent.Interceptor
}

func NewServiceContext(c config.Config) *ServiceContext {
    // 1. 创建ent客户端
    db := ent.NewClient(...)

    // 2. 创建数据权限拦截器
    interceptor := hooks.NewEnhancedDataPermInterceptor(&hooks.EnhancedDataPermissionConfig{
        Enabled:           true,
        DepartmentField:   "department_id",
        UserField:         "created_by",
        SkipSystemContext: true,
        UseStateManager:   true,
    })

    // 3. 注册拦截器到ent
    db.Use(interceptor)

    // 4. 注册租户Hook (必须)
    db.Use(hooks.TenantMutationHook())
    db.Intercept(hooks.TenantQueryInterceptor())

    return &ServiceContext{
        Config:              c,
        DB:                  db,
        DataPermInterceptor: interceptor,
    }
}
```

#### 4.2.2 API服务集成

```go
// unified-io/api/internal/svc/service_context.go

package svc

import (
    "github.com/coder-lulu/newbee-common/v2/middleware/integration"
    "github.com/coder-lulu/newbee-common/v2/middleware/dataperm"
)

type ServiceContext struct {
    Config        config.Config
    IoRpc         iorpc.Io
    MiddlewareCtx *integration.MiddlewareContext
}

func NewServiceContext(c config.Config) *ServiceContext {
    // 1. 创建RPC客户端
    ioRpc := iorpc.NewIo(zrpc.MustNewClient(c.IoRpc))

    // 2. 创建Casbin提供者
    casbinProvider := dataperm.NewDefaultCasbinProvider(coreRpc, logger)

    // 3. 统一中间件集成
    middlewareCtx, err := integration.Setup(&integration.Config{
        Redis:     rds,
        JWTSecret: c.Auth.AccessSecret,
        Mode:      integration.Production,

        // 数据权限配置
        DataPermConfig: &integration.DataPermConfig{
            Enabled:         true,
            CasbinProvider:  casbinProvider,
            ServiceName:     "unified-io",
        },
    })

    if err != nil {
        panic("统一中间件集成失败: " + err.Error())
    }

    return &ServiceContext{
        Config:        c,
        IoRpc:         ioRpc,
        MiddlewareCtx: middlewareCtx,
    }
}
```

### 4.3 Logic层权限检查示例

```go
// unified-io/rpc/internal/logic/data_source/get_data_source_list_logic.go

package data_source

func (l *GetDataSourceListLogic) GetDataSourceList(in *io.DataSourceListReq) (*io.DataSourceListResp, error) {
    // ✅ Ent查询会自动应用数据权限过滤
    // 不需要手动添加任何过滤条件

    query := l.svcCtx.DB.DataSource.Query()

    // 应用业务过滤
    if in.Name != nil && *in.Name != "" {
        query.Where(datasource.NameContains(*in.Name))
    }

    if in.ProviderType != nil && *in.ProviderType != "" {
        query.Where(datasource.ProviderTypeEQ(*in.ProviderType))
    }

    // 执行查询 - 数据权限会自动应用
    // 用户只能看到其权限范围内的数据源
    results, err := query.
        Order(ent.Desc(datasource.FieldCreatedAt)).
        Offset(int((in.Page - 1) * in.PageSize)).
        Limit(int(in.PageSize)).
        All(l.ctx)

    if err != nil {
        return nil, err
    }

    // 字段脱敏会自动应用
    // 敏感字段（如密码）会根据用户角色自动掩码
    return convertToResponse(results), nil
}
```

### 4.4 敏感字段脱敏策略

```go
// unified-io/rpc/internal/permission/field_mask.go

package permission

// 定义敏感字段及其脱敏策略
var SensitiveFieldsConfig = map[string]dataperm.FieldMaskConfig{
    // 数据源凭证
    "data_source.password": {
        MaskType:     "full",      // 完全掩码
        AccessRoles:  []string{"admin", "system_admin"},
        Description:  "数据源密码",
    },
    "data_source.api_key": {
        MaskType:     "full",
        AccessRoles:  []string{"admin", "system_admin"},
        Description:  "API密钥",
    },
    "data_source.access_token": {
        MaskType:     "partial",   // 部分掩码
        AccessRoles:  []string{"admin", "system_admin", "operator"},
        MaskPattern:  "****{{last4}}",
        Description:  "访问令牌",
    },

    // 数据目标凭证
    "data_target.password": {
        MaskType:     "full",
        AccessRoles:  []string{"admin", "system_admin"},
        Description:  "目标密码",
    },

    // 个人敏感信息
    "user.email": {
        MaskType:     "email",     // 邮箱掩码: a****@example.com
        AccessRoles:  []string{"admin", "hr", "self"},
        Description:  "用户邮箱",
    },
    "user.phone": {
        MaskType:     "phone",     // 手机掩码: 138****5678
        AccessRoles:  []string{"admin", "hr", "self"},
        Description:  "用户手机",
    },
}
```

---

## 5. 开发实施步骤

### 5.1 阶段1: 数据模型与Schema (Week 1)

#### 任务清单

- [ ] **Task 1.1**: 创建io_resource_permissions表
  - 编写migration脚本
  - 创建必要的索引
  - 添加外键约束

- [ ] **Task 1.2**: 生成Ent Schema
  ```bash
  cd unified-io/rpc
  go run entgo.io/ent/cmd/ent new IoResourcePermission
  ```

- [ ] **Task 1.3**: 配置Ent Mixin
  ```go
  // unified-io/rpc/ent/schema/io_resource_permission.go

  func (IoResourcePermission) Mixin() []ent.Mixin {
      return []ent.Mixin{
          mixins.IDMixin{},
          mixins.TimeMixin{},
          mixins.StatusMixin{},
          mixins.TenantMixin{},    // 租户隔离
      }
  }
  ```

- [ ] **Task 1.4**: 生成Ent代码
  ```bash
  make gen-ent
  ```

---

### 5.2 阶段2: 权限Service实现 (Week 1-2)

#### Task 2.1: 创建权限检查Service

```go
// unified-io/rpc/internal/service/permission_service.go

package service

type PermissionService struct {
    db     *ent.Client
    redis  redis.UniversalClient
    logger logx.Logger
}

// CheckResourcePermission 检查资源权限
func (s *PermissionService) CheckResourcePermission(
    ctx context.Context,
    userID uint64,
    resourceType string,
    resourceID *uint64,
    action string,
) (bool, error) {
    // 1. 尝试从缓存获取
    cacheKey := fmt.Sprintf("perm:%d:%s:%s:%v", userID, resourceType, action, resourceID)
    cached, err := s.redis.Get(ctx, cacheKey).Result()
    if err == nil {
        return cached == "1", nil
    }

    // 2. 从数据库查询权限
    query := s.db.IoResourcePermission.Query().
        Where(
            ioresourcepermission.StatusEQ("active"),
            ioresourcepermission.ResourceTypeEQ(resourceType),
        )

    // 主体过滤 (用户/角色/部门)
    s.applySubjectFilter(ctx, query, userID)

    // 资源过滤
    if resourceID != nil {
        query.Where(ioresourcepermission.Or(
            ioresourcepermission.ResourceIDEQ(*resourceID),
            ioresourcepermission.ResourceIDIsNil(), // 全局权限
        ))
    } else {
        query.Where(ioresourcepermission.ResourceIDIsNil())
    }

    // 操作过滤
    operationMask := s.actionToMask(action)
    query.Where(
        ioresourcepermission.OperationsMaskGTE(operationMask),
    )

    // 3. 检查是否有有效权限
    count, err := query.Count(ctx)
    if err != nil {
        return false, err
    }

    allowed := count > 0

    // 4. 缓存结果 (5分钟)
    value := "0"
    if allowed {
        value = "1"
    }
    s.redis.Set(ctx, cacheKey, value, 5*time.Minute)

    return allowed, nil
}

// actionToMask 转换操作为掩码
func (s *PermissionService) actionToMask(action string) int {
    masks := map[string]int{
        "read":    1,
        "create":  2,
        "update":  4,
        "delete":  8,
        "execute": 16,
        "approve": 32,
        "export":  64,
    }
    return masks[action]
}
```

#### Task 2.2: 集成到Logic层

```go
// unified-io/rpc/internal/logic/data_source/create_data_source_logic.go

func (l *CreateDataSourceLogic) CreateDataSource(in *io.DataSourceInfo) (*io.BaseIDResp, error) {
    // 1. 权限检查
    allowed, err := l.svcCtx.PermissionService.CheckResourcePermission(
        l.ctx,
        l.getUserID(),
        "data_source",
        nil,  // 创建操作没有resourceID
        "create",
    )

    if err != nil {
        return nil, err
    }

    if !allowed {
        return nil, status.Error(codes.PermissionDenied, "没有创建数据源的权限")
    }

    // 2. 创建数据源
    created, err := l.svcCtx.DB.DataSource.Create().
        SetName(*in.Name).
        SetProviderType(*in.ProviderType).
        // ... 其他字段
        Save(l.ctx)

    if err != nil {
        return nil, err
    }

    return &io.BaseIDResp{Id: created.ID}, nil
}
```

---

### 5.3 阶段3: 数据权限拦截器集成 (Week 2)

#### Task 3.1: 注册数据权限拦截器

```go
// unified-io/rpc/internal/svc/service_context.go

func NewServiceContext(c config.Config) *ServiceContext {
    db := ent.NewClient(...)

    // 1. 注册租户Hook (优先级最高)
    db.Use(hooks.TenantMutationHook())
    db.Intercept(hooks.TenantQueryInterceptor())

    // 2. 注册数据权限拦截器
    db.Use(hooks.NewEnhancedDataPermInterceptor(&hooks.EnhancedDataPermissionConfig{
        Enabled:           true,
        DepartmentField:   "department_id",
        UserField:         "created_by",
        SkipSystemContext: true,
        UseStateManager:   true,
    }))

    return &ServiceContext{DB: db}
}
```

#### Task 3.2: 表Schema添加必要字段

确保所有需要数据权限控制的表包含以下字段：

```go
// unified-io/rpc/ent/schema/data_source.go

func (DataSource) Fields() []ent.Field {
    return []ent.Field{
        field.Uint64("department_id").
            Comment("部门ID"),
        field.Uint64("created_by").
            Comment("创建人ID"),
        field.Uint64("user_id").
            Optional().
            Comment("所属用户ID"),
        // ... 其他字段
    }
}
```

---

### 5.4 阶段4: API中间件集成 (Week 2-3)

#### Task 4.1: 配置统一中间件

```go
// unified-io/api/internal/svc/service_context.go

func NewServiceContext(c config.Config) *ServiceContext {
    // 1. 创建Casbin提供者
    coreRpc := corerpc.NewCore(zrpc.MustNewClient(c.CoreRpc))
    casbinProvider := dataperm.NewDefaultCasbinProvider(coreRpc, logger)

    // 2. 统一中间件集成
    middlewareCtx, err := integration.Setup(&integration.Config{
        Redis:     rds,
        JWTSecret: c.Auth.AccessSecret,
        Mode:      integration.Production,

        DataPermConfig: &integration.DataPermConfig{
            Enabled:        true,
            CasbinProvider: casbinProvider,
            ServiceName:    "unified-io",

            // 资源映射配置
            ResourceMapping: map[string]string{
                "/api/v1/data_source":    "data_source",
                "/api/v1/data_target":    "data_target",
                "/api/v1/provider":       "provider",
                "/api/v1/field_mapping":  "field_mapping",
                "/api/v1/discovery_task": "discovery_task",
                "/api/v1/sync_task":      "sync_task",
                "/api/v1/workflow":       "workflow",
            },

            // 操作映射配置
            ActionMapping: map[string]string{
                "GET":    "read",
                "POST":   "create",
                "PUT":    "update",
                "PATCH":  "update",
                "DELETE": "delete",
            },
        },
    })

    if err != nil {
        panic(err)
    }

    return &ServiceContext{
        MiddlewareCtx: middlewareCtx,
    }
}
```

#### Task 4.2: 应用中间件到服务

```go
// unified-io/api/unified_io.go

func main() {
    server := rest.MustNewServer(c.RestConf)
    defer server.Stop()

    ctx := svc.NewServiceContext(c)

    // 应用统一中间件
    integration.ApplyToServer(server, ctx.MiddlewareCtx)

    handler.RegisterHandlers(server, ctx)
    server.Start()
}
```

---

### 5.5 阶段5: 字段级权限与脱敏 (Week 3)

#### Task 5.1: 定义敏感字段配置

```go
// unified-io/rpc/internal/config/sensitive_fields.go

package config

import "github.com/coder-lulu/newbee-common/v2/middleware/dataperm"

var SensitiveFieldsConfig = map[string]dataperm.FieldMaskConfig{
    // 数据源凭证
    "data_source.password": {
        TableName:    "io_data_sources",
        FieldName:    "password",
        MaskType:     "full",
        AccessRoles:  []string{"admin", "system_admin"},
        Description:  "数据源密码",
    },
    "data_source.api_key": {
        TableName:    "io_data_sources",
        FieldName:    "api_key",
        MaskType:     "full",
        AccessRoles:  []string{"admin", "system_admin"},
        Description:  "API密钥",
    },
    "data_source.access_token": {
        TableName:    "io_data_sources",
        FieldName:    "access_token",
        MaskType:     "partial",
        AccessRoles:  []string{"admin", "system_admin", "operator"},
        MaskPattern:  "{{prefix4}}****{{suffix4}}",
        Description:  "访问令牌",
    },

    // 数据目标凭证
    "data_target.password": {
        TableName:    "io_data_targets",
        FieldName:    "password",
        MaskType:     "full",
        AccessRoles:  []string{"admin", "system_admin"},
        Description:  "目标密码",
    },
    "data_target.connection_string": {
        TableName:    "io_data_targets",
        FieldName:    "connection_string",
        MaskType:     "partial",
        AccessRoles:  []string{"admin", "system_admin"},
        MaskPattern:  "{{schema}}://****:****@{{host}}/{{database}}",
        Description:  "连接字符串",
    },
}
```

#### Task 5.2: 注册字段掩码处理器

```go
// unified-io/rpc/internal/svc/service_context.go

func NewServiceContext(c config.Config) *ServiceContext {
    // ...

    // 创建字段掩码处理器
    fieldMaskProcessor := dataperm.NewFieldMaskProcessor(logger, SensitiveFieldsConfig)

    // 注册自定义掩码策略
    fieldMaskProcessor.RegisterCustomMaskStrategy("connection_string", &ConnectionStringMaskStrategy{})

    return &ServiceContext{
        FieldMaskProcessor: fieldMaskProcessor,
    }
}
```

#### Task 5.3: 在Logic中应用字段掩码

```go
// unified-io/rpc/internal/logic/data_source/get_data_source_by_id_logic.go

func (l *GetDataSourceByIdLogic) GetDataSourceById(in *io.IDReq) (*io.DataSourceInfo, error) {
    // 查询数据
    ds, err := l.svcCtx.DB.DataSource.Get(l.ctx, in.Id)
    if err != nil {
        return nil, err
    }

    // 转换为响应
    resp := &io.DataSourceInfo{
        Id:           &ds.ID,
        Name:         &ds.Name,
        ProviderType: &ds.ProviderType,
        Host:         &ds.Host,
        Password:     &ds.Password,          // 可能被掩码
        ApiKey:       &ds.ApiKey,            // 可能被掩码
        AccessToken:  &ds.AccessToken,       // 可能被掩码
    }

    // 应用字段掩码
    userRoles := l.getUserRoles()
    maskedResp := l.svcCtx.FieldMaskProcessor.ProcessFieldMasks(
        l.ctx,
        "data_source",
        resp,
        userRoles,
    )

    return maskedResp.(*io.DataSourceInfo), nil
}
```

---

### 5.6 阶段6: Casbin规则集成 (Week 3-4)

#### Task 6.1: 定义Casbin策略模型

```ini
# unified-io/etc/casbin_model.conf

[request_definition]
r = sub, obj, act, dom

[policy_definition]
p = sub, obj, act, dom

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.obj == p.obj && r.act == p.act && r.dom == p.dom
```

#### Task 6.2: 初始化默认策略

```go
// unified-io/rpc/internal/plugin/unified_io_plugin.go

package plugin

// InitializeDefaultPolicies 初始化默认Casbin策略
func (p *UnifiedIOPlugin) InitializeDefaultPolicies(ctx context.Context, tenantID uint64) error {
    systemCtx := hooks.NewSystemContext(ctx)

    // 1. 创建管理员角色的数据权限规则
    _, err := p.svcCtx.DB.CasbinRule.Create().
        SetPtype("d").                              // 数据权限规则
        SetV0("admin").                             // 角色代码
        SetV1(fmt.Sprintf("%d", tenantID)).        // 租户ID
        SetV2("*").                                 // 资源类型（所有）
        SetV3("all").                               // 数据权限范围
        SetV4("").                                  // 自定义部门ID列表（空）
        SetServiceName("unified-io").
        SetRuleName("管理员默认数据权限").
        SetDescription("管理员拥有全部数据权限").
        SetCategory("data_permission").
        SetStatus(1).
        SetTenantID(tenantID).
        Save(systemCtx)

    if err != nil {
        return err
    }

    // 2. 创建普通用户角色的数据权限规则
    _, err = p.svcCtx.DB.CasbinRule.Create().
        SetPtype("d").
        SetV0("user").
        SetV1(fmt.Sprintf("%d", tenantID)).
        SetV2("*").
        SetV3("own").                               // 仅本人数据
        SetV4("").
        SetServiceName("unified-io").
        SetRuleName("普通用户默认数据权限").
        SetDescription("普通用户仅能查看自己创建的数据").
        SetCategory("data_permission").
        SetStatus(1).
        SetTenantID(tenantID).
        Save(systemCtx)

    if err != nil {
        return err
    }

    // 3. 发布Redis通知，触发Casbin重新加载
    updateMsg := fmt.Sprintf("UpdatePolicy:tenant_%d:data_perm", tenantID)
    return p.svcCtx.Redis.Publish(ctx, "casbin_watcher", updateMsg).Err()
}
```

---

### 5.7 阶段7: 测试与验证 (Week 4)

#### Task 7.1: 单元测试

```go
// unified-io/rpc/internal/service/permission_service_test.go

func TestPermissionService_CheckResourcePermission(t *testing.T) {
    tests := []struct {
        name         string
        userID       uint64
        resourceType string
        resourceID   *uint64
        action       string
        want         bool
        wantErr      bool
    }{
        {
            name:         "管理员可以访问所有资源",
            userID:       1,  // 管理员
            resourceType: "data_source",
            resourceID:   pointy.Uint64(100),
            action:       "read",
            want:         true,
            wantErr:      false,
        },
        {
            name:         "普通用户无法访问其他用户的资源",
            userID:       2,  // 普通用户
            resourceType: "data_source",
            resourceID:   pointy.Uint64(100),  // 其他用户创建的资源
            action:       "read",
            want:         false,
            wantErr:      false,
        },
        {
            name:         "部门经理可以访问本部门资源",
            userID:       3,  // 部门经理
            resourceType: "data_source",
            resourceID:   pointy.Uint64(101),  // 本部门资源
            action:       "read",
            want:         true,
            wantErr:      false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := permService.CheckResourcePermission(
                ctx,
                tt.userID,
                tt.resourceType,
                tt.resourceID,
                tt.action,
            )

            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            if got != tt.want {
                t.Errorf("got = %v, want %v", got, tt.want)
            }
        })
    }
}
```

#### Task 7.2: 集成测试

```go
// unified-io/rpc/test/integration/data_permission_test.go

func TestDataPermissionIntegration(t *testing.T) {
    // 场景1: 租户A的用户只能看到租户A的数据
    t.Run("租户隔离测试", func(t *testing.T) {
        ctxA := context.WithValue(context.Background(), "tenantId", uint64(1))
        ctxB := context.WithValue(context.Background(), "tenantId", uint64(2))

        // 租户A创建数据源
        dsA, err := db.DataSource.Create().
            SetName("TenantA_DS").
            SetTenantID(1).
            Save(ctxA)
        require.NoError(t, err)

        // 租户B查询，应该看不到租户A的数据
        results, err := db.DataSource.Query().All(ctxB)
        require.NoError(t, err)
        assert.Empty(t, results)
    })

    // 场景2: 用户只能看到其数据权限范围内的数据
    t.Run("数据权限范围测试", func(t *testing.T) {
        // 用户1: 全部数据权限
        ctx1 := buildContext(userID1, dataScope=DataPermAll)
        results1, _ := db.DataSource.Query().All(ctx1)
        assert.Len(t, results1, 10)  // 看到所有10条数据

        // 用户2: 仅本人数据权限
        ctx2 := buildContext(userID2, dataScope=DataPermOwn)
        results2, _ := db.DataSource.Query().All(ctx2)
        assert.Len(t, results2, 2)   // 只看到自己创建的2条
    })

    // 场景3: 敏感字段自动脱敏
    t.Run("字段脱敏测试", func(t *testing.T) {
        ds, err := logic.GetDataSourceById(ctx, &io.IDReq{Id: 1})
        require.NoError(t, err)

        // 普通用户看到的密码是掩码
        assert.Equal(t, "********", *ds.Password)

        // 管理员看到的是明文
        adminCtx := buildAdminContext()
        dsAdmin, err := logic.GetDataSourceById(adminCtx, &io.IDReq{Id: 1})
        require.NoError(t, err)
        assert.NotEqual(t, "********", *dsAdmin.Password)
    })
}
```

---

## 6. 测试与验收

### 6.1 测试清单

| 测试类型 | 测试项 | 验收标准 |
|---------|--------|---------|
| **功能测试** | 权限检查正确性 | 100%通过 |
| | 数据权限过滤正确性 | 100%通过 |
| | 字段脱敏正确性 | 100%通过 |
| | Casbin集成 | 规则生效 |
| **安全测试** | 租户隔离 | 0泄露 |
| | 越权访问防护 | 100%拦截 |
| | 敏感信息保护 | 100%掩码 |
| **性能测试** | 权限检查延迟 | <50ms (P95) |
| | 缓存命中率 | >80% |
| | 数据库查询影响 | <10% overhead |

### 6.2 验收标准

#### 6.2.1 功能验收

- [ ] 用户只能访问其权限范围内的资源
- [ ] 五级数据权限(all/custom_dept/own_dept_and_sub/own_dept/own)正确生效
- [ ] 敏感字段根据角色正确脱敏
- [ ] 租户间数据完全隔离

#### 6.2.2 性能验收

- [ ] 权限检查P95延迟 <50ms
- [ ] Redis缓存命中率 >80%
- [ ] 数据库查询性能影响 <10%

#### 6.2.3 安全验收

- [ ] 租户隔离测试100%通过
- [ ] 越权访问尝试100%被拦截
- [ ] 敏感信息无泄露

---

## 7. 总结

### 7.1 关键设计亮点

1. **完全复用现有架构**
   - CMDB的CI权限模型作为参考
   - Common包的数据权限中间件提供底层能力
   - Core服务的Casbin系统统一管理权限规则

2. **多层防护**
   - API层: 统一中间件检查资源访问权限
   - RPC层: Service检查资源操作权限
   - DB层: Ent拦截器自动应用数据过滤
   - 字段层: 字段掩码处理器保护敏感信息

3. **高性能设计**
   - Redis缓存优化权限检查
   - 位掩码优化操作权限存储
   - 状态管理器优化上下文传递

4. **易于维护**
   - 声明式配置敏感字段
   - 统一的权限规则管理
   - 完整的测试套件

### 7.2 实施路线图

| 阶段 | 时间 | 任务 | 交付物 |
|------|------|------|--------|
| 阶段1 | Week 1 | 数据模型与Schema | Migration脚本, Ent Schema |
| 阶段2 | Week 1-2 | 权限Service实现 | PermissionService, 单元测试 |
| 阶段3 | Week 2 | 数据权限拦截器集成 | ServiceContext配置 |
| 阶段4 | Week 2-3 | API中间件集成 | 统一中间件配置 |
| 阶段5 | Week 3 | 字段级权限与脱敏 | 敏感字段配置, FieldMaskProcessor |
| 阶段6 | Week 3-4 | Casbin规则集成 | 默认策略初始化, Plugin实现 |
| 阶段7 | Week 4 | 测试与验证 | 测试报告, 性能报告 |

### 7.3 后续优化方向

1. **可视化权限管理界面**
   - 权限规则的可视化配置
   - 权限变更审批流程
   - 权限使用情况分析

2. **高级数据权限规则**
   - 基于标签的权限控制
   - 基于时间的动态权限
   - 基于条件的复杂权限规则

3. **性能优化**
   - 权限规则预编译
   - 批量权限检查优化
   - 分布式缓存优化

---

**文档版本**: v1.0
**审核人**: 架构师团队
**批准人**: CTO
**生效日期**: 2025-10-19
