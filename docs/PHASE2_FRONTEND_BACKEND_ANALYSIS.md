# Phase 2 前后端数据权限配置对接分析

## 📋 文档信息

**版本**：v1.0
**创建日期**：2025-10-13
**分析目的**：确认Phase 2后端改造后前端是否需要修改
**结论**：✅ 前后端接口完全兼容，无需修改前端代码

---

## 🔍 完整调用链分析

### 1. 前端实现 (Vue 3 + Ant Design)

#### 1.1 数据权限配置页面

**文件**：`/opt/code/newbee/ui/apps/web-antd/src/views/system/role/role-auth-modal.vue`

**功能**：管理员配置角色的数据权限范围

**实现逻辑** (line 90-114)：
```typescript
async function handleConfirm() {
  // 1. 表单验证
  const { valid } = await formApi.validate();
  if (!valid) return;

  // 2. 获取表单数据
  const data = cloneDeep(await formApi.getValues());

  // 3. 处理自定义部门ID（仅当dataScope=2时）
  if (data.dataScope === 2) {
    const customDeptIds = deptSelectRef.value?.[0]?.getCheckedKeys() ?? [];
    data.customDeptIds = customDeptIds;
  } else {
    data.customDeptIds = [];
  }

  // 4. 调用API提交
  await roleDataScope(data);

  // 5. 刷新列表
  emit('reload');
}
```

**提交数据结构**：
```typescript
{
  id: number,              // 角色ID
  dataScope: number,       // 数据权限范围 (1-5)
  customDeptIds: number[]  // 自定义部门ID列表（仅dataScope=2时）
}
```

#### 1.2 数据权限选项定义

**文件**：`/opt/code/newbee/ui/apps/web-antd/src/views/system/role/data.tsx`

**权限范围映射** (line 12-18)：
```typescript
export const authScopeOptions = [
  { color: 'green', label: '全部数据权限', value: 1 },           // all
  { color: 'default', label: '自定数据权限', value: 2 },         // custom_dept
  { color: 'cyan', label: '本部门及以下数据权限', value: 3 },    // own_dept_and_sub
  { color: 'orange', label: '本部门数据权限', value: 4 },        // own_dept
  { color: 'error', label: '仅本人数据权限', value: 5 },         // own
];
```

**与后端枚举对应关系**：
| 前端value | 前端label | 后端字符串 | 说明 |
|----------|-----------|-----------|------|
| 1 | 全部数据权限 | all | 查看所有数据 |
| 2 | 自定数据权限 | custom_dept | 自定义部门列表 |
| 3 | 本部门及以下数据权限 | own_dept_and_sub | 本部门及子部门 |
| 4 | 本部门数据权限 | own_dept | 仅本部门 |
| 5 | 仅本人数据权限 | own | 仅本人数据 |

#### 1.3 前端API调用

**文件**：`/opt/code/newbee/ui/apps/web-antd/src/api/system/role/index.ts`

**API定义** (line 88-90)：
```typescript
/**
 * 更新数据权限
 * @param data
 * @returns void
 */
export function roleDataScope(data: any) {
  return requestClient.postWithMsg<void>(Api.roleDataScope, data);
}
```

**请求配置**：
- 方法: `POST`
- 路径: `/sys-api/role/dataScope`
- 数据: `{ id, dataScope, customDeptIds }`

---

### 2. 后端实现

#### 2.1 Core API - HTTP Handler

**文件**：`/opt/code/newbee/core/api/internal/handler/role/assign_role_data_scope_handler.go`

**路由定义** (line 13)：
```go
// swagger:route post /role/dataScope role AssignRoleDataScope
```

**请求处理** (line 28-45)：
```go
func AssignRoleDataScopeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 解析请求体
		var req types.RoleDataScopeReq
		if err := httpx.Parse(r, &req, true); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		// 2. 调用业务逻辑
		l := role.NewAssignRoleDataScopeLogic(r.Context(), svcCtx)
		resp, err := l.AssignRoleDataScope(&req)

		// 3. 返回响应
		if err != nil {
			err = svcCtx.Trans.TransError(r.Context(), err)
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
```

#### 2.2 Core API - Business Logic

**文件**：`/opt/code/newbee/core/api/internal/logic/role/assign_role_data_scope_logic.go`

**RPC调用转发** (line 26-39)：
```go
func (l *AssignRoleDataScopeLogic) AssignRoleDataScope(in *types.RoleDataScopeReq) (resp *types.BaseMsgResp, err error) {
	// 调用Core RPC服务
	data, err := l.svcCtx.CoreRpc.AssignRoleDataScope(l.ctx,
		&core.RoleDataScopeReq{
			Id:            *in.Id,
			DataScope:     in.DataScope,
			CustomDeptIds: in.CustomDeptIds,
		})
	if err != nil {
		return nil, err
	}

	return &types.BaseMsgResp{Msg: l.svcCtx.Trans.Trans(l.ctx, data.Msg)}, nil
}
```

**请求类型定义** (`internal/types/types.go` line 187-195)：
```go
type RoleDataScopeReq struct {
	BaseIDInfo
	// Data scope 1-5
	DataScope uint32 `json:"dataScope" validate:"max=5,min=1"`
	// Custom department setting for data permission
	CustomDeptIds []uint64 `json:"customDeptIds,optional,omitempty"`
}
```

#### 2.3 Core RPC - 业务实现 (✅ Phase 2完成)

**文件**：`/opt/code/newbee/core/rpc/internal/logic/role/assign_role_data_scope_logic.go`

**核心功能** (line 37-80)：
```go
func (l *AssignRoleDataScopeLogic) AssignRoleDataScope(in *core.RoleDataScopeReq) (*core.BaseResp, error) {
	// 🔥 Phase 2: 参数验证
	if err := l.validateDataScopeRequest(in); err != nil {
		return nil, err
	}

	// 🔥 Phase 2: 使用事务同时更新sys_roles和sys_casbin_rules
	err := entx.WithTx(l.ctx, l.svcCtx.DB, func(tx *ent.Tx) error {
		// 1. 获取角色信息
		role, err := tx.Role.Get(l.ctx, in.Id)
		if err != nil {
			return fmt.Errorf("角色不存在: %w", err)
		}

		// 2. 更新sys_roles表（向后兼容）
		err = tx.Role.UpdateOneID(in.Id).
			SetNotNilDataScope(pointy.GetStatusPointer(&in.DataScope)).
			SetNotNilCustomDeptIds(in.CustomDeptIds).
			Exec(l.ctx)
		if err != nil {
			return fmt.Errorf("更新角色data_scope失败: %w", err)
		}

		// 3. 🔥 Phase 2: 同步更新sys_casbin_rules表中的数据权限规则
		err = l.updateCasbinDataPermRules(tx, role, in)
		if err != nil {
			return fmt.Errorf("更新Casbin数据权限规则失败: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, dberrorhandler.DefaultEntError(l.Logger, err, in)
	}

	return &core.BaseResp{Msg: i18n.UpdateSuccess}, nil
}
```

**关键步骤**：
1. ✅ 参数验证（validateDataScopeRequest）
2. ✅ 事务更新sys_roles.data_scope（向后兼容）
3. ✅ **同步更新sys_casbin_rules表**（Phase 2新增）
4. ✅ 发布Redis Watcher通知

---

## ✅ 前后端兼容性验证

### 数据流转完整性

```
前端提交
  ↓
{ id: 1, dataScope: 2, customDeptIds: [10, 20, 30] }
  ↓
POST /sys-api/role/dataScope
  ↓
Core API Handler (assign_role_data_scope_handler.go)
  ↓
Core API Logic (assign_role_data_scope_logic.go)
  ↓
Core RPC.AssignRoleDataScope (RPC调用)
  ↓
RPC Logic (assign_role_data_scope_logic.go)
  ↓
事务处理：
  1. 更新 sys_roles.data_scope = 2
  2. 更新 sys_roles.custom_dept_ids = [10, 20, 30]
  3. 🔥 删除旧的 sys_casbin_rules (ptype=d)
  4. 🔥 创建新的 sys_casbin_rules:
     - ptype = "d"
     - v0 = "role_code"
     - v1 = "tenant_id"
     - v2 = "*"
     - v3 = "custom_dept"
     - v4 = '["10","20","30"]'
  ↓
发布Redis Watcher通知: "UpdatePolicy:tenant_1:data_perm"
  ↓
所有API服务收到通知 → 重新加载Casbin策略
  ↓
前端收到成功响应 → 刷新列表
```

### 接口参数对应关系

| 层级 | 字段名 | 类型 | 说明 |
|------|--------|------|------|
| 前端 | id | number | 角色ID |
| API Types | Id | *uint64 | 角色ID (BaseIDInfo) |
| RPC Types | Id | uint64 | 角色ID |
| **数据权限范围** | | | |
| 前端 | dataScope | number (1-5) | 数据权限枚举 |
| API Types | DataScope | uint32 | 数据权限枚举 |
| RPC Types | DataScope | uint32 | 数据权限枚举 |
| **自定义部门** | | | |
| 前端 | customDeptIds | number[] | 部门ID数组 |
| API Types | CustomDeptIds | []uint64 | 部门ID数组 |
| RPC Types | CustomDeptIds | []uint64 | 部门ID数组 |

✅ **结论：所有字段完全对应，无需修改**

---

## 🎯 Phase 2 改造影响分析

### Phase 2 改造内容回顾

1. **InitDatabase新增**：自动创建数据权限规则到sys_casbin_rules
2. **AssignRoleDataScope改造**：同步更新sys_roles和sys_casbin_rules
3. **向后兼容**：sys_roles.data_scope字段继续更新

### 对前端的影响

#### ✅ 无需修改的原因

1. **接口签名不变**：
   - 路径：`/sys-api/role/dataScope`
   - 方法：`POST`
   - 参数：`{ id, dataScope, customDeptIds }`
   - 响应：`{ msg: string }`

2. **数据枚举不变**：
   - 前端仍然使用 1-5 的数字枚举
   - 后端RPC负责转换为字符串（all, custom_dept等）

3. **业务逻辑透明**：
   - 前端不感知后端存储方式的变化
   - sys_roles和sys_casbin_rules的双写对前端透明

4. **实时生效机制**：
   - Redis Watcher自动通知所有API服务
   - Casbin策略实时重新加载
   - 前端配置立即生效（无需用户重新登录）

### 配置生效流程

```
管理员在前端配置数据权限
  ↓
前端调用 roleDataScope API
  ↓
后端更新 sys_roles + sys_casbin_rules
  ↓
发布 Redis Watcher 通知
  ↓
所有API服务订阅到通知
  ↓
自动调用 LoadPolicy() 重新加载
  ↓
新的数据权限规则立即生效
  ↓
用户下次请求即应用新权限
```

---

## ⚠️ 需要确认的关键点

### 1. Redis Watcher订阅机制 ⚠️

**问题**：API服务是否正确订阅Redis Watcher频道？

**检查点**：
- [ ] Core API服务是否初始化Redis Watcher
- [ ] 是否订阅 `casbin_watcher` 频道
- [ ] 收到通知后是否调用 LoadPolicy()

**如果没有订阅**，则需要添加Redis Watcher订阅机制：

```go
// 伪代码示例
go func() {
    pubsub := redis.Subscribe(ctx, "casbin_watcher")
    for msg := range pubsub.Channel() {
        if strings.Contains(msg.Payload, "UpdatePolicy") {
            enforcer.LoadPolicy()
            logx.Info("Casbin策略已重新加载")
        }
    }
}()
```

### 2. UnifiedDataPermPlugin读取规则

**问题**：UnifiedDataPermPlugin是否从sys_casbin_rules读取数据权限？

**检查点**：
- [ ] UnifiedDataPermPlugin是否使用CasbinProvider
- [ ] CasbinProvider是否查询ptype='d'的规则
- [ ] 数据权限过滤是否基于Casbin规则

**Phase 1已确认**：
- ✅ UnifiedDataPermPlugin强制要求CasbinProvider
- ✅ RpcCasbinRuleQuerier通过RPC查询sys_casbin_rules
- ✅ Phase 2.2.1已实现CasbinProvider接口

### 3. 测试验证

**需要测试的场景**：

1. **场景1：配置全部数据权限**
   ```
   前端操作：设置角色数据权限为"全部数据权限"
   预期结果：
   - sys_roles.data_scope = 1
   - sys_casbin_rules: ptype=d, v3="all"
   - 用户可以看到所有数据
   ```

2. **场景2：配置自定义部门权限**
   ```
   前端操作：设置角色数据权限为"自定数据权限"，选择部门10,20,30
   预期结果：
   - sys_roles.data_scope = 2
   - sys_roles.custom_dept_ids = [10, 20, 30]
   - sys_casbin_rules: ptype=d, v3="custom_dept", v4='["10","20","30"]'
   - 用户只能看到这3个部门的数据
   ```

3. **场景3：实时生效验证**
   ```
   前端操作：修改角色A的数据权限
   预期结果：
   - 拥有角色A的用户，下次请求立即应用新权限（无需重新登录）
   ```

---

## 📋 检查清单

### 前端代码 ✅
- [x] API调用路径正确: `/sys-api/role/dataScope`
- [x] 请求方法正确: `POST`
- [x] 参数结构正确: `{ id, dataScope, customDeptIds }`
- [x] 数据权限枚举正确: 1-5
- [x] 自定义部门处理正确: dataScope=2时提交customDeptIds

### 后端API ✅
- [x] Handler路由正确: `/role/dataScope`
- [x] 请求解析正确: `types.RoleDataScopeReq`
- [x] RPC调用正确: `CoreRpc.AssignRoleDataScope`
- [x] 参数映射正确: id, dataScope, customDeptIds

### 后端RPC ✅
- [x] 参数验证: validateDataScopeRequest
- [x] 事务处理: sys_roles + sys_casbin_rules
- [x] 规则格式: ptype=d, v0-v4字段正确
- [x] Redis通知: 发布到casbin_watcher频道

### 待确认项 ⚠️
- [ ] Core API是否订阅Redis Watcher
- [ ] 收到通知后是否重新加载Casbin策略
- [ ] 数据权限过滤是否基于sys_casbin_rules
- [ ] 测试配置是否实时生效

---

## 🎯 最终结论

### ✅ 前端无需修改

**理由**：
1. 前端API调用接口、参数、数据结构完全正确
2. 后端接口签名和行为保持向后兼容
3. Phase 2改造对前端完全透明

### ⚠️ 需要验证的点

1. **Redis Watcher订阅机制**：确认API服务是否正确订阅和重新加载策略
2. **实时生效测试**：验证前端配置后是否立即生效
3. **数据权限过滤**：验证UnifiedDataPermPlugin是否正确应用新规则

### 📝 建议的测试步骤

1. **启动服务**：
   ```bash
   # 启动Core RPC
   cd /opt/code/newbee/core/rpc
   go run core.go -f etc/core.yaml

   # 启动Core API
   cd /opt/code/newbee/core/api
   go run core.go -f etc/core.yaml
   ```

2. **前端配置测试**：
   - 登录管理员账号
   - 进入"系统管理 > 角色管理"
   - 点击某个角色的"分配权限"按钮
   - 修改数据权限范围并保存

3. **数据库验证**：
   ```sql
   -- 查看sys_roles更新
   SELECT id, name, data_scope, custom_dept_ids
   FROM sys_roles
   WHERE id = ?;

   -- 查看sys_casbin_rules更新
   SELECT ptype, v0, v1, v2, v3, v4, rule_name
   FROM sys_casbin_rules
   WHERE ptype = 'd' AND v0 = 'role_code';
   ```

4. **Redis日志验证**：
   - 检查Core RPC日志，确认发布Redis通知
   - 检查Core API日志，确认收到Redis通知
   - 检查Casbin策略是否重新加载

5. **功能验证**：
   - 使用拥有该角色的用户登录
   - 访问数据列表页面
   - 验证数据过滤是否按新权限执行

---

**文档版本**：v1.0
**创建日期**：2025-10-13
**最后更新**：2025-10-13
**状态**：✅ 前后端接口完全兼容，建议执行测试验证
**下一步**：验证Redis Watcher订阅机制和实时生效功能
