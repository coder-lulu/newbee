# 新蜂资产管理平台

新蜂资产管理平台（Newbee）用于组织 IT 资产模型、资产数据、采集接入和运维任务。本仓库是平台的开发与部署工作区：通过 Git 子模块固定前端、核心服务、资产服务、采集组件等仓库的版本，并保存跨模块文档和开发工具。

平台采用 Vue Web 前端与 Go API/RPC 服务架构。核心服务提供用户、组织、租户及权限基础能力；CMDB 管理资产模型和实例；IO、Agent、Proxy 与运维服务承接数据采集、资源接入及任务执行。

## 获取完整项目

所有在用代码仓库均公开。建议递归克隆工作区，保持 `go.work` 和各服务的本地依赖路径一致：

```sh
git clone --recurse-submodules https://github.com/coder-lulu/newbee.git
cd newbee
git submodule update --init --recursive
```

已有工作区更新到本仓库记录的版本：

```sh
git pull --ff-only
git submodule update --init --recursive
```

各子模块有独立仓库与许可证。单独克隆后端模块时，需要按其 README 准备同级依赖；本工作区内的 `replace` 路径和 `go.work` 以完整目录布局为基础。

## 仓库与模块

| 本地路径 | 仓库 | 用途 | 文档 |
| --- | --- | --- | --- |
| `.` | [newbee](https://github.com/coder-lulu/newbee) | 平台工作区与跨模块资料 | 本文 |
| `ui` | [newbee-ui](https://github.com/coder-lulu/newbee-ui) | Web 管理前端 | [前端与部署](https://github.com/coder-lulu/newbee-ui#readme) |
| `core` | [newbee-core](https://github.com/coder-lulu/newbee-core) | 用户、组织、租户、权限及系统基础服务 | [核心服务与部署](https://github.com/coder-lulu/newbee-core#readme) |
| `common` | [newbee-common](https://github.com/coder-lulu/newbee-common) | 通用配置、中间件、数据权限与工具 | [公共库](https://github.com/coder-lulu/newbee-common#readme) |
| `cmdb/api` | [newbee-cmdb-api](https://github.com/coder-lulu/newbee-cmdb-api) | 资产管理 HTTP API | [CMDB API](https://github.com/coder-lulu/newbee-cmdb-api#readme) |
| `cmdb/rpc` | [newbee-cmdb-rpc](https://github.com/coder-lulu/newbee-cmdb-rpc) | 资产模型、实例、关系及发现服务 | [CMDB RPC](https://github.com/coder-lulu/newbee-cmdb-rpc#readme) |
| `ops-center` | [newbee-ops](https://github.com/coder-lulu/newbee-ops) | 运维任务与执行调度 | [运维中心](https://github.com/coder-lulu/newbee-ops#readme) |
| `unified-io` | [newbee-io](https://github.com/coder-lulu/newbee-io) | IO provider 与统一接入工作区 | [统一 IO](https://github.com/coder-lulu/newbee-io#readme) |
| `unified-io/api` | [newbee-io-api](https://github.com/coder-lulu/newbee-io-api) | 统一接入 HTTP API | [IO API](https://github.com/coder-lulu/newbee-io-api#readme) |
| `unified-io/rpc` | [newbee-io-rpc](https://github.com/coder-lulu/newbee-io-rpc) | IO 配置、采集与后台任务服务 | [IO RPC](https://github.com/coder-lulu/newbee-io-rpc#readme) |
| `nb-agent` | [nb-agent](https://github.com/coder-lulu/nb-agent) | 主机侧 Agent 与主机信息接入 | [Agent](https://github.com/coder-lulu/nb-agent#readme) |
| `newbee-proxy` | [newbee-proxy](https://github.com/coder-lulu/newbee-proxy) | 协议代理、远程连接和任务执行 | [Proxy](https://github.com/coder-lulu/newbee-proxy#readme) |
| `job` | [newbee-job](https://github.com/coder-lulu/newbee-job) | 定时与异步任务服务 | [任务服务](https://github.com/coder-lulu/newbee-job#readme) |
| `packages/asset-selector-sdk` | [newbee-asset-selector-sdk](https://github.com/coder-lulu/newbee-asset-selector-sdk) | Vue 资产选择组件与 SDK | [资产选择 SDK](https://github.com/coder-lulu/newbee-asset-selector-sdk#readme) |
| `templates/service-template` | [newbee-service-template](https://github.com/coder-lulu/newbee-service-template) | API/RPC 服务开发模板 | [服务模板](https://github.com/coder-lulu/newbee-service-template#readme) |

根工作区包含 12 个直接子模块，`unified-io` 再管理 API、RPC 两个子模块，共对应 15 个在用仓库。

## 部署入口

先完成核心服务与前端部署，再按需要接入资产、采集和运维服务。仅启动核心与前端可验证系统基础功能；资产与运维页面需要相应后端服务。

| 组件 | 准备内容 | 部署说明 |
| --- | --- | --- |
| 核心服务 | Go 1.25.1+、数据库、Redis；配置 RPC/API 并首次初始化数据库 | [配置、初始化、构建与 Linux 部署](https://github.com/coder-lulu/newbee-core#readme) |
| Web 前端 | Node.js 20.10.0+、项目指定的 pnpm 10.10.0；配置 API 地址并构建静态文件 | [开发、生产构建与 Nginx 部署](https://github.com/coder-lulu/newbee-ui#readme) |
| 资产管理 | CMDB RPC/API 及其数据库配置，与核心权限服务连接 | [CMDB RPC](https://github.com/coder-lulu/newbee-cmdb-rpc#readme)、[CMDB API](https://github.com/coder-lulu/newbee-cmdb-api#readme) |
| 采集与运维 | 按功能启用 IO、Agent、Proxy、运维中心或 Job | 上表中对应模块 README |

当前核心配置示例中，RPC 监听 `9100`、API 监听 `9101`。前端使用 `/sys-api` 接口前缀，开发代理或生产反向代理需要剥离该前缀，再转发到核心 API。前端启动端口以应用配置和终端输出为准。

核心服务首次部署顺序为：准备数据库与 Redis → 配置并启动 Core RPC → 初始化数据库 → 启动 Core API → 部署前端。完整命令及初始化接口见核心 README。

## 配置与开发约定

- 真实服务配置保留在本机。将服务的 `*.yaml.example` 复制为同名 `*.yaml` 后，修改数据库、Redis、服务地址与密钥。模板中的地址和占位符需要按实际环境填写。
- 只有启用 `conf.UseEnv()` 的服务入口才会展开配置中的环境变量；其他服务须先将占位符替换为配置值，具体以模块 README 为准。
- 前端环境文件应放在实际应用目录，具体加载位置及生产配置方式见前端 README。本地私密覆盖文件使用 `.env.local` 或 `.env.<mode>.local`。
- Go 服务使用根目录 `go.work` 协调本地模块。进入各模块目录执行其构建或测试命令；Agent、前端和 SDK 的独立工具链见各自文档。
- `init-databases.sh`、`start_service.sh` 面向 Bash 环境；使用前核对服务地址与目录映射。首次核心初始化优先使用核心 README 中的明确步骤。
- 编译产物、数据库运行文件、私钥、依赖缓存和本地日志不提交。诊断产物统一写入 `logs/`。

## 延伸阅读

- [平台架构设计](架构设计.md)
- [资产类型自动发现设计](CIType_Auto_Discovery_Design.md)
- [数据库初始化工具说明](INIT_DATABASES_README.md)
- [仓库开发约定](AGENTS.md)
- [服务模板说明](templates/README.md)

## 许可证与上游

本工作区自身内容使用 [MIT](LICENSE)。各子仓库保留独立许可证；上游来源与版权声明见对应仓库 README 和 LICENSE。

| 许可证 | 仓库 |
| --- | --- |
| MIT | `newbee`、`newbee-ui`、`newbee-proxy`、`newbee-io`、`newbee-asset-selector-sdk` |
| Apache-2.0 | `newbee-core`、`newbee-common`、`newbee-ops`、`newbee-cmdb-api`、`newbee-cmdb-rpc`、`newbee-io-api`、`newbee-io-rpc`、`newbee-job`、`newbee-service-template` |
| BSD-3-Clause | `nb-agent` |

核心服务、公共库和任务服务分别保留 Simple Admin 相关上游的 Apache 2.0 授权；前端保留 Vben 的 MIT 授权；Agent 保留 AcePanel 的 BSD 3-Clause 授权。第三方依赖与保留的源码声明遵循各自许可证。
