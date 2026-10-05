# NewBee 微服务数据库迁移工具使用说明

## 工具简介

`init-databases.sh` 是一个统一的数据库初始化工具，用于批量调用各个微服务的RPC `initDatabase` 接口进行数据库迁移。

## 功能特性

- ✅ 自动发现并调用所有微服务的initDatabase接口
- ✅ 智能检测服务是否在线
- ✅ 支持单个或批量初始化
- ✅ 彩色输出，清晰显示执行状态
- ✅ 统计报告，展示成功/失败/跳过数量

## 前置要求

1. **grpcurl工具** - 用于调用gRPC接口
   ```bash
   # 如果未安装，执行：
   go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
   ```

2. **微服务运行** - 需要初始化的微服务必须处于运行状态
   ```bash
   # 示例：启动core服务
   cd /opt/code/newbee/core/rpc
   go run core.go
   ```

## 使用方法

### 1. 初始化所有微服务

```bash
# 在项目根目录执行
./init-databases.sh
```

输出示例：
```
============================================
  NewBee 微服务数据库迁移工具
============================================

[SUCCESS] grpcurl 已安装

==> 初始化 core 数据库
[INFO] core 服务在线，开始初始化数据库...
[SUCCESS] core 数据库初始化成功
    响应: {"msg": "database initialized successfully"}

==> 初始化 cmdb 数据库
[WARNING] cmdb 服务未运行 (127.0.0.1:9200)，跳过

==> 初始化 unified-io 数据库
[INFO] unified-io 服务在线，开始初始化数据库...
[SUCCESS] unified-io 数据库初始化成功
    响应: {"msg": "database initialized successfully"}

============================================
  初始化结果统计
============================================
总服务数: 4
成功: 2
跳过: 2
失败: 0
============================================
```

### 2. 初始化单个微服务

```bash
# 只初始化core服务
./init-databases.sh -s core

# 只初始化ops-center服务
./init-databases.sh -s ops-center
```

### 3. 查看支持的服务列表

```bash
./init-databases.sh --list
```

输出：
```
支持的微服务:
  - core (端口: 9100, 包: core, 服务: Core)
  - cmdb (端口: 9200, 包: cmdb, 服务: Cmdb)
  - unified-io (端口: 9500, 包: io, 服务: Io)
  - ops-center (端口: 9600, 包: ops, 服务: Ops)
```

### 4. 查看帮助信息

```bash
./init-databases.sh --help
```

## 支持的微服务

| 服务名 | RPC端口 | Proto包 | Proto服务 | 说明 |
|--------|---------|---------|-----------|------|
| core | 9100 | core | Core | 核心服务（用户、角色、权限等） |
| cmdb | 9200 | cmdb | Cmdb | 配置管理数据库 |
| unified-io | 9500 | io | Io | 统一数据输入输出服务 |
| ops-center | 9600 | ops | Ops | 运维中心服务 |

## 使用场景

### 场景1：修改了Schema，需要迁移数据库

```bash
# 1. 修改 ent schema
vim core/rpc/ent/schema/user.go

# 2. 生成ent代码
cd core/rpc
make gen-rpc

# 3. 重启RPC服务
./core

# 4. 执行数据库迁移
cd /opt/code/newbee
./init-databases.sh -s core
```

### 场景2：全新部署，需要初始化所有数据库

```bash
# 1. 启动所有微服务
./start-all-services.sh  # 假设有这个脚本

# 2. 执行全部初始化
./init-databases.sh
```

### 场景3：添加新的微服务

如果新增了微服务（例如 `fms`），需要修改脚本添加配置：

```bash
# 编辑 init-databases.sh
vim init-databases.sh

# 在SERVICES数组中添加：
SERVICES=(
    "core:9100:core:Core"
    "cmdb:9200:cmdb:Cmdb"
    "unified-io:9500:io:Io"
    "ops-center:9600:ops:Ops"
    "fms:9102:fms:Fms"  # 新增
)
```

## 错误处理

### 错误1：grpcurl未安装

**错误信息**：
```
[ERROR] grpcurl 未安装，请先安装: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

**解决方法**：
```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

### 错误2：服务未运行

**错误信息**：
```
[WARNING] core 服务未运行 (127.0.0.1:9100)，跳过
```

**解决方法**：
```bash
# 启动对应服务
cd /opt/code/newbee/core/rpc
go run core.go
```

### 错误3：数据库连接失败

**错误信息**：
```
[ERROR] core 数据库初始化失败
    错误: rpc error: code = Unavailable desc = connection error
```

**解决方法**：
1. 检查数据库是否运行
2. 检查配置文件中的数据库连接信息
3. 检查网络连接

### 错误4：权限不足

**错误信息**：
```
bash: ./init-databases.sh: Permission denied
```

**解决方法**：
```bash
chmod +x init-databases.sh
```

## 工作原理

1. **服务发现**：使用 `nc` 命令检测服务端口是否可达
2. **gRPC调用**：使用 `grpcurl` 工具调用各服务的 `initDatabase` 方法
3. **状态跟踪**：记录成功、失败、跳过的服务数量
4. **结果统计**：最后输出汇总报告

## 注意事项

⚠️ **重要提醒**：

1. **数据安全**：`initDatabase` 接口会执行数据库迁移操作，可能会修改数据库结构。生产环境使用前请务必备份数据库。

2. **服务顺序**：建议按以下顺序初始化：
   - 第一步：`core` （核心服务，包含基础数据）
   - 第二步：其他服务（`cmdb`, `unified-io`, `ops-center`）

3. **依赖关系**：某些服务可能依赖其他服务的数据，注意初始化顺序。

4. **幂等性**：`initDatabase` 接口应该是幂等的，重复调用不会造成问题。

5. **超时设置**：大型数据库迁移可能需要较长时间，注意调整RPC超时配置。

## 进阶用法

### 自定义超时时间

如果迁移时间较长，可以修改各服务的配置文件：

```yaml
# core/rpc/etc/core.yaml
Timeout: 60000  # 60秒超时，单位：毫秒
```

### 并行执行（高级）

默认是串行执行，如果需要并行可以修改脚本：

```bash
# 在脚本中添加后台执行
init_database "$service_name" "$port" "$package" "$proto_service" &
```

⚠️ 注意：并行执行可能导致数据库死锁，不推荐使用。

## 扩展开发

### 添加新的微服务

1. 在 `SERVICES` 数组中添加配置
2. 确保新服务的proto中定义了 `initDatabase` 方法
3. 测试脚本执行

### 集成到CI/CD

```yaml
# .gitlab-ci.yml 示例
deploy:
  script:
    - ./deploy.sh
    - ./init-databases.sh
    - echo "Database migration completed"
```

## 常见问题（FAQ）

**Q1: 可以多次执行吗？**

A: 可以。`initDatabase` 接口应该是幂等的，多次执行不会造成问题。

**Q2: 是否支持回滚？**

A: 不支持。如需回滚，请使用数据库备份恢复。

**Q3: 如何查看详细的日志？**

A: 查看各服务的日志文件，路径在配置文件中定义（例如 `/home/data/logs/core/rpc`）。

**Q4: 可以远程执行吗？**

A: 可以。修改脚本中的 `127.0.0.1` 为远程服务器IP，但需要确保网络连通。

## 维护指南

- **定期检查**：确保grpcurl工具是最新版本
- **更新配置**：新增微服务时及时更新 `SERVICES` 数组
- **日志清理**：定期清理老旧的日志文件
- **权限管理**：确保脚本有执行权限

## 版本历史

- **v1.0.0** (2025-01-15)
  - 初始版本
  - 支持4个微服务（core, cmdb, unified-io, ops-center）
  - 基础功能：批量初始化、单个初始化、列表查看

## 技术支持

如有问题，请联系：
- 项目维护者
- 查看项目文档：`/opt/code/newbee/CLAUDE.md`

---

**祝使用愉快！** 🚀
