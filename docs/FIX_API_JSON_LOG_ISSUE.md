# 修复API服务JSON日志问题

## 🎯 问题描述

用户反馈：Core API服务仍然输出JSON格式日志，而不是彩色格式。

```json
{"@timestamp":"2025-10-07T22:56:09.357+08:00","caller":"audit/core_client_adapter.go:51","content":"Core RPC CreateAuditLog failed",...}
```

---

## 🔍 根本原因

**问题**：`conf.MustLoad()` 在加载配置时会**自动初始化logx**，此时彩色Writer还未设置。

**原代码执行顺序**：
```go
conf.MustLoad() → 初始化logx(JSON格式)
       ↓
logwriter.EnableColored() → 设置彩色Writer（已太晚）
       ↓
后续日志输出 → 仍然是JSON格式
```

---

## ✅ 解决方案

### 关键修改

**必须在 `conf.MustLoad()` 之后立即调用彩色日志设置**，在任何其他组件初始化之前。

### 修改后的代码

#### Core API (`core/api/core.go`)

```go
func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// ✅ 关键：必须在conf.MustLoad之后立即调用
	logwriter.EnableColoredLoggingForDevMode(c.Mode)

	server := rest.MustNewServer(c.RestConf, rest.WithCors(c.CROSConf.Address))
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	// ...
}
```

#### Unified-IO API (`unified-io/api/io.go`)

```go
func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// ✅ 关键：必须在conf.MustLoad之后立即调用
	logwriter.EnableColoredLoggingForDevMode(c.Mode)

	server := rest.MustNewServer(c.RestConf, rest.WithCors(c.CROSConf.Address))
	// ...
}
```

#### Core RPC (`core/rpc/core.go`)

```go
func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// ✅ 关键：必须在conf.MustLoad之后立即调用
	logwriter.EnableColoredLoggingForDevMode(c.Mode)

	ctx := svc.NewServiceContext(c)
	// ...
}
```

#### Unified-IO RPC (`unified-io/rpc/io.go`)

```go
func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// ✅ 关键：必须在conf.MustLoad之后立即调用
	logwriter.EnableColoredLoggingForDevMode(c.Mode)

	ctx := svc.NewServiceContext(c)
	// ...
}
```

---

## 📋 代码修改清单

| 服务 | 文件 | 修改内容 | 状态 |
|------|------|---------|------|
| Core API | `core/api/core.go` | 调整logwriter调用位置 | ✅ 已修改 |
| IO API | `unified-io/api/io.go` | 调整logwriter调用位置 | ✅ 已修改 |
| Core RPC | `core/rpc/core.go` | 调整logwriter调用位置 | ✅ 已修改 |
| IO RPC | `unified-io/rpc/io.go` | 调整logwriter调用位置 | ✅ 已修改 |

---

## 🚀 验证步骤

### 1. 停止当前运行的服务

```bash
# 如果服务正在运行，先停止
# 使用 Ctrl+C 或者找到进程并kill
pkill -f "core.api"
pkill -f "core.rpc"
```

### 2. 重新编译服务

```bash
# Core API
cd /opt/code/newbee/core/api
go build -v .

# Core RPC
cd /opt/code/newbee/core/rpc
go build -v .
```

### 3. 启动服务并验证

```bash
# 启动Core API
cd /opt/code/newbee/core/api
./core &

# 或使用go run
go run core.go
```

### 4. 观察日志输出

**预期输出（彩色格式）**：
```
[23:10:15] INFO  svc/service_context.go:50 | ✅ Core service initialized
[23:10:15] INFO  hooks/unified_hook.go:369 | Registered mutation hook | field_type=tenant_id
[23:10:16] ERROR user/login.go:42 | Login failed | error="invalid credentials"
```

**如果仍是JSON格式**：
```json
{"@timestamp":"...","level":"info",...}
```

说明需要：
1. 确认代码修改已保存
2. 确认服务已重启（而不是重新加载）
3. 检查配置文件Mode是否为dev

---

## ⚙️ 配置检查

### 确认配置文件

```bash
# 检查Mode配置
cat /opt/code/newbee/core/api/etc/core.yaml | grep "^Mode:"
# 应输出: Mode: dev

# 检查Encoding配置
cat /opt/code/newbee/core/api/etc/core.yaml | grep -A 5 "^Log:"
# 应包含: Encoding: json
```

### 配置要点

```yaml
# etc/core.yaml
Mode: dev  # 必须设置

Log:
  Encoding: json  # 必须是json
  Level: info
```

---

## 🎨 效果对比

### 修改前（JSON）

```json
{"@timestamp":"2025-10-07T22:56:09.357+08:00","caller":"audit/core_client_adapter.go:51","content":"Core RPC CreateAuditLog failed","error":"rpc error: code = Internal desc = common.databaseError","level":"error"}
```
❌ 难以阅读
❌ 级别不明显

### 修改后（彩色）

```
[22:56:09] ERROR audit/core_client_adapter.go:51 | Core RPC CreateAuditLog failed | error="rpc error: code = Internal desc = common.databaseError"
[22:56:09] ERROR svc/core_audit_writer.go:74 | High-performance audit log creation failed | error="rpc error..."
```
✅ 格式清晰
✅ 级别用颜色区分（ERROR红色）
✅ 易于快速定位问题

---

## 🐛 故障排查

### 问题1：修改后仍是JSON格式

**可能原因**：
1. 服务未重启
2. 配置Mode不是dev
3. 代码修改未保存

**解决方法**：
```bash
# 1. 确认代码修改
grep -n "EnableColoredLoggingForDevMode" /opt/code/newbee/core/api/core.go
# 应在第48行左右

# 2. 确认配置
grep "^Mode:" /opt/code/newbee/core/api/etc/core.yaml

# 3. 完全重启服务
pkill -f core.api
cd /opt/code/newbee/core/api
go run core.go
```

### 问题2：部分日志有颜色，部分没有

**原因**：不同组件的日志可能在彩色Writer设置之前就输出了

**解决**：确保彩色Writer在 `conf.MustLoad()` 之后立即调用

### 问题3：编译错误

**检查**：
```bash
cd /opt/code/newbee/core/api
go mod tidy
go build -v .
```

---

## 📊 技术细节

### go-zero logx初始化机制

```go
conf.MustLoad(file, &c, conf.UseEnv())
// ↑ 内部会调用 logx.MustSetup(c.Log)
// ↑ 此时logx已经用JSON格式初始化

logwriter.EnableColoredLoggingForDevMode(c.Mode)
// ↑ 必须在这里立即调用，覆盖之前的设置
```

### logx.SetWriter 工作原理

```go
// logx内部
type atomicWriter struct {
    writer io.Writer  // 当前Writer
}

// SetWriter会原子替换Writer
func SetWriter(w Writer) {
    infoLog.swap(w)   // 替换INFO日志Writer
    errorLog.swap(w)  // 替换ERROR日志Writer
    // ...
}
```

因此，即使logx已经初始化，调用 `SetWriter` 仍然有效，但必须在任何日志输出之前调用。

---

## ✅ 验证清单

### 代码修改验证

- [ ] Core API - logwriter调用在conf.MustLoad之后
- [ ] IO API - logwriter调用在conf.MustLoad之后
- [ ] Core RPC - logwriter调用在conf.MustLoad之后
- [ ] IO RPC - logwriter调用在conf.MustLoad之后

### 配置验证

- [ ] Mode: dev 已设置
- [ ] Encoding: json 已设置
- [ ] 配置文件路径正确

### 运行验证

- [ ] 服务已重启（不是重新加载）
- [ ] 启动日志显示彩色格式
- [ ] 业务日志显示彩色格式
- [ ] ERROR级别显示红色

---

## 📝 后续改进

### 可选优化

1. **添加启动日志**：
```go
logwriter.EnableColoredLoggingForDevMode(c.Mode)
if c.Mode == service.DevMode {
    logx.Infow("✅ Colored logging enabled", logx.Field("mode", c.Mode))
}
```

2. **添加配置验证**：
```go
if c.Mode == service.DevMode && c.Log.Encoding != "json" {
    logx.Errorw("⚠️ Warning: Encoding should be 'json' for colored logging")
}
```

---

## 📚 相关文档

- **快速开始**：`COLORED_LOG_QUICK_START.md`
- **使用指南**：`COLORED_LOG_GUIDE.md`
- **配置验证**：`COLORED_LOG_CONFIG_VERIFICATION.md`

---

**修复时间**：2025-10-07
**状态**：✅ 代码已修改，等待用户重启验证
**下一步**：重启服务并观察日志输出
