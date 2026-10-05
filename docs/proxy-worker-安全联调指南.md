# Proxy（worker）安全联调指南（JWT/Origin/限流）

> 目的：帮助开发/测试快速验证新增的 JWT 鉴权、Origin 白名单与速率限制。默认处于“观察模式”（不拒绝，仅记录）。

## 1. 启用配置（worker/etc/agent.yaml）

Security:
  EnableTLS: false
  SkipVerify: true
  JWT:
    Enabled: true
    Secret: "dev-secret"
    AllowQueryToken: true
    Enforce: false        # 观察模式，联调稳定后改 true
  OriginWhitelist:
    - "http://localhost:5173"
    - "http://127.0.0.1:5173"
  RateLimit:
    Enabled: true
    RequestsPerSecond: 50

说明：
- Enforce=false 时，缺 token 或非法 token 不会拒绝（仅记录日志）。切到 true 后，将返回 401/403。
- Origin 白名单为空数组时不校验；非空时严格匹配。

## 2. 生成开发 Token（HS256）

示例 Go 片段（claims 可按需增减）：

package main
import (
  "fmt"
  "time"
  jwt "github.com/golang-jwt/jwt/v4"
)
func main() {
  claims := jwt.MapClaims{
    "tenantId": "t-001",
    "userId":   "u-001",
    "ciId":     "ci-001",
    "protocol": "ssh",            // ssh|telnet|rdp|vnc
    "exp":      time.Now().Add(30*time.Minute).Unix(),
  }
  token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
  s, _ := token.SignedString([]byte("dev-secret"))
  fmt.Println(s)
}

也可使用 jwt.io 调试（Algorithm=HS256，Secret=dev-secret）。

## 3. HTTP 冒烟（不走 WS）

- 健康检查（不受保护路径）：
  curl -i http://127.0.0.1:8889/health

- 受保护路径（示例：/api/ssh/websocket 握手）
  curl -i \
    -H "Origin: http://localhost:5173" \
    -H "Authorization: Bearer $TOKEN" \
    "http://127.0.0.1:8889/api/ssh/websocket"

注意：直接 curl 握手只验证中间件（返回 400/101 均可接受），后续需用 WS 客户端建链。

## 4. WebSocket 建链

A) Guacamole 模式（RDP/VNC/TELNET/SSH）
- 路径：/api/rdp/websocket | /api/vnc/websocket | /api/telnet/websocket | /api/ssh/websocket
- 示例（RDP）：
  websocat \
    -H="Origin: http://localhost:5173" \
    -H="Authorization: Bearer $TOKEN" \
    "ws://127.0.0.1:8889/api/rdp/websocket?hostname=192.168.1.10&port=3389&username=Administrator&password=***"

B) 统一 SSH 隧道（原生 SSH 插件）
- 路径：/ws/ssh
- 步骤：先建立 WS，再发送 JSON 首帧（SSHTunnelRequest）
  websocat -H="Origin: http://localhost:5173" -H="Authorization: Bearer $TOKEN" \
    ws://127.0.0.1:8889/ws/ssh
  {"target":"192.168.1.10","port":22,"username":"root","password":"***","auth_type":"password","cols":120,"rows":30,"session_id":"s-001"}

若配置 AllowQueryToken=true，可改用：
  ws://127.0.0.1:8889/ws/ssh?token=$TOKEN

附：握手签名（sig）
- 当通过 Ops Center 创建会话时，返回的 wsUrl 会带上查询参数签名 `sig`，其由 `sessionId|protocol|ciId|sorted(params)` 使用 HMAC-SHA256 计算（密钥同 JWT Secret）。
- Worker 在 WS 握手前会自动校验 `sig`，防止参数被篡改（观察模式下仅告警，强制模式下拒绝）。手工联调时无需自算 `sig`，推荐直接使用 Ops Center 的 wsUrl。

## 5. 速率限制验证

将 RPS 暂时调低：RequestsPerSecond: 5，然后执行：
for i in {1..50}; do curl -s -o /dev/null \
  -H "Origin: http://localhost:5173" \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:8889/api/ssh/websocket"; done

预期：部分请求返回 429 Too Many Requests。

### 快速冒烟脚本
可使用脚本一键冒烟验证（需安装 curl 和 openssl）：

```
BASE_URL=http://127.0.0.1:8889 \
JWT_SECRET=dev-secret \
ORIGIN_ALLOWED=http://localhost:5173 \
ORIGIN_BLOCKED=http://evil \
bash worker/scripts/security_smoke.sh
```

脚本覆盖：
- /health 可达性
- 受保护路径无/过期/有效 JWT 的响应
- Origin 白名单预检（OPTIONS）允许/拒绝
- /status 快速多次请求触发 429（若启用限流）

## 6. 日志期望
- 缺 token："JWT missing (observe)"
- Token 过期："JWT expired (observe)"
- 非白名单 Origin："Origin not in whitelist (observe)"
- 速率超限：HTTP 429

## 7. 切换强制模式
- 将 Security.JWT.Enforce 改为 true，重启 worker。
- 预期：缺 token 或非法 token 请求返回 401；非白名单 Origin 返回 403。

## 8. 受保护路径列表（默认）
- /api/rdp/websocket
- /api/vnc/websocket
- /api/telnet/websocket
- /api/ssh/websocket
- /ws/ssh
- /ws/telnet
- /api/db/websocket

（如需扩展为可配置清单，可后续在 Security 下新增 ProtectedPaths）

## 9. 常见问题
- 仍可连上？检查是否处于观察模式（Enforce=false）。
- 401/403 但已带 token？检查 Secret 是否一致；检查 token 是否过期；检查 Origin 是否在白名单。
- RDP/VNC 黑屏？确认 guacd 地址 / 端口正确，网络可达，凭证正确。
