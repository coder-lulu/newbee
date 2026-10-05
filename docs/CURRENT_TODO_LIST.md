# DataPerm统一化项目 - 当前待办任务清单

## 📋 文档信息

**版本**：v1.0
**更新日期**：2025-10-13
**当前阶段**：Phase 2 - 核心开发已完成
**下一步**：测试验证和文档更新

---

## ✅ 已完成工作回顾

### Phase 1: 配置统一化 (100% 完成)
- ✅ DataPerm中间件多版本清理
- ✅ 强制使用UnifiedDataPermPlugin
- ✅ 所有服务配置更新
- ✅ 统一权限配置架构设计

### Phase 2: 核心开发 (85% 完成)
- ✅ **Task 2.1**: InitDatabase添加insertDataPermRules方法
- ✅ **Task 2.2**: AssignRoleDataScope添加updateCasbinDataPermRules方法
- ✅ **Task 2.2.1**: RpcCasbinRuleQuerier实现CasbinProvider接口
- ✅ **前后端对接分析**: 确认无需修改前端代码
- ✅ **代码编译验证**: Core RPC和Core API编译通过

---

## 🎯 当前待办任务

### 优先级1：立即执行 🔴

#### Task 2.3: 功能验证测试 (预计2小时)

**目标**：验证Phase 2核心功能正常工作

**测试步骤**：

1. **启动服务** (15分钟)
   ```bash
   # 1. 启动Core RPC服务
   cd /opt/code/newbee/core/rpc
   go run core.go -f etc/core.yaml

   # 2. 启动Core API服务 (新窗口)
   cd /opt/code/newbee/core/api
   go run core.go -f etc/core.yaml

   # 3. 启动前端服务 (新窗口)
   cd /opt/code/newbee/ui/apps/web-antd
   pnpm dev
   ```

2. **数据库初始化验证** (10分钟)
   - [ ] 重新初始化数据库（如果需要）
   - [ ] 验证sys_casbin_rules表中存在数据权限规则（ptype='d'）
   - [ ] 验证默认角色（superadmin, user）的数据权限规则已创建

   **验证SQL**：
   ```sql
   -- 查询数据权限规则
   SELECT
       ptype,
       v0 as role_code,
       v1 as tenant_id,
       v2 as resource_type,
       v3 as data_scope,
       v4 as custom_depts,
       rule_name,
       description
   FROM sys_casbin_rules
   WHERE ptype = 'd' AND tenant_id = 1
   ORDER BY v0;
   ```

   **预期结果**：
   ```
   ptype | role_code  | tenant_id | resource_type | data_scope        | custom_depts | rule_name
   ------|------------|-----------|---------------|-------------------|--------------|-------------
   d     | superadmin | 1         | *             | all               |              | 超级管理员数据权限
   d     | user       | 1         | *             | own_dept_and_sub  |              | 普通用户数据权限
   ```

3. **前端配置数据权限测试** (30分钟)

   **测试场景1：配置全部数据权限**
   - [ ] 登录管理员账号
   - [ ] 进入"系统管理 > 角色管理"
   - [ ] 选择一个测试角色，点击"分配权限"
   - [ ] 设置数据权限为"全部数据权限"
   - [ ] 保存并验证数据库更新

   **验证SQL**：
   ```sql
   -- 验证sys_roles表
   SELECT id, name, code, data_scope, custom_dept_ids
   FROM sys_roles
   WHERE code = 'test_role';

   -- 验证sys_casbin_rules表
   SELECT ptype, v0, v1, v2, v3, v4, rule_name
   FROM sys_casbin_rules
   WHERE ptype = 'd' AND v0 = 'test_role' AND tenant_id = 1;
   ```

   **预期结果**：
   - sys_roles.data_scope = 1
   - sys_casbin_rules: v3 = "all"

   **测试场景2：配置自定义部门权限**
   - [ ] 设置数据权限为"自定数据权限"
   - [ ] 选择3个部门（例如：部门A、B、C）
   - [ ] 保存并验证数据库更新

   **预期结果**：
   - sys_roles.data_scope = 2
   - sys_roles.custom_dept_ids = [部门A ID, 部门B ID, 部门C ID]
   - sys_casbin_rules: v3 = "custom_dept", v4 = JSON数组

   **测试场景3：配置本部门及子部门权限**
   - [ ] 设置数据权限为"本部门及以下数据权限"
   - [ ] 保存并验证数据库更新

   **预期结果**：
   - sys_roles.data_scope = 3
   - sys_casbin_rules: v3 = "own_dept_and_sub"

4. **Redis Watcher同步验证** (15分钟)
   - [ ] 修改角色数据权限后，查看Core RPC日志
   - [ ] 确认发布Redis通知：`UpdatePolicy:tenant_1:data_perm`
   - [ ] 查看Core API日志
   - [ ] 确认收到Redis通知并重新加载策略

   **查看日志**：
   ```bash
   # Core RPC日志中应该看到：
   # ✅ Published data permission policy update notification to Redis

   # Core API日志中应该看到（如果有监听）：
   # Casbin策略已重新加载
   ```

5. **数据权限过滤功能验证** (30分钟)

   **准备工作**：
   - [ ] 创建测试用户A，分配角色为"全部数据权限"
   - [ ] 创建测试用户B，分配角色为"本部门数据权限"
   - [ ] 创建测试用户C，分配角色为"仅本人数据权限"

   **验证步骤**：
   - [ ] 使用用户A登录，访问用户列表/部门列表
   - [ ] 确认可以看到所有数据
   - [ ] 使用用户B登录，访问用户列表/部门列表
   - [ ] 确认只能看到本部门及子部门的数据
   - [ ] 使用用户C登录，访问用户列表/部门列表
   - [ ] 确认只能看到自己的数据

6. **多租户隔离验证** (15分钟)
   - [ ] 如果系统有多个租户，验证租户1的数据权限规则
   - [ ] 切换到租户2，验证数据权限规则独立
   - [ ] 确认租户间数据权限不互相影响

**完成标准**：
- ✅ 所有测试场景通过
- ✅ 数据库数据正确
- ✅ Redis通知正常发布和接收
- ✅ 数据权限过滤功能正常

---

### 优先级2：短期完成 🟡

#### Task 2.4: 编写单元测试 (预计4小时)

**目标**：为Phase 2新增代码编写单元测试，覆盖率≥80%

**测试文件清单**：

1. **init_database_logic_test.go**
   ```go
   // 文件路径：/opt/code/newbee/core/rpc/internal/logic/base/init_database_logic_test.go

   // 测试用例：
   - [ ] TestInsertDataPermRules_Success
   - [ ] TestInsertDataPermRules_EmptyRules
   - [ ] TestInsertDataPermRules_DuplicateRules
   - [ ] TestInsertDataPermRules_RedisPublishFailure
   ```

2. **assign_role_data_scope_logic_test.go**
   ```go
   // 文件路径：/opt/code/newbee/core/rpc/internal/logic/role/assign_role_data_scope_logic_test.go

   // 测试用例：
   - [ ] TestAssignRoleDataScope_All
   - [ ] TestAssignRoleDataScope_CustomDept
   - [ ] TestAssignRoleDataScope_OwnDeptAndSub
   - [ ] TestAssignRoleDataScope_OwnDept
   - [ ] TestAssignRoleDataScope_Own
   - [ ] TestValidateDataScopeRequest_ValidScopes
   - [ ] TestValidateDataScopeRequest_InvalidScope
   - [ ] TestValidateDataScopeRequest_CustomDeptMissingIds
   - [ ] TestUpdateCasbinDataPermRules_Success
   - [ ] TestUpdateCasbinDataPermRules_DeleteOldRules
   - [ ] TestUpdateCasbinDataPermRules_Transaction
   - [ ] TestDataScopeEnumToString
   ```

3. **rpc_querier_test.go**
   ```go
   // 文件路径：/opt/code/newbee/core/api/internal/casbin/rpc_querier_test.go

   // 测试用例：
   - [ ] TestCheckPermissionWithRoles_Allowed
   - [ ] TestCheckPermissionWithRoles_Denied
   - [ ] TestCheckPermissionWithRoles_RPCError
   - [ ] TestGetUserRolesWithCache_Success
   - [ ] TestGetUserRolesWithCache_EmptyRoles
   - [ ] TestGetUserRolesWithCache_UserNotFound
   ```

**测试覆盖率目标**：
- init_database_logic.go: insertDataPermRules方法 ≥ 85%
- assign_role_data_scope_logic.go: 所有新增方法 ≥ 90%
- rpc_querier.go: CasbinProvider接口方法 ≥ 85%

**完成标准**：
- ✅ 所有测试用例编写完成
- ✅ 所有测试用例通过
- ✅ 代码覆盖率达标

---

#### Task 2.5: 性能基准测试 (预计2小时)

**目标**：验证Phase 2改造后性能无明显退化

**测试项目**：

1. **Casbin规则加载性能**
   - [ ] 测试LoadPolicy()执行时间
   - [ ] 基准：< 500ms（10000条规则）
   - [ ] 对比Phase 1和Phase 2的加载性能

2. **数据权限检查性能**
   - [ ] 测试单次权限检查耗时
   - [ ] 基准：< 10ms（含缓存）
   - [ ] 测试1000次连续权限检查的平均耗时

3. **AssignRoleDataScope性能**
   - [ ] 测试单次数据权限分配耗时
   - [ ] 基准：< 200ms（含事务和Redis通知）
   - [ ] 验证事务回滚性能

4. **数据库查询性能**
   - [ ] 测试sys_casbin_rules表查询性能
   - [ ] 验证索引有效性（ptype, tenant_id, v0）
   - [ ] 大数据量场景测试（10000+规则）

**性能测试工具**：
```bash
# 使用Go benchmark
go test -bench=. -benchmem ./internal/logic/role/
go test -bench=. -benchmem ./internal/logic/base/
```

**完成标准**：
- ✅ 所有性能指标满足基准要求
- ✅ 无明显性能退化
- ✅ 生成性能测试报告

---

#### Task 2.6: 文档更新 (预计3小时)

**目标**：更新相关技术文档，反映Phase 2改造内容

**文档清单**：

1. **更新数据权限集成指南** (1小时)
   - [ ] 文件：`/opt/code/newbee/docs/data_permission_integration_guide.md`
   - [ ] 更新内容：
     - 新增sys_casbin_rules表结构说明
     - 更新数据权限配置流程
     - 添加InitDatabase数据权限规则创建说明
     - 添加AssignRoleDataScope使用示例

2. **更新API文档** (30分钟)
   - [ ] 文件：`/opt/code/newbee/docs/core-rpc-api.md`
   - [ ] 更新内容：
     - AssignRoleDataScope接口说明
     - 参数验证规则
     - 返回值说明
     - 使用示例

3. **更新数据库设计文档** (30分钟)
   - [ ] 文件：`/opt/code/newbee/docs/database-design.md`
   - [ ] 更新内容：
     - sys_casbin_rules表数据权限规则说明
     - 规则格式说明（ptype=d）
     - v0-v4字段含义
     - 索引设计

4. **创建Phase 2发布说明** (1小时)
   - [ ] 文件：`/opt/code/newbee/docs/PHASE2_RELEASE_NOTES.md`
   - [ ] 内容：
     - 版本号：v2.0
     - 发布日期
     - 新功能说明
     - API变更说明（无破坏性变更）
     - 升级指南
     - 已知问题
     - 后续计划（Phase 3）

5. **更新DATAPERM_ROADMAP.md** (15分钟)
   - [ ] 更新Phase 2进度为100%
   - [ ] 更新完成日期
   - [ ] 添加Phase 2成果总结

**完成标准**：
- ✅ 所有文档更新完成
- ✅ 文档内容准确无误
- ✅ 示例代码可运行

---

### 优先级3：未来计划 📅

#### Phase 3: 清理阶段 (预计2周)

**前置条件**：
- ✅ Phase 2全部完成
- ✅ 生产环境稳定运行至少1周
- ✅ 所有服务已完成数据库重新初始化
- ✅ 确认sys_roles.data_scope字段不再使用

**待完成任务**：

1. **Task 3.1**: 移除废弃代码
   - [ ] 删除`/opt/code/newbee/common/middleware/dataperm/plugin.go`
   - [ ] 确认无其他服务引用旧版插件
   - [ ] 更新import引用

2. **Task 3.2**: 移除冗余数据库字段
   - [ ] 创建数据库迁移脚本（DROP COLUMN data_scope）
   - [ ] 移除ent schema中的data_scope字段定义
   - [ ] 移除相关查询逻辑
   - [ ] 重新生成ent代码

3. **Task 3.3**: 全面文档更新
   - [ ] 更新CLAUDE.md编码准则
   - [ ] 更新数据权限集成指南
   - [ ] 更新多租户架构文档
   - [ ] 创建v2.1发布说明

4. **Task 3.4**: 代码审查和优化
   - [ ] UnifiedDataPermPlugin性能优化
   - [ ] Casbin规则缓存策略优化
   - [ ] 代码规范性检查
   - [ ] 单元测试覆盖率检查

---

## 📅 时间规划

### 本周计划 (2025-10-13 至 2025-10-20)

| 日期 | 任务 | 预计用时 | 优先级 |
|------|------|---------|--------|
| 10-13 (今天) | Task 2.3: 功能验证测试 | 2小时 | 🔴 高 |
| 10-14 | Task 2.4: 编写单元测试 (Part 1) | 4小时 | 🟡 中 |
| 10-15 | Task 2.4: 编写单元测试 (Part 2) | 4小时 | 🟡 中 |
| 10-16 | Task 2.5: 性能基准测试 | 2小时 | 🟡 中 |
| 10-17 | Task 2.6: 文档更新 | 3小时 | 🟡 中 |
| 10-18 | Phase 2完成验收和总结 | 2小时 | 🟡 中 |

**Phase 2预计完成日期**：2025-10-18

### 下个月计划 (2025-10-25 至 2025-11-05)

| 阶段 | 预计开始 | 预计完成 |
|------|----------|----------|
| Phase 3: 清理阶段 | 2025-10-25 | 2025-11-05 |

---

## ✅ 今日优先任务

### 立即开始：Task 2.3 功能验证测试

1. **第一步：启动所有服务** (15分钟)
   ```bash
   # 终端1：Core RPC
   cd /opt/code/newbee/core/rpc
   go run core.go -f etc/core.yaml

   # 终端2：Core API
   cd /opt/code/newbee/core/api
   go run core.go -f etc/core.yaml

   # 终端3：前端
   cd /opt/code/newbee/ui/apps/web-antd
   pnpm dev
   ```

2. **第二步：数据库验证** (10分钟)
   - 执行验证SQL，确认数据权限规则已创建

3. **第三步：前端测试** (30分钟)
   - 登录管理员账号
   - 测试配置数据权限功能
   - 验证数据库更新

4. **第四步：功能验证** (30分钟)
   - 创建测试用户
   - 分配不同数据权限
   - 验证数据过滤效果

5. **第五步：记录测试结果** (15分钟)
   - 创建测试报告
   - 截图关键步骤
   - 记录发现的问题

**预计总耗时**：2小时

---

## 📊 进度跟踪

### Phase 2 完成度

| 任务 | 状态 | 完成度 |
|------|------|--------|
| 2.1 InitDatabase修改 | ✅ 完成 | 100% |
| 2.2 AssignRoleDataScope修改 | ✅ 完成 | 100% |
| 2.2.1 CasbinProvider接口实现 | ✅ 完成 | 100% |
| 2.3 功能验证测试 | ⏳ 待执行 | 0% |
| 2.4 单元测试 | ⏳ 待执行 | 0% |
| 2.5 性能测试 | ⏳ 待执行 | 0% |
| 2.6 文档更新 | ⏳ 待执行 | 0% |
| **整体进度** | **🚧 进行中** | **58%** |

---

## 🎯 本周目标

**核心目标**：完成Phase 2所有任务，达到可发布状态

**关键交付物**：
1. ✅ 功能验证测试报告
2. ✅ 单元测试代码（覆盖率≥80%）
3. ✅ 性能测试报告
4. ✅ 更新后的技术文档
5. ✅ Phase 2发布说明

**成功标准**：
- 所有功能测试通过
- 所有单元测试通过
- 性能指标满足要求
- 文档完整准确
- 可以安全发布到生产环境

---

## 📚 相关文档

- **项目路线图**：`/opt/code/newbee/docs/DATAPERM_ROADMAP.md`
- **Phase 2完成总结**：`/opt/code/newbee/docs/PHASE2_COMPLETION_SUMMARY.md`
- **前后端对接分析**：`/opt/code/newbee/docs/PHASE2_FRONTEND_BACKEND_ANALYSIS.md`
- **编码规范**：`/opt/code/newbee/CLAUDE.md`

---

**文档版本**：v1.0
**创建日期**：2025-10-13
**最后更新**：2025-10-13
**负责人**：开发团队
**审核人**：架构组
