# UI：运维中心 菜单与权限设计

> 前端：ui/（pnpm monorepo, Vue3 + Vite + @vben），后端权限初始化由 core/rpc 初始化脚本完成（对齐现有风格）。

## 1. 菜单结构
- 运维中心（ops:dashboard:view）
  - 会话管理（ops:session:list）
    - 新建会话（ops:session:create）
    - 进行中会话（ops:session:active）
    - 历史/回放（ops:session:audit）
  - 任务编排（ops:task:list）
    - 新建任务（ops:task:create）
    - 历史与日志（ops:task:logs）
  - 代理节点（ops:proxy:list）
    - 注册 & 健康（ops:proxy:health）
  - 访问画像（ops:profile:list）
    - 编辑（ops:profile:edit）
  - 凭证绑定（ops:credential:bind）
    - 短期凭证（ops:credential:issue）
  - 审计与报表（ops:audit:view）

## 2. 路由建议
- /ops/dashboard
- /ops/session (list/detail)
- /ops/task (list/detail/logs)
- /ops/proxy (list/detail)
- /ops/profile (list/edit)
- /ops/credential (list/bind)
- /ops/audit (search/detail)

## 3. 权限点与后端对齐
- 权限码与后端接口一一对应，初始化方式参考 core/rpc 的菜单/权限种子（init_database_menu_data.go 的风格）。

## 4. 原子任务
1) 菜单/路由骨架与权限指令（v-auth 指令绑定）
   - 测试：不同角色下的可见性；
2) 会话列表/建立/回放入口 & WS 连接（使用 token）
   - 测试：合法令牌连接成功；过期/伪造拒绝；
3) 任务编排页 & 批量执行表单
   - 测试：校验与执行回传；
4) 代理节点列表 & 健康状态可视化
   - 测试：多 Proxy 状态刷新；
5) 访问画像表单 & 绑定 CI 选择器
   - 测试：CI 查询与权限校验；
6) 审计查询与导出
   - 测试：权限过滤与导出文件校验；

## 5. 验收标准
- 权限驱动菜单/按钮生效；
- 会话与任务可用；
- 页面错误处理与空态/加载态完善。

