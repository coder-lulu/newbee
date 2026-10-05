# API日志格式问题修复总结

## 🎯 问题

Core API服务输出JSON格式日志，而不是彩色格式。

---

## ✅ 根本原因

**`conf.MustLoad()` 在加载配置时会自动初始化logx**，而我们的彩色Writer设置在它之后，导致设置无效。

---

## 🔧 解决方案

**调整代码顺序**：在 `conf.MustLoad()` 之后**立即**调用彩色日志设置。

### 修改前
```go
conf.MustLoad(*configFile, &c, conf.UseEnv())
server := rest.MustNewServer(...)
// ...在这里调用彩色日志设置（太晚了）
logwriter.EnableColoredLoggingForDevMode(c.Mode)
```

### 修改后 ✅
```go
conf.MustLoad(*configFile, &c, conf.UseEnv())
// ✅ 立即调用
logwriter.EnableColoredLoggingForDevMode(c.Mode)
server := rest.MustNewServer(...)
```

---

## 📋 已修改的文件

- ✅ `/opt/code/newbee/core/api/core.go`
- ✅ `/opt/code/newbee/core/rpc/core.go`
- ✅ `/opt/code/newbee/unified-io/api/io.go`
- ✅ `/opt/code/newbee/unified-io/rpc/io.go`

---

## 🚀 如何验证

### 1. 重启Core API服务

```bash
# 停止当前服务
pkill -f core.api

# 重新启动
cd /opt/code/newbee/core/api
go run core.go
```

### 2. 观察日志

**预期输出**（彩色格式）：
```
[23:10:15] INFO  svc/service_context.go:50 | ✅ Core service initialized
[23:10:15] ERROR audit/core.go:51 | CreateAuditLog failed | error="..."
```

**如果仍是JSON**：
```json
{"@timestamp":"...","level":"info",...}
```
请查看详细故障排查文档：`FIX_API_JSON_LOG_ISSUE.md`

---

## ⚙️ 配置要求

确保配置文件包含：

```yaml
Mode: dev  # 必须设置

Log:
  Encoding: json  # 必须是json
  Level: info
```

---

## 📚 详细文档

完整故障排查和验证步骤：`FIX_API_JSON_LOG_ISSUE.md`

---

**修复状态**：✅ 代码已修改
**下一步**：重启服务验证效果
