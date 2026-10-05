# nb-agent 对接与演进设计

> 角色：主机侧执行器（命令/文件/巡检/采集），Phase 1 通过 HTTP/REST 由 Ops Center 直接调用；Phase 2/3 引入 Agent↔Proxy 长连接。

## 1. 现状
- 业务路由基于 chi（internal/route/http.go），提供丰富系统管理能力；
- 存在 RemoteTask 模型（Source=proxy/center），但无 Proxy 端任务分发器实现；
- 无统一中间件（integration），多租户/权限由 Ops Center 兜底。

## 2. 对接策略（Phase 1）
- Ops Center 以 HTTP/REST 调用 nb-agent 执行任务（命令/脚本/文件），nb-agent 回传结果。
- 结果与审计由 Ops Center 汇总；nb-agent 仅做执行侧。

## 3. 长连接演进（Phase 2/3）
- 参考 `docs/agent-interface-spec.md`：ConnectionManager/Message 协议/任务执行器接口；
- Agent 向 Proxy 注册与心跳；任务由 Proxy 下发；结果回传至 Ops Center。

## 4. 接口契约（建议）
- POST /agent/task/execute
  - 入参：{ taskId, command:{content,timeout}, targets:[{agentId|host}], env, workingDir }
  - 返回：{ taskId, status }
- GET /agent/task/{id}/status | /result

## 5. 原子任务（含关键点与测试）
1) 任务执行契约对齐
   - 关键点：与 Ops Center 的任务结构映射；
   - 测试：执行/失败/超时路径与幂等。
2) 结果回传结构标准化
   - 关键点：stdout/stderr/exitCode/metadata；
   - 测试：多并发与日志量级。
3)（可选）任务标签/优先级
   - 关键点：资源池隔离；
   - 测试：压力下公平性。
4)（Phase 2/3）Proxy 连接管理
   - 关键点：断连重连、幂等、防重；
   - 测试：抖动网络下稳定性。

## 6. 验收标准
- 批量任务稳定执行；
- 结果语义一致并可追踪；
- 演进不影响现有 HTTP 接口。

