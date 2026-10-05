# 新蜂资产管理平台演示数据

为本机已启用的资产管理、输入输出、运维中心和系统管理填充关联演示数据。默认每个业务列表新增 30 条；每个资产模型新增 30 条实例，每个演示字典和停用定时任务各有 30 条明细或历史日志。菜单、接口目录等已满足分页的系统数据沿用现有记录。

演示名称使用 `演示-`，业务标识使用 `newbee-demo`、`demo.` 或 `demo-`。重复导入按稳定业务标识查询，只创建缺少的记录；不删除原数据。资产实例额外标记 `metadata.demo=newbee-demo-v1`。

## 本机导入

前提是本机 Core、CMDB、IO、Ops、Job RPC 与前端已启动，默认租户中存在管理员。工具读取该管理员真实角色和部门，以租户 1 上下文调用服务。数据库复用 `127.0.0.1:3306/newbee`。

在项目根目录执行 PowerShell：

```powershell
$seedSources = @(Get-ChildItem scripts/demo-data -Filter '*.go' | Where-Object Name -NotLike '*_test.go' | ForEach-Object FullName)

# 默认只查询和规划，不写数据库
go run $seedSources --modules core,cmdb,io,io-extra,ops,job,job-history --out logs/demo-data/plan

# 如恢复的备份缺少当前代码所需表结构，先执行限定范围的迁移
go run $seedSources --apply --modules io-audit-schema,ops-schema --out logs/demo-data/schema

# 填充；再次运行同一命令核验幂等性
go run $seedSources --apply --modules core,cmdb,io,io-extra,ops,job,job-history --out logs/demo-data/final

# 通过前端同源代理核验真实列表接口
go run $seedSources --modules verify,isolation --out logs/demo-data/verification
```

项目使用 `go.work`，根目录不是 Go module，因此采用明确的源文件列表。`--count` 可选 20–30，默认 30。各阶段的计数与完成状态写入指定目录的 `seed-report.json`；即使中途出错也保留已完成部分的报告。

少数没有创建 RPC 的审计实体使用公开 Ent client，注册统一 hooks，保留可信租户和部门上下文。数据库及短效验证令牌的私有配置来自 `logs/local-run/private-runtime.json`；文件、备份、日志、令牌和编译产物不提交仓库。导入前应保存数据库备份；本次备份为 `logs/demo-data/before-demo.sql`。

## 数据范围与行为

| 模块 | 演示内容 |
| --- | --- |
| 系统管理 | 部门、岗位、停用角色与用户、字典及明细、参数、停用租户和 OAuth 提供商、失效 token、标记为模拟的审计记录 |
| 资产管理 | 属性、模型分组、模型、各模型实例、模型和实例关系、仅关联停用演示角色的过期权限 |
| 输入输出 | 发现模板、停用 Provider schema、数据目标、发现池、导入导出历史、字段映射、任务和映射日志、Worker 指标、停用计划、配置和审计、CI 变更及生命周期终态 |
| 运维中心 | 离线节点和代理、停用分组、脚本分类及不可调度脚本、访问配置、已关闭会话、成功终态任务、代理历史指标 |
| 定时任务 | 停用任务及每条任务 30 条关联历史日志 |

演示工具不执行任务、不运行脚本、不进行设备发现，也不创建有效访问凭据。演示账号、OAuth 提供商、数据目标和计划停用；过期权限不会扩大现有用户权限。真实本机代理 `newbee-local-proxy` 保留自己的在线状态和心跳。运维任务查询页可使用 `demo-task-01` 至 `demo-task-30` 查看历史示例。

IO 的 WorkerMetrics 与 Job 的 Task/TaskLog 在现有 schema 中属于全局表，没有 TenantMixin；仅对这些确切实体声明服务或工具级配置，统一 hooks 仍注册，含租户字段的业务实体继续隔离。不能把这几类全局指标和任务日志当作具备租户隔离的数据使用。

监控、详情、选择预览和统计功能复用以上真实记录；它们不是独立的分页业务表。模型分组使用 6 类资源组织模型。列表页默认 10 或 20 行时，30 条记录可以覆盖完整首屏并翻页。
