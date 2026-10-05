# CMDB 访问画像（AccessProfile）集成设计

> 目标：以 CI 为真相源，维护“访问画像”以驱动会话/任务参数（协议/端口/凭证/跳板/首选 Proxy 等）。

## 1. 存储位置与原则
- 优先在 Ops Center 内落库 AccessProfile，并以 ci_id 引用 CMDB 的 CI；避免污染 CMDB 核心模型。
- 保持与 CMDB 的弱耦合：仅拉取必要字段用于展示与一致性校验。

## 2. 模型字段建议
- AccessProfile：
  - ci_id（外键引用 CI）、capabilities（ssh|telnet|rdp|vnc|agent[]）、ports(map)、credential_ref（string）、prefer_proxy（string）、jump_chain（string[]）、tags(map)
  - 审计字段与 `TenantMixin`。

## 3. API 设计
- /ops/profile
  - POST /create、PUT /update、GET /{ciId}、GET /list?page=…、DELETE /{ciId}
- 校验：
  - CI 存在性（调用 CMDB）；
  - 协议/端口合法性；
  - 凭证引用权限范围；

## 4. 选路与参数生成
- 会话：根据画像生成握手参数（端口、分辨率、跳板链、首选 Proxy）；
- 任务：根据画像选择 executor（agent 优先）与凭证引用；

## 5. 缓存与一致性
- 本地/Redis 缓存画像；CI 变更订阅（后续）或定时刷新；
- 更新画像时写入审计记录；

## 6. 原子任务与测试
1) 表结构与 CRUD
   - 测试：Ent Hook 与多租户隔离；
2) CI 联查与校验
   - 测试：CI 不存在/权限不符时拒绝；
3) 画像驱动参数生成
   - 测试：不同协议/端口/跳板组合；
4) 缓存与刷新策略
   - 测试：一致性与失效；

## 7. 验收标准
- 画像完整性与有效性校验通过；
- 会话/任务参数自动化生成正确；
- 多租户隔离有效。

