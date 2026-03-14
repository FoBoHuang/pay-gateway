# Google Play 支付接入指南

本文档详细介绍如何接入 Google Play 内购和订阅功能。

## 📋 目录

- [功能概述](#功能概述)
- [配置](#配置)
- [支付流程](#支付流程)
- [API 接口](#api-接口)
- [Webhook 处理](#webhook-处理)
- [安全机制](#安全机制)
- [最佳实践](#最佳实践)

## 功能概述

支持的功能：

| 功能 | 说明 | 状态 |
|-----|------|-----|
| 一次性购买 | 消耗型/非消耗型商品 | ✅ |
| 订阅 | 自动续期订阅 | ✅ |
| 购买验证 | 服务端验证购买令牌 | ✅ |
| 确认购买 | Acknowledge | ✅ |
| 消费购买 | 消耗型商品消费 | ✅ |
| Webhook | 实时开发者通知 | ✅ |

## 配置

### 1. 创建服务账号

1. 访问 [Google Cloud Console](https://console.cloud.google.com/)
2. 创建项目或选择现有项目
3. 启用 **Google Play Android Developer API**
4. 创建服务账号，授予 "财务" 权限
5. 下载 JSON 密钥文件

### 2. 配置 config.toml

```toml
[google]
service_account_file = "configs/google-service-account.json"
package_name = "com.example.app"

# Webhook 安全（RTDN 通过 Pub/Sub 推送）
webhook_url = "https://your-domain.com/webhook/google"  # JWT audience，与 Pub/Sub 订阅 endpoint 一致
verify_push_jwt = true   # 必须为 true，验证请求来自 Google Pub/Sub
expected_subscription = "projects/your-project/subscriptions/your-rtdn-sub"  # 可选，校验订阅来源
```

### 3. 配置 Google Play Console

1. 进入 **设置 > API 访问权限**
2. 关联上一步创建的服务账号
3. 配置 **实时开发者通知**：
   - 主题名称：`projects/your-project/topics/play-notifications`
   - Webhook URL：`https://your-domain.com/webhook/google`

## 支付流程

### 一次性购买流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           一次性购买流程                                      │
└─────────────────────────────────────────────────────────────────────────────┘

    客户端                     服务端                      Google Play
      │                         │                            │
      │  1. 创建订单             │                            │
      │ ─────────────────────> │                            │
      │  POST /google/purchases │                            │
      │                         │                            │
      │  返回 order_id          │                            │
      │ <───────────────────── │                            │
      │                         │                            │
      │  2. 调起 Google Play 支付界面                         │
      │ ─────────────────────────────────────────────────> │
      │                         │                            │
      │  返回 purchaseToken     │                            │
      │ <───────────────────────────────────────────────── │
      │                         │                            │
      │  3. 验证购买             │                            │
      │ ─────────────────────> │                            │
      │  POST /google/verify-purchase                       │
      │  (product_id, token, order_id)                      │
      │                         │  调用 Google API 验证      │
      │                         │ ─────────────────────────> │
      │                         │                            │
      │                         │  返回购买状态              │
      │                         │ <───────────────────────── │
      │  返回验证结果            │                            │
      │ <───────────────────── │  更新订单状态               │
      │                         │                            │
      │  4. 确认购买             │                            │
      │ ─────────────────────> │                            │
      │  POST /google/acknowledge-purchase                  │
      │                         │  调用 Acknowledge API      │
      │                         │ ─────────────────────────> │
      │                         │                            │
      │  返回成功               │                            │
      │ <───────────────────── │                            │
      │                         │                            │
      │  5. (可选) 消费购买      │                            │
      │ ─────────────────────> │                            │
      │  POST /google/consume-purchase                      │
      │                         │                            │
      └                         └                            └
```

### 订阅流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                             订阅流程                                         │
└─────────────────────────────────────────────────────────────────────────────┘

    客户端                     服务端                      Google Play
      │                         │                            │
      │  1. 创建订阅订单        │                            │
      │ ─────────────────────> │                            │
      │  POST /google/subscriptions                         │
      │                         │                            │
      │  返回 order_id          │                            │
      │ <───────────────────── │                            │
      │                         │                            │
      │  2. 调起订阅界面        │                            │
      │ ─────────────────────────────────────────────────> │
      │                         │                            │
      │  返回 purchaseToken     │                            │
      │ <───────────────────────────────────────────────── │
      │                         │                            │
      │  3. 验证订阅             │                            │
      │ ─────────────────────> │                            │
      │  POST /google/verify-subscription                   │
      │                         │  调用 Google API 验证      │
      │                         │ ─────────────────────────> │
      │                         │                            │
      │  返回订阅状态            │                            │
      │ <───────────────────── │                            │
      │                         │                            │
      │  4. 确认订阅             │                            │
      │ ─────────────────────> │                            │
      │  POST /google/acknowledge-subscription              │
      │                         │                            │
      └                         │                            │
                                │                            │
      ┌─────────────── 后续续费/状态变更 ───────────────────┐
      │                         │                            │
      │                         │  Webhook 通知              │
      │                         │ <───────────────────────── │
      │                         │  POST /webhook/google      │
      │                         │                            │
      │                         │  更新订阅状态              │
      │                         │  (续费/取消/过期等)         │
      └                         └                            └
```

## API 接口

### 创建内购订单

```http
POST /api/v1/google/purchases
Content-Type: application/json

{
  "user_id": 1,
  "product_id": "premium_upgrade",
  "title": "高级版升级",
  "description": "解锁所有高级功能",
  "quantity": 1,
  "currency": "USD",
  "price": 999,
  "developer_payload": "user_123"
}
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "order_no": "ORD20240101120000abcd1234",
    "user_id": 1,
    "product_id": "premium_upgrade",
    "type": "PURCHASE",
    "status": "CREATED",
    "payment_method": "GOOGLE_PLAY",
    "total_amount": 999,
    "currency": "USD"
  }
}
```

### 创建订阅订单

```http
POST /api/v1/google/subscriptions
Content-Type: application/json

{
  "user_id": 1,
  "product_id": "monthly_premium",
  "title": "月度会员",
  "description": "每月自动续费",
  "currency": "USD",
  "price": 999,
  "period": "P1M",
  "developer_payload": "user_123"
}
```

### 验证购买

```http
POST /api/v1/google/verify-purchase
Content-Type: application/json

{
  "product_id": "premium_upgrade",
  "purchase_token": "token_from_google_play",
  "order_id": 1
}
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "kind": "androidpublisher#productPurchase",
    "purchaseTimeMillis": "1704067200000",
    "purchaseState": 0,
    "consumptionState": 0,
    "orderId": "GPA.xxxx-xxxx-xxxx",
    "acknowledgementState": 0
  }
}
```

### 验证订阅

```http
POST /api/v1/google/verify-subscription
Content-Type: application/json

{
  "subscription_id": "monthly_premium",
  "purchase_token": "token_from_google_play",
  "order_id": 1
}
```

### 确认购买

```http
POST /api/v1/google/acknowledge-purchase
Content-Type: application/json

{
  "product_id": "premium_upgrade",
  "purchase_token": "token_from_google_play",
  "developer_payload": "user_123"
}
```

### 确认订阅

```http
POST /api/v1/google/acknowledge-subscription
Content-Type: application/json

{
  "subscription_id": "monthly_premium",
  "purchase_token": "token_from_google_play",
  "developer_payload": "user_123"
}
```

### 消费购买

适用于消耗型商品（如游戏金币）：

```http
POST /api/v1/google/consume-purchase
Content-Type: application/json

{
  "product_id": "100_coins",
  "purchase_token": "token_from_google_play"
}
```

### 获取订阅状态

```http
GET /api/v1/google/subscriptions/status?subscription_id=monthly_premium&purchase_token=xxx
```

### 获取用户订阅列表

```http
GET /api/v1/google/users/1/subscriptions
```

## Webhook 处理

### 通知类型

| 类型 | 说明 |
|-----|------|
| `SUBSCRIPTION_RECOVERED` | 从账号保留状态恢复 |
| `SUBSCRIPTION_RENEWED` | 活跃订阅已续费 |
| `SUBSCRIPTION_CANCELED` | 订阅已取消（非自愿/自愿） |
| `SUBSCRIPTION_PURCHASED` | 新订阅购买 |
| `SUBSCRIPTION_ON_HOLD` | 进入账号保留状态 |
| `SUBSCRIPTION_IN_GRACE_PERIOD` | 进入宽限期 |
| `SUBSCRIPTION_RESTARTED` | 用户重新激活订阅 |
| `SUBSCRIPTION_PRICE_CHANGE_CONFIRMED` | 价格变更已确认 |
| `SUBSCRIPTION_DEFERRED` | 订阅续订已延期 |
| `SUBSCRIPTION_PAUSED` | 订阅已暂停 |
| `SUBSCRIPTION_PAUSE_SCHEDULE_CHANGED` | 暂停计划变更 |
| `SUBSCRIPTION_REVOKED` | 订阅在到期前被撤销 |
| `SUBSCRIPTION_EXPIRED` | 订阅已过期 |
| `ONE_TIME_PRODUCT_PURCHASED` | 一次性商品购买 |
| `ONE_TIME_PRODUCT_CANCELED` | 一次性商品购买取消 |

### Webhook 处理流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Webhook 处理流程                                    │
└─────────────────────────────────────────────────────────────────────────────┘

  Google Play                  服务端                        数据库
      │                          │                             │
      │  1. 发送通知              │                             │
      │ ──────────────────────> │                             │
      │  POST /webhook/google    │                             │
      │  {message: {data: base64}}                             │
      │                          │                             │
      │                          │  2. 解码 & 解析通知         │
      │                          │  - base64 decode            │
      │                          │  - JSON parse               │
      │                          │                             │
      │                          │  3. 根据通知类型处理        │
      │                          │                             │
      │                          │  SUBSCRIPTION_RENEWED:      │
      │                          │  - 调用 VerifySubscription  │
      │                          │  - 更新订阅状态             │
      │                          │ ─────────────────────────> │
      │                          │                             │
      │                          │  SUBSCRIPTION_CANCELED:     │
      │                          │  - 标记订阅为已取消         │
      │                          │ ─────────────────────────> │
      │                          │                             │
      │                          │  SUBSCRIPTION_EXPIRED:      │
      │                          │  - 标记订阅为已过期         │
      │                          │ ─────────────────────────> │
      │                          │                             │
      │  4. 返回 200 OK         │                             │
      │ <────────────────────── │                             │
      │                          │                             │
      └                          └                             └
```

## 安全机制

### 安全机制概览

| 环节 | 机制 | 说明 |
|------|------|------|
| **购买验证** | Google API 服务端验证 | 通过 Android Publisher API 验证 `purchaseToken`，不依赖客户端可信 |
| **购买验证** | 包名校验 | 校验 `packageName` 与配置一致，防止跨应用伪造 |
| **Webhook** | Pub/Sub JWT 验签 | 验证推送请求的 JWT 签名，确保请求来自 Google Pub/Sub |
| **Webhook** | 订阅名校验 | 可选校验 `subscription`，确保消息来自指定 Pub/Sub 订阅 |
| **Webhook** | 二次验证 | 关键事件处理前调用 Google API 再次确认状态 |

### 与 Apple 安全机制的区别

| | Google Play | Apple |
|---|---|---|
| 消息体 | 明文 JSON（`{"message":{"data":"base64..."}}`) | **签名的 JWS**（`signedPayload`） |
| 验证方式 | HTTP `Authorization` 头中的 JWT | **消息体本身就是签名数据** |
| 公钥来源 | `googleapis.com/oauth2/v3/certs`（在线获取 JWK Set） | JWS header 的 `x5c` 证书链 → Apple Root CA G3 |
| 核心区别 | 安全验证在**传输层**——消息体明文 + 单独的 JWT 认证 | 安全验证在**数据层**——消息体自带签名，一体化验证 |

### 购买验证安全

- 服务端调用 **Google Play Android Developer API** 验证 `purchaseToken`，结果来自 Google 官方，无法伪造
- 校验 `packageName` 与配置的 `package_name` 一致，防止将其他应用的购买凭证用于本应用
- 验证通过后再进行 Acknowledge/Consume 等操作，确保状态与 Google 权威数据一致

### Webhook 安全验证

| 机制 | 说明 | 配置 |
|------|------|------|
| **Pub/Sub JWT** | **必选**：验证推送请求的 JWT 签名，确保请求来自 Google Pub/Sub | `webhook_url` + `verify_push_jwt=true` |
| **包名校验** | 校验通知中的 `packageName` 与配置一致，防止跨应用伪造 | `package_name` |
| **订阅名校验** | 可选：校验 `subscription` 字段，确保消息来自指定 Pub/Sub 订阅 | `expected_subscription` |
| **二次验证** | 所有关键事件处理前调用 Google API 确认状态 | 自动 |

> **重要**：配置了 `webhook_url` 后，**必须**同时设置 `verify_push_jwt=true`，否则 Webhook 请求会被拒绝（403）。

Google Pub/Sub 发送的 Webhook 请求格式：

```http
POST https://api.myapp.com/webhook/google
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOi...
Content-Type: application/json

{"message":{"data":"base64encoded...","messageId":"123456"}}
```

#### 验签流程

1. 从 `Authorization: Bearer <token>` 中提取 JWT
2. 调用 `idtoken.Validate(ctx, token, webhookURL)` 验证：
   - **签名验证**：从 Google 公钥端点 `https://www.googleapis.com/oauth2/v3/certs` 获取 JWK Set，通过 JWT header 中的 `kid` 匹配公钥，验证 RS256 签名
   - **过期检查**：检查 JWT 的 `exp` 字段
   - **audience 校验**：检查 JWT 中的 `aud` 是否等于配置的 `webhook_url`，防止 JWT 被重放到其他端点
3. 校验通过后解码消息体中的 `data`（base64），解析通知内容
4. 校验 `packageName`、`subscription`（可选）等字段

#### 防伪造原理

```
攻击者伪造请求 → 没有 Google 服务账号私钥 → 无法生成合法 JWT 签名
               → idtoken.Validate 验证失败 → 403 拒绝

攻击者重放其他端点的 JWT → aud 不匹配 webhook_url → 403 拒绝
```

> **公钥无需手动配置**：Google 的公钥公开发布在 `googleapis.com/oauth2/v3/certs`，库内部会自动获取并缓存，定期轮换时自动刷新。

### 启用 Pub/Sub JWT 验证

1. 在 Google Cloud Console 创建推送订阅时**勾选「启用身份验证」**
2. 在 config.toml 或环境变量中配置：
   - `webhook_url`：Webhook 完整 URL（与 Pub/Sub 订阅的 push endpoint 一致，作为 JWT audience）
   - `verify_push_jwt = true`：必须为 true
   - `expected_subscription`（可选）：期望的订阅全名，如 `projects/xxx/subscriptions/yyy`

## 最佳实践

### 1. 幂等性处理

Webhook 可能重复发送，确保处理逻辑是幂等的：

```go
// 使用 notification_id 或 order_id 作为幂等键
func HandleWebhook(notification *WebhookNotification) error {
    // 检查是否已处理
    if isProcessed(notification.NotificationID) {
        return nil // 已处理，直接返回成功
    }
    
    // 处理通知
    err := processNotification(notification)
    if err != nil {
        return err
    }
    
    // 标记为已处理
    markAsProcessed(notification.NotificationID)
    return nil
}
```

### 2. 及时确认购买（3 天 Acknowledge 合规）

购买后必须在 3 天内调用 Acknowledge，否则 Google 会自动退款。

**多层保障机制**：

| 层级 | 触发时机 | 说明 |
|------|----------|------|
| **主流程** | Verify 成功后 | `VerifyPurchase`/`VerifySubscription` 在验证成功且 `acknowledgementState == 0` 时自动调用 Acknowledge |
| **Webhook 兜底** | 收到 `ONE_TIME_PRODUCT_PURCHASED` / `SUBSCRIPTION_PURCHASED` | 二次验证后若仍未确认，再次尝试 Acknowledge，覆盖 Verify 流程中 Acknowledge 失败的情况 |
| **客户端重试** | 手动 | 可单独调用 `acknowledge-purchase` / `acknowledge-subscription` 接口 |

若主流程 Acknowledge 失败，仅记录日志不阻断；Webhook 收到购买通知时会再次尝试，形成兜底。

### 3. 订阅状态判断

```go
func GetSubscriptionStatus(subscription *SubscriptionResponse) string {
    now := time.Now()
    expiryTime := parseMillis(subscription.ExpiryTimeMillis)
    
    // 已过期
    if expiryTime.Before(now) && !subscription.AutoRenewing {
        return "EXPIRED"
    }
    
    // 已取消但未过期
    if subscription.CancelReason != 0 && expiryTime.After(now) {
        return "CANCELLED_ACTIVE"
    }
    
    // 正常活跃
    if subscription.PaymentState == 1 && expiryTime.After(now) {
        return "ACTIVE"
    }
    
    return "UNKNOWN"
}
```

### 4. 错误处理

```go
// 使用指数退避重试
func RetryWithBackoff(fn func() error, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        err := fn()
        if err == nil {
            return nil
        }
        
        // 指数退避
        delay := time.Duration(math.Pow(2, float64(i))) * time.Second
        time.Sleep(delay)
    }
    return errors.New("max retries exceeded")
}
```

## 相关文档

- [Google Play Billing 官方文档](https://developer.android.com/google/play/billing)
- [实时开发者通知](https://developer.android.com/google/play/billing/getting-ready#real-time-developer-notifications)
- [服务端验证](https://developers.google.com/android-publisher/api-ref/rest/v3/purchases.products)

