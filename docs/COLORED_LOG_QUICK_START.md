# 彩色日志快速开始指南

## 🎯 一分钟快速启用

### 当前状态

**所有核心服务已配置完成** ✅

只需确保配置文件中的 `Mode: dev`，重启服务即可看到彩色日志。

---

## 🚀 快速验证

### 1. 检查配置

```bash
# Core API
grep "^Mode:" /opt/code/newbee/core/api/etc/core.yaml
# 应输出: Mode: dev

# Core RPC
grep "^Mode:" /opt/code/newbee/core/rpc/etc/core.yaml
# 应输出: Mode: dev
```

### 2. 启动服务

```bash
# 启动Core API
cd /opt/code/newbee/core/api
go run core.go
```

### 3. 观察日志

**预期输出（带颜色）**：
```
[22:54:10] INFO  svc/service_context.go:50 | ✅ Core service initialized
[22:54:10] INFO  hooks/unified_hook.go:369 | Registered mutation hook | field_type=tenant_id
```

**如果看到JSON格式**：
```json
{"@timestamp":"2025-10-07T22:54:10.000+08:00","level":"info",...}
```
说明配置未生效，请检查：
1. `Mode` 是否设置为 `dev`
2. 服务是否已重启

---

## ⚙️ 环境切换

### 开发环境（彩色日志）

```yaml
# etc/core.yaml
Mode: dev
```

### 生产环境（JSON日志）

```yaml
# etc/core.yaml
Mode: prod  # 或直接删除Mode配置
```

---

## 🎨 输出效果对比

### 开发环境输出

```
[22:54:10] INFO  main.go:17 | ✅ 服务启动成功 | service=core-api | port=9101
[22:54:10] ERROR user.go:42 | 登录失败 | error="invalid password" | user_id=123
```

- 🟢 **INFO** - 绿色
- 🔴 **ERROR** - 红色
- ⚪ **时间/字段** - 灰色
- 🔷 **文件位置** - 青色

### 生产环境输出

```json
{"@timestamp":"2025-10-07T22:54:10.000+08:00","level":"info","content":"✅ 服务启动成功","service":"core-api","port":9101}
{"@timestamp":"2025-10-07T22:54:10.000+08:00","level":"error","content":"登录失败","error":"invalid password","user_id":123}
```

- ✅ 标准JSON
- ✅ 兼容日志收集工具

---

## 📋 配置清单

### 已完成的服务

| 服务 | 端口 | 配置文件 | 状态 |
|------|------|---------|------|
| Core RPC | 9100 | `core/rpc/etc/core.yaml` | ✅ |
| Core API | 9101 | `core/api/etc/core.yaml` | ✅ |
| IO RPC | 9500 | `unified-io/rpc/etc/io.yaml` | ✅ |
| IO API | 9501 | `unified-io/api/etc/io.yaml` | ✅ |

### 配置要点

每个服务都包含：
1. ✅ **顶层Mode配置** - `Mode: dev`
2. ✅ **Log Encoding配置** - `Encoding: json`
3. ✅ **代码集成** - `logwriter.EnableColoredLoggingForDevMode(c.Mode)`

---

## 🔧 新服务集成（3步）

### 步骤1：添加配置

```yaml
# etc/your-service.yaml
Mode: dev  # 开发环境

Log:
  Encoding: json  # 必须是json
  Level: info
```

### 步骤2：导入包

```go
import "github.com/coder-lulu/newbee-common/utils/logwriter"
```

### 步骤3：启用彩色日志

```go
func main() {
    var c config.Config
    conf.MustLoad(*configFile, &c, conf.UseEnv())

    // 启用彩色日志（仅开发环境）
    logwriter.EnableColoredLoggingForDevMode(c.Mode)

    // ... 其他代码
}
```

**完成！** 🎉

---

## ⚠️ 注意事项

### 1. 生产环境必须使用 `Mode: prod`

```yaml
# ✅ 正确
Mode: prod

# ❌ 错误（会导致日志收集工具无法解析）
Mode: dev
```

### 2. 日志编码必须是 `json`

```yaml
# ✅ 正确
Encoding: json

# ❌ 错误（彩色Writer无法解析）
Encoding: plain
```

### 3. 性能考虑

- 开发环境：性能开销 <1%
- 生产环境：使用JSON，性能最优
- 高频日志：建议降低日志级别

---

## 🐛 故障排查

### 日志还是JSON格式？

**检查清单**：
- [ ] `Mode: dev` 是否存在？
- [ ] 服务是否已重启？
- [ ] 代码是否调用了彩色日志函数？

```bash
# 快速检查
grep "Mode:" etc/core.yaml
grep "EnableColored" core.go
```

### 终端显示乱码？

**原因**：终端不支持ANSI颜色

**解决**：
- 使用现代终端（iTerm2、Windows Terminal等）
- 或改用 `Mode: prod`

---

## 📚 完整文档

详细文档位于 `/opt/code/newbee/docs/`：

- **使用指南** - `COLORED_LOG_GUIDE.md`
- **配置验证** - `COLORED_LOG_CONFIG_VERIFICATION.md`
- **完整报告** - `FINAL_COLORED_LOG_COMPLETION_REPORT.md`

---

## 📞 需要帮助？

1. 查阅完整文档
2. 检查配置清单
3. 运行验证测试
4. 联系开发团队

---

**最后更新**：2025-10-07
**状态**：✅ 生产就绪
