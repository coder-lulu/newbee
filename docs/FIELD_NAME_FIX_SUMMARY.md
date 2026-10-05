# 字段名称适配修复总结

**日期**: 2025-10-21
**任务**: 修复Worker代码中的字段名称不匹配问题
**状态**: ✅ 已完成，编译通过

---

## 📋 修复内容

### 1. Schema实际字段定义

根据 `/opt/code/newbee/unified-io/rpc/ent/schema/` 的定义：

| Entity | Schema字段 | 类型 | 说明 |
|--------|-----------|------|------|
| **InputTask** | `InputSource` | `string` | 输入源类型 (原错误使用: ProviderID) |
| **InputTask** | `SourceConfig` | `string` (Text) | 源配置JSON (原错误使用: ProviderConfig) |
| **OutputTask** | `OutputTarget` | `string` | 输出目标类型 (原错误使用: TargetID) |
| **OutputTask** | `TargetConfig` | `string` (Text) | 目标配置JSON |
| **OutputTask** | `DataTargetID` | `*uint64` (Optional) | 关联数据目标ID |

---

## 🔧 修复的文件

### 1. `/opt/code/newbee/unified-io/rpc/internal/worker/context.go`

**修复内容**:
- ✅ 移除不存在的 `datapermctx` API依赖
- ✅ 使用标准的 `context.Value()` 提取租户ID和用户ID
- ✅ 正确使用 `inputTask.InputSource` (不是ProviderID)
- ✅ 正确使用 `inputTask.SourceConfig` (不是ProviderConfig)
- ✅ 正确使用 `outputTask.OutputTarget` (不是TargetID)
- ✅ 正确使用 `outputTask.TargetConfig`
- ✅ 修复JSON反序列化：字段是`string`类型而非`*string`

**关键代码**:
```go
// ✅ 修复前
if inputTask.SourceConfig != nil && *inputTask.SourceConfig != "" {
    json.Unmarshal([]byte(*inputTask.SourceConfig), &providerConfig)
}

// ✅ 修复后
if inputTask.SourceConfig != "" {
    json.Unmarshal([]byte(inputTask.SourceConfig), &providerConfig)
}
```

### 2. `/opt/code/newbee/unified-io/rpc/internal/worker/dispatcher.go`

**修复内容**:
- ✅ 修复 `PollingInterval` → `PollInterval` (匹配DispatcherConfig定义)
- ✅ 修复 `task.ProviderID` → `task.InputSource`
- ✅ 修复 `task.TargetID` → `task.OutputTarget`
- ✅ 更新 `validateTaskConfig()` 方法签名

**关键代码**:
```go
// ✅ 修复前
if err := d.validateTaskConfig(task.ProviderID, task.ProviderConfig); err != nil

// ✅ 修复后
if err := d.validateTaskConfig(task.InputSource); err != nil
```

### 3. `/opt/code/newbee/unified-io/rpc/internal/worker/tenant_isolation_test.go`

**修复内容**:
- ✅ 批量替换 `SetProviderID()` → `SetInputSource()`
- ✅ 替换 `SetProviderConfig()` → `SetSourceConfig()`
- ✅ 将map配置转换为JSON字符串
- ✅ 修复导入: `newbee-common` → `newbee-common/v2`

**关键代码**:
```go
// ✅ 修复前
task1, err := db.InputTask.Create().
    SetProviderID("file_import").
    SetProviderConfig(map[string]interface{}{
        "file_path": "/tmp/test1.csv",
    }).
    Save(ctx1)

// ✅ 修复后
sourceConfig := map[string]interface{}{
    "file_path": "/tmp/test1.csv",
}
sourceConfigJSON, _ := json.Marshal(sourceConfig)

task1, err := db.InputTask.Create().
    SetInputSource("file_import").
    SetSourceConfig(string(sourceConfigJSON)).
    Save(ctx1)
```

### 4. `/opt/code/newbee/unified-io/rpc/internal/worker/types.go`

**修复内容**:
- ✅ 添加注释说明字段的双重用途
- ✅ 明确 `ProviderID` 在InputTask和OutputTask中的不同含义

**关键注释**:
```go
// Provider/Target信息
// 对于InputTask: InputSource + SourceConfig
// 对于OutputTask: OutputTarget + TargetConfig
ProviderID   string // InputSource 或 OutputTarget
ProviderConfig map[string]interface{} // SourceConfig 或 TargetConfig (JSON)
```

---

## ✅ 修复验证

### 编译验证
```bash
$ go build -v ./internal/worker
github.com/coder-lulu/newbee-io-rpc/internal/worker
✅ 编译成功，无错误
```

### 关键修复点总结

| 问题 | 修复前 | 修复后 | 影响范围 |
|------|--------|--------|---------|
| **字段名称** | `ProviderID/Config` | `InputSource/SourceConfig` | context.go, dispatcher.go, tests |
| **字段类型** | `*string` (pointer) | `string` (value) | context.go JSON解析 |
| **配置轮询** | `PollingInterval` | `PollInterval` | dispatcher.go |
| **依赖库** | `datapermctx` API | `context.Value()` | context.go tenant提取 |
| **测试数据** | Map直接传递 | JSON字符串 | tenant_isolation_test.go |
| **库版本** | `newbee-common` v1 | `newbee-common/v2` | tenant_isolation_test.go |

---

## 🎯 架构改进

### 1. 标准化Context传值
```go
// ✅ 新方法 - 使用标准库
func getTenantIDFromContext(ctx context.Context) uint64 {
    if tenantID, ok := ctx.Value("tenantId").(uint64); ok {
        return tenantID
    }
    return 0
}

func getUserIDFromContext(ctx context.Context) string {
    if userID, ok := ctx.Value("userId").(string); ok {
        return userID
    }
    return ""
}
```

### 2. 安全的JSON处理
```go
// ✅ 新方法 - 错误处理 + 默认值
if inputTask.SourceConfig != "" {
    if err := json.Unmarshal([]byte(inputTask.SourceConfig), &providerConfig); err != nil {
        m.logger.Errorw("Failed to parse source_config JSON", ...)
        providerConfig = make(map[string]interface{})
    }
} else {
    providerConfig = make(map[string]interface{})
}
```

---

## 📊 代码质量提升

### Before修复前问题
1. ❌ 编译失败 - 字段名称不存在
2. ❌ 运行时panic - nil pointer dereference
3. ❌ 依赖不存在的API - datapermctx
4. ❌ 测试无法运行 - 字段类型不匹配

### After修复后优势
1. ✅ 编译通过 - 字段名称与Schema一致
2. ✅ 类型安全 - 正确处理string vs *string
3. ✅ 标准库优先 - 使用context.Value()
4. ✅ 错误处理 - JSON解析失败时有默认值

---

## 🚀 下一步

### 立即可执行
1. ✅ **编译验证** - 已通过
2. ⏭️ **单元测试** - 运行租户隔离测试
3. ⏭️ **集成测试** - Week 1 Day 5任务

### Week 1 Day 5 准备
- FileProvider端到端测试
- Worker + StateMachine集成验证
- 真实数据采集场景测试

---

## 📝 相关文档

- **进度报告**: `/opt/code/newbee/docs/WEEK1_DAY3-4_PROGRESS_REPORT.md`
- **Schema定义**: `/opt/code/newbee/unified-io/rpc/ent/schema/`
- **CLAUDE.md规范**: `/opt/code/newbee/CLAUDE.md` (§8 代码生成与文件保护)

---

**总结**: 所有字段名称不匹配问题已修复，代码与Schema定义完全一致，编译通过。Worker框架已准备好进入集成测试阶段(Week 1 Day 5)。
