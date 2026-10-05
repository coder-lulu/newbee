# Proxy（worker）安全冒烟清单（观察→强制）

> 目的：按步骤验证 JWT/Origin/限流 在观察模式与强制模式下的表现，形成可复用清单。

## 前置
- agent 启动方式（任选其一）：
  - go run worker/cmd/agent/main.go -f worker/etc/agent.yaml
  - 或编译后执行：./agent -f worker/etc/agent.yaml
- 配置：worker/etc/agent.yaml（建议先使用默认：Enforce=false）

## 用例 1：不带 Token 访问受保护路径（观察模式）
- 请求：
  curl -i -H "Origin: http://localhost:5173" \
    http://127.0.0.1:8889/api/ssh/websocket
- 期望：HTTP 400/101 任一（握手行为由后端控制）；日志包含
  JWT missing (observe)
- 备注：只验证中间件放行，不校验实际连接是否成功

## 用例 2：带无效 Token（签名不匹配）
- 构造：使用不同 secret 生成 Token
- 请求：同上，Authorization: Bearer <invalid>
- 期望：日志包含 JWT invalid (observe)

## 用例 3：带有效 Token
- 使用 docs/proxy-worker-安全联调指南.md 的生成方法，claims 至少包含：tenantId/userId/ciId/protocol/exp
- 请求：
  curl -i \
    -H "Origin: http://localhost:5173" \
    -H "Authorization: Bearer $TOKEN" \
    http://127.0.0.1:8889/api/ssh/websocket
- 期望：
  - 中间件放行；
  - 后续由具体 handler 决定返回（400/101 均可接受）。

## 用例 4：Origin 不在白名单
- 请求：
  curl -i -H "Origin: http://evil.local" \
    -H "Authorization: Bearer $TOKEN" \
    http://127.0.0.1:8889/api/ssh/websocket
- 期望：日志包含 Origin not in whitelist (observe)

## 用例 5：速率限制（RPS=5）
- 设置：Security.RateLimit.RequestsPerSecond: 5
- 请求：
  for i in {1..50}; do curl -s -o /dev/null \
    -H "Origin: http://localhost:5173" \
    -H "Authorization: Bearer $TOKEN" \
    http://127.0.0.1:8889/api/ssh/websocket; done
- 期望：部分请求返回 429 Too Many Requests

## 切换强制模式
- 修改：Security.JWT.Enforce: true，重启 agent
- 重跑用例 1、2、4：
  - 用例 1：返回 401 Unauthorized
  - 用例 2：返回 401 Unauthorized
  - 用例 4：返回 403 Forbidden

## WebSocket 建链补充
- websocat -H="Origin: http://localhost:5173" -H="Authorization: Bearer $TOKEN" \
  ws://127.0.0.1:8889/ws/ssh
- 首帧发送 SSHTunnelRequest JSON。若 AllowQueryToken=true，可改用 ws://.../ws/ssh?token=$TOKEN

## 通过标准
- 观察模式：所有非法场景仅记录日志、不拒绝；有效 Token 放行
- 强制模式：
  - 无/无效 Token → 401
  - 非白名单 Origin → 403
  - 限流命中 → 429

