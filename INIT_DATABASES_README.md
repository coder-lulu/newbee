# 新蜂资产管理平台数据库初始化

平台主数据库使用 MySQL。仓库提供 Ent Schema 和基础数据初始化逻辑，不需要导入本机数据库备份。初始化工具调用已运行的 RPC 服务，不负责创建 MySQL 实例、数据库或服务账号，也不负责启动服务。

完整首次部署步骤见 [空库部署说明](docs/deployment.md)。已有环境升级前应备份数据库并审查 Schema 差异。

## 使用前准备

1. 递归克隆工作区：`git submodule update --init --recursive`。
2. 创建空的 `newbee` 数据库和专用账号，准备 Redis。
3. 复制每个服务的 `*.yaml.example`，填写本环境数据库、Redis、JWT、服务地址等配置。
4. 启动 Core RPC，并将初始化阶段的服务端 `Timeout` 设为 `300000` 毫秒。
5. 安装 Bash 和 grpcurl。脚本显式指定仓库内 proto，不依赖服务端反射，也不依赖 netcat。

```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@v1.9.3
bash init-databases.sh --list
bash init-databases.sh --dry-run -s core
bash init-databases.sh -s core
```

Core 初始化成功后，再启动 CMDB、Ops、IO 和 Job RPC，随后初始化这些服务。Core API 和其他 API 在相应数据库初始化成功后启动。

```bash
bash init-databases.sh -s cmdb -s ops-center -s unified-io -s job
```

若所有 RPC 已经运行，可一次初始化：

```bash
bash init-databases.sh
```

## 服务与调用顺序

| 服务 | 默认地址         | RPC 方法                 | 初始化内容                                               |
| ---- | ---------------- | ------------------------ | -------------------------------------------------------- |
| Core | `127.0.0.1:9100` | `core.Core/initDatabase` | 系统表、默认租户、管理员、角色、菜单、接口及权限基础数据 |
| CMDB | `127.0.0.1:9200` | `cmdb.Cmdb/initDatabase` | 资产表、基础模型和属性、Core 菜单及接口目录              |
| Ops  | `127.0.0.1:9600` | `ops.Ops/initDatabase`   | 运维表、Core 菜单及接口目录                              |
| IO   | `127.0.0.1:9500` | `io.Io/initDatabase`     | 接入表、Provider 目录、Core 菜单及接口目录               |
| Job  | `127.0.0.1:9105` | `job.Job/initDatabase`   | 定时任务与日志表；不创建或执行任务                       |

选择多个服务时，脚本保持上述依赖顺序并去重。Core 初始化失败会停止后续调用；其他 RPC 失败或不可达会计入失败，最终返回非零状态，不能视为成功。

CMDB、Ops、IO 的目录登记使用默认租户 `1`，并合并其 `superadmin` 的菜单授权；不会授权普通角色。Core 未初始化、未配置 Core RPC 或目录登记失败时，初始化会返回错误。默认租户不是 `1` 的已有环境不适用首次部署步骤。

## 修改地址和超时

```bash
NEWBEE_CORE_RPC=127.0.0.1:19100 \
NEWBEE_CMDB_RPC=127.0.0.1:19200 \
NEWBEE_OPS_RPC=127.0.0.1:19600 \
NEWBEE_IO_RPC=127.0.0.1:19500 \
NEWBEE_JOB_RPC=127.0.0.1:19105 \
bash init-databases.sh
```

`GRPCURL` 可指定工具路径；`NEWBEE_INIT_TIMEOUT` 为客户端超时秒数，默认 `300`。服务端 `Timeout` 独立生效，客户端参数不能覆盖它。

初始化阶段关闭 Ops 的 `WorkerManager.Enabled`、IO 的 `TaskWorker.Enabled` 和 Job 的 `AsynqConf.Enable`、`TaskConf.EnableDPTask`、`TaskConf.EnableScheduledTask`。这些开关在模板中默认关闭；建表成功后可以启用 Ops Worker 恢复并重启 RPC，确认任务配置后再按需启用 IO 和 Job 后台任务。

重复初始化保留已有业务记录、列和索引，菜单和接口目录按现有记录复用。Core 已存在接口数据时会跳过基础数据插入，因此该入口不能替代完整的数据修复或正式版本迁移。Core 调用超时后后台初始化可能继续，先检查日志和结果，再决定是否重试。

## 回归检查

脚本测试用模拟 RPC，不连接数据库：

```bash
python scripts/tests/test_init_databases.py
```

Windows 可通过环境变量 `NEWBEE_TEST_BASH` 指定 Git Bash 的完整路径。测试产物放在忽略的 `logs/clone-deploy/tests/`。
