# 新蜂资产管理平台空库部署

以下为 MySQL、Redis、Go 服务和 Web 前端的单机部署流程。主数据库由源码中的 Ent Schema 创建，并由服务写入必要基础数据；不需要项目维护者本机的 SQL 备份。

## 1. 获取源码和工具

```bash
git clone --recurse-submodules https://github.com/coder-lulu/newbee.git
cd newbee
git submodule update --init --recursive
```

需要 Go 1.25.1+、MySQL 8、Redis 7、Bash、grpcurl。前端本地构建需要 Node.js 20.10.0+ 和 pnpm 10.10.0，也可以使用前端提供的 Dockerfile，在镜像中安装和构建依赖。

```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@v1.9.3
```

## 2. 创建数据库并配置服务

在准备好的 MySQL 实例中创建空库和专用账号。以下命令中的密码需要替换为自己的值；主机授权范围根据服务部署位置调整。

```sql
CREATE DATABASE newbee CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'newbee'@'%' IDENTIFIED BY 'REPLACE_WITH_YOUR_PASSWORD';
GRANT ALL PRIVILEGES ON newbee.* TO 'newbee'@'%';
```

准备 Redis。多个模块共享同一个数据库和 Redis 逻辑库；隔离验收环境必须使用单独的数据库和 Redis 实例或逻辑库。

复制配置模板：

```bash
for spec in core:core cmdb:cmdb ops-center:ops unified-io:io; do
  module=${spec%%:*}; config=${spec##*:}
  cp "$module/rpc/etc/$config.yaml.example" "$module/rpc/etc/$config.yaml"
  cp "$module/api/etc/$config.yaml.example" "$module/api/etc/$config.yaml"
done
cp job/etc/job.yaml.example job/etc/job.yaml
```

| 配置            | 设置要求                                                                                                |
| --------------- | ------------------------------------------------------------------------------------------------------- |
| 数据库          | 所有 `DatabaseConf` 指向上述空库；CMDB API 的对应项叫 `CasbinDatabaseConf`                              |
| Redis           | 所有 `RedisConf.Host`、`Pass`、`Db` 保持一致                                                            |
| RPC 地址        | API 指向对应 RPC；CMDB、Ops、IO 的 `CoreRpc` 指向 Core RPC；IO 同时配置 CMDB/Ops RPC                    |
| JWT             | API 的 `Middleware.auth.accessSecret` 使用同一个自行生成的密钥；Ops API 的 `Auth.AccessSecret` 同步设置 |
| OAuth 加密      | Core RPC `EncryptionKey` 设置独立的 32 字节密钥并保存                                                   |
| HTTP 请求加密   | 首次联调保持前端请求加密和 API `Middleware.encryption.enabled` 关闭                                     |
| 超时            | 首次初始化的 RPC `Timeout: 300000`，完成后恢复正常值                                                    |
| 后台任务        | IO `TaskWorker.Enabled: false`；Job `AsynqConf.Enable: false`，两类 TaskConf 调度开关为 false           |
| Ops Worker 恢复 | 首次建表前 `WorkerManager.Enabled: false`；初始化完成后按需改为 true 并重启 Ops RPC                     |
| 日志和指标      | `Log.Path` 使用可写目录，指标端口不能互相冲突                                                           |
| API 监听        | 使用前端 Docker 镜像时，API `Host` 必须为容器可访问的宿主机地址；模板监听 `0.0.0.0`                     |

配置模板中的地址和占位符必须替换。Core、CMDB、Ops、IO 和 Job 的服务入口会展开 YAML `${变量名}`，也可以在受保护的本地配置中直接填写值。按各文件使用的变量名注入数据库密码和 API JWT 密钥；不要把本地配置提交 Git。

本页部署了 Job，因此还需将 Core API 的 `JobRpc.Enabled` 设为 `true`，地址指向 Job RPC，才能使用前端的定时任务入口。未部署的 MCMS 连接保持禁用；开启 Job RPC 连接不会自动开启 Job 的消费者或调度器。

## 3. 编译并首次初始化

在工作区根目录构建；产物可放在忽略的 `logs/deployment/bin/`：

```bash
mkdir -p logs/deployment/bin
go build -o logs/deployment/bin/core-rpc ./core/rpc/core.go
go build -o logs/deployment/bin/core-api ./core/api/core.go
go build -o logs/deployment/bin/cmdb-rpc ./cmdb/rpc/cmdb.go
go build -o logs/deployment/bin/cmdb-api ./cmdb/api/cmdb.go
go build -o logs/deployment/bin/ops-rpc ./ops-center/rpc/ops.go
go build -o logs/deployment/bin/ops-api ./ops-center/api/ops.go
go build -o logs/deployment/bin/io-rpc ./unified-io/rpc/io.go
go build -o logs/deployment/bin/io-api ./unified-io/api/io.go
go build -o logs/deployment/bin/job ./job/job.go
```

下列每个长驻进程在独立终端或服务管理器中运行；运行环境需包含本机配置使用的变量。

```bash
logs/deployment/bin/core-rpc -f core/rpc/etc/core.yaml
```

等待 Core RPC 可连接后，在另一终端初始化：

```bash
bash init-databases.sh -s core
```

Core 成功后启动其他 RPC：

```bash
logs/deployment/bin/cmdb-rpc -f cmdb/rpc/etc/cmdb.yaml
logs/deployment/bin/ops-rpc -f ops-center/rpc/etc/ops.yaml
logs/deployment/bin/io-rpc -f unified-io/rpc/etc/io.yaml
logs/deployment/bin/job -f job/etc/job.yaml
```

然后初始化业务模块：

```bash
bash init-databases.sh -s cmdb -s ops-center -s unified-io -s job
```

所有初始化返回成功后启动 API：

```bash
logs/deployment/bin/core-api -f core/api/etc/core.yaml
logs/deployment/bin/cmdb-api -f cmdb/api/etc/cmdb.yaml
logs/deployment/bin/ops-api -f ops-center/api/etc/ops.yaml
logs/deployment/bin/io-api -f unified-io/api/etc/io.yaml
```

Core API 空库启动会加载权限表，因此必须在 Core 初始化之后运行。首次启动 CMDB RPC 可能记录标准关系表尚不存在的日志；建表成功后重启 CMDB RPC，让其启动预置逻辑再次执行。

## 4. 部署前端

前端 Dockerfile 会冻结锁文件安装依赖、按工作区顺序构建 `web-antd` 并提供 Nginx 同源 API 代理。默认上游端口对应上面的 API；Linux 上需提供宿主机映射。

```bash
docker build -f ui/scripts/deploy/Dockerfile -t newbee-ui:local ui
docker run -d --name newbee-ui --restart unless-stopped \
  --add-host=host.docker.internal:host-gateway \
  -p 127.0.0.1:8080:8080 newbee-ui:local
```

打开 `http://127.0.0.1:8080`。如果 API 位于其他机器或使用不同端口，在 `docker run` 中设置 `CORE_API_UPSTREAM`、`CMDB_API_UPSTREAM`、`IO_API_UPSTREAM` 和 `OPS_API_UPSTREAM`，值为 `主机:端口`。

本地开发和自行部署静态产物的方式见 [前端 README](../ui/README.md)。生产编译使用根目录 `pnpm build:antd`，它会先构建共享工作区依赖；不要跳过依赖构建直接执行应用包的 Vite 命令。

## 5. 验收及正式运行

首次登录使用租户 `1`、账号 `admin`、密码 `123456`，登录后立即修改初始密码。此账号由 Core 初始化代码创建。确认首页、资产实例、模型、Provider、输入任务、脚本和运维节点列表可以加载，空业务表应返回空列表而不是报缺表或权限目录错误。

完成首次初始化后，将 Core API `ProjectConf.AllowInit` 设为 false；RPC 仅在本机或受控内网访问。按业务需要部署 Proxy/Agent，再启用 IO Worker 或 Job 消费者、调度器；建表操作本身不执行运维任务。

Linux 二进制运行、systemd、前端 Nginx 和组件部署细节见各模块 README。仓库中的旧 SimpleAdmin Compose 模板没有替换为完整平台的一键启动入口，使用本页的源码服务与当前前端镜像部署。
