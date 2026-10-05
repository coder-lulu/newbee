# Proxy（复用 worker）服务设计

> 角色：协议/会话网关与区域代理（Phase 1 仅协议网关，Phase 2 增加注册/心跳/选路信号）。
> 框架：go-zero rest（现有），增强安全与管理面。

## 1. 现状与问题
- 已具备 WS 隧道与 guacd 代理：/api/rdp|vnc|telnet|ssh/websocket。
- 风险：Upgrader.CheckOrigin 允许任意来源；无统一 JWT 鉴权/租户透传；无注册/心跳；审计缺口。

## 2. 增强目标（Phase 1）
- 统一鉴权：
  - HTTP/WS 中间件校验短期 JWT（Authorization: Bearer/兼容 ?token=）。
  - 校验 claims：exp、tenantId、userId、ciId、protocol 一致性。
  - 限流：同用户/租户并发/速率阈值，超限拒绝。
- 兼容开关：
  - `security.jwt.enabled`（观察模式/强制模式），`security.origin_whitelist`，`security.rate_limit`。
  - `security.protected_paths`：受保护路径可配置（为空时使用内置默认列表）。
- 审计钩子：
  - 连接/断开/异常、命令透传（SSH/Telnet）、握手参数（脱敏）埋点。
  - 轻量本地日志（AUDIT ...），后续可对接 Ops Center 审计聚合。

## 3. 管理接口（Phase 2）
- POST /api/proxy/register（Proxy→Center）
  - 入参：proxyId, region, az, endpoints, capabilities, labels
  - 返回：jwksUri、mtls 策略、heartbeatInterval
- POST /api/proxy/heartbeat（Proxy→Center）
  - 入参：负载、容量、会话数、状态
- GET /api/proxy/metrics：Prom 指标

## 4. WS 握手与连接流程
1) 验证 JWT（Header 优先）；
2) 校验 tenant/user/ci/protocol；
3) 限流与并发控制（按租户/用户/端点）；
4) 建立隧道并打审计点；
5) 连接关闭/异常记录审计；
6) 采样/聚合后上报 Ops Center。

## 5. 配置与热更新
- 支持通过 Redis/配置中心热更新：白名单、限流、审计等级。
- 灰度：观察模式记录不拒绝 → 强制模式拒绝非法连接。

## 6. 原子任务（含关键点与测试）
1) JWT 中间件（WS/HTTP）
   - 关键点：握手阶段拿到 token、多来源支持；
   - 测试：过期/伪造/跨租户拒绝，兼容旧路径。
2) Origin 白名单与速率限制
   - 关键点：并发/速率可配置、热更新；
   - 测试：压测下限流行为正确。
3) WS Handler 改造：读取 claims、透传上下文
   - 关键点：日志脱敏；
   - 测试：会话建立→命令传输→关闭全流程。
4) 审计埋点
   - 关键点：体量控制与异步落盘；
   - 测试：大并发采样与写入。
5) 注册/心跳/metrics（Phase 2）
   - 关键点：状态一致性、过期淘汰；
   - 测试：多 Proxy 混合心跳乱序处理。

## 7. 验收标准
- 非鉴权连接全部拦截（强制模式）；
- 审计记录可检索；
- 心跳/指标与选路对齐 Ops Center；
- 在压测下无资源泄漏。
