# RocketMQ 订单超时自动取消方案

本文档介绍支付网关中基于 Apache RocketMQ 延迟消息实现订单超时自动取消的方案，并深入讲解 RocketMQ 事务消息（半消息）机制及其适用场景。

## 目录

- [方案概述](#方案概述)
- [延迟消息方案](#延迟消息方案)
- [RocketMQ 事务消息（半消息）](#rocketmq-事务消息半消息)
- [为什么本项目不使用事务消息](#为什么本项目不使用事务消息)
- [场景选型对照表](#场景选型对照表)
- [配置说明](#配置说明)
- [部署架构](#部署架构)

---

## 方案概述

支付网关支持两种订单超时取消策略，二者互为兜底：

| 策略 | 触发方式 | 精度 | 依赖 |
|------|---------|------|------|
| RocketMQ 延迟消息 | 订单创建后发送延迟消息，到期后精确取消 | 秒级 | RocketMQ |
| 定时任务轮询 | 每分钟扫描过期订单批量取消 | 分钟级 | 仅数据库 |

当 RocketMQ 未启用或消息发送失败时，自动降级为定时任务轮询，保证业务可用性。

---

## 延迟消息方案

### 核心流程

```mermaid
sequenceDiagram
    participant Client as 客户端
    participant Gateway as 支付网关
    participant DB as PostgreSQL
    participant MQ as RocketMQ Broker
    participant Consumer as 延迟消费者

    Client->>Gateway: 创建订单请求
    Gateway->>DB: BEGIN 事务 → INSERT 订单
    DB-->>Gateway: 订单创建成功
    Gateway->>MQ: 发送延迟消息（delay=30min）
    MQ-->>Gateway: 发送成功
    Gateway-->>Client: 返回订单信息

    Note over MQ: 消息暂存，30分钟后投递

    MQ->>Consumer: 延迟到期，投递消息
    Consumer->>DB: 查询订单状态
    alt 状态为 CREATED + PENDING
        Consumer->>DB: UPDATE 订单状态为 CANCELLED
        Note over Consumer: 取消成功
    else 已支付 / 已取消
        Note over Consumer: 跳过（幂等）
    end
```

### 关键设计

**1. 先写后发，定时兜底**

```
数据库事务提交 → 发送延迟消息（失败不影响订单） → 定时任务兜底
```

消息发送失败仅打 WARN 日志，不回滚订单。定时任务每分钟扫描 `expired_at < NOW()` 的待支付订单进行批量取消。

**2. 幂等消费**

消费者使用乐观条件更新，天然防重：

```sql
UPDATE orders
SET status = 'CANCELLED', refund_reason = '订单超时自动取消'
WHERE id = ? AND status = 'CREATED' AND payment_status = 'PENDING'
```

即使延迟消息与定时任务同时到达，或消息重复投递，也不会重复取消或误取消已支付订单。

**3. 接口解耦**

通过 `OrderDelayCancelSender` 接口注入各支付服务，支持测试 Mock，不耦合 RocketMQ 实现细节。

---

## RocketMQ 事务消息（半消息）

### 什么是事务消息

RocketMQ 事务消息是一种**分布式事务**解决方案，保证**本地数据库事务**和**消息发送**的原子性——要么都成功，要么都失败。

### 核心流程

```mermaid
sequenceDiagram
    participant Producer as 消息生产者
    participant Broker as RocketMQ Broker
    participant Consumer as 消息消费者
    participant DB as 本地数据库

    Producer->>Broker: ① 发送半消息（Half Message）
    Note over Broker: 消息暂存<br/>Consumer 不可见
    Broker-->>Producer: 半消息发送成功

    Producer->>DB: ② 执行本地事务
    Note over DB: 如：扣减库存、创建订单

    alt 本地事务成功
        Producer->>Broker: ③ 发送 Commit
        Broker->>Consumer: 消息变为可消费
        Consumer->>Consumer: 处理业务逻辑
    else 本地事务失败
        Producer->>Broker: ③ 发送 Rollback
        Note over Broker: 丢弃消息
    end

    Note over Broker: ④ 若长时间未收到 Commit/Rollback

    Broker->>Producer: 事务回查（Check）
    Producer->>DB: 查询本地事务状态
    DB-->>Producer: 返回事务状态
    Producer->>Broker: 返回 Commit 或 Rollback
```

### 四个阶段详解

| 阶段 | 动作 | 说明 |
|------|------|------|
| ① 发送半消息 | Producer → Broker | 消息写入 Broker 但标记为"不可消费"，Consumer 看不到 |
| ② 执行本地事务 | Producer → 本地 DB | 执行数据库操作（如创建订单、扣减库存） |
| ③ 二次确认 | Producer → Broker | 本地事务成功发 Commit（消息可消费），失败发 Rollback（丢弃消息） |
| ④ 事务回查 | Broker → Producer | 若 Broker 长时间未收到确认（网络超时等），主动回查 Producer 的事务状态 |

### 解决的核心问题

事务消息解决的是**两个系统操作的原子性**问题：

```mermaid
flowchart TD
    A[传统方案的两难困境] --> B{先操作数据库还是先发消息？}

    B -->|先写DB后发消息| C[DB成功 + 消息失败<br/>= 下游系统不知道]
    B -->|先发消息后写DB| D[消息成功 + DB失败<br/>= 下游收到无效消息]

    E[事务消息方案] --> F[半消息 → 本地事务 → 确认/回滚]
    F --> G[DB和消息 保持一致<br/>回查机制兜底]

    style C fill:#ffcccc
    style D fill:#ffcccc
    style G fill:#ccffcc
```

**典型场景**：电商下单扣库存

1. 半消息发送到 Broker（消费者还看不到）
2. 本地扣减库存 → 成功则 Commit，库存不足则 Rollback
3. Commit 后消费者才收到"创建订单"消息
4. 若过程中网络断开，Broker 回查库存服务确认事务状态

---

## 为什么本项目不使用事务消息

### 对比分析

```mermaid
flowchart LR
    subgraph 事务消息方案
        A1[发送半消息] --> A2[创建订单]
        A2 --> A3{事务成功?}
        A3 -->|是| A4[Commit]
        A3 -->|否| A5[Rollback]
        A6[实现事务回查接口] -.-> A2
    end

    subgraph 当前方案
        B1[创建订单] --> B2{事务成功?}
        B2 -->|是| B3[发送延迟消息]
        B3 -->|失败| B4[定时任务兜底]
        B2 -->|否| B5[返回错误]
    end

    style A6 fill:#fff3cd
    style B4 fill:#d4edda
```

### 三个不需要的理由

**1. 消息丢失不会导致业务错误**

延迟消息只是一个"到期提醒"：到时间了去检查这笔订单是否需要取消。即使消息丢了：
- 不会导致订单被错误支付
- 不会导致订单被错误取消
- 最坏情况只是取消**延迟**了几十秒（等定时任务扫到）

**2. 有定时任务作为兜底**

原有的每分钟一次 `CancelExpiredOrders` 定时扫描始终在运行。延迟消息是"精确狙击"，定时任务是"地毯式扫描"，两者互补。

**3. 消费者天然幂等**

消费者收到消息后检查订单实际状态，只取消 `CREATED + PENDING` 的订单。无论消息重复投递、延迟到达、还是定时任务已提前取消，都不会产生副作用。

### 引入事务消息的代价

| 项目 | 影响 |
|------|------|
| 实现事务回查接口 | 需提供 `CheckLocalTransaction` 回调，查询 DB 判断订单是否已创建 |
| 维护事务状态表 | 需额外记录每条半消息对应的本地事务状态 |
| 增加链路复杂度 | 半消息 → 本地事务 → 二次确认，比先写后发复杂 |
| 排查难度增加 | 事务回查、半消息超时等异常场景增多 |

**结论**：在"订单超时取消"这个最终一致性场景下，"先写后发 + 定时兜底"已经足够可靠，引入事务消息是过度设计。

---

## 场景选型对照表

| 业务场景 | 一致性要求 | 推荐方案 | 原因 |
|---------|----------|---------|------|
| 下单扣库存 | 强一致 | **事务消息** | 订单创建了但库存没扣 = 超卖 |
| 跨服务转账 | 强一致 | **事务消息** | A 扣了钱但 B 没到账 = 资金损失 |
| 订单超时取消 | 最终一致 | **延迟消息 + 定时兜底** | 消息丢了只是取消晚一点，有兜底 |
| 发送通知/短信 | 最终一致 | **普通消息 + 重试** | 丢了可以重试或兜底 |
| 数据同步/刷缓存 | 最终一致 | **普通消息** | 偶尔丢失可接受，下次查询自动修复 |
| 积分/优惠券发放 | 最终一致 | **事务消息 或 本地消息表** | 用户感知强，但可以延迟补发 |

---

## 配置说明

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `ROCKETMQ_ENABLED` | `false` | 是否启用 RocketMQ |
| `ROCKETMQ_ENDPOINT` | `localhost:8081` | RocketMQ Proxy gRPC 地址 |
| `ROCKETMQ_ACCESS_KEY` | 空 | 访问密钥（ACL 开启时必填） |
| `ROCKETMQ_SECRET_KEY` | 空 | 密钥（ACL 开启时必填） |
| `ROCKETMQ_ORDER_DELAY_TOPIC` | `order-timeout-cancel` | 订单超时取消 Topic |
| `ROCKETMQ_CONSUMER_GROUP` | `pay-gateway-order-cancel-cg` | 消费者组名 |
| `ROCKETMQ_ORDER_TIMEOUT` | `30m` | 订单超时时间 |

### TOML 配置

```toml
[rocketmq]
enabled = true
endpoint = "localhost:8081"
order_delay_topic = "order-timeout-cancel"
consumer_group = "pay-gateway-order-cancel-cg"
order_timeout = "30m"
```

---

## 部署架构

### Docker Compose 服务

```mermaid
graph TB
    subgraph Docker Compose
        PG[(PostgreSQL)]
        Redis[(Redis)]
        NS[RocketMQ NameServer<br/>:9876]
        Broker[RocketMQ Broker<br/>+ Proxy :8081]
        App[Pay Gateway<br/>:8080]
        Nginx[Nginx<br/>:80/443]
    end

    App -->|GORM| PG
    App -->|go-redis| Redis
    App -->|gRPC| Broker
    Broker -->|注册| NS
    Nginx -->|反向代理| App

    Client((客户端)) -->|HTTPS| Nginx
```

### 启动命令

```bash
# 启动全部服务（含 RocketMQ）
docker-compose up -d

# 仅启动基础设施
docker-compose up -d postgres redis rocketmq-namesrv rocketmq-broker

# 不使用 RocketMQ（降级模式）
ROCKETMQ_ENABLED=false docker-compose up -d postgres redis pay-gateway
```

### Topic 自动创建

RocketMQ 5.x 默认开启 `autoCreateTopicEnable`，首次发送消息时会自动创建 Topic。生产环境建议手动预创建：

```bash
# 进入 Broker 容器
docker exec -it pay-gateway-rocketmq-broker bash

# 创建 Topic
sh mqadmin updateTopic -n localhost:9876 -t order-timeout-cancel -c DefaultCluster
```
