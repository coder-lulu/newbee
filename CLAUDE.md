# NewBee 编码准则

本文档规定了项目开发的核心准则，确保代码质量、一致性和可维护性。

## 1. 核心开发流程

### 1.1 自动生成文件保护规则（⚠️ 最高优先级）

**🔴 绝对禁止修改的文件** - 带有 `Code generated` 或 `DO NOT EDIT` 标记：

| 文件路径 | 生成命令 | 后果 |
|---------|---------|------|
| `rpc/internal/server/io_server.go` | `make gen-rpc` | 修改会在下次生成时**完全丢失** |
| `rpc/ioclient/*.go` | `make gen-rpc` | 修改会在下次生成时**完全丢失** |
| `rpc/types/**/*.pb.go` | `make gen-rpc` | 修改会在下次生成时**完全丢失** |
| `api/internal/handler/routes.go` | `make gen-api` | 修改会在下次生成时**完全丢失** |
| `api/internal/types/types.go` | `make gen-api` | 修改会在下次生成时**完全丢失** |
| `rpc/ent/` (非schema) | `make gen-ent` | 修改会在下次生成时**完全丢失** |

**识别方法**：
```bash
# 检查文件头部是否有生成标记
head -5 <文件路径> | grep -E "Code generated|DO NOT EDIT"
```

**⚠️ 如果需要扩展功能**：

| 需求 | ❌ 错误做法 | ✅ 正确做法 |
|------|-----------|-----------|
| 添加RPC方法验证 | 修改 `io_server.go` | 在 `internal/logic/**/*_logic.go` 中实现 |
| 添加统一拦截器 | 修改 `io_server.go` | 在 `service_context.go` 中注册拦截器 |
| 添加自定义服务 | 修改 `io_server.go` | 在 `service_context.go` 中添加自定义服务 |
| 修改路由配置 | 修改 `routes.go` | 在 `service_context.go` 中配置中间件 |

**详细文档**：参见 `unified-io/docs/AUTO_GENERATED_FILES_LIST.md`

---

### 1.2 数据模型与RPC接口变更流程（强制遵循）

**步骤顺序**：
1. **修改 ent Schema** → 2. **生成 ent 代码** → 3. **生成 RPC 代码**

```bash
# 步骤1: 修改 rpc/ent/schema/ 目录下的schema文件
# 步骤2: 生成ent代码
go run entgo.io/ent/cmd/ent generate --template glob="./ent/template/*.tmpl" ./ent/schema --feature sql/execquery,intercept,sql/modifier

# 步骤3: 生成RPC代码  
make gen-rpc
```

**⚠️ 代码生成前必做检查**：
```bash
# 1. 提交当前更改
git add . && git commit -m "执行make命令前备份"

# 2. 执行make命令
make gen-rpc

# 3. 检查变更 - 确保只有生成文件被修改
git diff

# 4. 确认service_context.go和logic文件未被意外修改
git diff rpc/internal/svc/service_context.go
git diff rpc/internal/logic/
```

**重要提醒**：
- 租户相关数据必须使用 `TenantMixin`
- 绕过此流程的变更不被允许
- **Logic文件保护**：将Makefile中 `--overwrite=true` 改为 `--overwrite=false`

---

### 1.3 API响应格式规范（强制遵循）

**🎯 标准API响应格式**：所有API接口必须返回统一的响应结构，确保前端能够正确解析。

#### 格式规范

**1. 单个对象响应**（如：详情查询、创建、更新）

API定义（.api文件）：
```go
// 示例：用户详情响应
UserInfoResp {
    Code int      `json:"code"`
    Msg  string   `json:"msg"`
    Data UserInfo `json:"data"`
}
```

返回JSON格式：
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "id": 1,
    "username": "admin",
    "nickname": "管理员"
  }
}
```

**2. 列表响应**（如：分页查询、列表查询）

API定义（.api文件）：
```go
// 步骤1：定义列表响应（外层包含code/msg）
UserListResp {
    Code int          `json:"code"`
    Msg  string       `json:"msg"`
    Data UserListData `json:"data"`
}

// 步骤2：定义列表数据（内层包含total和数据数组）
UserListData {
    Total uint64     `json:"total"`
    Data  []UserInfo `json:"data"`
}
```

返回JSON格式：
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "total": 100,
    "data": [
      {"id": 1, "username": "user1"},
      {"id": 2, "username": "user2"}
    ]
  }
}
```

**3. 简单响应**（如：删除、状态更新）

API定义（.api文件）：
```go
BaseResp {
    Code uint32 `json:"code"`
    Msg  string `json:"msg"`
}
```

返回JSON格式：
```json
{
  "code": 0,
  "msg": "操作成功"
}
```

#### Logic层实现规范

**单个对象响应示例**：
```go
func (l *GetUserByIdLogic) GetUserById(req *types.IDReq) (resp *types.UserInfoResp, err error) {
    // 调用RPC获取数据
    result, err := l.svcCtx.CoreRpc.GetUserById(l.ctx, &core.IDReq{Id: req.Id})
    if err != nil {
        return nil, err
    }

    // 构造响应 - 必须包含Code和Msg
    resp = &types.UserInfoResp{
        Code: 0,
        Msg:  "success",
        Data: types.UserInfo{
            Id:       result.Id,
            Username: result.Username,
            // ... 其他字段
        },
    }
    return resp, nil
}
```

**列表响应示例**：
```go
func (l *GetUserListLogic) GetUserList(req *types.UserListReq) (resp *types.UserListResp, err error) {
    // 调用RPC获取数据
    data, err := l.svcCtx.CoreRpc.GetUserList(l.ctx, &core.UserListReq{
        Page:     req.Page,
        PageSize: req.PageSize,
    })
    if err != nil {
        return nil, err
    }

    // 构造响应 - 双层嵌套结构
    resp = &types.UserListResp{
        Code: 0,
        Msg:  "success",
        Data: types.UserListData{
            Total: data.Total,
            Data:  make([]types.UserInfo, 0),
        },
    }

    // 转换数据
    for _, v := range data.Data {
        resp.Data.Data = append(resp.Data.Data, types.UserInfo{
            Id:       v.Id,
            Username: v.Username,
            // ... 其他字段
        })
    }

    return resp, nil
}
```

#### 常见错误

**❌ 错误示例1：缺少外层code/msg字段**
```go
// 错误的API定义
ScriptListResp {
    Total uint64       `json:"total"`
    Data  []ScriptItem `json:"data"`
}
// 问题：前端期待 {code, msg, data: {total, data}}
// 但实际返回 {total, data}
```

**❌ 错误示例2：Logic层未设置code和msg**
```go
// 错误的Logic实现
resp = &types.UserListResp{
    // 缺少 Code 和 Msg
    Data: types.UserListData{
        Total: data.Total,
        Data:  items,
    },
}
```

**✅ 正确示例：完整的响应结构**
```go
// 正确的API定义
ScriptListResp {
    Code int            `json:"code"`
    Msg  string         `json:"msg"`
    Data ScriptListData `json:"data"`
}

ScriptListData {
    Total uint64       `json:"total"`
    Data  []ScriptItem `json:"data"`
}

// 正确的Logic实现
resp = &types.ScriptListResp{
    Code: 0,
    Msg:  "success",
    Data: types.ScriptListData{
        Total: result.Total,
        Data:  items,
    },
}
```

#### 修复流程

当发现API返回格式不符合规范时：

1. **修改.api文件**：添加嵌套的数据结构
2. **重新生成代码**：运行 `goctl api go -api desc/all.api -dir . --style=go_zero`
3. **修改Logic层**：更新响应构造逻辑，确保包含code/msg
4. **编译验证**：运行 `go build -v .` 确保编译通过
5. **测试验证**：使用curl或前端测试接口返回格式

#### 参考示例

**标准实现参考**：
- `core/api/desc/core/user.api` - 用户API定义
- `core/api/internal/logic/user/get_user_list_logic.go` - 列表查询Logic
- `core/api/internal/logic/user/get_user_by_id_logic.go` - 详情查询Logic

---

### 1.4 树形分类查询规范（强制遵循）

**🎯 树形分类的递归查询**：当数据支持树形分类（如脚本分类、部门组织）时，查询父分类必须包含其所有子分类下的数据。

#### 业务场景

用户点击父级分类时，期望看到该分类及其所有子分类下的数据：
```
- 运维脚本（父分类ID: 1）
  ├── 备份脚本（子分类ID: 2）
  └── 监控脚本（子分类ID: 3）
      └── 性能监控（孙分类ID: 4）
```

**期望行为**：
- 点击"运维脚本" → 显示ID为 1, 2, 3, 4 所有分类下的脚本
- 点击"监控脚本" → 显示ID为 3, 4 分类下的脚本

#### 实现规范

**❌ 错误实现：只查询精确匹配的分类**

```go
// 错误：只查询当前分类ID，不包含子分类
if in.CategoryId != nil {
    predicates = append(predicates, script.CategoryIDEQ(*in.CategoryId))
}
```

**问题**：用户点击父分类时，看不到子分类下的数据。

**✅ 正确实现：递归查询所有子分类**

**步骤1：实现递归查询函数**

```go
// getAllCategoryIds 递归获取指定分类及其所有子分类的ID列表
func (l *GetScriptListLogic) getAllCategoryIds(categoryId uint64) ([]uint64, error) {
    // 结果集合，包含父分类ID本身
    categoryIds := []uint64{categoryId}

    // 递归查询所有子分类
    children, err := l.getChildCategoryIds(categoryId)
    if err != nil {
        return nil, err
    }

    categoryIds = append(categoryIds, children...)
    return categoryIds, nil
}

// getChildCategoryIds 递归查询所有子分类ID
func (l *GetScriptListLogic) getChildCategoryIds(parentId uint64) ([]uint64, error) {
    var result []uint64

    // 查询直接子分类
    directChildren, err := l.svcCtx.DB.ScriptCategory.Query().
        Where(scriptcategory.ParentIDEQ(parentId)).
        Select(scriptcategory.FieldID).
        All(l.ctx)

    if err != nil {
        return nil, err
    }

    // 如果没有子分类，返回空列表
    if len(directChildren) == 0 {
        return result, nil
    }

    // 遍历直接子分类，递归查询其子分类
    for _, child := range directChildren {
        // 添加当前子分类ID
        result = append(result, child.ID)

        // 递归查询该子分类的子分类
        grandChildren, err := l.getChildCategoryIds(child.ID)
        if err != nil {
            return nil, err
        }

        result = append(result, grandChildren...)
    }

    return result, nil
}
```

**步骤2：在查询逻辑中使用**

```go
// 支持按分类ID查询（包含子分类）
if in.CategoryId != nil {
    // 递归查询该分类及其所有子分类的ID
    categoryIds, err := l.getAllCategoryIds(*in.CategoryId)
    if err != nil {
        l.Logger.Errorw("Failed to get category ids",
            logx.Field("category_id", *in.CategoryId),
            logx.Field("error", err))
        // 如果查询失败，回退到只查询当前分类
        predicates = append(predicates, script.CategoryIDEQ(*in.CategoryId))
    } else {
        // 使用IN查询所有分类ID（包含父分类和所有子分类）
        predicates = append(predicates, script.CategoryIDIn(categoryIds...))
    }
}
```

#### Schema要求

树形分类的Schema必须包含 `parent_id` 字段和自关联Edge：

```go
func (ScriptCategory) Fields() []ent.Field {
    return []ent.Field{
        field.String("name").Comment("分类名称"),
        field.String("code").Comment("分类标识代码"),

        // 必须：支持树形结构的父分类ID
        field.Uint64("parent_id").
            Comment("父分类ID（支持树形结构）").
            Optional(),

        field.Uint32("sort_order").Comment("排序顺序").Default(0),
    }
}

func (ScriptCategory) Edges() []ent.Edge {
    return []ent.Edge{
        // 必须：父分类关系（自关联）
        edge.To("children", ScriptCategory.Type).
            From("parent").
            Field("parent_id").
            Unique(),
    }
}

func (ScriptCategory) Indexes() []ent.Index {
    return []ent.Index{
        // 必须：父分类查询索引
        index.Fields("tenant_id", "parent_id"),
    }
}
```

#### 性能优化

**1. 添加数据库索引**
```sql
-- 父分类查询索引（必须）
CREATE INDEX idx_parent_id ON ops_script_categories(tenant_id, parent_id);
```

**2. 缓存优化（可选）**
```go
// 对于频繁访问的分类树，可以缓存分类ID映射
type CategoryCache struct {
    mu        sync.RWMutex
    cache     map[uint64][]uint64 // categoryId -> [categoryId, child1, child2, ...]
    expireAt  time.Time
}

func (l *GetScriptListLogic) getAllCategoryIdsWithCache(categoryId uint64) ([]uint64, error) {
    // 检查缓存
    if ids, ok := l.getCachedCategoryIds(categoryId); ok {
        return ids, nil
    }

    // 缓存未命中，执行查询
    ids, err := l.getAllCategoryIds(categoryId)
    if err != nil {
        return nil, err
    }

    // 更新缓存
    l.setCachedCategoryIds(categoryId, ids)
    return ids, nil
}
```

#### 测试验证

**测试用例**：
```go
func TestGetScriptList_WithCategory(t *testing.T) {
    // 准备测试数据
    // 分类1（父）
    //   ├── 分类2（子）
    //   └── 分类3（子）
    //       └── 分类4（孙）

    // 测试1：查询父分类，应包含所有子分类的脚本
    result := GetScriptList(&ScriptListReq{CategoryId: 1})
    // 断言：应返回分类1,2,3,4下的所有脚本

    // 测试2：查询子分类，应包含其子分类的脚本
    result := GetScriptList(&ScriptListReq{CategoryId: 3})
    // 断言：应返回分类3,4下的脚本

    // 测试3：查询叶子分类，只返回该分类的脚本
    result := GetScriptList(&ScriptListReq{CategoryId: 4})
    // 断言：只返回分类4下的脚本
}
```

#### 适用场景

此规范适用于所有树形分类查询场景：
- ✅ 脚本分类查询
- ✅ 部门组织查询
- ✅ CMDB CI类型分组查询
- ✅ 菜单权限查询
- ✅ 其他任何支持树形结构的分类查询

#### 参考实现

**标准实现参考**：
- `ops-center/rpc/internal/logic/script/get_script_list_logic.go` - 脚本列表查询（含递归子分类）
- `ops-center/rpc/ent/schema/script_category.go` - 树形分类Schema定义

---

## 2. 多租户架构准则

### 2.1 新服务接入要求

**必要步骤**：
1. **导入公共库**：`github.com/coder-lulu/newbee-common`
2. **Schema配置**：业务实体必须包含 `mixins.TenantMixin{}`
3. **服务初始化**：注册租户Hook和拦截器
4. **API保护**：使用 `TenantCheck` 中间件
5. **系统操作**：使用 `hooks.NewSystemContext()` 处理全局操作

**ServiceContext初始化模板**：
```go
import "github.com/coder-lulu/newbee-common/orm/ent/hooks"

db := ent.NewClient(...)
// 必须注册
db.Use(hooks.TenantMutationHook())
db.Intercept(hooks.TenantQueryInterceptor())
```

**API定义模板**：
```go
@server(
    group: api_group
)
```

**重要架构变更**：
- 🎯 **JWT认证已迁移到统一中间件框架** - 无需在API文件中配置 `jwt: Auth`
- 🎯 **所有中间件统一管理** - 通过 `common/middleware/integration` 框架统一配置

### 2.2 租户安全编码规范

#### 核心安全准则
**禁止**：
- ❌ 使用原生SQL绕过ent的Hook机制
- ❌ 直接操作`tenant_id`字段
- ❌ 在业务逻辑中手动添加租户过滤条件
- ❌ 缓存跨租户的敏感数据

**必须**：
- ✅ 所有数据库操作通过ent进行
- ✅ 依赖底层Hook实现自动租户隔离
- ✅ 系统级操作使用SystemContext
- ✅ 敏感操作记录审计日志

#### Schema设计规范
```go
// ✅ 正确示例
func (BusinessEntity) Mixin() []ent.Mixin {
    return []ent.Mixin{
        mixins.IDMixin{},
        mixins.StatusMixin{},
        mixins.TenantMixin{}, // 必须包含
    }
}

// ❌ 错误示例：缺少TenantMixin - 安全漏洞！
func (BusinessEntity) Mixin() []ent.Mixin {
    return []ent.Mixin{
        mixins.IDMixin{},
        mixins.StatusMixin{},
        // 缺少TenantMixin
    }
}
```

#### 数据操作规范
```go
// ✅ 正确的业务查询
users, err := l.svcCtx.DB.User.Query().Where(user.StatusEQ(1)).All(l.ctx)

// ✅ 正确的系统操作
systemCtx := hooks.NewSystemContext(l.ctx)
err := l.svcCtx.DB.Role.Create().SetName("admin").Exec(systemCtx)

// ❌ 严禁原生SQL
err := l.svcCtx.DB.Driver().Query("SELECT * FROM users WHERE tenant_id = ?", tenantID)
```

## 3. 数据权限架构准则

### 3.1 新服务数据权限接入

**必要步骤**：
1. **导入数据权限库**：`github.com/coder-lulu/newbee-common/orm/ent/hooks`
2. **API中间件**：添加 `DataPerm` 中间件
3. **注册拦截器**：使用 `RegisterDataPermissionInterceptorsWithTenant`
4. **表字段要求**：包含 `department_id`, `user_id`, `tenant_id`
5. **权限范围**：支持五级数据权限（All, CustomDept, OwnDeptAndSub, OwnDept, Self）

**初始化模板**：
```go
import "github.com/coder-lulu/newbee-common/orm/ent/hooks"

// RPC服务中注册拦截器
hooks.RegisterDataPermissionInterceptorsWithTenant(db, 
    "users", "departments", "positions", "roles")

// API服务中添加中间件
@server(
    jwt: Auth
    middleware: Authority,TenantCheck,DataPerm  // 必须包含DataPerm
)
```

**详细集成说明**：参见 `data_permission_integration_guide.md`

### 3.2 数据权限规则存储架构（🔥 Phase 3 - 2025-10）

**重要架构变更**：从 v2.0 开始，数据权限范围从 `sys_roles.data_scope` 字段迁移到 `sys_casbin_rules` 表统一管理。

#### 数据权限规则结构 (ptype='d')

**表**: `sys_casbin_rules`

| 字段 | 说明 | 示例值 |
|------|------|--------|
| `ptype` | 规则类型，固定为 `'d'` (data permission) | `d` |
| `v0` | 角色代码 (Subject) | `admin` |
| `v1` | 租户ID (Domain) | `1` |
| `v2` | 资源类型 (通配符) | `*` |
| `v3` | 数据权限范围 | `all`, `custom_dept`, `own_dept_and_sub`, `own_dept`, `own` |
| `v4` | 自定义部门ID列表 (JSON) | `[1,2,3]` |
| `tenant_id` | 租户ID (用于租户隔离) | `1` |

#### 数据权限范围枚举

| 字符串值 (v3) | 枚举值 | 说明 |
|--------------|--------|------|
| `all` | `1` | 全部数据权限 |
| `custom_dept` | `2` | 自定义部门数据权限 |
| `own_dept_and_sub` | `3` | 本部门及下级部门数据权限 |
| `own_dept` | `4` | 仅本部门数据权限 |
| `own` | `5` | 仅本人数据权限 |

#### 迁移说明

**Phase 3架构变更**：
- ✅ **移除**: `sys_roles.data_scope` 字段（已废弃）
- ✅ **保留**: `sys_roles.custom_dept_ids` 字段（仍然存储，但查询时从casbin_rules读取）
- ✅ **保留**: Proto定义中的 `data_scope` 字段（向后兼容）
- ✅ **新增**: 运行时从 `sys_casbin_rules` 查询数据权限范围

**数据库迁移脚本**: `/opt/code/newbee/core/rpc/migrations/phase3_remove_data_scope_field.sql`

```sql
-- 1. 数据迁移：将sys_roles.data_scope同步到sys_casbin_rules
-- 2. 移除字段：ALTER TABLE sys_roles DROP COLUMN data_scope;
```

### 3.3 数据权限范围查询

#### 查询Helper函数

**位置**: `core/rpc/internal/logic/role/data_scope_helper.go`

```go
import (
    "github.com/coder-lulu/newbee-common/orm/ent/hooks"
    "github.com/coder-lulu/newbee-core/rpc/ent/casbinrule"
)

// getDataScopeFromCasbin 从sys_casbin_rules查询角色的数据权限范围
func getDataScopeFromCasbin(ctx context.Context, db *ent.Client, roleCode string, tenantID uint64) (uint32, error) {
    // 使用SystemContext绕过租户隔离
    systemCtx := hooks.NewSystemContext(ctx)

    // 查询数据权限规则（ptype='d'）
    rule, err := db.CasbinRule.Query().
        Where(
            casbinrule.PtypeEQ("d"),                      // 数据权限规则
            casbinrule.V0EQ(roleCode),                    // 角色代码
            casbinrule.V1EQ(fmt.Sprintf("%d", tenantID)), // 租户ID
        ).
        First(systemCtx)

    if err != nil {
        if ent.IsNotFound(err) {
            return 5, nil // 默认为 own (最严格的权限)
        }
        return 0, err
    }

    // 将v3字段（数据权限范围字符串）转换为枚举值
    return dataScopeStringToEnum(rule.V3), nil
}

// dataScopeStringToEnum 将数据权限范围字符串转换为枚举值
func dataScopeStringToEnum(dataScope string) uint32 {
    switch dataScope {
    case "all": return 1
    case "custom_dept": return 2
    case "own_dept_and_sub": return 3
    case "own_dept": return 4
    case "own": return 5
    default: return 5 // 默认为 own (最严格的权限)
    }
}

// dataScopeEnumToString 将数据权限范围枚举值转换为字符串（用于更新操作）
func dataScopeEnumToString(dataScope uint32) string {
    switch dataScope {
    case 1: return "all"
    case 2: return "custom_dept"
    case 3: return "own_dept_and_sub"
    case 4: return "own_dept"
    case 5: return "own"
    default: return "own" // 默认值
    }
}
```

#### 角色查询示例

**GetRoleById逻辑** - `core/rpc/internal/logic/role/get_role_by_id_logic.go`:

```go
func (l *GetRoleByIdLogic) GetRoleById(in *core.IDReq) (*core.RoleInfo, error) {
    // 查询角色基础信息
    result, err := l.svcCtx.DB.Role.Get(l.ctx, in.Id)
    if err != nil {
        return nil, dberrorhandler.DefaultEntError(l.Logger, err, in)
    }

    roleInfo := &core.RoleInfo{
        Id:            &result.ID,
        Name:          &result.Name,
        Code:          &result.Code,
        CustomDeptIds: result.CustomDeptIds,
    }

    // 🔥 Phase 3: 从sys_casbin_rules查询数据权限范围
    dataScope, err := getDataScopeFromCasbin(l.ctx, l.svcCtx.DB, result.Code, result.TenantID)
    if err != nil {
        logx.Errorw("Failed to query data scope from casbin",
            logx.Field("role_code", result.Code),
            logx.Field("tenant_id", result.TenantID),
            logx.Field("error", err))
        dataScope = 5 // own (最严格的权限)
    }
    roleInfo.DataScope = pointy.GetPointer(dataScope)

    return roleInfo, nil
}
```

#### 角色数据权限更新

**更新数据权限规则** - 在 `assign_role_data_scope_logic.go` 或 `update_role_logic.go` 中：

```go
// 🔥 Phase 3: 更新数据权限规则到sys_casbin_rules
func updateDataPermissionRule(ctx context.Context, tx *ent.Tx, roleCode string, tenantID uint64, dataScope uint32, customDeptIds []uint64) error {
    systemCtx := hooks.NewSystemContext(ctx)

    // 1. 删除旧的数据权限规则
    _, err := tx.CasbinRule.Delete().
        Where(
            casbinrule.PtypeEQ("d"),
            casbinrule.V0EQ(roleCode),
            casbinrule.TenantIDEQ(tenantID),
        ).
        Exec(systemCtx)
    if err != nil {
        return fmt.Errorf("failed to delete old data permission rule: %w", err)
    }

    // 2. 创建新的数据权限规则
    dataScopeStr := dataScopeEnumToString(dataScope)
    customDeptIdsJSON, _ := json.Marshal(customDeptIds)

    _, err = tx.CasbinRule.Create().
        SetPtype("d").
        SetV0(roleCode).
        SetV1(fmt.Sprintf("%d", tenantID)).
        SetV2("*").
        SetV3(dataScopeStr).
        SetV4(string(customDeptIdsJSON)).
        SetTenantID(tenantID).
        SetServiceName("core").
        SetStatus(1).
        Save(systemCtx)
    if err != nil {
        return fmt.Errorf("failed to create data permission rule: %w", err)
    }

    // 3. 发布Redis通知，触发Casbin策略重新加载
    updateMsg := fmt.Sprintf("UpdatePolicy:tenant_%d:data_perm", tenantID)
    return redis.Publish(ctx, "casbin_watcher", updateMsg).Err()
}
```

#### 租户初始化集成

**新租户初始化** - 在 `core_plugin_methods.go` 的 `initAdminDataPermissions()` 方法中：

```go
// 🔥 Phase 3: 为管理员角色初始化数据权限规则
func (p *CoreTenantPlugin) initAdminDataPermissions(ctx context.Context, tx *ent.Tx, adminRole *ent.Role, tenantID uint64) error {
    systemCtx := hooks.NewSystemContext(ctx)

    // 为租户管理员创建全部数据权限（dataScope="all"）
    _, err := tx.CasbinRule.Create().
        SetPtype("d").
        SetV0(adminRole.Code).                        // 角色代码
        SetV1(fmt.Sprintf("%d", tenantID)).          // 租户ID
        SetV2("*").                                   // 资源类型（所有）
        SetV3("all").                                 // 数据权限范围
        SetV4("").                                    // 自定义部门ID列表（空）
        SetServiceName("core").
        SetRuleName(fmt.Sprintf("%s数据权限", adminRole.Name)).
        SetDescription(fmt.Sprintf("角色%s的默认数据权限规则，数据范围：all（全部数据）", adminRole.Name)).
        SetCategory("data_permission").
        SetVersion("1.0.0").
        SetStatus(1).
        SetTenantID(tenantID).
        Save(systemCtx)

    if err != nil {
        return fmt.Errorf("failed to create admin data permission rule: %w", err)
    }

    // 发布Redis通知
    updateMsg := fmt.Sprintf("UpdatePolicy:tenant_%d:data_perm", tenantID)
    return p.svcCtx.Redis.Publish(ctx, "casbin_watcher", updateMsg).Err()
}
```

### 3.4 Phase 3架构优势

**统一管理**:
- ✅ 数据权限规则与API权限规则在同一张表 (`sys_casbin_rules`)
- ✅ 统一的审计日志和版本控制
- ✅ 统一的权限变更通知机制（Redis Watcher）

**灵活性**:
- ✅ 支持动态权限变更，无需修改数据库schema
- ✅ 支持细粒度的权限控制（通过v2字段可以指定具体资源类型）
- ✅ 便于扩展新的数据权限范围类型

**性能**:
- ✅ 减少JOIN查询（无需关联sys_roles表）
- ✅ 利用Casbin的缓存机制
- ✅ 支持批量权限查询优化

**向后兼容**:
- ✅ Proto定义保持不变
- ✅ 前端无需任何修改
- ✅ API响应格式完全兼容

## 4. 测试要求

### 4.1 必须包含的测试
- [ ] 租户隔离测试
- [ ] 数据权限隔离测试  
- [ ] SystemContext权限测试
- [ ] API安全测试
- [ ] 性能基准测试

### 4.2 测试模板
```go
// 租户隔离测试
func TestTenantIsolation(t *testing.T) {
    ctxA := context.WithValue(context.Background(), "tenantId", uint64(1))
    ctxB := context.WithValue(context.Background(), "tenantId", uint64(2))
    
    entityA, err := client.Entity.Create().SetName("test").Save(ctxA)
    require.NoError(t, err)
    
    entities, err := client.Entity.Query().All(ctxB)
    require.NoError(t, err)
    assert.Empty(t, entities) // 租户B看不到租户A的数据
}
```

## 5. 性能与监控规范

### 5.1 性能优化
- 合理使用 `tenant_id` 和 `department_id` 索引
- 避免跨租户的复杂JOIN查询
- 控制SystemContext使用频率
- 优化租户级别和权限级别的缓存策略

### 5.2 监控指标
- 租户数据隔离违规次数
- 数据权限违规次数
- SystemContext使用频率和来源
- 异常的跨租户/跨权限访问尝试

## 6. 发布前检查清单

### 6.1 Schema检查
- [ ] 业务实体使用了TenantMixin
- [ ] 数据权限相关表包含必要字段
- [ ] 系统级实体有明确的排除说明

### 6.2 代码检查  
- [ ] 正确注册租户和数据权限拦截器
- [ ] 无原生SQL绕过Hook机制
- [ ] SystemContext使用有合理性说明
- [ ] API包含必要的安全中间件

### 6.3 测试检查
- [ ] 租户隔离测试100%通过
- [ ] 数据权限隔离测试通过
- [ ] 性能测试满足要求
- [ ] 安全扫描无高危问题

### 6.4 违规后果
- 发现安全违规的代码必须立即回滚
- 相关开发人员必须重新学习安全规范
- 严重违规行为将影响绩效考核

---

## 7. 统一中间件架构准则 🎯

### 7.1 架构概述

**重要变更**：从 v2.0 开始，NewBee 采用统一中间件架构，**完全替代** go-zero 原生的 `jwt: Auth` 配置方式。

#### 核心优势
- ✅ **统一管理** - 所有中间件（Auth、TenantCheck、DataPerm、Audit）在同一框架中管理
- ✅ **性能优化** - 消除重复JWT验证，提升50%认证性能
- ✅ **配置简化** - 单点配置，支持环境预设（Production/Development/Testing）
- ✅ **可维护性** - 统一的中间件优先级和错误处理

### 7.2 核心配置

**ServiceContext集成模板**：
```go
import "github.com/coder-lulu/newbee-common/middleware/integration"

// 统一中间件集成 - 新版标准做法
result, err := integration.Setup(&integration.Config{
    Redis:     rds,
    JWTSecret: jwtSecret,
    Mode:      integration.Production, // Production/Development/Testing
})
if err != nil {
    panic("统一中间件集成失败: " + err.Error())
}

// 应用到服务
integration.ApplyToServer(server, result)
```

### 7.3 API定义规范

**✅ 新版正确写法**：
```go
@server(
    group: user  // 只需要指定group，无需jwt配置
)
service Core {
    @handler createUser
    post /user/create (UserInfo) returns (BaseMsgResp)
}
```

**❌ 旧版写法（已弃用）**：
```go
@server(
    jwt: Auth            // ❌ 已弃用，会导致重复认证
    group: user
)
```

### 7.4 JWT认证工作原理

#### 中间件执行链
```
1. Auth Plugin (优先级10) - 统一JWT认证
   ↓
2. TenantCheck Plugin (优先级20) - 租户验证
   ↓  
3. DataPerm Plugin (优先级30) - 数据权限
   ↓
4. Audit Plugin (优先级40) - 审计日志
   ↓
5. 业务处理器
```

#### 上下文管理
- **用户ID** - `result.ContextManager.GetUserID(ctx)`
- **租户ID** - `result.ContextManager.GetTenantID(ctx)`
- **部门ID** - `result.ContextManager.GetDeptID(ctx)`
- **数据权限** - `result.ContextManager.GetDataScope(ctx)`

### 7.5 环境配置预设

#### Production模式（默认）
```go
Mode: integration.Production
```
- 启用所有安全中间件
- 严格的错误处理
- 完整的审计日志

#### Development模式
```go
Mode: integration.Development
```
- 放宽认证检查
- 详细的调试日志
- 性能分析支持

#### Testing模式
```go
Mode: integration.Testing
```
- Mock中间件
- 最小化外部依赖
- 快速测试执行

### 7.6 迁移指南

#### 从旧版迁移步骤
1. **移除API文件中的 `jwt: Auth` 配置**
2. **确保ServiceContext使用统一集成**
3. **重新生成routes.go**
4. **验证认证功能正常**

#### 验证方法
```bash
# 测试公共接口
curl -s -o /dev/null -w "%{http_code}" "http://localhost:9100/captcha"
# 应返回: 200

# 测试认证接口（无token）
curl -s -o /dev/null -w "%{http_code}" -X POST "http://localhost:9100/user/list"
# 应返回: 401
```

### 7.7 常见问题排查

#### 问题1：认证失败
**症状**：所有需要认证的接口返回401
**检查**：
- JWT密钥配置是否正确
- 统一中间件是否正确初始化
- Auth插件是否已启用

#### 问题2：重复认证
**症状**：API响应慢，日志显示重复认证
**原因**：API文件中仍有 `jwt: Auth` 配置
**解决**：移除所有 `.api` 文件中的 `jwt: Auth`

#### 问题3：上下文信息缺失
**症状**：获取不到用户/租户信息
**检查**：
- 中间件执行顺序是否正确
- ContextManager是否正确注入

### 7.8 性能监控

#### 关键指标
- **认证延迟** - Auth Plugin处理时间
- **缓存命中率** - JWT Cache命中率
- **中间件执行时间** - 各插件耗时分析

#### 监控代码示例
```go
// 获取中间件性能指标
manager := result.Manager
metrics := manager.GetMetrics()
log.Printf("Auth cache hit rate: %.2f%%", metrics.AuthCacheHitRate)
```

---

---

## 8. 代码生成与文件保护规则 🛡️

### 8.1 文件分类与保护级别

#### 🔴 级别1 - 绝对禁止修改 (自动生成)

**RPC服务**:
- `internal/server/io_server.go` - gRPC路由分发器
- `ioclient/io.go` - RPC客户端代码
- `types/io/*.pb.go` - Protobuf消息定义
- `ent/**/*.go` (除schema外) - ORM生成代码

**API服务**:
- `internal/handler/routes.go` - HTTP路由注册
- `internal/types/types.go` - 请求/响应类型

**识别特征**: 文件头部包含以下任一标记
```go
// Code generated by goctl. DO NOT EDIT.
// Code generated by protoc-gen-go. DO NOT EDIT.
// Code generated by ent. DO NOT EDIT.
```

#### 🟢 级别2 - 完全安全 (手动维护)

**绝对不会被覆盖的文件**:
- `internal/svc/service_context.go` (RPC和API)
- `internal/config/config.go` (RPC和API)
- `internal/logic/**/*_logic.go` (业务逻辑)
- `internal/middleware/**/*.go` (自定义中间件)
- `ent/schema/*.go` (数据模型定义)
- 所有自定义包 (cache, monitoring, services, lifecycle等)

#### 🟡 级别3 - 条件保护 (取决于配置)

- `internal/logic/**/*_logic.go` (RPC) - 使用`--overwrite=false`时安全
- `internal/handler/**/*_handler.go` (API) - 首次生成后不覆盖

### 8.2 架构层次理解

```
┌─────────────────────────────────────────┐
│  gRPC/HTTP Request                      │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  ⚠️ 自动生成层 - 禁止修改                 │
│  ┌─────────────────────────────────┐   │
│  │ RPC: io_server.go               │   │
│  │ API: routes.go                  │   │
│  │ 职责: 路由分发                   │   │
│  └─────────────────────────────────┘   │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  ✅ 业务逻辑层 - 可以修改                 │
│  ┌─────────────────────────────────┐   │
│  │ internal/logic/**/*_logic.go    │   │
│  │ 职责: 业务逻辑实现               │   │
│  │ - 参数验证                      │   │
│  │ - 业务规则                      │   │
│  │ - 数据库操作                    │   │
│  │ - 调用其他服务                  │   │
│  └─────────────────────────────────┘   │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  ✅ 依赖注入层 - 可以修改                 │
│  ┌─────────────────────────────────┐   │
│  │ internal/svc/service_context.go │   │
│  │ 职责: 初始化和配置               │   │
│  │ - 数据库/Redis连接              │   │
│  │ - RPC客户端                    │   │
│  │ - 自定义服务                    │   │
│  │ - 监控组件                      │   │
│  │ - 拦截器注册                    │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

### 8.3 错误示例 vs 正确示例

#### ❌ 错误：修改自动生成文件

```go
// ❌ 在 internal/server/io_server.go 中添加验证
func (s *IoServer) CreateDataTarget(ctx context.Context, in *io.DataTargetInfo) (*io.BaseIDResp, error) {
    // 添加自定义验证 - 下次make gen-rpc会丢失！
    if in.TargetName == nil {
        return nil, errors.New("name required")
    }
    
    l := data_target.NewCreateDataTargetLogic(ctx, s.svcCtx)
    return l.CreateDataTarget(in)
}
```

#### ✅ 正确：在Logic层实现

```go
// ✅ 在 internal/logic/data_target/create_data_target_logic.go
func (l *CreateDataTargetLogic) CreateDataTarget(in *io.DataTargetInfo) (*io.BaseIDResp, error) {
    // 在logic层添加验证 - 不会被覆盖
    if in.TargetName == nil || *in.TargetName == "" {
        return nil, fmt.Errorf("target name is required")
    }
    
    // 自定义业务逻辑
    if err := l.validateTargetType(in.TargetType); err != nil {
        return nil, err
    }
    
    // 数据库操作
    result, err := l.svcCtx.DB.DataTarget.Create()...
}
```

#### ✅ 正确：在ServiceContext扩展

```go
// ✅ 在 internal/svc/service_context.go 添加自定义服务
type ServiceContext struct {
    Config           config.Config
    DB               *ent.Client
    Redis            redis.UniversalClient
    
    // 添加自定义服务 - 完全安全
    ValidationService *ValidationService
    CustomCache       *CustomCacheService
}

func NewServiceContext(c config.Config) *ServiceContext {
    // 初始化自定义服务
    validator := NewValidationService()
    cache := NewCustomCacheService(rds)
    
    return &ServiceContext{
        ValidationService: validator,
        CustomCache:       cache,
    }
}
```

### 8.4 Make命令安全检查清单

**执行 `make gen-rpc` 或 `make gen-api` 前**:

- [ ] 已提交当前所有更改到git
- [ ] 理解该命令会覆盖哪些文件
- [ ] 确认没有在自动生成文件中添加自定义代码
- [ ] 确认Makefile中`--overwrite`参数设置正确

**执行make命令后**:

- [ ] 运行 `git diff` 检查所有变更
- [ ] 确认只有预期的生成文件被修改
- [ ] 确认 `service_context.go` 未被修改
- [ ] 确认 `logic` 文件未被意外覆盖
- [ ] 运行 `go build -v .` 确保编译通过
- [ ] 运行 `make test` 确保测试通过

### 8.5 Makefile安全配置

**推荐修改**: 将 `--overwrite=true` 改为 `--overwrite=false`

```makefile
# RPC Makefile 修改
gen-rpc-ent-logic:
	goctls rpc ent --schema=./ent/schema \
	  --style=$(PROJECT_STYLE) \
	  --multiple=false \
	  --service_name=$(SERVICE) \
	  --output=./ \
	  --model=$(model) \
	  --group=$(group) \
	  --proto_out=./desc/$(shell echo $(model) | tr A-Z a-z).proto \
	  --i18n=$(PROJECT_I18N) \
	  --overwrite=false  # ✅ 改为false，保护已有logic
```

**效果**:
- ✅ proto文件仍然会更新
- ✅ 已存在的logic文件不会被覆盖
- ⚠️ 新增方法需要手动添加到logic

### 8.6 常见问题

**Q1: 为什么不能把io_server.go的代码移到service_context?**

**A**: 三个原因：
1. **职责不同** - io_server负责路由分发，service_context负责依赖注入
2. **生命周期不同** - io_server每次proto变更都重新生成
3. **框架要求** - go-zero要求server层独立，违反会导致框架无法正常工作

**Q2: 我需要在所有RPC调用前添加统一逻辑怎么办？**

**A**: 使用gRPC拦截器，在service_context中注册：

```go
// 在 internal/svc/service_context.go
import "google.golang.org/grpc"

func NewServiceContext(c config.Config) *ServiceContext {
    // 注册统一拦截器
    opts := []grpc.ServerOption{
        grpc.UnaryInterceptor(MyUnaryInterceptor),
    }
    // ...
}
```

**Q3: 生成文件被覆盖了，修改丢失了怎么办？**

**A**: 
1. 使用 `git checkout <文件>` 恢复
2. 检查git历史找回修改内容
3. 将修改移到正确的位置（logic层或service_context）
4. 更新团队，避免再次犯错

---

## 9. 模型生成命令

### 9.1 新服务完整生成

如果服务是新服务，已定义好所有模型：

```bash
# 生成所有模型的proto和logic
make gen-rpc-ent-logic model=all group=all
```

### 9.2 新增单个模型

如果新增了模型：

```bash
# 示例：新增Student模型
make gen-rpc-ent-logic model=Student group=student

# 注意：
# - model是schema中的结构体名（首字母大写）
# - group是logic包名（小写）
```

---

## 参考文档

### 核心文档
- **自动生成文件清单**：`unified-io/docs/AUTO_GENERATED_FILES_LIST.md` ⭐
- **Make命令安全性分析**：`unified-io/docs/MAKE_COMMAND_SAFETY_ANALYSIS.md` ⭐
- **编译测试报告**：`unified-io/docs/COMPILATION_TEST_REPORT.md`
- **代码完整性报告**：`unified-io/docs/CODE_COMPLETENESS_REPORT.md`

### 架构文档
- 多租户集成详细指南：`多租户集成指南.md`
- 数据权限集成详细指南：`数据权限集成指南.md`
- 统一中间件框架文档：`common/middleware/integration/`
- 公共库使用说明：`github.com/coder-lulu/newbee-common`

---

**最后更新**: 2025-10-13 (Phase 3: 数据权限架构统一)
**重要性**: ⭐⭐⭐⭐⭐ 所有开发者必读
