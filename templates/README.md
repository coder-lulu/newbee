# NewBee 引导式服务创建器

NewBee 引导式服务创建器提供一步一步的引导式体验，让您轻松创建完整的微服务架构。

## 🚀 快速开始

### 使用方法

```bash
cd /opt/code/newbee/templates
./newbee-service
```

### 创建流程

工具将引导您完成以下步骤：

1. **服务名称** - 为微服务选择合适的名称
2. **数据库配置** - 配置 MySQL 数据库连接
3. **缓存配置** - 配置 Redis 缓存服务
4. **端口配置** - 分配 API 和 RPC 服务端口
5. **配置确认** - 确认所有设置
6. **创建服务** - 自动生成完整的服务结构

## ✨ 主要特性

- **引导式界面** - 清晰的步骤指引和友好的用户界面
- **智能验证** - 实时检查输入格式和端口占用
- **智能推荐** - 自动推荐端口和配置默认值
- **错误恢复** - 输入错误时支持重新输入
- **配置管理** - 自动保存和跟踪服务配置

## 📋 生成的服务特性

### 架构支持
- **REST API 服务层** - 基于 go-zero 的 HTTP 服务
- **gRPC RPC 服务层** - 高性能的内部服务通信
- **数据库集成** - 支持 MySQL 和 Ent ORM
- **缓存支持** - Redis 集成用于会话和缓存
- **多租户架构** - 内置租户数据隔离
- **统一中间件** - JWT认证、权限控制、审计日志
- **监控集成** - Prometheus 指标采集

### 目录结构

```
服务名/
├── api/                    # API 服务
│   ├── desc/              # API 定义文件
│   ├── etc/服务名.yaml     # API 配置
│   ├── internal/          # 内部实现
│   └── 服务名.go          # API 入口
└── rpc/                   # RPC 服务
    ├── ent/               # 数据库实体
    ├── etc/服务名.yaml     # RPC 配置
    ├── internal/          # 内部实现
    ├── types/             # Proto 类型
    ├── 服务名client/       # RPC 客户端
    ├── 服务名.proto       # Proto 定义
    └── 服务名.go          # RPC 入口
```

## 🔧 开发流程

### 1. 创建服务
```bash
./newbee-service
```

### 2. 定义数据模型
编辑 `rpc/ent/schema/` 目录中的文件：

```go
// rpc/ent/schema/user.go
package schema

import (
    "github.com/coder-lulu/newbee-common/orm/ent/mixins"
    "entgo.io/ent"
    "entgo.io/ent/schema/field"
)

type User struct {
    ent.Schema
}

func (User) Fields() []ent.Field {
    return []ent.Field{
        field.String("username").NotEmpty(),
        field.String("email").NotEmpty(),
    }
}

func (User) Mixin() []ent.Mixin {
    return []ent.Mixin{
        mixins.IDMixin{},
        mixins.StatusMixin{},
        mixins.TenantMixin{}, // 多租户支持
    }
}
```

### 3. 定义 API 接口
编辑 `api/desc/` 目录中的 .api 文件：

```api
info(
    title: "用户服务API"
    desc: "用户管理接口"
    version: "v1.0"
)

type UserInfo {
    Id       uint64 `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
}

@server(
    group: user
)
service User {
    @handler createUser
    post /user/create (UserInfo) returns (BaseMsgResp)
    
    @handler getUserList
    post /user/list (PageInfoReq) returns (UserListResp)
}
```

### 4. 生成和启动
```bash
# 生成数据库代码
cd rpc && go run entgo.io/ent/cmd/ent generate ./ent/schema

# 生成 API 代码
cd ../api && goctl api go -api desc/*.api -dir .

# 启动 RPC 服务
cd ../rpc && go run .

# 启动 API 服务（新终端）
cd ../api && go run .
```

## 📝 配置说明

### 服务命名规则
- 必须以小写字母开头
- 只能包含小写字母、数字和连字符
- 长度在 2-20 个字符之间
- 不能使用保留名称（api, rpc, core 等）

### 端口分配
- API 端口：从 9600 开始自动分配
- RPC 端口：API 端口 + 1
- Prometheus API：API 端口 + 100
- Prometheus RPC：RPC 端口 + 100

## 🔍 故障排除

### 常见问题

**1. 缺少 jq 工具**
```bash
# Ubuntu/Debian
sudo apt install jq

# CentOS/RHEL
sudo yum install jq

# macOS
brew install jq
```

**2. 端口被占用**
- 工具会自动检测端口占用并推荐可用端口
- 也可以手动指定其他端口

**3. 权限问题**
- 确保对项目目录有写权限
- 检查模板目录是否存在

**4. 服务已存在**
- 工具会询问是否覆盖现有服务
- 或选择不同的服务名称

## 🎯 最佳实践

### 开发建议
1. **服务命名**：使用简洁、描述性的名称
2. **数据建模**：充分利用 TenantMixin 实现多租户
3. **API 设计**：遵循 RESTful 规范
4. **错误处理**：使用统一的错误处理机制
5. **测试**：编写完整的单元测试和集成测试

### 架构原则
- 遵循 NewBee 编码准则（参考 CLAUDE.md）
- 使用统一中间件框架
- 确保多租户数据隔离
- 实现完整的监控和日志

---

**版本**: v2.1.0  
**更新时间**: 2024-09  
**维护者**: NewBee Team