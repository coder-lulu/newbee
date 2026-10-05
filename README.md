# Newbee 开发工作区

本仓库保存跨模块文档、配置示例、开发脚本、服务模板入口和各独立仓库的固定版本。
独立服务使用 Git 子模块管理，克隆时请同时获取子模块：

```sh
git clone --recurse-submodules https://github.com/coder-lulu/newbee.git
cd newbee
git submodule update --init --recursive
```

## 模块

| 路径 | GitHub 仓库 |
| --- | --- |
| `ui` | `coder-lulu/newbee-ui` |
| `core` | `coder-lulu/newbee-core` |
| `common` | `coder-lulu/newbee-common` |
| `ops-center` | `coder-lulu/newbee-ops` |
| `cmdb/api` | `coder-lulu/newbee-cmdb-api` |
| `cmdb/rpc` | `coder-lulu/newbee-cmdb-rpc` |
| `nb-agent` | `coder-lulu/nb-agent` |
| `newbee-proxy` | `coder-lulu/newbee-proxy` |
| `unified-io` | `coder-lulu/newbee-io`，内含 API/RPC 子模块 |
| `job` | `coder-lulu/newbee-job` |
| `packages/asset-selector-sdk` | `coder-lulu/newbee-asset-selector-sdk` |
| `templates/service-template` | `coder-lulu/newbee-service-template` |

IO API 和 RPC 分别保存于 `coder-lulu/newbee-io-api`、`coder-lulu/newbee-io-rpc`。
全部模块仓库均公开，可匿名递归克隆。

## 开发配置

- Go 工作区使用根目录 `go.work`，Go 版本要求为 1.25.1 或更高。
- 前端使用 Node.js 20+ 和 pnpm；开发命令见 `ui/package.json`。
- 各服务 `etc/` 目录中的真实运行配置不提交。首次运行时，将 `*.yaml.example` 复制为同名 `*.yaml`，填写本地配置或设置示例中使用的环境变量。
- 前端私密值使用 `.env.local` 或 `.env.<mode>.local`；这些文件不纳入版本控制。
- 编译产物、依赖缓存、运行日志和本地工具设置不提交。诊断产物放在 `logs/`。

服务配置示例和工作区快照用于保存现有开发状态；各模块的测试、构建和运行要求以其自身文档为准。

## 许可证

独立 Newbee 模块采用 MIT。有上游或基于上游源码派生的模块保留上游许可证和版权声明；依赖自身的许可证由对应依赖项目提供。

| 仓库 | 许可证 |
| --- | --- |
| `newbee-common` | Apache-2.0 |
| `newbee-core` | Apache-2.0 |
| `nb-agent` | BSD-3-Clause |
| `newbee-ops` | Apache-2.0 |
| `newbee-ui` | MIT |
| `newbee-cmdb-api` | Apache-2.0 |
| `newbee-cmdb-rpc` | Apache-2.0 |
| `newbee-proxy` | MIT |
| `newbee-io-api` | Apache-2.0 |
| `newbee-io-rpc` | Apache-2.0 |
| `newbee-io` | MIT |
| `newbee-job` | Apache-2.0 |
| `newbee-asset-selector-sdk` | MIT |
| `newbee-service-template` | Apache-2.0 |
| `newbee` | MIT |
