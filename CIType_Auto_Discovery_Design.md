# CIType自动发现配置设计方案

## 1. 总体架构设计

### 1.1 设计目标

基于前期对CMDB、前端UI和IO服务的深入分析，我们需要在各CIType上实现自动发现配置，使每个CI类型都能配置多种发现方式，首先实现属性发现功能。

### 1.2 核心设计原则

1. **多源发现**: 一个CIType可以配置多种发现方式（文件导入、API采集、Agent扫描等）
2. **属性映射**: 灵活的源字段到CI属性的映射配置
3. **优先级管理**: 多个发现源的数据冲突时的优先级处理
4. **增量更新**: 支持属性级别的增量发现和更新
5. **可扩展性**: 支持未来扩展关系发现、依赖发现等功能

## 2. 数据模型设计

### 2.1 CIType发现配置数据模型

#### 核心Entity: CiTypeDiscoveryConfig

```go
// CiTypeDiscoveryConfig CI类型发现配置
func (CiTypeDiscoveryConfig) Fields() []ent.Field {
    return []ent.Field{
        field.Uint64("id"),
        
        // 基础信息
        field.Uint64("ci_type_id").Comment("CI类型ID"),
        field.String("config_name").MaxLen(200).Comment("配置名称"),
        field.String("description").Optional().Comment("配置描述"),
        
        // 发现配置
        field.String("discovery_mode").Default("attribute").Comment("发现模式: attribute, relation, dependency"),
        field.String("discovery_type").Comment("发现类型: file, api, sdk, builtin, agent"),
        field.String("provider_id").Comment("发现提供者ID"),
        field.JSON("provider_config", map[string]interface{}{}).Comment("提供者配置JSON"),
        
        // 属性映射配置
        field.JSON("attribute_mappings", []interface{}{}).Comment("属性映射配置JSON"),
        field.JSON("discovery_rules", map[string]interface{}{}).Comment("发现规则JSON"),
        field.JSON("filter_conditions", map[string]interface{}{}).Comment("过滤条件JSON"),
        
        // 执行配置
        field.String("execution_mode").Default("manual").Comment("执行模式: manual, scheduled, triggered"),
        field.JSON("schedule_config", map[string]interface{}{}).Optional().Comment("调度配置JSON"),
        field.Int("priority").Default(100).Comment("优先级(数字越小优先级越高)"),
        field.Int("batch_size").Default(100).Comment("批处理大小"),
        field.Int("timeout_seconds").Default(300).Comment("超时时间(秒)"),
        
        // 数据处理配置
        field.String("conflict_resolution").Default("priority").Comment("冲突解决策略: priority, merge, latest"),
        field.Bool("auto_create_ci").Default(false).Comment("是否自动创建CI实例"),
        field.Bool("auto_update_attributes").Default(true).Comment("是否自动更新属性"),
        field.JSON("notification_config", map[string]interface{}{}).Optional().Comment("通知配置JSON"),
        
        // 状态字段
        field.Bool("enabled").Default(true).Comment("是否启用"),
        field.String("status").Default("active").Comment("状态: active, inactive, error"),
        field.Time("last_executed_at").Optional().Comment("上次执行时间"),
        field.Time("next_execution_at").Optional().Comment("下次执行时间"),
        
        // 统计信息
        field.JSON("execution_stats", map[string]interface{}{}).Optional().Comment("执行统计JSON"),
        field.Text("last_error").Optional().Comment("最后错误信息"),
        
        // 租户和用户
        field.Uint64("tenant_id").Comment("租户ID"),
        field.Uint64("created_by").Comment("创建人ID"),
        field.Uint64("updated_by").Optional().Comment("更新人ID"),
    }
}

func (CiTypeDiscoveryConfig) Edges() []ent.Edge {
    return []ent.Edge{
        // 关联CI类型
        edge.From("ci_type", CiType.Type).
            Ref("discovery_configs").
            Field("ci_type_id").
            Unique().
            Required(),
            
        // 关联发现历史
        edge.To("execution_histories", DiscoveryExecutionHistory.Type),
        
        // 关联属性映射规则
        edge.To("attribute_mapping_rules", AttributeMappingRule.Type),
    }
}

func (CiTypeDiscoveryConfig) Mixin() []ent.Mixin {
    return []ent.Mixin{
        mixins.TenantMixin{},
        mixins.TimeMixin{},
        mixins.StatusMixin{},
    }
}
```

#### 属性映射规则Entity: AttributeMappingRule

```go
// AttributeMappingRule 属性映射规则
func (AttributeMappingRule) Fields() []ent.Field {
    return []ent.Field{
        field.Uint64("id"),
        
        // 关联信息
        field.Uint64("discovery_config_id").Comment("发现配置ID"),
        field.Uint64("ci_attribute_id").Comment("CI属性ID"),
        
        // 映射配置
        field.String("source_field").Comment("源字段名"),
        field.String("source_path").Optional().Comment("源字段路径(支持JSON路径)"),
        field.String("target_attribute").Comment("目标属性名"),
        
        // 转换配置
        field.String("transform_type").Default("direct").Comment("转换类型: direct, lookup, script, template"),
        field.JSON("transform_config", map[string]interface{}{}).Optional().Comment("转换配置JSON"),
        field.String("default_value").Optional().Comment("默认值"),
        
        // 验证配置
        field.JSON("validation_rules", map[string]interface{}{}).Optional().Comment("验证规则JSON"),
        field.String("validation_regex").Optional().Comment("验证正则表达式"),
        field.Bool("is_required").Default(false).Comment("是否必填"),
        field.Bool("is_unique").Default(false).Comment("是否唯一"),
        
        // 处理配置
        field.Int("priority").Default(100).Comment("映射优先级"),
        field.Bool("enabled").Default(true).Comment("是否启用"),
        field.String("update_strategy").Default("overwrite").Comment("更新策略: overwrite, merge, append"),
        
        // 统计信息
        field.Int64("success_count").Default(0).Comment("成功次数"),
        field.Int64("failed_count").Default(0).Comment("失败次数"),
        field.Time("last_success_at").Optional().Comment("上次成功时间"),
        
        // 租户字段
        field.Uint64("tenant_id").Comment("租户ID"),
    }
}

func (AttributeMappingRule) Edges() []ent.Edge {
    return []ent.Edge{
        // 关联发现配置
        edge.From("discovery_config", CiTypeDiscoveryConfig.Type).
            Ref("attribute_mapping_rules").
            Field("discovery_config_id").
            Unique().
            Required(),
            
        // 关联CI属性
        edge.From("ci_attribute", CiAttribute.Type).
            Ref("mapping_rules").
            Field("ci_attribute_id").
            Unique().
            Required(),
    }
}
```

#### 发现执行历史Entity: DiscoveryExecutionHistory

```go
// DiscoveryExecutionHistory 发现执行历史
func (DiscoveryExecutionHistory) Fields() []ent.Field {
    return []ent.Field{
        field.Uint64("id"),
        
        // 关联信息
        field.Uint64("discovery_config_id").Comment("发现配置ID"),
        field.String("execution_id").Comment("执行ID(UUID)"),
        
        // 执行信息
        field.String("trigger_type").Comment("触发类型: manual, scheduled, api"),
        field.Uint64("triggered_by").Optional().Comment("触发人ID"),
        field.Time("started_at").Comment("开始时间"),
        field.Time("completed_at").Optional().Comment("完成时间"),
        field.Int("duration_seconds").Optional().Comment("执行时长(秒)"),
        
        // 执行状态
        field.String("status").Comment("状态: running, completed, failed, cancelled"),
        field.String("stage").Optional().Comment("当前阶段: connecting, discovering, transforming, persisting"),
        field.Int("progress").Default(0).Comment("进度百分比(0-100)"),
        
        // 统计信息
        field.Int64("total_records").Default(0).Comment("总记录数"),
        field.Int64("processed_records").Default(0).Comment("已处理记录数"),
        field.Int64("success_records").Default(0).Comment("成功记录数"),
        field.Int64("failed_records").Default(0).Comment("失败记录数"),
        field.Int64("skipped_records").Default(0).Comment("跳过记录数"),
        field.Int64("created_cis").Default(0).Comment("创建的CI数量"),
        field.Int64("updated_cis").Default(0).Comment("更新的CI数量"),
        
        // 错误信息
        field.Text("error_message").Optional().Comment("错误信息"),
        field.JSON("error_details", map[string]interface{}{}).Optional().Comment("错误详情JSON"),
        field.JSON("validation_errors", []interface{}{}).Optional().Comment("验证错误JSON"),
        
        // 执行结果
        field.JSON("execution_result", map[string]interface{}{}).Optional().Comment("执行结果JSON"),
        field.JSON("performance_metrics", map[string]interface{}{}).Optional().Comment("性能指标JSON"),
        
        // 租户字段
        field.Uint64("tenant_id").Comment("租户ID"),
    }
}

func (DiscoveryExecutionHistory) Edges() []ent.Edge {
    return []ent.Edge{
        // 关联发现配置
        edge.From("discovery_config", CiTypeDiscoveryConfig.Type).
            Ref("execution_histories").
            Field("discovery_config_id").
            Unique().
            Required(),
    }
}
```

### 2.2 扩展现有模型

#### 扩展CiType模型

```go
// 在CiType.Edges()中添加
edge.To("discovery_configs", CiTypeDiscoveryConfig.Type),
```

#### 扩展CiAttribute模型

```go
// 在CiAttribute.Fields()中添加
field.Bool("discoverable").Default(false).Comment("是否可发现"),
field.String("discovery_source").Optional().Comment("发现源标识"),
field.JSON("discovery_metadata", map[string]interface{}{}).Optional().Comment("发现元数据JSON"),

// 在CiAttribute.Edges()中添加
edge.To("mapping_rules", AttributeMappingRule.Type),
```

## 3. 属性发现实现方案

### 3.1 发现引擎扩展

#### CI属性发现引擎

```go
type CiAttributeDiscoveryEngine struct {
    db               *ent.Client
    providerRegistry *provider.ProviderRegistry
    transformEngine  *transform.Engine
    conflictResolver *ConflictResolver
    logger           logx.Logger
}

type AttributeDiscoveryRequest struct {
    ConfigID    uint64                 `json:"config_id"`
    CiTypeID    uint64                 `json:"ci_type_id"`
    TenantID    uint64                 `json:"tenant_id"`
    UserID      uint64                 `json:"user_id"`
    TriggerType string                 `json:"trigger_type"` // manual, scheduled, api
    Options     map[string]interface{} `json:"options"`
}

type AttributeDiscoveryResult struct {
    ExecutionID      string                    `json:"execution_id"`
    Status           string                    `json:"status"`
    TotalRecords     int64                     `json:"total_records"`
    ProcessedRecords int64                     `json:"processed_records"`
    SuccessRecords   int64                     `json:"success_records"`
    FailedRecords    int64                     `json:"failed_records"`
    CreatedCIs       int64                     `json:"created_cis"`
    UpdatedCIs       int64                     `json:"updated_cis"`
    Errors           []string                  `json:"errors"`
    Warnings         []string                  `json:"warnings"`
    Metadata         map[string]interface{}    `json:"metadata"`
}
```

#### 核心执行逻辑

```go
func (e *CiAttributeDiscoveryEngine) ExecuteDiscovery(ctx context.Context, req *AttributeDiscoveryRequest) (*AttributeDiscoveryResult, error) {
    // 1. 获取发现配置
    config, err := e.db.CiTypeDiscoveryConfig.Get(ctx, req.ConfigID)
    if err != nil {
        return nil, fmt.Errorf("failed to get discovery config: %w", err)
    }
    
    // 2. 创建执行历史记录
    executionID := uuid.New().String()
    history, err := e.createExecutionHistory(ctx, config, executionID, req)
    if err != nil {
        return nil, fmt.Errorf("failed to create execution history: %w", err)
    }
    
    // 3. 获取Provider并执行数据发现
    provider, err := e.providerRegistry.Get(config.ProviderID)
    if err != nil {
        return nil, fmt.Errorf("provider not found: %s", config.ProviderID)
    }
    
    // 4. 执行数据发现
    discoveryResult, err := provider.DiscoverWithContext(ctx, config.ProviderConfig)
    if err != nil {
        e.updateExecutionHistory(ctx, history.ID, "failed", err.Error())
        return nil, fmt.Errorf("discovery failed: %w", err)
    }
    
    // 5. 获取属性映射规则
    mappingRules, err := e.db.AttributeMappingRule.Query().
        Where(attributemappingrule.DiscoveryConfigIDEQ(config.ID)).
        Where(attributemappingrule.EnabledEQ(true)).
        Order(ent.Asc(attributemappingrule.FieldPriority)).
        All(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get mapping rules: %w", err)
    }
    
    // 6. 处理每条发现的记录
    result := &AttributeDiscoveryResult{
        ExecutionID:  executionID,
        Status:       "completed",
        TotalRecords: discoveryResult.TotalRecords,
    }
    
    for _, record := range discoveryResult.Records {
        err := e.processRecord(ctx, config, mappingRules, record, result)
        if err != nil {
            result.FailedRecords++
            result.Errors = append(result.Errors, err.Error())
        } else {
            result.SuccessRecords++
        }
        result.ProcessedRecords++
    }
    
    // 7. 更新执行历史
    e.updateExecutionHistoryWithResult(ctx, history.ID, result)
    
    return result, nil
}
```

#### 记录处理逻辑

```go
func (e *CiAttributeDiscoveryEngine) processRecord(
    ctx context.Context,
    config *ent.CiTypeDiscoveryConfig,
    mappingRules []*ent.AttributeMappingRule,
    sourceRecord map[string]interface{},
    result *AttributeDiscoveryResult,
) error {
    // 1. 应用过滤条件
    if !e.applyFilterConditions(sourceRecord, config.FilterConditions) {
        result.SkippedRecords++
        return nil
    }
    
    // 2. 查找或创建CI实例
    ciInstance, isNew, err := e.findOrCreateCiInstance(ctx, config, sourceRecord, mappingRules)
    if err != nil {
        return fmt.Errorf("failed to find or create CI instance: %w", err)
    }
    
    if isNew {
        result.CreatedCIs++
    }
    
    // 3. 执行属性映射和更新
    updated, err := e.updateCiAttributes(ctx, ciInstance, sourceRecord, mappingRules)
    if err != nil {
        return fmt.Errorf("failed to update CI attributes: %w", err)
    }
    
    if updated && !isNew {
        result.UpdatedCIs++
    }
    
    return nil
}
```

#### 属性更新逻辑

```go
func (e *CiAttributeDiscoveryEngine) updateCiAttributes(
    ctx context.Context,
    ciInstance *ent.CiInstance,
    sourceRecord map[string]interface{},
    mappingRules []*ent.AttributeMappingRule,
) (bool, error) {
    updated := false
    
    // 获取CI实例当前属性
    currentAttributes, err := e.getCiInstanceAttributes(ctx, ciInstance.ID)
    if err != nil {
        return false, err
    }
    
    // 准备属性更新
    attributeUpdates := make(map[string]interface{})
    
    // 处理每个映射规则
    for _, rule := range mappingRules {
        // 1. 提取源字段值
        sourceValue, exists := e.extractSourceValue(sourceRecord, rule)
        if !exists && rule.IsRequired {
            return false, fmt.Errorf("required source field not found: %s", rule.SourceField)
        }
        
        if !exists && rule.DefaultValue != nil {
            sourceValue = rule.DefaultValue
        }
        
        if !exists {
            continue
        }
        
        // 2. 执行数据转换
        transformedValue, err := e.transformValue(sourceValue, rule)
        if err != nil {
            return false, fmt.Errorf("transform failed for field %s: %w", rule.SourceField, err)
        }
        
        // 3. 执行数据验证
        if err := e.validateValue(transformedValue, rule); err != nil {
            return false, fmt.Errorf("validation failed for field %s: %w", rule.TargetAttribute, err)
        }
        
        // 4. 检查是否需要更新
        currentValue := currentAttributes[rule.TargetAttribute]
        if e.shouldUpdateAttribute(currentValue, transformedValue, rule) {
            attributeUpdates[rule.TargetAttribute] = transformedValue
            updated = true
        }
    }
    
    // 5. 批量更新属性
    if len(attributeUpdates) > 0 {
        err := e.batchUpdateAttributes(ctx, ciInstance.ID, attributeUpdates)
        if err != nil {
            return false, fmt.Errorf("failed to update attributes: %w", err)
        }
    }
    
    return updated, nil
}
```

### 3.2 冲突解决策略

#### 冲突解决器

```go
type ConflictResolver struct {
    strategy string // priority, merge, latest
}

func (r *ConflictResolver) ResolveConflict(
    currentValue interface{},
    newValue interface{},
    rule *ent.AttributeMappingRule,
    strategy string,
) (interface{}, error) {
    switch strategy {
    case "priority":
        // 基于映射规则优先级
        return r.resolveByCiTypePriority(currentValue, newValue, rule)
    case "merge":
        // 合并值（适用于数组、对象类型）
        return r.mergeValues(currentValue, newValue)
    case "latest":
        // 总是使用最新值
        return newValue, nil
    case "manual":
        // 需要人工确认的冲突
        return r.createConflictRecord(currentValue, newValue, rule)
    default:
        return newValue, nil
    }
}

func (r *ConflictResolver) resolveByCiTypePriority(
    currentValue interface{},
    newValue interface{},
    rule *ent.AttributeMappingRule,
) (interface{}, error) {
    // 如果当前值为空，使用新值
    if currentValue == nil {
        return newValue, nil
    }
    
    // 检查当前值的来源优先级
    // 这里需要查询当前值是由哪个发现配置设置的
    // 如果新值的优先级更高，则更新
    
    return newValue, nil
}
```

### 3.3 增量更新机制

#### 变更检测

```go
type AttributeChangeDetector struct {
    db     *ent.Client
    hasher hash.Hash
}

func (d *AttributeChangeDetector) DetectChanges(
    ctx context.Context,
    ciInstanceID uint64,
    newAttributes map[string]interface{},
) (*AttributeChanges, error) {
    // 1. 获取当前属性
    currentAttributes, err := d.getCiInstanceAttributes(ctx, ciInstanceID)
    if err != nil {
        return nil, err
    }
    
    changes := &AttributeChanges{
        CiInstanceID: ciInstanceID,
        Added:        make(map[string]interface{}),
        Updated:      make(map[string]AttributeChange),
        Removed:      make([]string, 0),
    }
    
    // 2. 检测新增和更新
    for key, newValue := range newAttributes {
        if currentValue, exists := currentAttributes[key]; exists {
            if !d.valuesEqual(currentValue, newValue) {
                changes.Updated[key] = AttributeChange{
                    OldValue: currentValue,
                    NewValue: newValue,
                }
            }
        } else {
            changes.Added[key] = newValue
        }
    }
    
    // 3. 检测删除的属性（可选）
    for key := range currentAttributes {
        if _, exists := newAttributes[key]; !exists {
            changes.Removed = append(changes.Removed, key)
        }
    }
    
    return changes, nil
}

type AttributeChanges struct {
    CiInstanceID uint64                         `json:"ci_instance_id"`
    Added        map[string]interface{}         `json:"added"`
    Updated      map[string]AttributeChange     `json:"updated"`
    Removed      []string                       `json:"removed"`
}

type AttributeChange struct {
    OldValue interface{} `json:"old_value"`
    NewValue interface{} `json:"new_value"`
}
```

## 4. API接口设计

### 4.1 CIType发现配置API

```go
// CreateCiTypeDiscoveryConfig 创建CI类型发现配置
func (l *CreateCiTypeDiscoveryConfigLogic) CreateCiTypeDiscoveryConfig(
    req *types.CreateCiTypeDiscoveryConfigReq,
) (resp *types.CiTypeDiscoveryConfigInfo, err error) {
    // 1. 验证CI类型是否存在
    ciType, err := l.svcCtx.DB.CiType.Get(l.ctx, req.CiTypeId)
    if err != nil {
        return nil, fmt.Errorf("CI type not found: %w", err)
    }
    
    // 2. 验证Provider是否存在
    provider, err := l.svcCtx.ProviderRegistry.Get(req.ProviderId)
    if err != nil {
        return nil, fmt.Errorf("provider not found: %s", req.ProviderId)
    }
    
    // 3. 验证Provider配置
    err = provider.ValidateConfigWithContext(l.ctx, req.ProviderConfig)
    if err != nil {
        return nil, fmt.Errorf("invalid provider config: %w", err)
    }
    
    // 4. 创建发现配置
    config, err := l.svcCtx.DB.CiTypeDiscoveryConfig.Create().
        SetCiTypeID(req.CiTypeId).
        SetConfigName(req.ConfigName).
        SetDescription(req.Description).
        SetDiscoveryMode(req.DiscoveryMode).
        SetDiscoveryType(req.DiscoveryType).
        SetProviderID(req.ProviderId).
        SetProviderConfig(req.ProviderConfig).
        SetAttributeMappings(req.AttributeMappings).
        SetDiscoveryRules(req.DiscoveryRules).
        SetFilterConditions(req.FilterConditions).
        SetExecutionMode(req.ExecutionMode).
        SetScheduleConfig(req.ScheduleConfig).
        SetPriority(req.Priority).
        SetBatchSize(req.BatchSize).
        SetTimeoutSeconds(req.TimeoutSeconds).
        SetConflictResolution(req.ConflictResolution).
        SetAutoCreateCi(req.AutoCreateCi).
        SetAutoUpdateAttributes(req.AutoUpdateAttributes).
        SetNotificationConfig(req.NotificationConfig).
        SetTenantID(l.svcCtx.TenantID).
        SetCreatedBy(l.svcCtx.UserID).
        Save(l.ctx)
    if err != nil {
        return nil, dberrorhandler.DefaultEntError(l.Logger, err, req)
    }
    
    // 5. 创建属性映射规则
    for _, mapping := range req.AttributeMappings {
        _, err := l.svcCtx.DB.AttributeMappingRule.Create().
            SetDiscoveryConfigID(config.ID).
            SetCiAttributeID(mapping.CiAttributeId).
            SetSourceField(mapping.SourceField).
            SetSourcePath(mapping.SourcePath).
            SetTargetAttribute(mapping.TargetAttribute).
            SetTransformType(mapping.TransformType).
            SetTransformConfig(mapping.TransformConfig).
            SetDefaultValue(mapping.DefaultValue).
            SetValidationRules(mapping.ValidationRules).
            SetValidationRegex(mapping.ValidationRegex).
            SetIsRequired(mapping.IsRequired).
            SetIsUnique(mapping.IsUnique).
            SetPriority(mapping.Priority).
            SetEnabled(mapping.Enabled).
            SetUpdateStrategy(mapping.UpdateStrategy).
            SetTenantID(l.svcCtx.TenantID).
            Save(l.ctx)
        if err != nil {
            return nil, fmt.Errorf("failed to create mapping rule: %w", err)
        }
    }
    
    return l.convertToResponse(config), nil
}

// TriggerDiscovery 触发发现执行
func (l *TriggerDiscoveryLogic) TriggerDiscovery(
    req *types.TriggerDiscoveryReq,
) (resp *types.TriggerDiscoveryResp, err error) {
    // 1. 获取发现配置
    config, err := l.svcCtx.DB.CiTypeDiscoveryConfig.Get(l.ctx, req.ConfigId)
    if err != nil {
        return nil, fmt.Errorf("discovery config not found: %w", err)
    }
    
    // 2. 检查配置是否启用
    if !config.Enabled {
        return nil, fmt.Errorf("discovery config is disabled")
    }
    
    // 3. 创建发现任务
    discoveryRequest := &AttributeDiscoveryRequest{
        ConfigID:    req.ConfigId,
        CiTypeID:    config.CiTypeID,
        TenantID:    l.svcCtx.TenantID,
        UserID:      l.svcCtx.UserID,
        TriggerType: "manual",
        Options:     req.Options,
    }
    
    // 4. 异步执行发现任务
    go func() {
        result, err := l.svcCtx.DiscoveryEngine.ExecuteDiscovery(context.Background(), discoveryRequest)
        if err != nil {
            l.Logger.Errorw("Discovery execution failed",
                logx.Field("config_id", req.ConfigId),
                logx.Field("error", err))
        } else {
            l.Logger.Infow("Discovery execution completed",
                logx.Field("config_id", req.ConfigId),
                logx.Field("execution_id", result.ExecutionID),
                logx.Field("total_records", result.TotalRecords),
                logx.Field("success_records", result.SuccessRecords))
        }
    }()
    
    return &types.TriggerDiscoveryResp{
        TaskId:  fmt.Sprintf("discovery_%d_%d", req.ConfigId, time.Now().Unix()),
        Message: "Discovery task has been queued",
    }, nil
}
```

### 4.2 前端API定义

```typescript
// types/discovery.ts
export interface CiTypeDiscoveryConfig {
  id?: number;
  ci_type_id: number;
  config_name: string;
  description?: string;
  discovery_mode: 'attribute' | 'relation' | 'dependency';
  discovery_type: 'file' | 'api' | 'sdk' | 'builtin' | 'agent';
  provider_id: string;
  provider_config: Record<string, any>;
  attribute_mappings: AttributeMapping[];
  discovery_rules: Record<string, any>;
  filter_conditions: Record<string, any>;
  execution_mode: 'manual' | 'scheduled' | 'triggered';
  schedule_config?: Record<string, any>;
  priority: number;
  batch_size: number;
  timeout_seconds: number;
  conflict_resolution: 'priority' | 'merge' | 'latest' | 'manual';
  auto_create_ci: boolean;
  auto_update_attributes: boolean;
  notification_config?: Record<string, any>;
  enabled: boolean;
  status: 'active' | 'inactive' | 'error';
  last_executed_at?: string;
  next_execution_at?: string;
  execution_stats?: Record<string, any>;
  last_error?: string;
}

export interface AttributeMapping {
  id?: number;
  ci_attribute_id: number;
  source_field: string;
  source_path?: string;
  target_attribute: string;
  transform_type: 'direct' | 'lookup' | 'script' | 'template';
  transform_config?: Record<string, any>;
  default_value?: string;
  validation_rules?: Record<string, any>;
  validation_regex?: string;
  is_required: boolean;
  is_unique: boolean;
  priority: number;
  enabled: boolean;
  update_strategy: 'overwrite' | 'merge' | 'append';
}

// api/discovery.ts
export namespace CiTypeDiscoveryAPI {
  // 创建发现配置
  export function createDiscoveryConfig(data: CiTypeDiscoveryConfig) {
    return requestClient.postWithMsg<CiTypeDiscoveryConfig>(
      '/cmdb-api/ci_type_discovery_config/create',
      data
    );
  }

  // 获取CI类型的发现配置列表
  export function getDiscoveryConfigs(ciTypeId: number) {
    return requestClient.post<CiTypeDiscoveryConfig[]>(
      '/cmdb-api/ci_type_discovery_config/list',
      { ci_type_id: ciTypeId }
    );
  }

  // 触发发现执行
  export function triggerDiscovery(configId: number, options?: Record<string, any>) {
    return requestClient.postWithMsg<{ task_id: string; message: string }>(
      '/cmdb-api/ci_type_discovery_config/trigger',
      { config_id: configId, options }
    );
  }

  // 获取执行历史
  export function getExecutionHistory(configId: number, page = 1, pageSize = 20) {
    return requestClient.post<{
      list: DiscoveryExecutionHistory[];
      total: number;
    }>('/cmdb-api/discovery_execution_history/list', {
      discovery_config_id: configId,
      page,
      page_size: pageSize,
    });
  }

  // 获取执行状态
  export function getExecutionStatus(executionId: string) {
    return requestClient.post<DiscoveryExecutionHistory>(
      '/cmdb-api/discovery_execution_history/get',
      { execution_id: executionId }
    );
  }
}
```

## 5. 前端UI组件扩展

### 5.1 CIType发现配置管理界面

```vue
<!-- CiTypeDiscoveryManager.vue -->
<template>
  <div class="ci-type-discovery-manager">
    <!-- 头部操作栏 -->
    <div class="header-actions">
      <Button type="primary" @click="showCreateModal = true">
        <PlusOutlined /> 添加发现配置
      </Button>
      <Button @click="refreshConfigs">
        <ReloadOutlined /> 刷新
      </Button>
    </div>

    <!-- 发现配置列表 -->
    <Table
      :columns="configColumns"
      :dataSource="discoveryConfigs"
      :loading="loading"
      rowKey="id"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status'">
          <Tag :color="getStatusColor(record.status)">
            {{ getStatusText(record.status) }}
          </Tag>
        </template>
        
        <template v-if="column.key === 'execution_stats'">
          <div class="stats-info">
            <div>成功: {{ record.execution_stats?.success_records || 0 }}</div>
            <div>失败: {{ record.execution_stats?.failed_records || 0 }}</div>
          </div>
        </template>
        
        <template v-if="column.key === 'actions'">
          <Space>
            <Button size="small" @click="triggerDiscovery(record.id)">
              执行
            </Button>
            <Button size="small" @click="editConfig(record)">
              编辑
            </Button>
            <Button size="small" @click="viewHistory(record.id)">
              历史
            </Button>
            <Popconfirm
              title="确定删除此配置吗？"
              @confirm="deleteConfig(record.id)"
            >
              <Button size="small" danger>删除</Button>
            </Popconfirm>
          </Space>
        </template>
      </template>
    </Table>

    <!-- 创建/编辑配置模态框 -->
    <DiscoveryConfigModal
      v-model:visible="showCreateModal"
      :ci-type-id="ciTypeId"
      :config="editingConfig"
      @success="handleConfigSaved"
    />

    <!-- 执行历史模态框 -->
    <ExecutionHistoryModal
      v-model:visible="showHistoryModal"
      :config-id="selectedConfigId"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { Button, Table, Tag, Space, Popconfirm, message } from 'ant-design-vue';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons-vue';
import { CiTypeDiscoveryAPI } from '@/api/discovery';
import type { CiTypeDiscoveryConfig } from '@/types/discovery';

const props = defineProps<{
  ciTypeId: number;
}>();

const discoveryConfigs = ref<CiTypeDiscoveryConfig[]>([]);
const loading = ref(false);
const showCreateModal = ref(false);
const showHistoryModal = ref(false);
const editingConfig = ref<CiTypeDiscoveryConfig | null>(null);
const selectedConfigId = ref<number | null>(null);

const configColumns = [
  {
    title: '配置名称',
    dataIndex: 'config_name',
    key: 'config_name',
  },
  {
    title: '发现类型',
    dataIndex: 'discovery_type',
    key: 'discovery_type',
  },
  {
    title: '提供者',
    dataIndex: 'provider_id',
    key: 'provider_id',
  },
  {
    title: '执行模式',
    dataIndex: 'execution_mode',
    key: 'execution_mode',
  },
  {
    title: '状态',
    dataIndex: 'status',
    key: 'status',
  },
  {
    title: '执行统计',
    dataIndex: 'execution_stats',
    key: 'execution_stats',
  },
  {
    title: '上次执行',
    dataIndex: 'last_executed_at',
    key: 'last_executed_at',
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
  },
];

const loadDiscoveryConfigs = async () => {
  loading.value = true;
  try {
    const res = await CiTypeDiscoveryAPI.getDiscoveryConfigs(props.ciTypeId);
    discoveryConfigs.value = res.data || [];
  } catch (error) {
    message.error('加载发现配置失败');
  } finally {
    loading.value = false;
  }
};

const triggerDiscovery = async (configId: number) => {
  try {
    const res = await CiTypeDiscoveryAPI.triggerDiscovery(configId);
    message.success(res.msg || '发现任务已启动');
    // 刷新配置状态
    setTimeout(() => {
      loadDiscoveryConfigs();
    }, 1000);
  } catch (error) {
    message.error('启动发现任务失败');
  }
};

const editConfig = (config: CiTypeDiscoveryConfig) => {
  editingConfig.value = config;
  showCreateModal.value = true;
};

const viewHistory = (configId: number) => {
  selectedConfigId.value = configId;
  showHistoryModal.value = true;
};

const deleteConfig = async (configId: number) => {
  try {
    await CiTypeDiscoveryAPI.deleteDiscoveryConfig(configId);
    message.success('删除成功');
    loadDiscoveryConfigs();
  } catch (error) {
    message.error('删除失败');
  }
};

const handleConfigSaved = () => {
  showCreateModal.value = false;
  editingConfig.value = null;
  loadDiscoveryConfigs();
};

const refreshConfigs = () => {
  loadDiscoveryConfigs();
};

const getStatusColor = (status: string) => {
  switch (status) {
    case 'active': return 'green';
    case 'inactive': return 'gray';
    case 'error': return 'red';
    default: return 'blue';
  }
};

const getStatusText = (status: string) => {
  switch (status) {
    case 'active': return '活跃';
    case 'inactive': return '非活跃';
    case 'error': return '错误';
    default: return status;
  }
};

onMounted(() => {
  loadDiscoveryConfigs();
});
</script>
```

### 5.2 属性映射配置组件

```vue
<!-- AttributeMappingEditor.vue -->
<template>
  <div class="attribute-mapping-editor">
    <Row :gutter="16">
      <!-- 左侧: CI属性列表 -->
      <Col :span="8">
        <Card title="CI类型属性" size="small">
          <div class="attribute-list">
            <div
              v-for="attr in ciAttributes"
              :key="attr.id"
              :class="['attribute-item', { mapped: isMapped(attr.id) }]"
              @click="selectCiAttribute(attr)"
            >
              <div class="attr-name">{{ attr.name }}</div>
              <div class="attr-type">{{ attr.data_type }}</div>
              <div v-if="attr.is_required" class="attr-required">必填</div>
            </div>
          </div>
        </Card>
      </Col>

      <!-- 中间: 映射配置 -->
      <Col :span="8">
        <Card title="属性映射规则" size="small">
          <div class="mapping-list">
            <div
              v-for="(mapping, index) in mappings"
              :key="mapping.tempId || mapping.id"
              class="mapping-item"
            >
              <Card size="small" :title="`映射 ${index + 1}`">
                <template #extra>
                  <Button
                    size="small"
                    type="text"
                    danger
                    @click="removeMapping(index)"
                  >
                    删除
                  </Button>
                </template>

                <Form layout="vertical" :model="mapping">
                  <Row :gutter="8">
                    <Col :span="12">
                      <FormItem label="源字段">
                        <Select
                          v-model:value="mapping.source_field"
                          placeholder="选择源字段"
                          @change="updateMapping(index, mapping)"
                        >
                          <SelectOption
                            v-for="field in sourceFields"
                            :key="field.name"
                            :value="field.name"
                          >
                            {{ field.display_name || field.name }}
                          </SelectOption>
                        </Select>
                      </FormItem>
                    </Col>
                    <Col :span="12">
                      <FormItem label="目标属性">
                        <Select
                          v-model:value="mapping.ci_attribute_id"
                          placeholder="选择CI属性"
                          @change="updateMapping(index, mapping)"
                        >
                          <SelectOption
                            v-for="attr in ciAttributes"
                            :key="attr.id"
                            :value="attr.id"
                          >
                            {{ attr.name }}
                          </SelectOption>
                        </Select>
                      </FormItem>
                    </Col>
                  </Row>

                  <Row :gutter="8">
                    <Col :span="12">
                      <FormItem label="转换类型">
                        <Select
                          v-model:value="mapping.transform_type"
                          @change="updateMapping(index, mapping)"
                        >
                          <SelectOption value="direct">直接映射</SelectOption>
                          <SelectOption value="lookup">查找表</SelectOption>
                          <SelectOption value="script">脚本转换</SelectOption>
                          <SelectOption value="template">模板转换</SelectOption>
                        </Select>
                      </FormItem>
                    </Col>
                    <Col :span="12">
                      <FormItem label="更新策略">
                        <Select
                          v-model:value="mapping.update_strategy"
                          @change="updateMapping(index, mapping)"
                        >
                          <SelectOption value="overwrite">覆盖</SelectOption>
                          <SelectOption value="merge">合并</SelectOption>
                          <SelectOption value="append">追加</SelectOption>
                        </Select>
                      </FormItem>
                    </Col>
                  </Row>

                  <Row :gutter="8">
                    <Col :span="12">
                      <FormItem>
                        <Checkbox
                          v-model:checked="mapping.is_required"
                          @change="updateMapping(index, mapping)"
                        >
                          必填字段
                        </Checkbox>
                      </FormItem>
                    </Col>
                    <Col :span="12">
                      <FormItem>
                        <Checkbox
                          v-model:checked="mapping.enabled"
                          @change="updateMapping(index, mapping)"
                        >
                          启用映射
                        </Checkbox>
                      </FormItem>
                    </Col>
                  </Row>

                  <!-- 高级配置 -->
                  <Collapse>
                    <CollapsePanel header="高级配置">
                      <FormItem label="默认值">
                        <Input
                          v-model:value="mapping.default_value"
                          placeholder="当源字段为空时使用的默认值"
                          @change="updateMapping(index, mapping)"
                        />
                      </FormItem>

                      <FormItem label="验证正则">
                        <Input
                          v-model:value="mapping.validation_regex"
                          placeholder="数据验证正则表达式"
                          @change="updateMapping(index, mapping)"
                        />
                      </FormItem>

                      <FormItem label="优先级">
                        <InputNumber
                          v-model:value="mapping.priority"
                          :min="1"
                          :max="999"
                          @change="updateMapping(index, mapping)"
                        />
                      </FormItem>
                    </CollapsePanel>
                  </Collapse>
                </Form>
              </Card>
            </div>

            <!-- 添加映射按钮 -->
            <Button
              type="dashed"
              block
              @click="addMapping"
              style="margin-top: 16px"
            >
              <PlusOutlined /> 添加映射规则
            </Button>
          </div>
        </Card>
      </Col>

      <!-- 右侧: 源字段列表 -->
      <Col :span="8">
        <Card title="发现字段" size="small">
          <div class="source-fields-list">
            <div
              v-for="field in sourceFields"
              :key="field.name"
              :class="['field-item', { mapped: isSourceFieldMapped(field.name) }]"
              @click="selectSourceField(field)"
            >
              <div class="field-name">{{ field.display_name || field.name }}</div>
              <div class="field-type">{{ field.type }}</div>
              <div v-if="field.required" class="field-required">必填</div>
            </div>
          </div>
        </Card>
      </Col>
    </Row>
  </div>
</template>

<script setup lang="ts">
// ... 组件实现代码
</script>
```

## 6. 技术实现路线图

### Phase 1: 基础架构 (2-3周)
1. 创建数据模型Schema
2. 实现核心Entity和Edge
3. 生成数据库迁移脚本
4. 实现基础API接口

### Phase 2: 发现引擎 (3-4周)
1. 实现CiAttributeDiscoveryEngine
2. 集成现有ProviderRegistry
3. 实现属性映射和转换逻辑
4. 实现冲突解决机制

### Phase 3: 前端界面 (2-3周)
1. 实现CIType发现配置管理界面
2. 实现属性映射配置编辑器
3. 实现执行历史和监控界面
4. 集成到现有CMDB前端

### Phase 4: 测试和优化 (2周)
1. 单元测试和集成测试
2. 性能优化和调优
3. 用户体验优化
4. 文档编写

### Phase 5: 扩展功能 (未来)
1. 关系发现
2. 依赖发现
3. 智能推荐
4. 自动化运维集成

## 7. 关键技术要点

### 7.1 数据一致性保证
- 使用数据库事务确保原子性
- 实现乐观锁避免并发冲突
- 版本控制机制跟踪变更

### 7.2 性能优化策略
- 批量处理减少数据库访问
- 连接池管理提高效率
- 缓存机制减少重复计算
- 异步处理提升响应速度

### 7.3 扩展性设计
- 插件化Provider架构
- 可配置的转换引擎
- 灵活的映射规则系统
- 模块化的前端组件

### 7.4 监控和可观测性
- 详细的执行历史记录
- 实时性能指标收集
- 错误追踪和告警
- 审计日志完整记录

这套设计方案为CIType自动发现配置提供了完整的解决方案，既保持了与现有系统的兼容性，又具备了良好的扩展性和可维护性。通过分阶段实施，可以逐步完善功能，最终实现强大的自动化CI管理能力。