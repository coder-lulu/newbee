# Ops Center 生成与构建脚本（本地/CI 通用）

## 快速开始（本地）

```
chmod +x scripts/ops-center-generate-and-build.sh
GOCTLS_BIN=goctls \
scripts/ops-center-generate-and-build.sh
```

说明：
- 自动执行以下步骤：
  - Ent 代码生成：`go run entgo.io/ent/cmd/ent generate ./ent/schema --feature sql/execquery,intercept,sql/modifier`
  - RPC 代码生成（若环境安装了 `goctls`）：`goctls rpc ent ...`
  - 构建 `ops-center/rpc` 与 `ops-center/api`（使用模块内 `.gocache` 规避沙箱权限）
- 若未安装 `goctls`，脚本会跳过 RPC 生成，仅执行构建（保持兼容）

## CI 模板（GitHub Actions）

仓库已提供示例工作流：`.github/workflows/ops-center-generate-build.yml`

要点：
- 自动在 PR 与手动触发时运行
- 缓存 Go 构建产物与模块缓存
- 可选安装 `protoc`
- 直接调用 `scripts/ops-center-generate-and-build.sh`

## 常见问题

- goctls 不存在：
  - 仅跳过 RPC 生成；可在构建机安装团队统一版本的 goctls 后再启用
- 代码生成覆盖：
  - 业务逻辑请放在非生成目录（internal/domain、pkg 等）；生成目录仅保留骨架
- 排序/分页行为：
  - AccessProfile 列表排序与分页已下沉 RPC；API 仅透传 `page/size/sortBy/order`

