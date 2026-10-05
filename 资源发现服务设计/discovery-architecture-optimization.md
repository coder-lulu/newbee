# 发现配置系统架构优化与职责分离方案

## 执行摘要

**当前问题**：
- ⚠️ 配置逻辑和执行逻辑混合在同一服务中
- ⚠️ 没有明确的调度层，任务分发逻辑不清晰
- ⚠️ Worker/Agent角色定位模糊（谁负责执行？）
- ⚠️ 配置存储分散，缺乏版本管理和审计
- ⚠️ 前端直接与多个后端服务交互，增加复杂度

**优化目标**：
- ✅ 清晰的三层架构：配置层 → 调度层 → 执行层
- ✅ 明确的服务职责边界
- ✅ 统一的配置存储和管理
- ✅ 可扩展的任务调度机制
- ✅ 简化的前端交互模型

---

## 一、架构优化方案

### 1.1 三层架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│                         前端层 (Web UI)                          │
│  - 发现配置向导 (5步流程)                                         │
│  - 配置管理界面                                                   │
│  - 任务监控界面                                                   │
└────────────────────────────┬────────────────────────────────────┘
                             │ REST API
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                      配置层 (Configuration)                       │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Discovery-Config-API (新服务或扩展IO-API)                │   │
│  │  职责:                                                   │   │
│  │  - DiscoveryPool CRUD                                   │   │
│  │  - DiscoveryTemplate管理                                │   │
│  │  - FieldMapping配置                                     │   │
│  │  - Provider Schema查询                                  │   │
│  │  - 配置验证                                             │   │
│  │  - 配置版本管理                                         │   │
│  └─────────────────────────────────────────────────────────┘   │
└────────────────────────────┬────────────────────────────────────┘
                             │ gRPC/消息队列
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                      调度层 (Orchestration)                      │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Discovery-Scheduler (新服务)                             │   │
│  │  职责:                                                   │   │
│  │  - 任务调度 (定时、手动、事件触发)                       │   │
│  │  - Worker/Agent选择策略                                 │   │
│  │  - 任务分发                                             │   │
│  │  - 任务监控                                             │   │
│  │  - 失败重试                                             │   │
│  │  - 结果聚合                                             │   │
│  └─────────────────────────────────────────────────────────┘   │
└────────────────────────────┬────────────────────────────────────┘
                             │ gRPC
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                      执行层 (Execution)                          │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Worker节点 (多实例)                                       │   │
│  │  职责:                                                   │   │
│  │  - 接收任务                                             │   │
│  │  - 执行Discovery (调用Provider)                         │   │
│  │  - 数据采集                                             │   │
│  │  - 上报结果                                             │   │
│  │  - 健康检查                                             │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Agent节点 (可选，用于特殊环境)                            │   │
│  │  职责:                                                   │   │
│  │  - 受限网络环境发现                                     │   │
│  │  - 特定Provider支持                                     │   │
│  │  - 代理模式执行                                         │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 服务职责矩阵

| 服务 | 职责 | 不应该做 | 依赖 |
|------|------|---------|------|
| **Discovery-Config-API** | • 配置CRUD<br>• 配置验证<br>• 模板管理<br>• Schema查询<br>• 版本管理 | ❌ 执行发现<br>❌ 调度任务<br>❌ 选择Worker | Core-RPC (CI类型)<br>IO-RPC (Provider) |
| **Discovery-Scheduler** | • 任务调度<br>• Worker选择<br>• 任务分发<br>• 监控聚合<br>• 失败重试 | ❌ 配置管理<br>❌ 实际执行发现<br>❌ 数据入库 | Config-API<br>Ops-Center-API (Worker)<br>Core-RPC (CMDB) |
| **Worker节点** | • 执行任务<br>• 数据采集<br>• 结果上报<br>• 健康检查 | ❌ 配置管理<br>❌ 任务调度<br>❌ Worker选择 | Provider插件<br>Scheduler |
| **Agent节点** | • 特殊环境发现<br>• 代理执行<br>• 结果转发 | ❌ 配置管理<br>❌ 任务调度 | Provider插件<br>Scheduler |

---

## 二、详细职责分离设计

### 2.1 配置层 (Discovery-Config-API)

#### 2.1.1 核心职责

**配置管理**：
```go
// 1. DiscoveryPool配置
type DiscoveryPoolConfig struct {
    ID              uint64
    TenantID        uint64
    CiTypeID        uint64
    Name            string
    Description     string

    // 发现配置
    ProviderID      string                 // vmware_vcenter, aliyun_ecs等
    ProviderConfig  map[string]interface{} // 连接参数（加密存储）

    // 执行配置
    ExecutionMode   string                 // worker, agent
    WorkerID        *string                // 指定Worker
    WorkerGroupID   *uint64                // 指定WorkerGroup
    AgentID         *string                // 指定Agent

    // 调度配置
    ScheduleType    string                 // manual, cron, event
    CronExpression  *string                // 定时表达式
    Enabled         bool                   // 是否启用

    // 映射配置
    FieldMappings   []FieldMappingInfo     // 字段映射规则
    AutoImport      bool                   // 是否自动入库

    // 元数据
    Version         int                    // 配置版本
    CreatedAt       time.Time
    UpdatedAt       time.Time
    CreatedBy       uint64
    UpdatedBy       uint64
}

// 2. FieldMapping配置
type FieldMappingInfo struct {
    MappingID       uint64
    PoolID          uint64

    // 源字段
    SourceField     string                 // 发现字段名
    SourceFieldPath *string                // JSON路径（嵌套字段）
    SourceDataType  string                 // string, integer, float等

    // 目标字段
    TargetField     string                 // CI属性名
    TargetAttrID    uint64                 // CI属性ID
    TargetDataType  string

    // 转换配置
    TransformType   string                 // direct, calculate, lookup, conditional
    TransformConfig *string                // JSON配置
    DefaultValue    *string
    Required        bool
}
```

**API接口设计**：
```go
// Discovery Config API - REST接口
type DiscoveryConfigAPI interface {
    // DiscoveryPool管理
    CreatePool(ctx context.Context, req *CreatePoolReq) (*PoolInfo, error)
    UpdatePool(ctx context.Context, req *UpdatePoolReq) error
    DeletePool(ctx context.Context, poolId uint64) error
    GetPool(ctx context.Context, poolId uint64) (*PoolDetailInfo, error)
    ListPools(ctx context.Context, req *ListPoolsReq) (*PageResult[PoolInfo], error)

    // 配置验证
    ValidatePoolConfig(ctx context.Context, config *PoolConfig) (*ValidationResult, error)
    TestConnection(ctx context.Context, providerConfig *ProviderConfig) (*TestResult, error)

    // 字段映射
    SaveMappings(ctx context.Context, poolId uint64, mappings []FieldMappingInfo) error
    GetMappings(ctx context.Context, poolId uint64) ([]FieldMappingInfo, error)
    ValidateMappings(ctx context.Context, mappings []FieldMappingInfo) (*ValidationResult, error)

    // 模板管理
    SaveAsTemplate(ctx context.Context, req *SaveTemplateReq) (*TemplateInfo, error)
    LoadTemplate(ctx context.Context, templateId uint64) (*TemplateInfo, error)
    ListTemplates(ctx context.Context, req *ListTemplatesReq) (*PageResult[TemplateInfo], error)

    // 版本管理
    GetPoolHistory(ctx context.Context, poolId uint64) ([]PoolVersion, error)
    RollbackToVersion(ctx context.Context, poolId uint64, version int) error
}
```

**存储设计**：
```sql
-- 1. discovery_pools表（主配置表）
CREATE TABLE `discovery_pools` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL COMMENT '租户ID',
  `ci_type_id` bigint unsigned NOT NULL COMMENT 'CI类型ID',
  `name` varchar(100) NOT NULL COMMENT '发现池名称',
  `description` text COMMENT '描述',

  -- 发现配置
  `provider_id` varchar(50) NOT NULL COMMENT 'Provider ID',
  `provider_config` text NOT NULL COMMENT 'Provider配置（加密JSON）',

  -- 执行配置
  `execution_mode` varchar(20) NOT NULL DEFAULT 'worker' COMMENT 'worker|agent',
  `worker_id` varchar(50) COMMENT '指定Worker ID',
  `worker_group_id` bigint unsigned COMMENT '指定WorkerGroup ID',
  `agent_id` varchar(50) COMMENT '指定Agent ID',

  -- 调度配置
  `schedule_type` varchar(20) NOT NULL DEFAULT 'manual' COMMENT 'manual|cron|event',
  `cron_expression` varchar(100) COMMENT 'Cron表达式',
  `enabled` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',

  -- 映射配置
  `auto_import` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否自动入库',

  -- 版本和审计
  `version` int NOT NULL DEFAULT 1 COMMENT '配置版本',
  `status` tinyint unsigned NOT NULL DEFAULT 1 COMMENT '状态：1=正常 2=禁用',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `created_by` bigint unsigned COMMENT '创建人',
  `updated_by` bigint unsigned COMMENT '更新人',

  PRIMARY KEY (`id`),
  KEY `idx_tenant_id` (`tenant_id`),
  KEY `idx_ci_type_id` (`ci_type_id`),
  KEY `idx_provider_id` (`provider_id`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='发现池配置表';

-- 2. field_mappings表（字段映射表）
CREATE TABLE `field_mappings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `pool_id` bigint unsigned NOT NULL COMMENT '发现池ID',

  -- 源字段
  `source_field` varchar(100) NOT NULL COMMENT '源字段名',
  `source_field_path` varchar(255) COMMENT 'JSON路径',
  `source_data_type` varchar(20) NOT NULL COMMENT '源数据类型',

  -- 目标字段
  `target_field` varchar(100) NOT NULL COMMENT '目标字段名',
  `target_attr_id` bigint unsigned NOT NULL COMMENT 'CI属性ID',
  `target_data_type` varchar(20) NOT NULL COMMENT '目标数据类型',

  -- 转换配置
  `transform_type` varchar(20) NOT NULL DEFAULT 'direct' COMMENT 'direct|calculate|lookup|conditional|custom',
  `transform_config` text COMMENT '转换配置（JSON）',
  `default_value` varchar(255) COMMENT '默认值',
  `required` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否必填',

  -- 审计
  `status` tinyint unsigned NOT NULL DEFAULT 1,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (`id`),
  KEY `idx_tenant_id` (`tenant_id`),
  KEY `idx_pool_id` (`pool_id`),
  KEY `idx_target_attr_id` (`target_attr_id`),
  CONSTRAINT `fk_mapping_pool` FOREIGN KEY (`pool_id`) REFERENCES `discovery_pools` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='字段映射配置表';

-- 3. discovery_pool_versions表（版本历史表）
CREATE TABLE `discovery_pool_versions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `pool_id` bigint unsigned NOT NULL,
  `version` int NOT NULL COMMENT '版本号',
  `config_snapshot` text NOT NULL COMMENT '配置快照（JSON）',
  `change_summary` varchar(500) COMMENT '变更摘要',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `created_by` bigint unsigned COMMENT '创建人',

  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pool_version` (`pool_id`, `version`),
  CONSTRAINT `fk_version_pool` FOREIGN KEY (`pool_id`) REFERENCES `discovery_pools` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='发现池版本历史表';

-- 4. mapping_templates表（映射模板表）
CREATE TABLE `mapping_templates` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `name` varchar(100) NOT NULL COMMENT '模板名称',
  `description` text COMMENT '描述',
  `ci_type_id` bigint unsigned NOT NULL COMMENT 'CI类型ID',
  `provider_id` varchar(50) NOT NULL COMMENT 'Provider ID',
  `mappings` text NOT NULL COMMENT '映射配置（JSON）',
  `usage_count` int NOT NULL DEFAULT 0 COMMENT '使用次数',
  `status` tinyint unsigned NOT NULL DEFAULT 1,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `created_by` bigint unsigned,

  PRIMARY KEY (`id`),
  KEY `idx_tenant_id` (`tenant_id`),
  KEY `idx_ci_type_provider` (`ci_type_id`, `provider_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='映射模板表';
```

#### 2.1.2 配置验证逻辑

**验证流程**：
```go
// 配置验证服务
type ConfigValidationService struct {
    db           *ent.Client
    providerMgr  *ProviderManager
    ciTypeClient coreclient.CiTypeClient
}

// ValidatePoolConfig 验证发现池配置
func (s *ConfigValidationService) ValidatePoolConfig(ctx context.Context, config *PoolConfig) (*ValidationResult, error) {
    result := &ValidationResult{Valid: true, Errors: []string{}}

    // 1. 验证Provider配置
    provider, err := s.providerMgr.GetProvider(config.ProviderID)
    if err != nil {
        result.Valid = false
        result.Errors = append(result.Errors, fmt.Sprintf("Provider不存在: %s", config.ProviderID))
        return result, nil
    }

    // 2. 验证Provider参数
    schema := provider.GetSchema()
    for _, param := range schema.ParameterSchema {
        value, exists := config.ProviderConfig[param.Name]
        if param.Required && !exists {
            result.Valid = false
            result.Errors = append(result.Errors, fmt.Sprintf("必填参数缺失: %s", param.Label))
        }
        // 验证参数类型和格式
        if exists {
            if err := validateParameterValue(param, value); err != nil {
                result.Valid = false
                result.Errors = append(result.Errors, err.Error())
            }
        }
    }

    // 3. 验证执行配置
    if config.ExecutionMode == "worker" {
        if config.WorkerID == nil && config.WorkerGroupID == nil {
            result.Valid = false
            result.Errors = append(result.Errors, "Worker模式必须指定Worker或WorkerGroup")
        }
    } else if config.ExecutionMode == "agent" {
        if config.AgentID == nil {
            result.Valid = false
            result.Errors = append(result.Errors, "Agent模式必须指定Agent")
        }
    }

    // 4. 验证调度配置
    if config.ScheduleType == "cron" {
        if config.CronExpression == nil {
            result.Valid = false
            result.Errors = append(result.Errors, "定时调度必须提供Cron表达式")
        } else {
            if !isValidCronExpression(*config.CronExpression) {
                result.Valid = false
                result.Errors = append(result.Errors, "无效的Cron表达式")
            }
        }
    }

    return result, nil
}

// ValidateMappings 验证字段映射配置
func (s *ConfigValidationService) ValidateMappings(ctx context.Context, poolID uint64, mappings []FieldMappingInfo) (*ValidationResult, error) {
    result := &ValidationResult{Valid: true, Errors: []string{}, Warnings: []string{}}

    // 1. 获取发现池配置
    pool, err := s.db.DiscoveryPool.Get(ctx, poolID)
    if err != nil {
        return nil, err
    }

    // 2. 获取Provider的fieldSchema
    provider, _ := s.providerMgr.GetProvider(pool.ProviderID)
    fieldSchema := provider.GetSchema().FieldSchema

    // 3. 获取CI类型的属性定义
    ciAttrs, err := s.ciTypeClient.GetCiTypeAttributes(ctx, &core.IDReq{Id: pool.CiTypeID})
    if err != nil {
        return nil, err
    }

    // 4. 检查必填属性是否都被映射
    requiredAttrs := filterRequiredAttributes(ciAttrs.Data)
    mappedAttrIDs := extractMappedAttributeIDs(mappings)
    for _, reqAttr := range requiredAttrs {
        if !contains(mappedAttrIDs, reqAttr.ID) {
            result.Valid = false
            result.Errors = append(result.Errors, fmt.Sprintf("必填属性未映射: %s", reqAttr.Alias))
        }
    }

    // 5. 检查类型兼容性
    for _, mapping := range mappings {
        sourceField := findFieldInSchema(fieldSchema, mapping.SourceField)
        targetAttr := findAttributeByID(ciAttrs.Data, mapping.TargetAttrID)

        if sourceField != nil && targetAttr != nil {
            compatible := checkTypeCompatibility(sourceField.DataType, targetAttr.ValueType)
            if !compatible.Compatible {
                if mapping.TransformType == "direct" {
                    result.Valid = false
                    result.Errors = append(result.Errors, compatible.Error)
                } else {
                    result.Warnings = append(result.Warnings, compatible.Warning)
                }
            }
        }
    }

    // 6. 检查重复映射
    usedSourceFields := make(map[string]bool)
    usedTargetAttrs := make(map[uint64]bool)
    for _, mapping := range mappings {
        if usedSourceFields[mapping.SourceField] {
            result.Warnings = append(result.Warnings, fmt.Sprintf("源字段重复映射: %s", mapping.SourceField))
        }
        usedSourceFields[mapping.SourceField] = true

        if usedTargetAttrs[mapping.TargetAttrID] {
            result.Valid = false
            result.Errors = append(result.Errors, fmt.Sprintf("目标属性重复映射: %d", mapping.TargetAttrID))
        }
        usedTargetAttrs[mapping.TargetAttrID] = true
    }

    return result, nil
}
```

---

### 2.2 调度层 (Discovery-Scheduler)

#### 2.2.1 核心职责

**调度服务**：
```go
// Discovery Scheduler - 核心调度服务
type DiscoveryScheduler struct {
    db              *ent.Client
    redis           redis.UniversalClient
    configClient    DiscoveryConfigClient
    workerClient    WorkerClient
    agentClient     AgentClient
    cmdbClient      CMDBClient

    // 任务队列
    taskQueue       *TaskQueue

    // Worker管理器
    workerManager   *WorkerManager

    // 调度器
    cronScheduler   *cron.Cron
}

// 任务定义
type DiscoveryTask struct {
    TaskID          string
    PoolID          uint64
    TenantID        uint64
    TriggerType     string                 // manual, cron, event
    TriggerBy       uint64                 // 触发人ID

    // 执行配置
    ExecutionMode   string                 // worker, agent
    TargetWorkerID  *string
    TargetAgentID   *string

    // 任务状态
    Status          string                 // pending, running, success, failed, cancelled
    StartTime       *time.Time
    EndTime         *time.Time

    // 结果统计
    TotalCount      int
    SuccessCount    int
    FailedCount     int
    ErrorMessage    *string

    // 重试配置
    RetryCount      int
    MaxRetries      int
}
```

**调度接口**：
```go
// Scheduler API接口
type SchedulerAPI interface {
    // 任务管理
    SubmitTask(ctx context.Context, poolId uint64, triggerType string) (*TaskInfo, error)
    CancelTask(ctx context.Context, taskId string) error
    GetTaskStatus(ctx context.Context, taskId string) (*TaskStatus, error)
    ListTasks(ctx context.Context, req *ListTasksReq) (*PageResult[TaskInfo], error)

    // 调度管理
    EnableSchedule(ctx context.Context, poolId uint64) error
    DisableSchedule(ctx context.Context, poolId uint64) error
    TriggerManually(ctx context.Context, poolId uint64) (*TaskInfo, error)

    // 监控
    GetPoolMetrics(ctx context.Context, poolId uint64) (*PoolMetrics, error)
    GetWorkerLoad(ctx context.Context, workerId string) (*WorkerLoad, error)
}
```

**核心调度逻辑**：
```go
// SubmitTask 提交发现任务
func (s *DiscoveryScheduler) SubmitTask(ctx context.Context, poolID uint64, triggerType string) (*TaskInfo, error) {
    // 1. 获取发现池配置
    pool, err := s.configClient.GetPool(ctx, poolID)
    if err != nil {
        return nil, fmt.Errorf("获取配置失败: %w", err)
    }

    if !pool.Enabled {
        return nil, errors.New("发现池已禁用")
    }

    // 2. 创建任务记录
    task := &DiscoveryTask{
        TaskID:        uuid.New().String(),
        PoolID:        poolID,
        TenantID:      pool.TenantID,
        TriggerType:   triggerType,
        ExecutionMode: pool.ExecutionMode,
        Status:        "pending",
        MaxRetries:    3,
    }

    // 3. 选择执行节点
    var targetWorkerID, targetAgentID *string

    if pool.ExecutionMode == "worker" {
        // Worker模式：选择Worker
        workerID, err := s.selectWorker(ctx, pool)
        if err != nil {
            return nil, fmt.Errorf("选择Worker失败: %w", err)
        }
        targetWorkerID = &workerID
        task.TargetWorkerID = targetWorkerID

    } else if pool.ExecutionMode == "agent" {
        // Agent模式：选择Agent
        agentID, err := s.selectAgent(ctx, pool)
        if err != nil {
            return nil, fmt.Errorf("选择Agent失败: %w", err)
        }
        targetAgentID = &agentID
        task.TargetAgentID = targetAgentID
    }

    // 4. 保存任务到数据库
    dbTask, err := s.db.DiscoveryTask.Create().
        SetTaskID(task.TaskID).
        SetPoolID(task.PoolID).
        SetTenantID(task.TenantID).
        SetTriggerType(task.TriggerType).
        SetExecutionMode(task.ExecutionMode).
        SetNillableTargetWorkerID(targetWorkerID).
        SetNillableTargetAgentID(targetAgentID).
        SetStatus(task.Status).
        Save(ctx)
    if err != nil {
        return nil, fmt.Errorf("保存任务失败: %w", err)
    }

    // 5. 加入任务队列
    if err := s.taskQueue.Enqueue(task); err != nil {
        return nil, fmt.Errorf("加入任务队列失败: %w", err)
    }

    // 6. 异步执行任务
    go s.executeTask(context.Background(), task)

    return &TaskInfo{
        TaskID:   dbTask.TaskID,
        PoolID:   dbTask.PoolID,
        Status:   dbTask.Status,
        CreateAt: dbTask.CreatedAt,
    }, nil
}

// selectWorker 选择Worker节点
func (s *DiscoveryScheduler) selectWorker(ctx context.Context, pool *PoolConfig) (string, error) {
    // 1. 如果指定了Worker，直接使用
    if pool.WorkerID != nil {
        // 检查Worker是否在线
        worker, err := s.workerClient.GetWorker(ctx, *pool.WorkerID)
        if err != nil {
            return "", err
        }
        if worker.Status != "online" {
            return "", fmt.Errorf("指定的Worker不在线: %s", *pool.WorkerID)
        }
        return *pool.WorkerID, nil
    }

    // 2. 如果指定了WorkerGroup，从Group中选择
    if pool.WorkerGroupID != nil {
        picker := &WorkerPicker{
            GroupID:              *pool.WorkerGroupID,
            RequiredCapabilities: getRequiredCapabilities(pool.ProviderID),
            Strategy:             "least_connections", // 或从Group配置读取
        }

        worker, err := s.workerClient.PickWorkerFromGroup(ctx, picker)
        if err != nil {
            return "", fmt.Errorf("从WorkerGroup选择失败: %w", err)
        }
        return worker.WorkerID, nil
    }

    // 3. 自动选择（全局负载均衡）
    picker := &WorkerPicker{
        RequiredCapabilities: getRequiredCapabilities(pool.ProviderID),
        Strategy:             "least_connections",
    }

    worker, err := s.workerClient.PickWorker(ctx, picker)
    if err != nil {
        return "", fmt.Errorf("自动选择Worker失败: %w", err)
    }
    return worker.WorkerID, nil
}

// executeTask 执行任务
func (s *DiscoveryScheduler) executeTask(ctx context.Context, task *DiscoveryTask) {
    // 1. 更新任务状态为running
    s.updateTaskStatus(ctx, task.TaskID, "running")
    startTime := time.Now()

    // 2. 获取配置
    pool, _ := s.configClient.GetPool(ctx, task.PoolID)
    mappings, _ := s.configClient.GetMappings(ctx, task.PoolID)

    // 3. 构建执行请求
    execReq := &ExecuteDiscoveryReq{
        TaskID:         task.TaskID,
        ProviderID:     pool.ProviderID,
        ProviderConfig: pool.ProviderConfig,
        FieldMappings:  mappings,
    }

    // 4. 发送到Worker/Agent执行
    var result *ExecuteDiscoveryResp
    var err error

    if task.ExecutionMode == "worker" {
        result, err = s.workerClient.ExecuteDiscovery(ctx, *task.TargetWorkerID, execReq)
    } else {
        result, err = s.agentClient.ExecuteDiscovery(ctx, *task.TargetAgentID, execReq)
    }

    // 5. 处理执行结果
    if err != nil {
        // 失败处理
        s.handleTaskFailure(ctx, task, err)
        return
    }

    // 6. 更新任务状态
    endTime := time.Now()
    s.db.DiscoveryTask.UpdateOneID(task.TaskID).
        SetStatus("success").
        SetStartTime(startTime).
        SetEndTime(endTime).
        SetTotalCount(result.TotalCount).
        SetSuccessCount(result.SuccessCount).
        SetFailedCount(result.FailedCount).
        Save(ctx)

    // 7. 如果配置了自动入库，触发数据入库
    if pool.AutoImport {
        go s.importDiscoveryData(ctx, task.TaskID, result.Data)
    }
}

// handleTaskFailure 处理任务失败
func (s *DiscoveryScheduler) handleTaskFailure(ctx context.Context, task *DiscoveryTask, err error) {
    task.RetryCount++

    // 判断是否需要重试
    if task.RetryCount < task.MaxRetries {
        // 指数退避重试
        backoffDuration := time.Duration(math.Pow(2, float64(task.RetryCount))) * time.Second
        time.Sleep(backoffDuration)

        // 重新提交任务
        go s.executeTask(ctx, task)
        return
    }

    // 达到最大重试次数，标记为失败
    s.db.DiscoveryTask.UpdateOneID(task.TaskID).
        SetStatus("failed").
        SetEndTime(time.Now()).
        SetErrorMessage(err.Error()).
        Save(ctx)
}
```

**调度器初始化**：
```go
// InitScheduler 初始化调度器
func (s *DiscoveryScheduler) InitScheduler(ctx context.Context) error {
    s.cronScheduler = cron.New(cron.WithSeconds())

    // 加载所有启用的定时任务
    pools, err := s.configClient.ListPools(ctx, &ListPoolsReq{
        Enabled:      true,
        ScheduleType: "cron",
    })
    if err != nil {
        return err
    }

    // 为每个Pool注册定时任务
    for _, pool := range pools.Data {
        if pool.CronExpression != nil {
            _, err := s.cronScheduler.AddFunc(*pool.CronExpression, func() {
                s.SubmitTask(context.Background(), pool.ID, "cron")
            })
            if err != nil {
                logx.Errorf("添加定时任务失败: poolId=%d, error=%v", pool.ID, err)
            }
        }
    }

    // 启动调度器
    s.cronScheduler.Start()

    return nil
}
```

**存储设计**：
```sql
-- 1. discovery_tasks表（任务执行记录表）
CREATE TABLE `discovery_tasks` (
  `task_id` varchar(50) NOT NULL COMMENT '任务ID（UUID）',
  `tenant_id` bigint unsigned NOT NULL,
  `pool_id` bigint unsigned NOT NULL COMMENT '发现池ID',

  -- 触发信息
  `trigger_type` varchar(20) NOT NULL COMMENT 'manual|cron|event',
  `trigger_by` bigint unsigned COMMENT '触发人ID',

  -- 执行信息
  `execution_mode` varchar(20) NOT NULL COMMENT 'worker|agent',
  `target_worker_id` varchar(50) COMMENT '目标Worker ID',
  `target_agent_id` varchar(50) COMMENT '目标Agent ID',

  -- 任务状态
  `status` varchar(20) NOT NULL COMMENT 'pending|running|success|failed|cancelled',
  `start_time` datetime COMMENT '开始时间',
  `end_time` datetime COMMENT '结束时间',

  -- 结果统计
  `total_count` int COMMENT '总记录数',
  `success_count` int COMMENT '成功数',
  `failed_count` int COMMENT '失败数',
  `error_message` text COMMENT '错误信息',

  -- 重试信息
  `retry_count` int NOT NULL DEFAULT 0 COMMENT '重试次数',
  `max_retries` int NOT NULL DEFAULT 3 COMMENT '最大重试次数',

  -- 审计
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (`task_id`),
  KEY `idx_tenant_id` (`tenant_id`),
  KEY `idx_pool_id` (`pool_id`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`),
  CONSTRAINT `fk_task_pool` FOREIGN KEY (`pool_id`) REFERENCES `discovery_pools` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='发现任务执行记录表';

-- 2. discovery_task_logs表（任务详细日志表）
CREATE TABLE `discovery_task_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `task_id` varchar(50) NOT NULL,
  `log_level` varchar(10) NOT NULL COMMENT 'INFO|WARN|ERROR',
  `log_message` text NOT NULL,
  `log_data` text COMMENT '日志详细数据（JSON）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (`id`),
  KEY `idx_task_id` (`task_id`),
  KEY `idx_created_at` (`created_at`),
  CONSTRAINT `fk_log_task` FOREIGN KEY (`task_id`) REFERENCES `discovery_tasks` (`task_id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='发现任务日志表';
```

---

### 2.3 执行层 (Worker/Agent)

#### 2.3.1 Worker节点职责

**Worker核心功能**：
```go
// Worker节点 - 发现执行引擎
type Worker struct {
    workerID        string
    config          *WorkerConfig

    // Provider管理器
    providerMgr     *ProviderManager

    // 任务执行器
    executor        *TaskExecutor

    // 结果上报器
    reporter        *ResultReporter

    // 健康检查
    healthChecker   *HealthChecker
}

// 执行发现任务
func (w *Worker) ExecuteDiscovery(ctx context.Context, req *ExecuteDiscoveryReq) (*ExecuteDiscoveryResp, error) {
    // 1. 获取Provider
    provider, err := w.providerMgr.GetProvider(req.ProviderID)
    if err != nil {
        return nil, fmt.Errorf("Provider不存在: %s", req.ProviderID)
    }

    // 2. 执行发现
    startTime := time.Now()
    rawData, err := provider.Discover(ctx, req.ProviderConfig)
    if err != nil {
        // 上报错误
        w.reporter.ReportError(ctx, req.TaskID, err)
        return nil, err
    }

    // 3. 应用字段映射
    mappedData, err := w.applyFieldMappings(rawData, req.FieldMappings)
    if err != nil {
        return nil, fmt.Errorf("字段映射失败: %w", err)
    }

    // 4. 数据验证
    validatedData, failedItems := w.validateMappedData(mappedData)

    // 5. 构建响应
    resp := &ExecuteDiscoveryResp{
        TaskID:       req.TaskID,
        TotalCount:   len(rawData),
        SuccessCount: len(validatedData),
        FailedCount:  len(failedItems),
        Data:         validatedData,
        FailedItems:  failedItems,
        Duration:     time.Since(startTime).Milliseconds(),
    }

    // 6. 上报结果
    w.reporter.ReportResult(ctx, resp)

    return resp, nil
}

// applyFieldMappings 应用字段映射
func (w *Worker) applyFieldMappings(rawData []map[string]interface{}, mappings []FieldMappingInfo) ([]map[string]interface{}, error) {
    result := make([]map[string]interface{}, 0, len(rawData))

    for _, item := range rawData {
        mappedItem := make(map[string]interface{})

        for _, mapping := range mappings {
            // 获取源字段值
            sourceValue, err := w.getFieldValue(item, mapping.SourceField, mapping.SourceFieldPath)
            if err != nil {
                if mapping.Required {
                    return nil, fmt.Errorf("必填字段缺失: %s", mapping.SourceField)
                }
                // 使用默认值
                if mapping.DefaultValue != nil {
                    sourceValue = *mapping.DefaultValue
                } else {
                    continue
                }
            }

            // 应用转换
            transformedValue, err := w.applyTransform(sourceValue, mapping)
            if err != nil {
                return nil, fmt.Errorf("字段转换失败: %s -> %s, error: %w",
                    mapping.SourceField, mapping.TargetField, err)
            }

            // 设置目标字段
            mappedItem[mapping.TargetField] = transformedValue
        }

        result = append(result, mappedItem)
    }

    return result, nil
}

// applyTransform 应用字段转换
func (w *Worker) applyTransform(value interface{}, mapping FieldMappingInfo) (interface{}, error) {
    switch mapping.TransformType {
    case "direct":
        // 直接映射
        return value, nil

    case "calculate":
        // 计算转换（如 MB → GB）
        config := parseTransformConfig(mapping.TransformConfig)
        numValue, ok := value.(float64)
        if !ok {
            return nil, fmt.Errorf("calculate转换需要数值类型")
        }

        switch config.Operation {
        case "divide":
            return numValue / config.Operand, nil
        case "multiply":
            return numValue * config.Operand, nil
        default:
            return value, nil
        }

    case "lookup":
        // 查找表映射
        config := parseTransformConfig(mapping.TransformConfig)
        strValue, ok := value.(string)
        if !ok {
            return nil, fmt.Errorf("lookup转换需要字符串类型")
        }

        if mappedValue, exists := config.LookupTable[strValue]; exists {
            return mappedValue, nil
        }
        return value, nil

    case "format":
        // 格式化转换（如日期格式）
        config := parseTransformConfig(mapping.TransformConfig)
        return formatValue(value, config.SourceFormat, config.TargetFormat)

    case "conditional":
        // 条件映射
        config := parseTransformConfig(mapping.TransformConfig)
        return evaluateCondition(value, config.ConditionRules)

    default:
        return value, nil
    }
}
```

**Provider接口标准**：
```go
// Provider接口 - 所有发现Provider必须实现
type DiscoveryProvider interface {
    // 获取Provider信息
    GetID() string
    GetName() string
    GetCategory() string
    GetVersion() string

    // 获取Schema定义
    GetSchema() *ProviderSchemaInfo

    // 测试连接
    TestConnection(ctx context.Context, config map[string]interface{}) error

    // 执行发现
    Discover(ctx context.Context, config map[string]interface{}) ([]map[string]interface{}, error)

    // 获取支持的能力
    GetCapabilities() []string
}

// Provider Schema定义
type ProviderSchemaInfo struct {
    ID              string
    Name            string
    Category        string
    Version         string
    Description     string
    Icon            string

    // 参数定义
    ParameterSchema []ParameterDefinition

    // 字段定义
    FieldSchema     []FieldDefinition

    // 能力要求
    RequiredCapabilities []string
}

// 参数定义
type ParameterDefinition struct {
    Name        string
    Label       string
    Type        string                 // text, password, number, select, etc.
    Required    bool
    Default     *string
    Options     []SelectOption
    Validation  *ValidationRule
    Placeholder string
    Help        string
}

// 字段定义
type FieldDefinition struct {
    Name        string
    Label       string
    DataType    string                 // string, integer, float, boolean, date
    Path        *string                // JSON路径（嵌套字段）
    Description string
}
```

#### 2.3.2 Agent节点职责

**Agent与Worker的区别**：

| 特性 | Worker | Agent |
|------|--------|-------|
| **部署位置** | 云端/中心机房 | 客户机房/隔离网络 |
| **网络要求** | 需要访问目标资源 | 仅需访问Scheduler |
| **使用场景** | 公有云、标准网络环境 | 私有云、专有网络、隔离环境 |
| **Provider支持** | 全部 | 部分（根据环境） |
| **数据流向** | 直接采集 → Scheduler | 采集 → 缓存 → 上报 |

**Agent实现**：
```go
// Agent节点 - 特殊环境发现代理
type Agent struct {
    agentID         string
    config          *AgentConfig

    // 与Scheduler的连接
    schedulerConn   *grpc.ClientConn

    // Provider管理器（子集）
    providerMgr     *ProviderManager

    // 本地缓存
    cache           *LocalCache
}

// Agent与Worker的主要区别在于：
// 1. Agent主动轮询任务（拉模式），Worker接收任务（推模式）
// 2. Agent可能需要缓存结果后批量上报
// 3. Agent可能运行部分Provider（受环境限制）
```

---

## 三、功能分配与交互流程

### 3.1 完整发现流程

```
1️⃣ 用户配置阶段（前端 + Config-API）
   ┌─────────────────────────────────────────────┐
   │ 用户在前端完成5步配置向导                    │
   │  Step 1: 选择发现方式 (Provider)             │
   │  Step 2: 配置连接参数                        │
   │  Step 3: 选择执行代理 (Worker/Agent)         │
   │  Step 4: 配置字段映射                        │
   │  Step 5: 保存配置                            │
   └─────────────────┬───────────────────────────┘
                     │ POST /api/discovery-pools
                     ▼
   ┌─────────────────────────────────────────────┐
   │ Config-API处理                               │
   │  - 验证配置                                  │
   │  - 保存DiscoveryPool                         │
   │  - 保存FieldMappings                         │
   │  - 返回PoolID                                │
   └─────────────────┬───────────────────────────┘
                     │
                     ▼ 配置完成，等待调度

2️⃣ 任务触发阶段（Scheduler）
   ┌─────────────────────────────────────────────┐
   │ 触发方式:                                    │
   │  - 手动触发: 用户点击"立即执行"              │
   │  - 定时触发: Cron表达式到期                  │
   │  - 事件触发: CI变更/告警等                   │
   └─────────────────┬───────────────────────────┘
                     │
                     ▼
   ┌─────────────────────────────────────────────┐
   │ Scheduler.SubmitTask()                      │
   │  1. 加载DiscoveryPool配置                   │
   │  2. 选择Worker/Agent                        │
   │     - 指定节点: 检查健康状态                 │
   │     - 自动选择: 负载均衡算法                 │
   │  3. 创建DiscoveryTask记录                   │
   │  4. 加入任务队列                             │
   │  5. 异步执行executeTask()                   │
   └─────────────────┬───────────────────────────┘
                     │
                     ▼

3️⃣ 任务分发阶段（Scheduler → Worker/Agent）
   ┌─────────────────────────────────────────────┐
   │ Scheduler.executeTask()                     │
   │  - 构建ExecuteDiscoveryReq                  │
   │  - gRPC调用Worker/Agent                     │
   └─────────────────┬───────────────────────────┘
                     │ gRPC Request
                     ▼
   ┌─────────────────────────────────────────────┐
   │ Worker.ExecuteDiscovery()                   │
   │  1. 获取Provider实例                         │
   │  2. 调用Provider.Discover()                 │
   │     → 连接目标系统                           │
   │     → 采集原始数据                           │
   │  3. 应用FieldMappings                       │
   │     → 字段转换                               │
   │     → 类型转换                               │
   │     → 默认值填充                             │
   │  4. 数据验证                                 │
   │  5. 构建ExecuteDiscoveryResp                │
   └─────────────────┬───────────────────────────┘
                     │ gRPC Response
                     ▼

4️⃣ 结果处理阶段（Scheduler + CMDB）
   ┌─────────────────────────────────────────────┐
   │ Scheduler接收Worker响应                      │
   │  - 更新Task状态为success                     │
   │  - 记录统计信息                              │
   │  - 保存任务日志                              │
   └─────────────────┬───────────────────────────┘
                     │
                     ▼ 如果AutoImport=true
   ┌─────────────────────────────────────────────┐
   │ Scheduler.importDiscoveryData()             │
   │  - 调用CMDB-RPC批量创建/更新CI               │
   │  - 更新CI属性                                │
   │  - 记录导入日志                              │
   └─────────────────────────────────────────────┘
```

### 3.2 关键交互序列

#### 交互1：用户创建发现配置
```
Frontend          Config-API          Provider-Manager    Worker-API
   │                   │                      │               │
   │ 1. 选择Provider   │                      │               │
   ├──────────────────>│ 2. 获取Schema        │               │
   │                   ├─────────────────────>│               │
   │                   │<─────────────────────┤               │
   │<──────────────────┤ 3. 返回参数定义      │               │
   │                   │                      │               │
   │ 4. 填写参数       │                      │               │
   │ 5. 测试连接       │                      │               │
   ├──────────────────>│ 6. 验证参数          │               │
   │                   ├─────────────────────>│ 7. TestConn   │
   │                   │<─────────────────────┤ 8. Success    │
   │<──────────────────┤ 9. 连接成功          │               │
   │                   │                      │               │
   │ 10. 选择Worker    │                      │               │
   ├──────────────────>│ 11. 查询Worker列表   │               │
   │                   ├─────────────────────────────────────>│
   │                   │<─────────────────────────────────────┤
   │<──────────────────┤ 12. 返回在线Worker    │               │
   │                   │                      │               │
   │ 13. 配置映射      │                      │               │
   │ 14. 保存配置      │                      │               │
   ├──────────────────>│ 15. 创建Pool&Mappings│               │
   │                   │ 16. 保存到DB         │               │
   │<──────────────────┤ 17. 返回PoolID       │               │
```

#### 交互2：定时任务执行
```
Cron-Scheduler    Scheduler        Config-API    Worker-API      Worker-Node
   │                   │                │             │               │
   │ 1. Cron触发      │                │             │               │
   ├─────────────────>│ 2. 加载Pool    │             │               │
   │                   ├───────────────>│             │               │
   │                   │<───────────────┤             │               │
   │                   │ 3. 选择Worker  │             │               │
   │                   ├───────────────────────────>│               │
   │                   │<───────────────────────────┤ 4. 返回WorkerID│
   │                   │ 5. 创建Task    │             │               │
   │                   │ 6. gRPC分发    │             │               │
   │                   ├─────────────────────────────────────────────>│
   │                   │                │             │ 7. Discover   │
   │                   │                │             │ 8. ApplyMapping│
   │                   │<─────────────────────────────────────────────┤
   │                   │ 9. 更新Task状态│             │ 10. 返回结果  │
   │                   │ 11. 触发导入   │             │               │
   │                   │<───────CMDB-RPC──────>     │               │
```

---

## 四、存储方案总结

### 4.1 核心表设计

| 表名 | 所属服务 | 职责 | 关键字段 |
|------|---------|------|---------|
| `discovery_pools` | Config-API | 发现池配置 | provider_id, provider_config, execution_mode, schedule_type, cron_expression |
| `field_mappings` | Config-API | 字段映射规则 | source_field, target_attr_id, transform_type, transform_config |
| `mapping_templates` | Config-API | 映射模板 | ci_type_id, provider_id, mappings (JSON) |
| `discovery_pool_versions` | Config-API | 配置版本历史 | pool_id, version, config_snapshot (JSON) |
| `discovery_tasks` | Scheduler | 任务执行记录 | task_id, pool_id, status, target_worker_id, start_time, end_time |
| `discovery_task_logs` | Scheduler | 任务详细日志 | task_id, log_level, log_message |

### 4.2 配置加密存储

**敏感信息加密**：
```go
// 敏感参数加密服务
type ConfigEncryptionService struct {
    secretKey []byte
}

// EncryptProviderConfig 加密Provider配置
func (s *ConfigEncryptionService) EncryptProviderConfig(config map[string]interface{}) (string, error) {
    // 识别敏感字段
    sensitiveFields := []string{"password", "access_key", "secret_key", "token", "api_key"}

    for _, field := range sensitiveFields {
        if value, exists := config[field]; exists {
            // AES加密
            encrypted, err := s.encrypt(value.(string))
            if err != nil {
                return "", err
            }
            config[field] = encrypted
        }
    }

    // 序列化为JSON
    jsonBytes, err := json.Marshal(config)
    if err != nil {
        return "", err
    }

    return string(jsonBytes), nil
}

// DecryptProviderConfig 解密Provider配置
func (s *ConfigEncryptionService) DecryptProviderConfig(encrypted string) (map[string]interface{}, error) {
    var config map[string]interface{}
    if err := json.Unmarshal([]byte(encrypted), &config); err != nil {
        return nil, err
    }

    sensitiveFields := []string{"password", "access_key", "secret_key", "token", "api_key"}

    for _, field := range sensitiveFields {
        if value, exists := config[field]; exists {
            decrypted, err := s.decrypt(value.(string))
            if err != nil {
                return nil, err
            }
            config[field] = decrypted
        }
    }

    return config, nil
}
```

### 4.3 配置版本管理

**版本控制逻辑**：
```go
// UpdatePool 更新发现池配置（带版本管理）
func (s *DiscoveryConfigService) UpdatePool(ctx context.Context, req *UpdatePoolReq) error {
    return s.db.WithTx(ctx, func(tx *ent.Tx) error {
        // 1. 查询当前配置
        currentPool, err := tx.DiscoveryPool.Get(ctx, req.PoolID)
        if err != nil {
            return err
        }

        // 2. 保存当前配置为历史版本
        configSnapshot, _ := json.Marshal(currentPool)
        _, err = tx.DiscoveryPoolVersion.Create().
            SetPoolID(currentPool.ID).
            SetVersion(currentPool.Version).
            SetConfigSnapshot(string(configSnapshot)).
            SetChangeSummary(req.ChangeSummary).
            SetCreatedBy(req.UpdatedBy).
            Save(ctx)
        if err != nil {
            return err
        }

        // 3. 更新配置，增加版本号
        err = tx.DiscoveryPool.UpdateOneID(req.PoolID).
            SetName(req.Name).
            SetDescription(req.Description).
            SetProviderConfig(req.ProviderConfig).
            // ... 其他字段
            SetVersion(currentPool.Version + 1).
            SetUpdatedBy(req.UpdatedBy).
            Exec(ctx)

        return err
    })
}

// RollbackToVersion 回滚到指定版本
func (s *DiscoveryConfigService) RollbackToVersion(ctx context.Context, poolID uint64, version int) error {
    // 1. 查询指定版本的配置快照
    versionRecord, err := s.db.DiscoveryPoolVersion.Query().
        Where(
            discoverypoolversion.PoolIDEQ(poolID),
            discoverypoolversion.VersionEQ(version),
        ).
        Only(ctx)
    if err != nil {
        return fmt.Errorf("版本不存在: %w", err)
    }

    // 2. 反序列化配置快照
    var poolConfig DiscoveryPoolConfig
    if err := json.Unmarshal([]byte(versionRecord.ConfigSnapshot), &poolConfig); err != nil {
        return fmt.Errorf("配置快照解析失败: %w", err)
    }

    // 3. 应用配置（作为新版本）
    return s.UpdatePool(ctx, &UpdatePoolReq{
        PoolID:         poolID,
        Name:           poolConfig.Name,
        Description:    poolConfig.Description,
        ProviderConfig: poolConfig.ProviderConfig,
        // ... 其他字段
        ChangeSummary:  fmt.Sprintf("回滚到版本 %d", version),
    })
}
```

---

## 五、优化建议总结

### 5.1 立即实施（High Priority）

#### 1. 创建独立的Discovery-Scheduler服务

**理由**：
- ✅ 清晰的职责分离
- ✅ 独立扩展调度能力
- ✅ 不影响现有Config-API

**实施步骤**：
```bash
# 1. 创建新服务目录
cd /opt/code/newbee
mkdir -p discovery-scheduler/{api,rpc,internal}

# 2. 定义Proto接口
# discovery-scheduler/rpc/desc/scheduler.proto

# 3. 实现核心逻辑
# - 任务调度
# - Worker选择
# - 任务分发
# - 结果聚合

# 4. 集成到现有系统
# - Config-API调用Scheduler提交任务
# - Worker上报结果到Scheduler
```

#### 2. 标准化Provider接口

**理由**：
- ✅ 统一Provider实现规范
- ✅ 简化新Provider开发
- ✅ 便于Provider测试和调试

**实施步骤**：
1. 定义 `DiscoveryProvider` 接口
2. 提供 `BaseProvider` 抽象类
3. 编写Provider开发文档
4. 提供Provider脚手架工具

#### 3. 实现配置版本管理

**理由**：
- ✅ 配置变更可追溯
- ✅ 支持配置回滚
- ✅ 便于审计和故障排查

**实施步骤**：
1. 创建 `discovery_pool_versions` 表
2. 在 `UpdatePool` 中保存历史版本
3. 实现 `RollbackToVersion` 方法
4. 前端添加版本历史查看界面

### 5.2 近期实施（Medium Priority）

#### 4. 优化Worker选择策略

**当前问题**：Worker选择逻辑简单，未考虑：
- 网络距离
- 历史成功率
- 当前负载

**优化方案**：
```go
type WorkerSelector struct {
    strategy string // least_connections, weighted, locality_aware, smart
}

// SmartSelect 智能选择Worker
func (s *WorkerSelector) SmartSelect(ctx context.Context, pool *PoolConfig) (string, error) {
    // 1. 获取候选Worker列表
    candidates := s.getCandidates(pool)

    // 2. 过滤不可用的Worker
    candidates = s.filterAvailable(candidates)

    // 3. 评分排序
    scores := make(map[string]float64)
    for _, worker := range candidates {
        score := 0.0

        // 3.1 负载评分（40%）
        score += s.scoreByLoad(worker) * 0.4

        // 3.2 网络距离评分（30%）
        score += s.scoreByLocality(worker, pool) * 0.3

        // 3.3 历史成功率评分（30%）
        score += s.scoreBySuccessRate(worker, pool.ProviderID) * 0.3

        scores[worker.ID] = score
    }

    // 4. 选择最高分的Worker
    return s.selectTopScored(scores)
}
```

#### 5. 增强任务监控和告警

**监控指标**：
- 任务成功率
- 任务平均耗时
- Worker负载分布
- 失败任务分布

**告警规则**：
- 任务连续失败 > 3次
- 任务执行时间 > 阈值
- Worker长时间无响应
- 发现数据量异常变化

### 5.3 长期规划（Low Priority）

#### 6. 支持分布式任务执行

**场景**：大规模发现任务（如发现10000+虚拟机）

**方案**：
- 任务分片：将大任务拆分为多个子任务
- 并行执行：多个Worker并行处理
- 结果合并：Scheduler聚合所有子任务结果

#### 7. 实现Discovery-as-Code

**场景**：通过代码/配置文件定义发现配置

**示例**：
```yaml
# discovery-config.yaml
discovery_pools:
  - name: "阿里云ECS发现"
    provider: "aliyun_ecs"
    schedule: "0 0 * * *"
    config:
      access_key: "${ALIYUN_ACCESS_KEY}"
      secret_key: "${ALIYUN_SECRET_KEY}"
      region: "cn-hangzhou"
    mappings:
      - source: "InstanceId"
        target: "instance_id"
      - source: "InstanceName"
        target: "hostname"
      - source: "PrivateIpAddress"
        target: "ip_address"
        transform:
          type: "format"
          config:
            pattern: "extract_first"
```

---

## 六、总结

### 优化前 vs 优化后对比

| 维度 | 优化前 | 优化后 |
|------|--------|--------|
| **架构清晰度** | ⭐⭐☆☆☆ 配置和执行混合 | ⭐⭐⭐⭐⭐ 三层架构清晰 |
| **职责分离** | ⭐⭐☆☆☆ 职责模糊 | ⭐⭐⭐⭐⭐ 各司其职 |
| **可扩展性** | ⭐⭐⭐☆☆ 新增功能困难 | ⭐⭐⭐⭐⭐ 易于扩展 |
| **可维护性** | ⭐⭐☆☆☆ 修改影响面大 | ⭐⭐⭐⭐☆ 模块独立 |
| **可测试性** | ⭐⭐☆☆☆ 集成测试困难 | ⭐⭐⭐⭐☆ 单元测试友好 |
| **配置管理** | ⭐⭐☆☆☆ 无版本控制 | ⭐⭐⭐⭐☆ 版本+审计 |
| **任务调度** | ⭐⭐☆☆☆ 简单定时 | ⭐⭐⭐⭐☆ 灵活调度 |
| **监控能力** | ⭐⭐☆☆☆ 缺乏监控 | ⭐⭐⭐⭐☆ 完善监控 |

### 核心价值

**配置层（Config-API）**：
- ✅ 专注配置管理，提供完整的CRUD、验证、模板、版本管理
- ✅ 不关心任务执行细节，仅提供配置服务
- ✅ 易于扩展新的配置类型

**调度层（Scheduler）**：
- ✅ 统一的任务调度中心，支持手动、定时、事件触发
- ✅ 智能Worker选择，支持多种负载均衡策略
- ✅ 任务监控和失败重试，提高可靠性

**执行层（Worker/Agent）**：
- ✅ 专注任务执行，与调度逻辑完全解耦
- ✅ 标准化的Provider接口，易于开发新Provider
- ✅ 支持多种部署模式（Worker/Agent），适应不同网络环境

---

**生成时间**：2025-12-24
**文档版本**：v1.0
**适用范围**：发现配置系统架构优化
