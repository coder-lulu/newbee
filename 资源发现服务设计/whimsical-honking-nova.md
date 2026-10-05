# Unified-IO 服务改造方案：增强发现调度能力

## 执行摘要

**决策**：✅ **改造 unified-io 服务，不新增独立的 Discovery-Scheduler 服务**

**核心理由**：
1. ✅ Unified-IO 已经是 Discovery 的主控服务
2. ✅ 95% 的调度组件已存在（Schema、TaskWorker、StateMachine、Queue）
3. ✅ 定时调度只是缺失的最后5%功能
4. ✅ 符合 NewBee "模块化微服务"架构哲学
5. ✅ 避免过度拆分，降低系统复杂度

---

## 一、服务现状分析

### 1.1 Unified-IO 当前定位

**服务性质**：综合性的数据集成平台服务

**核心职责**（已完成）：
```
Unified-IO (数据集成平台)
├── 数据源管理 (DataTarget)
├── 自动发现 (DiscoveryPool, DiscoveryProvider, DiscoveryTemplate)
├── 数据映射 (FieldMapping)
├── 任务管理 (InputTask, OutputTask)
├── 任务日志 (TaskLog, MappingLog)
└── 监控指标 (WorkerMetrics)
```

**服务规模**：
- 代码量：~150+文件，4685行业务代码
- 数据模型：12个核心实体
- RPC方法：64个
- 成熟度：中等规模，架构清晰

### 1.2 已有的执行引擎（可复用）

| 组件 | 代码行数 | 功能状态 |
|------|---------|---------|
| **DiscoveryEngine** | 240行 | ✅ 完整的数据发现执行流程 |
| **TaskWorker** | 501行 | ⚠️ 轮询模式，但未支持定时检查 |
| **StateMachine** | 303行 | ✅ 完整的任务状态机 |
| **AsyncTaskManager** | - | ✅ Redis异步任务存储 |
| **Queue系统** | - | ✅ Outbox Pattern事件分发 |

### 1.3 定时调度功能现状

**Schema支持**（已完成）：
```go
// ✅ InputTask 已有字段
type InputTask struct {
    task_type    string // "manual" | "scheduled" | "triggered"
    scheduled_at *time.Time // 计划执行时间
    // ...
}

// ✅ DiscoveryPool 已有字段
type DiscoveryPool struct {
    schedule string // Cron表达式（字段存在但未使用）
    // ...
}
```

**执行逻辑**（未完成）：
- ❌ TaskWorker 未检查 scheduled_at 字段
- ❌ 无 Cron 表达式解析逻辑
- ❌ 无定时事件触发机制

**完成度评估**：**95%** 组件已存在，仅缺 5% 执行逻辑

---

## 二、不新增服务的理由

### 2.1 与NewBee服务拆分原则的对比

| 新增服务的判断标准 | Discovery-Scheduler | 结论 |
|------------------|---------------------|------|
| 有清晰的业务边界（与现有服务无强依赖） | ❌ 强依赖 unified-io 的配置和数据模型 | **不满足** |
| 代码规模预计5000+行 | ❌ 预计仅增加1000-2000行 | **不满足** |
| 有独立的数据库schema（3个以上核心实体） | ❌ 只有 discovery_tasks 1个新表 | **不满足** |
| 有独立的维护团队 | ❌ 与 unified-io 共享团队 | **不满足** |
| 复用性强（被多个服务使用） | ❌ 只被 unified-io 使用 | **不满足** |

**结论**：0/5 条件满足，**不应新增服务**

### 2.2 NewBee 架构哲学

**核心原则**："有原则的适度拆分"

```
不追求极端的微服务细粒度，
而是在可维护性和灵活性之间找到平衡点：

✓ 业务域为拆分单位
✓ 数据库为隔离手段
✓ Core为中心枢纽
✓ 模块化为组织方式
```

**现有服务规模**：
- Core：16,736行（19个模块）- 大型服务
- CMDB：18,203行（28个模块）- 大型服务
- Unified-IO：4,685行（14个模块）- **中型服务**
- Ops-Center：7,333行（12个模块）- 中型服务

**Unified-IO 的定位**：
- ✅ 是"数据集成平台"，调度是其核心能力
- ✅ 服务规模适中，有扩展空间
- ✅ 保持适度粒度，避免过度拆分

### 2.3 过度拆分的风险

如果新增 Discovery-Scheduler 服务：

| 风险 | 影响 |
|------|------|
| **数据强耦合** | Scheduler 需要频繁访问 unified-io 的 DiscoveryPool、FieldMapping 等配置 |
| **分布式事务** | 需要跨服务事务（Scheduler创建任务 → unified-io执行） |
| **网络开销** | 频繁的 RPC 调用增加延迟 |
| **运维复杂度** | 多一个服务需要部署、监控、故障排查 |
| **代码重复** | 需要在两个服务间复制类型定义、配置逻辑 |

---

## 三、改造方案

### 3.1 短期方案（1-2周，快速可行）

**目标**：补全 scheduled 任务的执行逻辑

#### 修改1：TaskWorker 增加定时检查

**文件**：`/opt/code/newbee/unified-io/rpc/internal/worker/task_worker.go`

**当前代码分析**：
```go
// 当前实现（Line 207-224）- pullLoop方法
func (w *TaskWorker) pullLoop(ctx context.Context) {
    ticker := time.NewTicker(w.config.PullInterval)
    defer ticker.Stop()

    w.processOnce(ctx)  // 立即执行一次

    for {
        select {
        case <-ticker.C:
            w.processOnce(ctx)  // ⚠️ 只处理pending任务，不考虑scheduled_at
        case <-ctx.Done():
            return
        }
    }
}

// processOnce方法（Line 243-298）- 查询pending任务
tasks, err := w.db.InputTask.Query().
    Where(inputtask.TaskStatusEQ("pending")).       // 只看pending状态
    Order(ent.Asc(inputtask.FieldCreatedAt)).
    Limit(pullSize).
    All(systemCtx)
// ❌ 问题：没有检查task_type和scheduled_at字段
// ❌ 问题：manual和scheduled任务混在一起处理
```

**改造方案 - 三步走**：

**Step 1 - 修改 pullLoop 方法**（Line 207，添加scheduled ticker）：
```go
// pullLoop 任务拉取循环
func (w *TaskWorker) pullLoop(ctx context.Context) {
    ticker := time.NewTicker(w.config.PullInterval)
    defer ticker.Stop()

    // ⭐ 新增：定时任务检查ticker（每分钟检查一次）
    scheduledTicker := time.NewTicker(1 * time.Minute)
    defer scheduledTicker.Stop()

    // 立即执行一次
    w.processOnce(ctx)

    for {
        select {
        case <-ticker.C:
            w.processOnce(ctx)  // 处理 manual/triggered 任务

        // ⭐ 新增：定时任务检查分支
        case <-scheduledTicker.C:
            w.processScheduledTasks(ctx)

        case <-ctx.Done():
            logx.Info("Pull loop stopped")
            return
        }
    }
}
```

**Step 2 - 新增 processScheduledTasks 方法**（在 Line 298 后添加）：
```go
// processScheduledTasks 处理到期的定时任务
func (w *TaskWorker) processScheduledTasks(ctx context.Context) {
    // 检查当前并发数（复用现有逻辑）
    w.metrics.mu.RLock()
    currentProcessing := w.metrics.CurrentProcessing
    w.metrics.mu.RUnlock()

    if currentProcessing >= int64(w.config.MaxConcurrent) {
        logx.Debugf("Max concurrent tasks reached (%d), skipping scheduled pull", w.config.MaxConcurrent)
        return
    }

    // 计算可拉取的任务数
    availableSlots := int(int64(w.config.MaxConcurrent) - currentProcessing)
    pullSize := w.config.BatchSize
    if pullSize > availableSlots {
        pullSize = availableSlots
    }

    // 🔥 使用SystemContext查询到期的定时任务（绕过租户隔离，因为Worker是后台进程）
    systemCtx := hooks.NewSystemContext(ctx)

    // ⭐ 查询到期的scheduled任务
    tasks, err := w.db.InputTask.Query().
        Where(
            inputtask.TaskTypeEQ("scheduled"),           // 任务类型为scheduled
            inputtask.TaskStatusEQ("pending"),           // 状态为pending
            inputtask.ScheduledAtNotNil(),               // scheduled_at不为空
            inputtask.ScheduledAtLTE(time.Now()),        // 到期时间 <= 当前时间
        ).
        Order(ent.Asc(inputtask.FieldScheduledAt)).      // 按计划时间排序
        Limit(pullSize).
        All(systemCtx)

    if err != nil {
        logx.Errorw("Failed to query scheduled tasks",
            logx.Field("error", err))
        return
    }

    if len(tasks) == 0 {
        logx.Debug("No scheduled tasks due")
        return
    }

    // 更新指标（复用现有metrics）
    w.metrics.mu.Lock()
    w.metrics.TotalPulled += int64(len(tasks))
    w.metrics.LastPullTime = time.Now()
    w.metrics.mu.Unlock()

    logx.Infow("Pulled scheduled tasks",
        logx.Field("count", len(tasks)),
        logx.Field("current_processing", currentProcessing))

    // ⭐ 分发任务（复用现有的processTask逻辑，无需修改）
    for _, task := range tasks {
        w.incrementProcessing()
        go w.processTask(ctx, task)  // 复用现有的任务处理流程
    }
}
```

**Step 3 - 验证Schema支持**：
已验证 `/opt/code/newbee/unified-io/rpc/ent/schema/input_task.go` 包含：
- Line 26: `task_type` 字段（支持 "scheduled" 值）
- Line 31: `scheduled_at` 字段（Optional, Nillable）

**关键设计决策**：
1. ✅ **复用processTask逻辑** - scheduled任务使用相同的执行流程，无需重复代码
2. ✅ **独立的ticker** - 每分钟检查一次，避免影响manual任务的10秒轮询
3. ✅ **并发控制** - 复用现有的MaxConcurrent限制和指标统计
4. ✅ **SystemContext** - 使用系统上下文绕过租户隔离（Worker是后台进程）
5. ✅ **向后兼容** - 不影响现有的manual/triggered任务

**影响范围**：
- ✅ 单个文件修改（task_worker.go）
- ✅ 代码行数：约80行（1个方法修改 + 1个新方法）
- ✅ 无需修改其他组件（复用现有的processTask、StateMachine、Metrics）
- ✅ 无需数据库迁移（Schema已支持）
- 工作量：**1-2天**（包括单元测试）

**测试方法**：
```go
// 创建一个scheduled任务测试
task := client.InputTask.Create().
    SetTaskName("测试定时任务").
    SetTaskType("scheduled").
    SetScheduledAt(time.Now().Add(30 * time.Second)).  // 30秒后执行
    SetInputSource("api").
    SetTaskStatus("pending").
    SetTenantID(1).
    Save(systemCtx)

// 等待30秒后，观察TaskWorker日志：
// - 应显示 "Pulled scheduled tasks" (count=1)
// - 任务状态应变为 "processing" → "completed"
```

#### 修改2：Create Logic 支持 scheduled 参数

**文件**：`/opt/code/newbee/unified-io/rpc/internal/logic/inputtask/create_input_task_logic.go`

**新增验证**：
```go
func (l *CreateInputTaskLogic) CreateInputTask(in *io.InputTaskInfo) (*io.BaseIDResp, error) {
    // 验证定时参数
    if in.TaskType != nil && *in.TaskType == "scheduled" {
        if in.ScheduledAt == nil {
            return nil, errors.New("scheduled任务必须提供ScheduledAt时间")
        }

        scheduledTime := time.Unix(*in.ScheduledAt, 0)
        if scheduledTime.Before(time.Now()) {
            return nil, errors.New("ScheduledAt不能是过去的时间")
        }
    }

    // 现有创建逻辑...
}
```

**工作量**：**半天**

#### 修改3：API 层暴露 scheduled 参数

**文件**：`/opt/code/newbee/unified-io/api/desc/input_task.api`

**当前定义**（已支持，无需修改）：
```go
type InputTaskInfo {
    TaskType     string  `json:"taskType,optional,default=manual"` // manual|scheduled|triggered
    ScheduledAt  int64   `json:"scheduledAt,optional"`             // Unix时间戳
    // ...
}
```

**工作量**：**无需修改**

---

### 3.2 中期方案（1个月，生产级）

**目标**：支持 Cron 表达式周期性调度

#### 改造1：集成 Cron 解析库

**依赖**：
```go
import "github.com/robfig/cron/v3"
```

**实现**：
```go
type CronScheduler struct {
    cron      *cron.Cron
    db        *ent.Client
    taskQueue chan *ent.DiscoveryPool
}

func (s *CronScheduler) Start() {
    s.cron = cron.New(cron.WithSeconds())

    // 加载所有启用的 DiscoveryPool
    pools, _ := s.db.DiscoveryPool.Query().
        Where(discoverypool.ScheduleNEQ("")).  // 有Cron表达式
        Where(discoverypool.StatusEQ(1)).     // 启用状态
        All(context.Background())

    // 为每个Pool注册定时任务
    for _, pool := range pools {
        _, err := s.cron.AddFunc(pool.Schedule, func() {
            s.createScheduledTask(pool)
        })
        if err != nil {
            logx.Errorf("注册Cron失败: poolId=%d, schedule=%s, error=%v",
                pool.ID, pool.Schedule, err)
        }
    }

    s.cron.Start()
}

func (s *CronScheduler) createScheduledTask(pool *ent.DiscoveryPool) {
    // 创建 InputTask
    task, err := s.db.InputTask.Create().
        SetTaskType("scheduled").
        SetScheduledAt(time.Now()). // 立即执行
        SetTaskStatus("pending").
        SetDiscoveryPool(pool).
        Save(context.Background())

    if err != nil {
        logx.Errorf("创建定时任务失败: %v", err)
    }
}
```

**集成到 ServiceContext**：
```go
// /opt/code/newbee/unified-io/rpc/internal/svc/service_context.go
type ServiceContext struct {
    Config       config.Config
    DB           *ent.Client
    TaskWorker   *worker.TaskWorker
    CronScheduler *worker.CronScheduler // 新增
}

func NewServiceContext(c config.Config) *ServiceContext {
    svcCtx := &ServiceContext{
        DB: db,
        TaskWorker: worker.NewTaskWorker(db, ...),
        CronScheduler: worker.NewCronScheduler(db), // 新增
    }

    // 启动定时调度器
    svcCtx.CronScheduler.Start()

    return svcCtx
}
```

**工作量**：**3-5天**

#### 改造2：分布式锁（避免重复执行）

**问题**：多个 unified-io 实例同时运行时，可能重复触发定时任务

**解决方案**：使用 Redis 分布式锁

```go
func (s *CronScheduler) createScheduledTask(pool *ent.DiscoveryPool) {
    lockKey := fmt.Sprintf("discovery:schedule:lock:%d", pool.ID)

    // 尝试获取锁（30秒过期）
    lock, err := s.redis.SetNX(context.Background(), lockKey, "locked", 30*time.Second).Result()
    if err != nil || !lock {
        // 其他实例已经创建了任务，跳过
        return
    }

    defer s.redis.Del(context.Background(), lockKey)

    // 创建任务（原有逻辑）
    s.db.InputTask.Create()...
}
```

**工作量**：**1-2天**

#### 改造3：动态更新 Cron 配置

**需求**：用户修改 DiscoveryPool 的 schedule 后，立即生效

**实现**：
```go
// 监听配置变更（通过Redis订阅或定期刷新）
func (s *CronScheduler) ReloadSchedules() {
    s.cron.Stop()

    // 清除所有任务
    for _, entry := range s.cron.Entries() {
        s.cron.Remove(entry.ID)
    }

    // 重新加载
    pools, _ := s.db.DiscoveryPool.Query()...
    for _, pool := range pools {
        s.cron.AddFunc(pool.Schedule, ...)
    }

    s.cron.Start()
}

// 在 UpdateDiscoveryPool Logic 中触发重载
func (l *UpdateDiscoveryPoolLogic) UpdateDiscoveryPool(...) {
    // 更新数据库...

    // 触发Cron重载
    l.svcCtx.CronScheduler.ReloadSchedules()
}
```

**工作量**：**2-3天**

---

### 3.3 长期方案（2-3个月，完整优化）

#### 优化1：智能 Worker 选择

**集成 Ops-Center 的 Worker 管理**：

```go
// Unified-IO Service Context 已有 OpsRpc 客户端
type ServiceContext struct {
    OpsRpc opsclient.Ops // ✅ 已存在
}

// 在执行任务时选择 Worker
func (w *TaskWorker) executeTask(task *ent.InputTask) {
    pool := task.Edges.DiscoveryPool

    // 调用 Ops-Center 选择 Worker
    workerResp, err := w.svcCtx.OpsRpc.PickWorker(ctx, &ops.WorkerPickReq{
        Strategy:             "least_connections",
        RequiredCapabilities: getRequiredCapabilities(pool.ProviderID),
        PreferredRegion:      pool.Region,
    })

    if err != nil {
        // 失败处理：重试或标记为failed
    }

    // 将任务分发到选定的 Worker
    w.dispatchToWorker(workerResp.WorkerID, task)
}
```

**工作量**：**5-7天**

#### 优化2：任务编排能力

**支持任务依赖**：
```go
type DiscoveryTask struct {
    DependsOn []uint64 // 依赖的其他任务ID
}

// 执行前检查依赖
func (w *TaskWorker) canExecute(task *ent.InputTask) bool {
    for _, depID := range task.DependsOn {
        depTask, _ := w.db.InputTask.Get(ctx, depID)
        if depTask.TaskStatus != "completed" {
            return false // 依赖任务未完成
        }
    }
    return true
}
```

**工作量**：**7-10天**

#### 优化3：监控和告警

**指标收集**：
- 任务成功率
- 任务平均耗时
- Worker负载分布
- 失败任务分布

**告警规则**：
- 任务连续失败 > 3次
- 任务执行时间 > 阈值
- Worker长时间无响应

**工作量**：**5-7天**

---

## 四、数据库变更

### 4.1 新增表（仅1个）

```sql
-- discovery_tasks 表（任务执行记录，可选）
CREATE TABLE `io_discovery_tasks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `task_id` varchar(50) NOT NULL COMMENT '任务ID（UUID）',
  `pool_id` bigint unsigned NOT NULL COMMENT '发现池ID',
  `tenant_id` bigint unsigned NOT NULL,

  -- 触发信息
  `trigger_type` varchar(20) NOT NULL COMMENT 'manual|cron|event',
  `trigger_by` bigint unsigned COMMENT '触发人ID',

  -- 执行信息
  `target_worker_id` varchar(50) COMMENT '目标Worker ID',
  `status` varchar(20) NOT NULL COMMENT 'pending|running|success|failed',
  `start_time` datetime COMMENT '开始时间',
  `end_time` datetime COMMENT '结束时间',

  -- 结果统计
  `total_count` int COMMENT '总记录数',
  `success_count` int COMMENT '成功数',
  `failed_count` int COMMENT '失败数',
  `error_message` text COMMENT '错误信息',

  -- 重试信息
  `retry_count` int NOT NULL DEFAULT 0,

  PRIMARY KEY (`task_id`),
  KEY `idx_pool_id` (`pool_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='发现任务执行记录';
```

**说明**：这个表是可选的，用于记录任务执行历史。也可以复用现有的 `io_input_tasks` 表。

### 4.2 修改现有表（无需修改）

**DiscoveryPool 表**：
- ✅ `schedule` 字段已存在（Cron表达式）
- ✅ `enabled` 字段已存在（是否启用）

**InputTask 表**：
- ✅ `task_type` 字段已存在（manual|scheduled|triggered）
- ✅ `scheduled_at` 字段已存在（计划执行时间）

**结论**：Schema 完全满足需求，**无需数据库迁移**

---

## 五、关键文件清单

### 5.1 核心修改文件

| 文件路径 | 修改内容 | 优先级 | 工作量 |
|---------|---------|--------|--------|
| `/opt/code/newbee/unified-io/rpc/internal/worker/task_worker.go` | 增加定时任务检查逻辑 | **高** | 1-2天 |
| `/opt/code/newbee/unified-io/rpc/internal/worker/cron_scheduler.go` | 新增Cron调度器（新文件） | 中 | 3-5天 |
| `/opt/code/newbee/unified-io/rpc/internal/logic/inputtask/create_input_task_logic.go` | 增加scheduled参数验证 | 高 | 0.5天 |
| `/opt/code/newbee/unified-io/rpc/internal/svc/service_context.go` | 集成CronScheduler | 中 | 0.5天 |

### 5.2 可选优化文件

| 文件路径 | 修改内容 | 优先级 | 工作量 |
|---------|---------|--------|--------|
| `/opt/code/newbee/unified-io/rpc/internal/worker/worker_selector.go` | Worker智能选择（新文件） | 低 | 5-7天 |
| `/opt/code/newbee/unified-io/rpc/internal/logic/discoverypool/update_discovery_pool_logic.go` | 触发Cron重载 | 中 | 1天 |

---

## 六、实施计划

### Phase 1：短期方案（1-2周）

**目标**：支持基本的定时调度

**任务清单**：
- [ ] TaskWorker 增加定时检查逻辑（1-2天）
- [ ] CreateInputTask 增加scheduled验证（0.5天）
- [ ] 编写单元测试（1天）
- [ ] 集成测试（2天）
- [ ] 文档更新（0.5天）

**验收标准**：
- ✅ 用户可以创建 scheduled 类型的 InputTask
- ✅ TaskWorker 自动检测到期任务并执行
- ✅ 支持一次性定时任务

### Phase 2：中期方案（1个月）

**目标**：支持 Cron 表达式周期性调度

**任务清单**：
- [ ] 集成 robfig/cron 库（1天）
- [ ] 实现 CronScheduler（3-5天）
- [ ] 实现分布式锁（1-2天）
- [ ] 实现动态配置更新（2-3天）
- [ ] 编写单元测试和集成测试（3天）
- [ ] 性能测试和优化（2天）
- [ ] 文档和运维手册（2天）

**验收标准**：
- ✅ DiscoveryPool 可配置 Cron 表达式
- ✅ 定时任务自动触发
- ✅ 多实例环境下无重复执行
- ✅ 配置变更立即生效

### Phase 3：长期优化（2-3个月）

**目标**：完整的调度能力

**任务清单**：
- [ ] 集成 Ops-Center Worker 管理（5-7天）
- [ ] 实现任务依赖和编排（7-10天）
- [ ] 实现监控和告警（5-7天）
- [ ] 实现任务失败重试和容错（3-5天）
- [ ] 性能优化和压力测试（5天）
- [ ] 完整的文档和最佳实践（3天）

**验收标准**：
- ✅ 智能 Worker 选择
- ✅ 支持任务依赖
- ✅ 完善的监控告警
- ✅ 高可用性和容错

---

## 七、优势总结

### 7.1 改造方案的优势

| 优势 | 说明 |
|------|------|
| **快速交付** | 95%组件已存在，1-2周即可上线基本功能 |
| **低风险** | 无需新增服务，不影响现有架构 |
| **低成本** | 无需增加部署、监控、运维资源 |
| **高内聚** | 配置和调度在同一服务，逻辑清晰 |
| **易维护** | 单一代码库，易于调试和问题排查 |
| **向后兼容** | 不影响现有 manual/triggered 任务 |

### 7.2 与新增服务的对比

| 维度 | 改造 Unified-IO | 新增 Scheduler 服务 |
|------|----------------|-------------------|
| **开发周期** | 1-2周（短期） | 1-2个月（包括基础设施） |
| **代码规模** | +1000-2000行 | +3000-5000行（含RPC、API、部署） |
| **系统复杂度** | 无变化 | ↑ 增加（多一个服务） |
| **运维成本** | 无变化 | ↑ 增加（部署、监控、故障排查） |
| **数据一致性** | 本地事务 | 需要分布式事务 |
| **网络开销** | 无 | 频繁 RPC 调用 |
| **扩展性** | 中（受单服务限制） | 高（独立扩展） |

**结论**：在当前阶段，改造方案的优势明显大于新增服务。

---

## 八、风险与缓解

### 8.1 潜在风险

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| **单点故障** | Unified-IO 故障导致调度停止 | 部署多实例 + Redis分布式锁 |
| **性能瓶颈** | 大量定时任务影响服务性能 | 独立的 CronScheduler goroutine |
| **扩展性限制** | 未来任务量激增时难以扩展 | 预留扩展点，支持水平扩展 |

### 8.2 未来演进路径

如果未来出现以下情况，可以考虑拆分为独立服务：

1. **调度逻辑占用资源 > 50%**
2. **需要独立的扩展策略**（调度层和执行层分离）
3. **有独立的维护团队**
4. **代码规模 > 5000行**

**当前评估**：以上条件均不满足，改造方案是最优选择。

---

## 九、总结

### 核心观点

1. ✅ **Unified-IO 已经是 Discovery 的主控服务**，定时调度是其核心能力的自然扩展
2. ✅ **95% 的组件已存在**，只需补全最后 5% 的执行逻辑
3. ✅ **符合 NewBee 架构哲学**，避免过度拆分
4. ✅ **快速交付**，1-2周即可上线基本功能
5. ✅ **低风险、低成本、高内聚**

### 行动建议

**立即执行**（本周）：
- 启动 Phase 1 短期方案开发
- 修改 TaskWorker 和 CreateInputTask 逻辑
- 编写单元测试

**近期执行**（本月）：
- 启动 Phase 2 中期方案开发
- 集成 Cron 调度器
- 实现分布式锁和动态配置

**长期规划**（下季度）：
- 评估 Phase 3 长期优化的必要性
- 持续监控性能和资源使用
- 根据业务发展决定是否拆分

---

**生成时间**：2025-12-24
**决策依据**：Unified-IO、Ops-Center、NewBee架构探索分析
**推荐方案**：✅ 改造 Unified-IO（不新增服务）
