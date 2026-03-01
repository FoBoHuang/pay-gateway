# Google Play 支付接入指南

本文档详细介绍如何接入 Google Play 内购和订阅功能。

## 📋 目录

- [功能概述](#功能概述)
- [配置](#配置)
- [支付流程](#支付流程)
- [API 接口](#api-接口)
- [Webhook 处理](#webhook-处理)
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
# 服务账号 JSON 文件内容或路径
service_account_file = '''
{
  "type": "service_account",
  "project_id": "your-project-id",
  "private_key_id": "xxx",
  "private_key": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n",
  "client_email": "xxx@xxx.iam.gserviceaccount.com",
  "client_id": "xxx",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token"
}
'''

# Android 应用包名
package_name = "com.example.app"
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

### Webhook 安全验证

| 机制 | 说明 | 配置 |
|------|------|------|
| **包名校验** | 校验通知中的 `packageName` 与配置一致，防止跨应用伪造 | `GOOGLE_PACKAGE_NAME` |
| **二次验证** | 所有关键事件（购买/取消/续订/过期等）处理前调用 Google API 确认状态 | 自动 |
| **Pub/Sub JWT** | 验证推送请求的 JWT 签名（需在 GCP 订阅中启用认证） | `GOOGLE_VERIFY_PUSH_JWT=true`、`GOOGLE_WEBHOOK_URL` |

### 启用 Pub/Sub JWT 验证

1. 在 Google Cloud Console 创建推送订阅时勾选「启用身份验证」
2. 配置环境变量：
   - `GOOGLE_WEBHOOK_URL`：Webhook 完整 URL（如 `https://your-domain.com/webhook/google`）
   - `GOOGLE_VERIFY_PUSH_JWT=true`

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

