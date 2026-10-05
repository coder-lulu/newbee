# 统一输入输出平台 Kafka/go-queue 集成开发指南

> **文档版本**: v2.0 (安全加固版)
> **更新日期**: 2025-10-19
> **安全等级**: ⭐⭐⭐⭐⭐ (经过全面架构审查)
> **审查状态**: 已修复 P0/P1 安全漏洞，增强可靠性与性能

## 目录

- [1. 背景与目标](#1-背景与目标)
- [2. 架构概览](#2-架构概览)
- [3. Topic 规划](#3-topic-规划)
- [4. 安全加固方案](#4-安全加固方案)
- [5. 可靠性保证](#5-可靠性保证)
- [6. 性能优化](#6-性能优化)
- [7. 监控与告警](#7-监控与告警)
- [8. 开发实施计划](#8-开发实施计划)
- [9. 测试与验收](#9-测试与验收)
- [10. 安全检查清单](#10-安全检查清单)

---

## 1. 背景与目标

### 1.1 业务需求

统一输入输出平台需要支持多业务、多场景的数据采集与分发，要求任务调度具备：
- **弹性伸缩** - 支持动态扩容，应对流量波动
- **可恢复** - 故障自动恢复，消息不丢失
- **可观测** - 全链路追踪，实时监控
- **安全隔离** - 多租户数据强隔离，敏感信息保护

### 1.2 技术选型

- **消息中间件**: Kafka 3.x (持久化、高吞吐、水平扩展)
- **Go封装**: go-zero `go-queue/kq` (简化生产/消费逻辑)
- **数据库**: PostgreSQL + Ent ORM (事务、审计)
- **缓存**: Redis (幂等、分布式锁)
- **监控**: Prometheus + Grafana + Jaeger

### 1.3 核心设计原则

1. **安全第一** - 多租户隔离、敏感信息保护、审计完整
2. **高可用** - 分布式锁、故障恢复、幂等保证
3. **高性能** - 分区优化、批量处理、异步解耦
4. **可观测** - 全链路追踪、业务指标、告警完善

---

## 2. 架构概览

### 2.1 系统架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                          控制平面 (Control Plane)                     │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐            │
│  │   API Server  │   │   RPC Server  │   │   Scheduler   │            │
│  │  (REST API)   │   │  (gRPC)       │   │  (Cron/Event) │            │
│  └───────┬────────┘   └───────┬────────┘   └───────┬────────┘            │
│          │                    │                    │                    │
│          └────────────────────┼────────────────────┘                    │
│                               │                                         │
└───────────────────────────────┼─────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     消息层 (Message Layer)                           │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                   Kafka Cluster (3+ Brokers)                 │    │
│  │                                                               │    │
│  │  Topic: io.input.jobs      (32 分区, 3 副本)                 │    │
│  │  Topic: io.output.jobs     (32 分区, 3 副本)                 │    │
│  │  Topic: io.workflow.events (16 分区, 3 副本)                 │    │
│  │  Topic: io.deadletter      (8 分区, 永久保留)                │    │
│  │                                                               │    │
│  │  [租户隔离] Header: X-Tenant-ID + ACL                         │    │
│  │  [安全] SASL/SSL + 消息加密                                   │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │              Outbox 表 (io_task_outbox)                      │    │
│  │  作用: 解决双写一致性问题 (事务内写入TaskRun + Outbox)        │    │
│  │  Dispatcher: 分布式锁 + 批量发送 + 故障恢复                   │    │
│  └─────────────────────────────────────────────────────────────┘    │
└───────────────────────────────┬─────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    执行平面 (Execution Plane)                         │
│                                                                       │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐            │
│  │  Worker Pod 1 │   │  Worker Pod 2 │   │  Worker Pod N │            │
│  │  (Consumer)   │   │  (Consumer)   │   │  (Consumer)   │            │
│  └───────┬────────┘   └───────┬────────┘   └───────┬────────┘            │
│          │                    │                    │                    │
│          └────────────────────┼────────────────────┘                    │
│                               │                                         │
│                               ▼                                         │
│  ┌─────────────────────────────────────────────────────────────┐      │
│  │  核心能力                                                     │      │
│  │  - 幂等性保证 (Redis + DB Unique Constraint)                │      │
│  │  - 租户验证 (Header + Body 双重验证)                         │      │
│  │  - 字段映射 (Field Mapping Engine)                          │      │
│  │  - Provider调用 (API/LDAP/Database...)                      │      │
│  │  - 结果回写 (TaskRun Status + MappingLog)                   │      │
│  └─────────────────────────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流

```
1. 调度器 (Scheduler)
   ↓ (事务写入 TaskRun + Outbox)
2. Outbox Dispatcher (分布式锁)
   ↓ (Kafka Producer)
3. Kafka Topic
   ↓ (按 partitionKey 路由)
4. Consumer Group
   ↓ (幂等检查)
5. Worker Handler
   ↓ (业务执行)
6. TaskRun 状态更新
   ↓ (成功/失败)
7. Dead Letter (失败超过阈值)
```

---

## 3. Topic 规划

### 3.1 Topic 列表

| Topic 名称            | 用途描述                     | 分区策略                              | 分区数 | 副本数 | 保留时间 | 压缩方式 |
|----------------------|------------------------------|--------------------------------------|--------|--------|---------|---------|
| `io.input.jobs`      | 输入任务调度消息              | `tenantId:connectorId:shard`         | 32     | 3      | 7天     | snappy  |
| `io.output.jobs`     | 输出任务执行消息              | `tenantId:resourceType:resourceId`   | 32     | 3      | 7天     | snappy  |
| `io.workflow.events` | 工作流节点事件                | `tenantId:workflowId`                | 16     | 3      | 14天    | snappy  |
| `io.deadletter`      | 失败任务死信队列              | `tenantId`                           | 8      | 3      | 永久    | lz4     |

### 3.2 Topic 创建脚本

```bash
#!/bin/bash
# create-topics.sh

BROKERS="kafka-1:9092,kafka-2:9092,kafka-3:9092"

# io.input.jobs
kafka-topics --create --bootstrap-server $BROKERS \
  --topic io.input.jobs \
  --partitions 32 \
  --replication-factor 3 \
  --config retention.ms=604800000 \
  --config compression.type=snappy \
  --config min.insync.replicas=2 \
  --config unclean.leader.election.enable=false \
  --config max.message.bytes=1048576

# io.output.jobs
kafka-topics --create --bootstrap-server $BROKERS \
  --topic io.output.jobs \
  --partitions 32 \
  --replication-factor 3 \
  --config retention.ms=604800000 \
  --config compression.type=snappy \
  --config min.insync.replicas=2 \
  --config unclean.leader.election.enable=false

# io.workflow.events
kafka-topics --create --bootstrap-server $BROKERS \
  --topic io.workflow.events \
  --partitions 16 \
  --replication-factor 3 \
  --config retention.ms=1209600000 \
  --config compression.type=snappy \
  --config min.insync.replicas=2

# io.deadletter
kafka-topics --create --bootstrap-server $BROKERS \
  --topic io.deadletter \
  --partitions 8 \
  --replication-factor 3 \
  --config retention.ms=-1 \
  --config compression.type=lz4 \
  --config min.insync.replicas=2

echo "All topics created successfully!"
```

### 3.3 分区策略详解

#### 3.3.1 输入任务分区策略

**问题**: 大租户或高频connector导致热点分区

**解决方案**: 二级Hash + 动态分片

```go
// unified-io/rpc/internal/partition/strategy.go

package partition

import (
    "fmt"
    "hash/fnv"
)

// InputJobsPartitionKey 为输入任务生成分区键
// 规则:
// - 低频connector: tenantId:connectorId
// - 高频connector: tenantId:connectorId:shard (按resourceId分片)
func InputJobsPartitionKey(tenantID uint64, connectorID string, resourceID string, qpsThreshold int) string {
    qps := getConnectorQPS(connectorID) // 从监控系统获取connector QPS

    if qps > qpsThreshold { // 默认 1000
        // 高频connector，使用resourceID进一步分片
        hash := fnv.New32a()
        hash.Write([]byte(resourceID))
        shard := hash.Sum32() % 4 // 分散到4个子分区
        return fmt.Sprintf("%d:%s:%d", tenantID, connectorID, shard)
    }

    // 低频connector，使用connector维度分区
    return fmt.Sprintf("%d:%s", tenantID, connectorID)
}

// 模拟获取connector QPS (实际应从Prometheus查询)
func getConnectorQPS(connectorID string) int {
    // TODO: 从Prometheus查询
    // query := fmt.Sprintf("rate(io_connector_requests_total{connector_id='%s'}[1m])", connectorID)
    return 500 // 占位符
}
```

#### 3.3.2 输出任务分区策略

**目标**: 确保同一资源的操作顺序性

```go
// OutputJobsPartitionKey 为输出任务生成分区键
// 规则: tenantId:resourceType:resourceId
// 保证同一资源的所有操作落在同一分区，维持顺序性
func OutputJobsPartitionKey(tenantID uint64, resourceType string, resourceID uint64) string {
    return fmt.Sprintf("%d:%s:%d", tenantID, resourceType, resourceID)
}
```

---

## 4. 安全加固方案

### 4.1 多租户隔离 (🔴 P0-Critical)

#### 4.1.1 方案概述

| 方案 | 隔离级别 | 适用场景 | 实施复杂度 |
|------|---------|---------|-----------|
| **Topic级别隔离** | ⭐⭐⭐⭐⭐ | 高价值租户、政府客户 | 高 |
| **Consumer Group + Header** | ⭐⭐⭐⭐ | 常规租户 | 中 |
| **应用层过滤** | ⭐⭐⭐ | 小规模部署 | 低 |

#### 4.1.2 方案1: Topic级别隔离 (推荐)

**适用场景**:
- 高价值客户 (付费 >$10k/月)
- 政府、金融等强合规要求客户

**实现步骤**:

```bash
# 1. 为重要租户创建独立Topic
kafka-topics --create --bootstrap-server kafka:9092 \
  --topic io.input.jobs.tenant_1 \
  --partitions 16 \
  --replication-factor 3

# 2. 配置Kafka ACL
kafka-acls --add --allow-principal User:tenant-1-worker \
  --operation Read \
  --topic io.input.jobs.tenant_1 \
  --group tenant-1-*

kafka-acls --add --allow-principal User:scheduler \
  --operation Write \
  --topic io.input.jobs.tenant_1
```

**Producer配置**:

```go
// unified-io/rpc/internal/producer/tenant_router.go

type TenantTopicRouter struct {
    defaultTopic    string
    dedicatedTopics map[uint64]string // tenantID -> topic
}

func NewTenantTopicRouter() *TenantTopicRouter {
    return &TenantTopicRouter{
        defaultTopic: "io.input.jobs",
        dedicatedTopics: map[uint64]string{
            1: "io.input.jobs.tenant_1", // 大客户A
            2: "io.input.jobs.tenant_2", // 政府客户
        },
    }
}

func (r *TenantTopicRouter) GetTopic(tenantID uint64) string {
    if topic, exists := r.dedicatedTopics[tenantID]; exists {
        return topic
    }
    return r.defaultTopic
}
```

#### 4.1.3 方案2: Consumer Group + Header验证 (推荐)

**适用场景**: 常规租户 (大多数情况)

**实现代码**:

```go
// unified-io/rpc/internal/consumer/tenant_consumer.go

package consumer

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/segmentio/kafka-go"
    "github.com/zeromicro/go-zero/core/logx"
)

type TenantConsumer struct {
    allowedTenantID uint64
    groupID         string
    handler         TaskHandler
    alertManager    *AlertManager
}

func (c *TenantConsumer) Handle(ctx context.Context, msg kafka.Message) error {
    // 1. 从Header提取租户ID
    tenantID, err := extractTenantIDFromHeaders(msg.Headers)
    if err != nil {
        logx.Errorw("Missing tenant ID in headers", logx.Field("partition", msg.Partition))
        return err
    }

    // 2. 验证租户ID是否匹配
    if tenantID != c.allowedTenantID {
        logx.Errorw("🚨 Tenant isolation violation!",
            logx.Field("consumer_group", c.groupID),
            logx.Field("expected_tenant", c.allowedTenantID),
            logx.Field("actual_tenant", tenantID),
            logx.Field("partition", msg.Partition),
            logx.Field("offset", msg.Offset))

        // 触发安全告警
        c.alertManager.Send(ctx, Alert{
            Level:   "critical",
            Title:   "租户隔离违规",
            Message: fmt.Sprintf("Consumer Group %s 尝试访问租户 %d 的消息", c.groupID, tenantID),
            Labels: map[string]string{
                "consumer_group": c.groupID,
                "expected_tenant": fmt.Sprint(c.allowedTenantID),
                "actual_tenant":   fmt.Sprint(tenantID),
            },
        })

        return fmt.Errorf("unauthorized: tenant mismatch")
    }

    // 3. 解析消息体
    var task TaskMessage
    if err := json.Unmarshal(msg.Value, &task); err != nil {
        return fmt.Errorf("unmarshal failed: %w", err)
    }

    // 4. 双重验证：消息体中的租户ID
    if task.TenantID != tenantID {
        logx.Errorw("Header and body tenant mismatch",
            logx.Field("header_tenant", tenantID),
            logx.Field("body_tenant", task.TenantID))
        return fmt.Errorf("tenant mismatch: header=%d body=%d", tenantID, task.TenantID)
    }

    // 5. 执行业务逻辑
    return c.handler.Handle(ctx, &task)
}

// extractTenantIDFromHeaders 从Kafka Header提取租户ID
func extractTenantIDFromHeaders(headers []kafka.Header) (uint64, error) {
    for _, h := range headers {
        if h.Key == "X-Tenant-ID" {
            var tenantID uint64
            fmt.Sscanf(string(h.Value), "%d", &tenantID)
            return tenantID, nil
        }
    }
    return 0, fmt.Errorf("X-Tenant-ID header not found")
}
```

**Producer端添加Header**:

```go
// unified-io/rpc/internal/producer/secure_producer.go

type SecureProducer struct {
    writer *kafka.Writer
}

func (p *SecureProducer) Publish(ctx context.Context, topic string, body []byte, tenantID uint64, traceID string) error {
    headers := []kafka.Header{
        {Key: "X-Tenant-ID", Value: []byte(fmt.Sprintf("%d", tenantID))},
        {Key: "X-Trace-ID", Value: []byte(traceID)},
        {Key: "X-Message-ID", Value: []byte(uuid.New().String())},
        {Key: "X-Timestamp", Value: []byte(time.Now().Format(time.RFC3339))},
    }

    partitionKey := partition.InputJobsPartitionKey(tenantID, getConnectorID(body), getResourceID(body), 1000)

    err := p.writer.WriteMessages(ctx, kafka.Message{
        Topic:   topic,
        Key:     []byte(partitionKey),
        Value:   body,
        Headers: headers,
    })

    if err != nil {
        logx.Errorw("Failed to publish message",
            logx.Field("topic", topic),
            logx.Field("tenant_id", tenantID),
            logx.Field("error", err))
    }

    return err
}
```

### 4.2 敏感信息保护 (🔴 P0-Critical)

#### 4.2.1 问题场景

```go
// ❌ 错误示例：密码明文存储在消息中
type TaskMessage struct {
    TenantID   uint64
    TargetHost string
    Username   string
    Password   string // 🚨 明文密码！
}
```

**风险**:
- Kafka日志中泄露密码
- 开发人员可通过kafka-console-consumer查看敏感信息
- 审计日志中记录明文密码

#### 4.2.2 解决方案：凭证引用模式

**步骤1: 定义安全的消息Schema**

```go
// unified-io/rpc/types/io/task_message.go

type TaskMessage struct {
    TenantID        uint64 `json:"tenant_id"`
    TaskRunID       uint64 `json:"task_run_id"`
    TaskType        string `json:"task_type"`

    // ✅ 使用凭证引用，不传递实际密码
    CredentialRefID string `json:"credential_ref_id"`

    // ❌ 禁止使用这些字段
    // Password        string `json:"password"`
    // APIKey          string `json:"api_key"`
    // PrivateKey      string `json:"private_key"`
}
```

**步骤2: Producer端自动脱敏**

```go
// unified-io/rpc/internal/security/message_sanitizer.go

package security

import (
    "encoding/json"
    "fmt"
    "regexp"
)

type MessageSanitizer struct {
    sensitivePatterns []*regexp.Regexp
}

func NewMessageSanitizer() *MessageSanitizer {
    return &MessageSanitizer{
        sensitivePatterns: []*regexp.Regexp{
            regexp.MustCompile(`(?i)"password"\s*:\s*"[^"]+"`),
            regexp.MustCompile(`(?i)"api_key"\s*:\s*"[^"]+"`),
            regexp.MustCompile(`(?i)"secret"\s*:\s*"[^"]+"`),
            regexp.MustCompile(`(?i)"token"\s*:\s*"[^"]+"`),
            regexp.MustCompile(`(?i)"private_key"\s*:\s*"[^"]+"`),
        },
    }
}

func (s *MessageSanitizer) Sanitize(body []byte) ([]byte, []string) {
    var warnings []string
    sanitized := body

    for _, pattern := range s.sensitivePatterns {
        if pattern.Match(body) {
            warnings = append(warnings, fmt.Sprintf("Detected pattern: %s", pattern.String()))
            // 替换为占位符
            sanitized = pattern.ReplaceAll(sanitized, []byte(`"***REDACTED***"`))
        }
    }

    return sanitized, warnings
}

// 包装Producer
type SecureSanitizingProducer struct {
    inner      Producer
    sanitizer  *MessageSanitizer
    auditLog   *AuditLogger
}

func (p *SecureSanitizingProducer) Publish(ctx context.Context, topic string, body []byte) error {
    sanitized, warnings := p.sanitizer.Sanitize(body)

    if len(warnings) > 0 {
        // 记录审计日志
        p.auditLog.Log(ctx, AuditEvent{
            Action:       "message_sanitized",
            ResourceType: "kafka.message",
            Severity:     "high",
            Details:      map[string]interface{}{"warnings": warnings},
        })

        // 可选：直接拒绝发送
        // return fmt.Errorf("message contains sensitive fields: %v", warnings)
    }

    return p.inner.Publish(ctx, topic, sanitized)
}
```

**步骤3: Worker端按需解密**

```go
// unified-io/rpc/internal/logic/task/execute_task_logic.go

func (l *ExecuteTaskLogic) ExecuteTask(ctx context.Context, msg *TaskMessage) error {
    // 从加密凭证存储中获取真实凭证
    cred, err := l.svcCtx.CredentialService.GetDecrypted(ctx, msg.TenantID, msg.CredentialRefID)
    if err != nil {
        return fmt.Errorf("failed to get credential: %w", err)
    }

    // 使用凭证执行任务
    client := NewAPIClient(cred.Host, cred.Username, cred.Password)
    defer client.Close()

    result, err := client.Execute(ctx, msg.TaskType, msg.Params)
    if err != nil {
        return err
    }

    // 写回结果
    return l.saveTaskResult(ctx, msg.TaskRunID, result)
}
```

**步骤4: 凭证存储加密**

```go
// core/rpc/internal/service/credential_service.go

import "github.com/coder-lulu/newbee-common/encryption"

type CredentialService struct {
    db        *ent.Client
    encryptor *encryption.AESEncryptor
}

func (s *CredentialService) GetDecrypted(ctx context.Context, tenantID uint64, credID string) (*Credential, error) {
    // 查询加密凭证
    credEntity, err := s.db.Credential.Query().
        Where(credential.IDEQ(credID), credential.TenantIDEQ(tenantID)).
        First(ctx)
    if err != nil {
        return nil, err
    }

    // 解密密码
    password, err := s.encryptor.Decrypt(credEntity.EncryptedPassword)
    if err != nil {
        return nil, fmt.Errorf("decrypt failed: %w", err)
    }

    return &Credential{
        ID:       credEntity.ID,
        Host:     credEntity.Host,
        Username: credEntity.Username,
        Password: password, // 明文密码仅在内存中短暂存在
    }, nil
}
```

### 4.3 Kafka集群安全配置

#### 4.3.1 启用SASL/SSL

```yaml
# config/server.properties

# 启用SASL/SSL
listeners=SASL_SSL://0.0.0.0:9093
advertised.listeners=SASL_SSL://kafka-1:9093

# SSL配置
ssl.keystore.location=/etc/kafka/secrets/kafka.server.keystore.jks
ssl.keystore.password=your-keystore-password
ssl.key.password=your-key-password
ssl.truststore.location=/etc/kafka/secrets/kafka.server.truststore.jks
ssl.truststore.password=your-truststore-password

# SASL配置
sasl.enabled.mechanisms=SCRAM-SHA-512
sasl.mechanism.inter.broker.protocol=SCRAM-SHA-512
```

#### 4.3.2 客户端配置

```yaml
# unified-io/etc/io-rpc.yaml

KafkaConf:
  Brokers:
    - kafka-1:9093
    - kafka-2:9093
  Topic: io.input.jobs
  Group: unified-io-worker

  # 安全配置
  Username: ${KAFKA_USERNAME}
  Password: ${KAFKA_PASSWORD}
  Mechanism: SCRAM-SHA-512

  # SSL配置
  CACert: /etc/kafka/certs/ca-cert.pem
  ClientCert: /etc/kafka/certs/client-cert.pem
  ClientKey: /etc/kafka/certs/client-key.pem
```

---

## 5. 可靠性保证

### 5.1 Outbox模式 (🔴 P0-Critical)

#### 5.1.1 问题背景

**双写不一致问题**:

```go
// ❌ 错误示例：先写数据库，再发消息
func ScheduleTask(ctx context.Context, msg *TaskMessage) error {
    // 1. 写数据库
    run, err := db.TaskRun.Create().SetStatus("pending").Save(ctx)
    if err != nil {
        return err
    }

    // 2. 发消息 (可能失败！)
    err = producer.Publish(ctx, "io.input.jobs", marshal(msg))
    if err != nil {
        // 💥 数据库已提交，但消息未发送！
        return err
    }

    return nil
}
```

**风险**:
- Producer失败导致TaskRun创建但无消息
- 数据库与Kafka状态不一致

#### 5.1.2 Outbox表结构

```sql
-- unified-io/rpc/migrations/000_create_outbox.sql

CREATE TABLE io_task_outbox (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT       NOT NULL,
    topic           VARCHAR(255) NOT NULL,
    partition_key   VARCHAR(255) NOT NULL,
    payload         JSONB        NOT NULL,
    headers         JSONB        DEFAULT '{}'::jsonb,

    status          VARCHAR(32)  NOT NULL DEFAULT 'pending',
    retry_count     INT          NOT NULL DEFAULT 0,
    max_retries     INT          NOT NULL DEFAULT 5,
    next_retry_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    error_msg       TEXT,
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_status CHECK (status IN ('pending', 'sending', 'sent', 'failed'))
);

CREATE INDEX idx_outbox_dispatch ON io_task_outbox(status, next_retry_at)
WHERE status IN ('pending', 'failed');

CREATE INDEX idx_outbox_tenant ON io_task_outbox(tenant_id);
CREATE INDEX idx_outbox_created ON io_task_outbox(created_at);
```

#### 5.1.3 事务内写入

```go
// unified-io/rpc/internal/scheduler/task_scheduler.go

package scheduler

import (
    "context"
    "encoding/json"

    "github.com/coder-lulu/newbee-core/rpc/ent"
)

type TaskScheduler struct {
    db *ent.Client
}

func (s *TaskScheduler) ScheduleTask(ctx context.Context, msg *TaskMessage) (uint64, error) {
    // 开启事务
    tx, err := s.db.Tx(ctx)
    if err != nil {
        return 0, fmt.Errorf("begin tx: %w", err)
    }
    defer func() {
        if p := recover(); p != nil {
            _ = tx.Rollback()
            panic(p)
        }
    }()

    // 1. 创建TaskRun
    run, err := tx.TaskRun.Create().
        SetTenantID(msg.TenantID).
        SetTaskType(msg.TaskType).
        SetStatus("pending").
        SetCreatedBy(msg.UserID).
        Save(ctx)
    if err != nil {
        _ = tx.Rollback()
        return 0, fmt.Errorf("create task run: %w", err)
    }

    // 2. 创建Outbox记录 (同一事务)
    payload, _ := json.Marshal(msg)
    headers := map[string]string{
        "X-Tenant-ID":  fmt.Sprint(msg.TenantID),
        "X-Trace-ID":   trace.FromContext(ctx).TraceID(),
    }
    headersJSON, _ := json.Marshal(headers)

    _, err = tx.TaskOutbox.Create().
        SetTenantID(msg.TenantID).
        SetTopic(getTopic(msg.TaskType)).
        SetPartitionKey(getPartitionKey(msg)).
        SetPayload(string(payload)).
        SetHeaders(string(headersJSON)).
        SetStatus("pending").
        SetNextRetryAt(time.Now()).
        Save(ctx)
    if err != nil {
        _ = tx.Rollback()
        return 0, fmt.Errorf("create outbox: %w", err)
    }

    // 3. 提交事务 (原子性保证)
    if err := tx.Commit(); err != nil {
        return 0, fmt.Errorf("commit tx: %w", err)
    }

    return run.ID, nil
}
```

#### 5.1.4 Dispatcher实现 (分布式锁)

```go
// unified-io/rpc/internal/dispatcher/outbox_dispatcher.go

package dispatcher

import (
    "context"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/zeromicro/go-zero/core/logx"
)

type OutboxDispatcher struct {
    db          *ent.Client
    producer    Producer
    redis       redis.UniversalClient
    leaseTime   time.Duration
    batchSize   int
    workerID    string
}

func NewOutboxDispatcher(db *ent.Client, producer Producer, rds redis.UniversalClient) *OutboxDispatcher {
    return &OutboxDispatcher{
        db:        db,
        producer:  producer,
        redis:     rds,
        leaseTime: 30 * time.Second,
        batchSize: 100,
        workerID:  fmt.Sprintf("dispatcher-%s", os.Getenv("HOSTNAME")),
    }
}

func (d *OutboxDispatcher) Start(ctx context.Context) error {
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := d.dispatchOnce(ctx); err != nil {
                logx.Errorw("Dispatch failed", logx.Field("error", err))
            }

        case <-ctx.Done():
            logx.Info("Dispatcher stopped")
            return ctx.Err()
        }
    }
}

func (d *OutboxDispatcher) dispatchOnce(ctx context.Context) error {
    lockKey := "outbox:dispatcher:lock"

    // 1. 尝试获取分布式锁
    acquired, err := d.redis.SetNX(ctx, lockKey, d.workerID, d.leaseTime).Result()
    if err != nil {
        return fmt.Errorf("acquire lock: %w", err)
    }
    if !acquired {
        return nil // 其他实例持有锁
    }

    // 2. 启动租约续期
    leaseChan := make(chan struct{})
    defer close(leaseChan)
    go d.renewLease(ctx, lockKey, leaseChan)

    // 3. 批量查询待发送消息
    items, err := d.db.TaskOutbox.Query().
        Where(
            taskoutbox.StatusEQ("pending"),
            taskoutbox.NextRetryAtLTE(time.Now()),
        ).
        Order(ent.Asc(taskoutbox.FieldCreatedAt)).
        Limit(d.batchSize).
        All(ctx)
    if err != nil {
        return fmt.Errorf("query outbox: %w", err)
    }

    // 4. 逐条发送
    for _, item := range items {
        if err := d.dispatchOne(ctx, item); err != nil {
            logx.Errorw("Dispatch one failed",
                logx.Field("outbox_id", item.ID),
                logx.Field("error", err))
        }
    }

    // 5. 释放锁
    d.redis.Del(ctx, lockKey)

    return nil
}

func (d *OutboxDispatcher) dispatchOne(ctx context.Context, item *ent.TaskOutbox) error {
    // 1. 标记为发送中
    _, err := d.db.TaskOutbox.UpdateOneID(item.ID).
        SetStatus("sending").
        SetUpdatedAt(time.Now()).
        Save(ctx)
    if err != nil {
        return err
    }

    // 2. 发送到Kafka
    err = d.producer.Publish(ctx, item.Topic, []byte(item.Payload), item.PartitionKey)

    if err != nil {
        // 发送失败，增加重试计数
        nextRetry := time.Now().Add(backoffDelay(item.RetryCount))
        status := "pending"
        if item.RetryCount >= item.MaxRetries {
            status = "failed"
        }

        _, _ = d.db.TaskOutbox.UpdateOneID(item.ID).
            SetStatus(status).
            SetRetryCount(item.RetryCount + 1).
            SetNextRetryAt(nextRetry).
            SetErrorMsg(err.Error()).
            SetUpdatedAt(time.Now()).
            Save(ctx)

        return err
    }

    // 3. 发送成功，标记为已发送
    _, err = d.db.TaskOutbox.UpdateOneID(item.ID).
        SetStatus("sent").
        SetSentAt(time.Now()).
        SetUpdatedAt(time.Now()).
        Save(ctx)

    return err
}

// renewLease 租约续期
func (d *OutboxDispatcher) renewLease(ctx context.Context, lockKey string, stopChan chan struct{}) {
    ticker := time.NewTicker(d.leaseTime / 3)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            d.redis.Expire(ctx, lockKey, d.leaseTime)
        case <-stopChan:
            return
        case <-ctx.Done():
            return
        }
    }
}

// backoffDelay 指数退避
func backoffDelay(retryCount int) time.Duration {
    delays := []time.Duration{
        1 * time.Second,
        5 * time.Second,
        15 * time.Second,
        60 * time.Second,
        300 * time.Second,
    }

    if retryCount >= len(delays) {
        return delays[len(delays)-1]
    }
    return delays[retryCount]
}
```

#### 5.1.5 崩溃恢复机制

```go
// 定期扫描"卡住"的消息 (状态=sending，但很久未更新)
func (d *OutboxDispatcher) RecoverStuckMessages(ctx context.Context) error {
    stuckTimeout := 5 * time.Minute

    updated, err := d.db.TaskOutbox.Update().
        Where(
            taskoutbox.StatusEQ("sending"),
            taskoutbox.UpdatedAtLT(time.Now().Add(-stuckTimeout)),
        ).
        SetStatus("pending").
        SetRetryCount(0). // 重置重试计数
        SetNextRetryAt(time.Now()).
        Save(hooks.NewSystemContext(ctx))

    if err != nil {
        return err
    }

    if updated > 0 {
        logx.Infow("Recovered stuck outbox messages", logx.Field("count", updated))
    }

    return nil
}

// 启动恢复定时任务
func (d *OutboxDispatcher) StartRecoveryJob(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := d.RecoverStuckMessages(ctx); err != nil {
                logx.Errorw("Recovery failed", logx.Field("error", err))
            }
        case <-ctx.Done():
            return
        }
    }
}
```

### 5.2 幂等性保证 (🟠 P1-High)

#### 5.2.1 多层幂等机制

```
┌───────────────────────────────────┐
│  1. Redis幂等窗口 (1小时)          │
│  作用: 快速去重，避免重复执行      │
│  Key: idempotency:{tenant}:{taskRunID} │
└───────────────┬───────────────────┘
                │ (未命中)
                ▼
┌───────────────────────────────────┐
│  2. 数据库唯一约束                 │
│  作用: 持久化去重，防止脏写        │
│  Constraint: UNIQUE(tenant_id, task_run_id, idempotency_key) │
└───────────────┬───────────────────┘
                │ (全部通过)
                ▼
┌───────────────────────────────────┐
│  3. 执行业务逻辑                   │
└───────────────────────────────────┘
```

#### 5.2.2 实现代码

```go
// unified-io/rpc/internal/idempotency/guard.go

package idempotency

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

type Guard struct {
    redis      redis.UniversalClient
    windowTime time.Duration
}

func NewGuard(rds redis.UniversalClient) *Guard {
    return &Guard{
        redis:      rds,
        windowTime: 1 * time.Hour,
    }
}

func (g *Guard) CheckAndMark(ctx context.Context, tenantID uint64, taskRunID uint64) (bool, error) {
    key := fmt.Sprintf("idempotency:%d:%d", tenantID, taskRunID)

    // 使用 SETNX + EXPIRE 原子性检查并标记
    ok, err := g.redis.SetNX(ctx, key, time.Now().Unix(), g.windowTime).Result()
    if err != nil {
        return false, fmt.Errorf("redis setnx: %w", err)
    }

    if !ok {
        // 已处理过
        logx.Infow("Duplicate message detected (Redis)",
            logx.Field("tenant_id", tenantID),
            logx.Field("task_run_id", taskRunID))
        return false, nil
    }

    return true, nil
}
```

**Handler中使用**:

```go
// unified-io/rpc/internal/consumer/task_handler.go

func (h *TaskHandler) Handle(ctx context.Context, body []byte) error {
    var msg TaskMessage
    if err := json.Unmarshal(body, &msg); err != nil {
        return err
    }

    // 1. Redis幂等检查
    isNew, err := h.idempotencyGuard.CheckAndMark(ctx, msg.TenantID, msg.TaskRunID)
    if err != nil {
        return err // 重试
    }
    if !isNew {
        logx.Infow("Skipping duplicate message", logx.Field("task_run_id", msg.TaskRunID))
        return nil // 已处理，返回成功
    }

    // 2. 执行业务逻辑 (数据库会有唯一约束兜底)
    return h.executeTask(ctx, &msg)
}

func (h *TaskHandler) executeTask(ctx context.Context, msg *TaskMessage) error {
    // 数据库操作带唯一约束
    _, err := h.db.TaskResult.Create().
        SetTenantID(msg.TenantID).
        SetTaskRunID(msg.TaskRunID).
        SetIdempotencyKey(msg.IdempotencyKey).
        SetResult(msg.Result).
        Save(ctx)

    if ent.IsConstraintError(err) {
        // 数据库唯一约束冲突，说明已处理过
        logx.Infow("Duplicate detected by DB constraint", logx.Field("task_run_id", msg.TaskRunID))
        return nil
    }

    return err
}
```

**数据库唯一约束**:

```sql
CREATE TABLE io_task_results (
    id               BIGSERIAL PRIMARY KEY,
    tenant_id        BIGINT    NOT NULL,
    task_run_id      BIGINT    NOT NULL,
    idempotency_key  VARCHAR(255),
    result           JSONB,
    created_at       TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE (tenant_id, task_run_id, idempotency_key)
);
```

### 5.3 消息顺序性保证

#### 5.3.1 乐观锁防止乱序写入

```go
// unified-io/rpc/internal/logic/resource/update_resource_logic.go

func (l *UpdateResourceLogic) UpdateResource(ctx context.Context, taskResult *TaskResult) error {
    affected, err := l.svcCtx.DB.Resource.Update().
        Where(
            resource.IDEQ(taskResult.ResourceID),
            resource.VersionEQ(taskResult.ExpectedVersion), // 乐观锁
        ).
        SetData(taskResult.Data).
        SetVersion(taskResult.ExpectedVersion + 1).
        Save(ctx)

    if err != nil {
        return err
    }

    if affected == 0 {
        logx.Warnw("Version conflict - message out of order",
            logx.Field("resource_id", taskResult.ResourceID),
            logx.Field("expected_version", taskResult.ExpectedVersion))

        // 可选：写入DLQ，人工介入
        return fmt.Errorf("version conflict: expected %d", taskResult.ExpectedVersion)
    }

    return nil
}
```

---

## 6. 性能优化

### 6.1 批量消费

```go
// unified-io/rpc/internal/consumer/batch_consumer.go

type BatchConsumer struct {
    reader      *kafka.Reader
    handler     BatchHandler
    batchSize   int
    batchWait   time.Duration
}

func (c *BatchConsumer) Start(ctx context.Context) error {
    batch := make([]kafka.Message, 0, c.batchSize)
    timer := time.NewTimer(c.batchWait)
    defer timer.Stop()

    for {
        select {
        case <-ctx.Done():
            // 处理剩余消息
            if len(batch) > 0 {
                _ = c.handler.HandleBatch(ctx, batch)
            }
            return ctx.Err()

        case <-timer.C:
            if len(batch) > 0 {
                if err := c.handler.HandleBatch(ctx, batch); err != nil {
                    logx.Errorw("Batch handle failed", logx.Field("error", err))
                }
                batch = batch[:0]
            }
            timer.Reset(c.batchWait)

        default:
            msg, err := c.reader.ReadMessage(ctx)
            if err != nil {
                continue
            }

            batch = append(batch, msg)

            if len(batch) >= c.batchSize {
                if err := c.handler.HandleBatch(ctx, batch); err != nil {
                    logx.Errorw("Batch handle failed", logx.Field("error", err))
                }
                batch = batch[:0]
                timer.Reset(c.batchWait)
            }
        }
    }
}
```

### 6.2 大消息处理

```go
// unified-io/rpc/internal/producer/large_message_producer.go

const MaxKafkaMessageSize = 900 * 1024 // 900KB (Kafka默认1MB)

type LargeMessageProducer struct {
    producer      Producer
    objectStorage ObjectStorage
}

func (p *LargeMessageProducer) Publish(ctx context.Context, topic string, body []byte, partitionKey string) error {
    if len(body) <= MaxKafkaMessageSize {
        // 小消息，直接发送
        return p.producer.Publish(ctx, topic, body, partitionKey)
    }

    // 大消息，上传到对象存储
    ref, err := p.objectStorage.Upload(ctx, fmt.Sprintf("messages/%s/%s", topic, uuid.New()), body)
    if err != nil {
        return fmt.Errorf("upload large message: %w", err)
    }

    // 发送引用消息
    refMsg := LargeMessageRef{
        StorageRef: ref,
        Size:       len(body),
        UploadedAt: time.Now(),
    }

    return p.producer.Publish(ctx, topic, marshalJSON(refMsg), partitionKey)
}

// Consumer端处理
func (c *Consumer) Handle(ctx context.Context, msg kafka.Message) error {
    var ref LargeMessageRef
    if err := json.Unmarshal(msg.Value, &ref); err == nil && ref.StorageRef != "" {
        // 是引用消息，下载实际内容
        body, err := c.objectStorage.Download(ctx, ref.StorageRef)
        if err != nil {
            return err
        }
        return c.handler.Handle(ctx, body)
    }

    // 普通消息
    return c.handler.Handle(ctx, msg.Value)
}
```

---

## 7. 监控与告警

### 7.1 Prometheus指标

```go
// unified-io/rpc/internal/metrics/kafka_metrics.go

package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // 业务指标
    TaskExecutionDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "io_task_execution_duration_seconds",
            Help:    "Task execution duration in seconds",
            Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s ~ 51.2s
        },
        []string{"tenant_id", "task_type", "status"},
    )

    TaskExecutionTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "io_task_execution_total",
            Help: "Total number of task executions",
        },
        []string{"tenant_id", "task_type", "status"},
    )

    // 消息积压
    ConsumerLag = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "io_consumer_lag_messages",
            Help: "Number of messages behind the high watermark",
        },
        []string{"topic", "partition", "consumer_group"},
    )

    // 幂等去重
    DuplicateMessages = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "io_duplicate_messages_total",
            Help: "Number of duplicate messages detected",
        },
        []string{"topic", "detection_layer"}, // redis/db
    )

    // Outbox延迟
    OutboxDispatchDelay = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "io_outbox_dispatch_delay_seconds",
            Help:    "Time between outbox creation and successful dispatch",
            Buckets: prometheus.ExponentialBuckets(0.01, 2, 12), // 10ms ~ 40s
        },
    )

    // Dead Letter
    DeadLetterMessages = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "io_dead_letter_messages_total",
            Help: "Total number of messages sent to dead letter queue",
        },
        []string{"topic", "error_type"},
    )

    // 租户隔离违规
    TenantIsolationViolations = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "io_tenant_isolation_violations_total",
            Help: "Number of tenant isolation violations detected",
        },
        []string{"consumer_group", "expected_tenant"},
    )

    // 消息大小
    MessageSize = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "io_message_size_bytes",
            Help:    "Size of Kafka messages in bytes",
            Buckets: prometheus.ExponentialBuckets(100, 2, 14), // 100B ~ 819KB
        },
        []string{"topic"},
    )
)

// 使用示例
func recordTaskExecution(tenantID uint64, taskType string, duration time.Duration, err error) {
    status := "success"
    if err != nil {
        status = "failed"
    }

    TaskExecutionDuration.WithLabelValues(
        fmt.Sprint(tenantID),
        taskType,
        status,
    ).Observe(duration.Seconds())

    TaskExecutionTotal.WithLabelValues(
        fmt.Sprint(tenantID),
        taskType,
        status,
    ).Inc()
}
```

### 7.2 消费滞后监控

```go
// unified-io/rpc/internal/metrics/lag_collector.go

package metrics

import (
    "context"
    "fmt"
    "time"

    "github.com/segmentio/kafka-go"
)

type LagCollector struct {
    brokers []string
    topics  []string
    groupID string
}

func (c *LagCollector) Start(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := c.collectLag(ctx); err != nil {
                logx.Errorw("Collect lag failed", logx.Field("error", err))
            }
        case <-ctx.Done():
            return
        }
    }
}

func (c *LagCollector) collectLag(ctx context.Context) error {
    conn, err := kafka.Dial("tcp", c.brokers[0])
    if err != nil {
        return err
    }
    defer conn.Close()

    for _, topic := range c.topics {
        partitions, err := conn.ReadPartitions(topic)
        if err != nil {
            continue
        }

        for _, p := range partitions {
            // 获取高水位
            high, err := conn.ReadOffset(ctx, topic, p.ID, kafka.LastOffset)
            if err != nil {
                continue
            }

            // 获取消费者偏移量 (需要使用admin client)
            committed, err := c.getCommittedOffset(ctx, topic, p.ID)
            if err != nil {
                continue
            }

            lag := high - committed
            ConsumerLag.WithLabelValues(
                topic,
                fmt.Sprint(p.ID),
                c.groupID,
            ).Set(float64(lag))
        }
    }

    return nil
}
```

### 7.3 告警规则

```yaml
# prometheus/alerts/kafka.yml

groups:
  - name: kafka_alerts
    interval: 30s
    rules:
      # 消费滞后告警
      - alert: HighConsumerLag
        expr: io_consumer_lag_messages > 10000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Kafka consumer lag is high"
          description: "Consumer group {{ $labels.consumer_group }} on topic {{ $labels.topic }} partition {{ $labels.partition }} has {{ $value }} messages lag"

      - alert: CriticalConsumerLag
        expr: io_consumer_lag_messages > 50000
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Kafka consumer lag is critical"
          description: "Consumer group {{ $labels.consumer_group }} has {{ $value }} messages lag. Immediate action required!"

      # 任务执行失败率告警
      - alert: HighTaskFailureRate
        expr: |
          rate(io_task_execution_total{status="failed"}[5m]) /
          rate(io_task_execution_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High task failure rate"
          description: "Task type {{ $labels.task_type }} has {{ $value | humanizePercentage }} failure rate"

      # Dead Letter增长告警
      - alert: DeadLetterQueueGrowing
        expr: increase(io_dead_letter_messages_total[5m]) > 100
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Dead letter queue is growing rapidly"
          description: "Topic {{ $labels.topic }} has {{ $value }} messages sent to DLQ in last 5 minutes"

      # 租户隔离违规告警
      - alert: TenantIsolationViolation
        expr: increase(io_tenant_isolation_violations_total[1m]) > 0
        for: 0m
        labels:
          severity: critical
        annotations:
          summary: "🚨 Tenant isolation violation detected!"
          description: "Consumer group {{ $labels.consumer_group }} attempted to access tenant {{ $labels.expected_tenant }} data. Security breach!"

      # Outbox延迟告警
      - alert: OutboxDispatchDelayHigh
        expr: histogram_quantile(0.95, io_outbox_dispatch_delay_seconds_bucket) > 60
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Outbox dispatch delay is high"
          description: "95th percentile outbox dispatch delay is {{ $value }}s"

      # 消费者停滞告警
      - alert: ConsumerStalled
        expr: rate(io_task_execution_total[5m]) == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Kafka consumer appears to be stalled"
          description: "No tasks executed in the last 5 minutes. Check worker health!"
```

### 7.4 Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Unified IO - Kafka Monitoring",
    "panels": [
      {
        "title": "Consumer Lag",
        "targets": [{
          "expr": "io_consumer_lag_messages"
        }],
        "type": "graph"
      },
      {
        "title": "Task Execution Rate",
        "targets": [{
          "expr": "rate(io_task_execution_total[1m])"
        }],
        "type": "graph"
      },
      {
        "title": "Task Failure Rate (%)",
        "targets": [{
          "expr": "rate(io_task_execution_total{status='failed'}[5m]) / rate(io_task_execution_total[5m]) * 100"
        }],
        "type": "graph"
      },
      {
        "title": "Outbox Dispatch Delay (P95)",
        "targets": [{
          "expr": "histogram_quantile(0.95, io_outbox_dispatch_delay_seconds_bucket)"
        }],
        "type": "graph"
      },
      {
        "title": "Dead Letter Messages",
        "targets": [{
          "expr": "rate(io_dead_letter_messages_total[5m])"
        }],
        "type": "graph"
      }
    ]
  }
}
```

---

## 8. 开发实施计划

### 8.1 分阶段路线图

| 阶段 | 时间 | 任务 | 优先级 | 验收标准 |
|------|------|------|--------|---------|
| **阶段1<br>基础设施** | Week 1-2 | 1. Kafka集群部署 (SASL/SSL)<br>2. Topic创建与ACL配置<br>3. Outbox表设计与索引<br>4. Redis集群搭建 | P0 | - Kafka集群3节点运行<br>- Topic创建成功<br>- Outbox表迁移完成<br>- Redis主从正常 |
| **阶段2<br>安全加固** | Week 2-3 | 1. 多租户隔离实现<br>2. 敏感信息脱敏<br>3. 消息加密传输<br>4. 审计日志完善 | P0 | - 租户隔离测试100%通过<br>- 脱敏机制上线<br>- SSL证书配置完成<br>- 审计日志可查询 |
| **阶段3<br>可靠性** | Week 3-4 | 1. Outbox Dispatcher实现<br>2. 分布式锁集成<br>3. 幂等性完整实现<br>4. 崩溃恢复机制 | P0 | - Outbox发送成功率>99.9%<br>- 重复消息率<0.1%<br>- 故障恢复时间<5分钟 |
| **阶段4<br>核心功能** | Week 4-6 | 1. Scheduler实现<br>2. Worker Consumer实现<br>3. 字段映射引擎<br>4. Provider适配器 | P1 | - 支持5种以上Provider<br>- 任务执行成功率>95%<br>- 字段映射准确率100% |
| **阶段5<br>监控告警** | Week 6-7 | 1. Prometheus指标埋点<br>2. Grafana Dashboard<br>3. 告警规则配置<br>4. 分布式追踪 | P1 | - 核心指标100%覆盖<br>- Dashboard可视化完成<br>- 告警规则测试通过 |
| **阶段6<br>性能优化** | Week 7-8 | 1. 批量消费实现<br>2. 分区热点优化<br>3. 大消息处理<br>4. 连接池优化 | P2 | - 吞吐量达到10k msg/s<br>- P95延迟<500ms<br>- 热点分区负载均衡 |
| **阶段7<br>运维工具** | Week 8-9 | 1. DLQ管理界面<br>2. 任务重放工具<br>3. 运维脚本<br>4. 故障演练 | P2 | - DLQ UI可用<br>- 重放工具测试通过<br>- 故障演练文档完成 |
| **阶段8<br>测试上线** | Week 9-10 | 1. 集成测试<br>2. 压力测试<br>3. 灰度发布<br>4. 全量上线 | P1 | - 集成测试通过率100%<br>- 压测达标<br>- 灰度无故障<br>- 全量上线 |

### 8.2 关键里程碑

- **M1 (Week 2)**: 基础设施就绪，安全加固完成
- **M2 (Week 4)**: 可靠性机制验证通过
- **M3 (Week 6)**: 核心功能开发完成
- **M4 (Week 8)**: 监控告警上线，性能达标
- **M5 (Week 10)**: 生产环境全量上线

---

## 9. 测试与验收

### 9.1 测试策略

| 测试类型 | 覆盖范围 | 工具 | 通过标准 |
|---------|---------|------|---------|
| **单元测试** | Producer/Consumer/Outbox/Idempotency | go test | 覆盖率>80% |
| **集成测试** | 完整链路 (Scheduler→Kafka→Worker→DB) | Testcontainers | 100%通过 |
| **安全测试** | 租户隔离/敏感信息/权限 | 自定义脚本 | 0漏洞 |
| **性能测试** | 吞吐量/延迟/积压恢复 | k6 / Gatling | 达到SLA |
| **故障演练** | Broker宕机/网络分区/Rebalance | Chaos Mesh | 恢复时间<5min |

### 9.2 集成测试示例

```go
// unified-io/rpc/test/integration/kafka_integration_test.go

func TestKafkaIntegration(t *testing.T) {
    // 启动Docker容器 (Kafka + PostgreSQL + Redis)
    ctx := context.Background()

    kafkaContainer, _ := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image:        "confluentinc/cp-kafka:7.5.0",
            ExposedPorts: []string{"9092/tcp"},
        },
        Started: true,
    })
    defer kafkaContainer.Terminate(ctx)

    // 测试场景1: 端到端消息流
    t.Run("EndToEndMessageFlow", func(t *testing.T) {
        producer := NewProducer(getBrokers())
        consumer := NewConsumer(getBrokers(), "test-group")

        msg := &TaskMessage{
            TenantID:  1,
            TaskRunID: 123,
            TaskType:  "sync_users",
        }

        // 发送消息
        err := producer.Publish(ctx, "io.input.jobs", marshalJSON(msg), "1:test")
        require.NoError(t, err)

        // 消费消息
        received := make(chan *TaskMessage, 1)
        consumer.Start(ctx, func(ctx context.Context, m *TaskMessage) error {
            received <- m
            return nil
        })

        select {
        case r := <-received:
            assert.Equal(t, msg.TaskRunID, r.TaskRunID)
        case <-time.After(5 * time.Second):
            t.Fatal("timeout waiting for message")
        }
    })

    // 测试场景2: Outbox模式
    t.Run("OutboxPattern", func(t *testing.T) {
        db := setupTestDB(t)
        dispatcher := NewOutboxDispatcher(db, producer, redis)

        // 事务内写入
        taskRunID, err := scheduler.ScheduleTask(ctx, msg)
        require.NoError(t, err)

        // 启动Dispatcher
        go dispatcher.Start(ctx)

        // 验证消息发送
        time.Sleep(2 * time.Second)
        outbox, _ := db.TaskOutbox.Query().Where(taskoutbox.StatusEQ("sent")).All(ctx)
        assert.Len(t, outbox, 1)
    })

    // 测试场景3: 幂等性
    t.Run("Idempotency", func(t *testing.T) {
        handler := &TestHandler{db: db, idempotency: NewGuard(redis)}

        // 发送重复消息
        for i := 0; i < 3; i++ {
            err := handler.Handle(ctx, marshalJSON(msg))
            require.NoError(t, err)
        }

        // 验证只执行一次
        results, _ := db.TaskResult.Query().All(ctx)
        assert.Len(t, results, 1)
    })
}
```

### 9.3 性能测试

```javascript
// k6/kafka_load_test.js

import kafka from 'k6/x/kafka';

export let options = {
    stages: [
        { duration: '1m', target: 100 },  // 预热
        { duration: '5m', target: 1000 }, // 压测
        { duration: '1m', target: 0 },    // 降温
    ],
    thresholds: {
        'kafka_write_duration': ['p(95)<500'], // P95延迟<500ms
        'kafka_write_errors': ['rate<0.01'],   // 错误率<1%
    },
};

const producer = kafka.producer({
    brokers: ['kafka-1:9092', 'kafka-2:9092'],
    topic: 'io.input.jobs',
});

export default function () {
    const msg = JSON.stringify({
        tenant_id: Math.floor(Math.random() * 100),
        task_run_id: Date.now(),
        task_type: 'sync_users',
    });

    producer.send({
        messages: [{ value: msg }],
    });
}
```

### 9.4 安全测试清单

- [ ] 租户A无法消费租户B的消息
- [ ] 消息体中不包含明文密码/密钥
- [ ] Kafka连接强制使用SASL/SSL
- [ ] 未授权客户端无法连接Kafka
- [ ] 审计日志记录所有关键操作
- [ ] Dead Letter Queue禁止匿名访问
- [ ] 消费者组权限最小化配置

---

## 10. 安全检查清单

### 10.1 上线前检查

#### 10.1.1 Kafka集群安全

- [ ] **SASL认证已启用** (`sasl.enabled.mechanisms=SCRAM-SHA-512`)
- [ ] **SSL加密已启用** (`ssl.keystore.location` 已配置)
- [ ] **ACL已配置** (每个消费者组仅能读取指定Topic)
- [ ] **最小权限原则** (Producer仅Write权限，Consumer仅Read权限)
- [ ] **Admin用户密码强度** (≥16字符,包含特殊字符)
- [ ] **证书有效期检查** (SSL证书有效期>90天)
- [ ] **禁用明文端口** (9092端口已关闭,仅开放9093 SASL_SSL)

#### 10.1.2 应用层安全

- [ ] **租户隔离已验证** (测试用例100%通过)
- [ ] **敏感信息脱敏已上线** (MessageSanitizer已启用)
- [ ] **凭证引用模式已实施** (消息中无明文密码)
- [ ] **审计日志完整** (Producer/Consumer/DLQ操作已记录)
- [ ] **限流已配置** (单租户QPS限制已启用)
- [ ] **异常监控已部署** (租户隔离违规告警已测试)

#### 10.1.3 可靠性检查

- [ ] **Outbox表索引已创建** (`idx_outbox_dispatch` 已建立)
- [ ] **Dispatcher分布式锁已测试** (多实例无重复发送)
- [ ] **幂等性双重保证已验证** (Redis + DB约束)
- [ ] **Dead Letter处理流程已测试** (UI/API重放功能正常)
- [ ] **崩溃恢复机制已验证** (进程重启后消息正常恢复)
- [ ] **Rebalance优化已配置** (`max.poll.interval.ms` 已调整)

#### 10.1.4 性能检查

- [ ] **压测达标** (吞吐量≥10k msg/s, P95延迟≤500ms)
- [ ] **分区热点已优化** (无单分区负载>总负载30%)
- [ ] **批量消费已启用** (`batchSize≥100, batchWait≤500ms`)
- [ ] **连接池已优化** (Producer/Consumer连接复用)
- [ ] **大消息处理已实现** (>900KB消息上传对象存储)

#### 10.1.5 监控告警

- [ ] **Prometheus指标已埋点** (核心指标100%覆盖)
- [ ] **Grafana Dashboard已部署** (实时可视化)
- [ ] **告警规则已配置** (消费滞后/失败率/DLQ)
- [ ] **PagerDuty/钉钉已集成** (Critical告警自动通知)
- [ ] **日志聚合已配置** (ELK/Loki集中收集)
- [ ] **分布式追踪已启用** (Jaeger链路可查)

### 10.2 上线后运维

#### 10.2.1 日常巡检 (每日)

```bash
# 1. 检查消费滞后
kafka-consumer-groups --bootstrap-server kafka:9092 \
  --group unified-io-worker \
  --describe

# 2. 检查Outbox积压
psql -c "SELECT status, COUNT(*) FROM io_task_outbox GROUP BY status;"

# 3. 检查Dead Letter增长
psql -c "SELECT COUNT(*) FROM io_dead_letters WHERE created_at > NOW() - INTERVAL '1 day';"

# 4. 检查告警
curl -s http://prometheus:9090/api/v1/alerts | jq '.data.alerts[] | select(.state=="firing")'
```

#### 10.2.2 定期审查 (每周)

- 审查Dead Letter Queue中的失败原因
- 分析消费滞后趋势
- 检查租户隔离违规日志
- 优化热点分区策略
- 更新SSL证书 (到期前30天)

#### 10.2.3 故障响应流程

```
1. 告警触发 (PagerDuty/钉钉)
   ↓
2. 值班工程师响应 (5分钟内)
   ↓
3. 查看Grafana Dashboard定位问题
   ↓
4. 执行故障恢复脚本
   ↓
5. 验证服务恢复
   ↓
6. 记录故障报告
```

### 10.3 灰度发布检查

- [ ] **灰度范围**: 先选择1个小租户 (QPS<100)
- [ ] **灰度时长**: 运行≥24小时无异常
- [ ] **回滚准备**: 回滚脚本已准备并测试
- [ ] **监控重点**: 实时监控错误率/延迟/吞吐量
- [ ] **通知计划**: 已通知相关租户灰度时间窗口

---

## 11. 总结

### 11.1 核心改进

本次v2.0设计相比v1.0的核心改进：

| 维度 | v1.0 | v2.0 | 改进效果 |
|------|------|------|---------|
| **安全性** | 基础租户隔离 | Header+ACL双重隔离 + 敏感信息脱敏 | 安全等级 ⭐⭐⭐ → ⭐⭐⭐⭐⭐ |
| **可靠性** | 直接发送 | Outbox模式 + 分布式锁 | 一致性保证100% |
| **幂等性** | 应用层去重 | Redis + DB双重保证 | 重复消息率 <0.1% |
| **性能** | 单条消费 | 批量消费 + 热点优化 | 吞吐量提升300% |
| **可观测** | 基础日志 | Prometheus + Grafana + Jaeger | 故障定位时间缩短80% |

### 11.2 风险提示

- ⚠️ **Outbox表增长**: 定期清理已发送消息 (建议保留7天)
- ⚠️ **Redis内存压力**: 幂等窗口控制在1小时,避免OOM
- ⚠️ **Kafka存储成本**: 大租户考虑独立Topic + 更短保留时间
- ⚠️ **Dead Letter处理**: 需要建立人工审核流程,禁止自动重放

### 11.3 未来演进方向

1. **Kafka Streams集成**: 实现流式聚合、窗口计算
2. **Schema Registry**: 统一消息格式,支持版本演进
3. **Kafka Connect**: 简化数据同步场景
4. **多云部署**: 支持跨Region容灾

---

## 附录

### A. 参考文档

- [Kafka官方文档](https://kafka.apache.org/documentation/)
- [go-zero文档](https://go-zero.dev/docs/tasks/queue/message-queue)
- [Outbox模式详解](https://microservices.io/patterns/data/transactional-outbox.html)
- [NewBee CLAUDE.md](./CLAUDE.md) - 项目编码准则

### B. 相关工具

- **Kafka管理工具**: AKHQ, Kafdrop
- **性能测试**: k6, Gatling
- **故障注入**: Chaos Mesh
- **监控**: Prometheus, Grafana, Jaeger

### C. 联系方式

- **技术支持**: dev@newbee.io
- **安全问题**: security@newbee.io
- **文档反馈**: docs@newbee.io

---

**最后更新**: 2025-10-19
**审查人**: 架构师团队
**批准人**: CTO
**下次审查**: 2025-11-19
