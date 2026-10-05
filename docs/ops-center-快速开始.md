# Ops Center 快速开始

> 目标：几分钟内跑通“Center 签发 Token → Proxy 鉴权 → 建立 WS 会话”的最小闭环。

## 前置环境
- Go 1.23+
- Redis（集成中间件需要，可本地 127.0.0.1:6379）

## 启动 Proxy（worker）

1) 编辑 `worker/etc/agent.yaml` 中的安全配置（默认已开启观察模式）：

Security:
  JWT:
    Enabled: true
    Secret: "dev-secret"
    AllowQueryToken: true
    Enforce: false
  OriginWhitelist:
    - "http://localhost:5173"
  RateLimit:
    Enabled: true
    RequestsPerSecond: 50

2) 启动：

go run worker/cmd/agent/main.go -f worker/etc/agent.yaml

## 启动 Ops Center

go run ops-center/api/ops.go -f ops-center/api/etc/ops.yaml

默认端口：9410。默认 Proxy 端点：`http://127.0.0.1:8889`。

## 创建会话（Center 签发 Token）

curl -s http://127.0.0.1:9410/ops/proxy/pick | jq

curl -s -X POST http://127.0.0.1:9410/ops/session/create \
  -H 'Content-Type: application/json' \
  -d '{"ciId":"ci-001","protocol":"ssh"}' | jq

响应示例：
{
  "sessionId": "...",
  "proxyId": "proxy-001",
  "wsUrl": "http://127.0.0.1:8889/api/ssh/websocket",
  "token": "eyJhbGciOi...",
  "expiresAt": 1749129999
}

## 建立 WS 会话

使用 websocat：

websocat \
  -H="Origin: http://localhost:5173" \
  -H="Authorization: Bearer $TOKEN" \
  ws://127.0.0.1:8889/api/ssh/websocket

注意：此路径是 guacd 模式的 SSH 隧道；也可以使用 `/ws/ssh` 搭配首帧 JSON（详见 `docs/proxy-worker-安全联调指南.md`）。

## 切换强制模式

将 `worker/etc/agent.yaml` 的 `Security.JWT.Enforce` 改为 `true`，重启 worker。此时不带或非法 token 将被拒绝（401/403）。

## 下一步
- 接入 /ops/proxy/register 与 /ops/proxy/heartbeat，改造为基于注册表的就近选路；
- 增加 AccessProfile 并通过会话参数自动化生成握手参数；
- 打通 /ops/task/* 链路，直调 nb-agent 执行任务。

