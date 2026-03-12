# Apple Store 支付接入指南

本文档详细介绍如何接入 Apple 应用内购买（IAP）和订阅功能。

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
| 自动续期订阅 | Auto-Renewable Subscription | ✅ |
| 收据验证 | Receipt Validation | ✅ |
| 交易验证 | Transaction Verification (推荐) | ✅ |
| 交易历史 | Transaction History | ✅ |
| Server Notifications V2 | 服务器通知 | ✅ |

## 配置

### 1. 创建 App Store Connect API 密钥

1. 登录 [App Store Connect](https://appstoreconnect.apple.com/)
2. 进入 **用户和访问 > 密钥 > App Store Connect API**
3. 点击 **+** 创建新密钥
4. 选择 **App 管理** 权限
5. 下载密钥文件（.p8），记录 Key ID 和 Issuer ID

### 2. 配置服务器通知

1. 进入 **我的 App > App 信息**
2. 配置 **App Store 服务器通知**：
   - 生产服务器 URL: `https://your-domain.com/webhook/apple`
   - 沙盒服务器 URL: `https://your-domain.com/webhook/apple`
   - 版本：选择 **版本 2**

### 3. 配置 config.toml

```toml
[apple]
# App Store Connect API 密钥 ID
key_id = "ABC123DEF4"

# 发行者 ID
issuer_id = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"

# Bundle ID
bundle_id = "com.example.app"

# 私钥内容或路径
private_key = '''
-----BEGIN PRIVATE KEY-----
MIGTAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBHkwdwIBAQQg...
-----END PRIVATE KEY-----
'''
# 或者使用文件路径
# private_key_path = "/path/to/AuthKey_ABC123DEF4.p8"

# 是否为沙盒环境
sandbox = true
```

## 支付流程

### 一次性购买流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           一次性购买流程                                      │
└─────────────────────────────────────────────────────────────────────────────┘

    客户端 (iOS)               服务端                      Apple Server
        │                        │                             │
        │  1. 创建订单           │                             │
        │ ────────────────────> │                             │
        │  POST /apple/purchases │                             │
        │                        │                             │
        │  返回 order_id         │                             │
        │ <──────────────────── │                             │
        │                        │                             │
        │  2. 发起 StoreKit 购买 │                             │
        │ ─────────────────────────────────────────────────> │
        │                        │                             │
        │  返回 Transaction      │                             │
        │ <───────────────────────────────────────────────── │
        │  (包含 transactionId, originalTransactionId)        │
        │                        │                             │
        │  3. 发送交易验证请求    │                             │
        │ ────────────────────> │                             │
        │  POST /apple/verify-transaction                     │
        │  {transaction_id, order_id}                         │
        │                        │                             │
        │                        │  调用 App Store Server API  │
        │                        │ ─────────────────────────> │
        │                        │  Get Transaction Info       │
        │                        │                             │
        │                        │  返回交易详情               │
        │                        │ <───────────────────────── │
        │                        │                             │
        │                        │  保存支付记录               │
        │                        │  更新订单状态               │
        │                        │                             │
        │  返回验证结果          │                             │
        │ <──────────────────── │                             │
        │                        │                             │
        │  4. 完成交易           │                             │
        │ ─────────────────────────────────────────────────> │
        │  finishTransaction()   │                             │
        │                        │                             │
        └                        └                             └
```

### 订阅流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              订阅流程                                        │
└─────────────────────────────────────────────────────────────────────────────┘

    客户端 (iOS)               服务端                      Apple Server
        │                        │                             │
        │  1. 创建订阅订单       │                             │
        │ ────────────────────> │                             │
        │  POST /apple/subscriptions                          │
        │                        │                             │
        │  返回 order_id         │                             │
        │ <──────────────────── │                             │
        │                        │                             │
        │  2. 发起 StoreKit 订阅 │                             │
        │ ─────────────────────────────────────────────────> │
        │                        │                             │
        │  返回 Transaction      │                             │
        │ <───────────────────────────────────────────────── │
        │                        │                             │
        │  3. 验证交易           │                             │
        │ ────────────────────> │                             │
        │  POST /apple/verify-transaction                     │
        │                        │                             │
        │                        │  验证 & 保存               │
        │                        │ ─────────────────────────> │
        │                        │ <───────────────────────── │
        │                        │                             │
        │  返回订阅状态          │                             │
        │ <──────────────────── │                             │
        │                        │                             │
        │  4. 完成交易           │                             │
        │ ─────────────────────────────────────────────────> │
        │                        │                             │
        └                        │                             │
                                 │                             │
        ┌──────────── 后续续费/状态变更 (Server Notification V2) ─────────────┐
        │                        │                             │
        │                        │  Webhook 通知               │
        │                        │ <───────────────────────── │
        │                        │  POST /webhook/apple        │
        │                        │  {signedPayload: "..."}     │
        │                        │                             │
        │                        │  解析 JWS 签名              │
        │                        │  - 验证签名                 │
        │                        │  - 解码 payload             │
        │                        │  - 提取交易/续订信息        │
        │                        │                             │
        │                        │  根据通知类型处理           │
        │                        │  - DID_RENEW: 续费成功      │
        │                        │  - EXPIRED: 订阅过期        │
        │                        │  - REFUND: 退款             │
        │                        │                             │
        │                        │  返回 200 OK               │
        │                        │ ─────────────────────────> │
        │                        │                             │
        └                        └                             └
```

## API 接口

### 创建内购订单

```http
POST /api/v1/apple/purchases
Content-Type: application/json

{
  "user_id": 1,
  "product_id": "com.example.premium",
  "title": "高级版",
  "description": "解锁所有功能",
  "quantity": 1,
  "currency": "USD",
  "price": 999,
  "developer_payload": "user_123"
}
```

**响应：**

```json
{
  "status": "success",
  "message": "内购订单创建成功",
  "data": {
    "id": 1,
    "order_no": "ORD20240101120000abcd1234",
    "user_id": 1,
    "product_id": "com.example.premium",
    "type": "PURCHASE",
    "status": "CREATED",
    "payment_method": "APPLE_STORE",
    "total_amount": 999,
    "currency": "USD"
  }
}
```

### 创建订阅订单

```http
POST /api/v1/apple/subscriptions
Content-Type: application/json

{
  "user_id": 1,
  "product_id": "com.example.subscription.monthly",
  "title": "月度会员",
  "description": "每月自动续费",
  "currency": "USD",
  "price": 999,
  "period": "P1M",
  "developer_payload": "user_123"
}
```

### 验证交易（推荐）

使用 App Store Server API 验证交易，这是推荐的验证方式：

```http
POST /api/v1/apple/verify-transaction
Content-Type: application/json

{
  "transaction_id": "1000000123456789",
  "order_id": 1
}
```

**响应：**

```json
{
  "status": "success",
  "message": "Transaction verified successfully",
  "data": {
    "transaction_id": "1000000123456789",
    "original_transaction_id": "1000000123456780",
    "product_id": "com.example.premium",
    "bundle_id": "com.example.app",
    "purchase_date": "2024-01-01T12:00:00Z",
    "quantity": 1,
    "status": "VERIFIED"
  }
}
```

### 验证收据（兼容旧版）

```http
POST /api/v1/apple/verify-receipt
Content-Type: application/json

{
  "receipt_data": "base64_encoded_receipt_data",
  "order_id": 1,
  "is_sandbox": true
}
```

### 获取交易历史

```http
GET /api/v1/apple/transactions/1000000123456780/history
```

**响应：**

```json
{
  "status": "success",
  "data": [
    {
      "transaction_id": "1000000123456789",
      "original_transaction_id": "1000000123456780",
      "product_id": "com.example.subscription.monthly",
      "purchase_date": "2024-01-01T12:00:00Z",
      "expires_date": "2024-02-01T12:00:00Z"
    },
    {
      "transaction_id": "1000000123456790",
      "original_transaction_id": "1000000123456780",
      "product_id": "com.example.subscription.monthly",
      "purchase_date": "2024-02-01T12:00:00Z",
      "expires_date": "2024-03-01T12:00:00Z"
    }
  ]
}
```

### 获取订阅状态

```http
GET /api/v1/apple/subscriptions/1000000123456780/status
```

## Webhook 处理

### Server Notification V2 通知类型

| 类型 | 子类型 | 说明 |
|-----|--------|-----|
| `SUBSCRIBED` | `INITIAL_BUY` | 首次订阅 |
| `SUBSCRIBED` | `RESUBSCRIBE` | 重新订阅 |
| `DID_RENEW` | - | 续费成功 |
| `DID_FAIL_TO_RENEW` | `GRACE_PERIOD` | 续费失败（宽限期内） |
| `DID_FAIL_TO_RENEW` | - | 续费失败 |
| `DID_CHANGE_RENEWAL_STATUS` | `AUTO_RENEW_ENABLED` | 开启自动续费 |
| `DID_CHANGE_RENEWAL_STATUS` | `AUTO_RENEW_DISABLED` | 关闭自动续费 |
| `DID_CHANGE_RENEWAL_PREF` | - | 续费偏好变更 |
| `EXPIRED` | `VOLUNTARY` | 用户取消导致过期 |
| `EXPIRED` | `BILLING_RETRY` | 计费重试失败过期 |
| `EXPIRED` | `PRICE_INCREASE` | 拒绝涨价导致过期 |
| `GRACE_PERIOD_EXPIRED` | - | 宽限期过期 |
| `REFUND` | - | 退款 |
| `REFUND_REVERSED` | - | 退款撤销 |
| `REVOKE` | - | 家庭共享撤销 |
| `CONSUMPTION_REQUEST` | - | 消耗请求 |
| `ONE_TIME_CHARGE` | - | 一次性购买 |
| `TEST` | - | 测试通知 |

### Webhook 处理流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Server Notification V2 处理流程                        │
└─────────────────────────────────────────────────────────────────────────────┘

  Apple Server                 服务端                        数据库
      │                          │                             │
      │  1. 发送签名通知         │                             │
      │ ──────────────────────> │                             │
      │  POST /webhook/apple     │                             │
      │  {signedPayload: "xxx.yyy.zzz"}                        │
      │                          │                             │
      │                          │  2. 解析 JWS                │
      │                          │  ┌──────────────────────┐   │
      │                          │  │ - 分割 header.payload.│   │
      │                          │  │   signature           │   │
      │                          │  │ - Base64 解码         │   │
      │                          │  │ - 验证签名            │   │
      │                          │  │ - 提取通知类型        │   │
      │                          │  └──────────────────────┘   │
      │                          │                             │
      │                          │  3. 解析交易信息           │
      │                          │  ┌──────────────────────┐   │
      │                          │  │ signedTransactionInfo │   │
      │                          │  │ - transactionId       │   │
      │                          │  │ - originalTransactionId│  │
      │                          │  │ - productId           │   │
      │                          │  │ - purchaseDate        │   │
      │                          │  │ - expiresDate         │   │
      │                          │  └──────────────────────┘   │
      │                          │                             │
      │                          │  4. 根据通知类型处理       │
      │                          │                             │
      │                          │  SUBSCRIBED / DID_RENEW:    │
      │                          │  - 创建/更新 ApplePayment   │
      │                          │  - 更新 Order 状态为 PAID   │
      │                          │ ─────────────────────────> │
      │                          │                             │
      │                          │  EXPIRED:                   │
      │                          │  - 更新订阅状态为 EXPIRED   │
      │                          │  - 更新 Order 状态         │
      │                          │ ─────────────────────────> │
      │                          │                             │
      │                          │  REFUND:                    │
      │                          │  - 创建退款记录            │
      │                          │  - 更新订单为 REFUNDED     │
      │                          │ ─────────────────────────> │
      │                          │                             │
      │  5. 返回 200 OK         │                             │
      │ <────────────────────── │                             │
      │                          │                             │
      └                          └                             └
```

### 订单关联机制

Apple Webhook 不直接包含 `order_id`，系统通过以下机制关联：

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           订单关联机制                                       │
└─────────────────────────────────────────────────────────────────────────────┘

  首次购买时：
  ┌──────────────────────────────────────────────────────────────────────────┐
  │ 1. 客户端创建订单 → 获得 order_id                                         │
  │ 2. 客户端完成购买 → 获得 transaction_id, original_transaction_id         │
  │ 3. 客户端调用 verify-transaction → 服务端保存 ApplePayment               │
  │    ApplePayment {                                                        │
  │      order_id: 1,                           ← 关联订单                    │
  │      transaction_id: "1000000123456789",                                 │
  │      original_transaction_id: "1000000123456780"  ← 关键：原始交易ID     │
  │    }                                                                     │
  └──────────────────────────────────────────────────────────────────────────┘

  后续 Webhook 通知时：
  ┌──────────────────────────────────────────────────────────────────────────┐
  │ 1. 收到 Webhook，提取 original_transaction_id                            │
  │ 2. 查询 ApplePayment WHERE original_transaction_id = "1000000123456780"  │
  │ 3. 获得 order_id = 1                                                     │
  │ 4. 更新对应的 Order 状态                                                  │
  └──────────────────────────────────────────────────────────────────────────┘
```

## 安全机制

### 安全机制概览

| 环节 | 机制 | 说明 |
|------|------|------|
| **购买验证** | 收据验证 (Receipt) | 将收据发送至 Apple 服务器验证，兼容旧版 |
| **购买验证** | App Store Server API | 通过 `Get Transaction Info` 获取交易详情，推荐方式 |
| **购买验证** | 签名交易解析 | `ParseSignedTransaction` 解析并验证交易 JWS 签名 |
| **Webhook** | JWS 验签 | `ParseNotificationV2WithClaim` 验证 `signedPayload` 的 JWS 签名 |
| **Webhook** | 证书链验证 | 从 JWS header `x5c` 获取证书链，使用 Apple Root CA G3 验证 |
| **Webhook** | 嵌套 JWT 解析 | 验签通过后解析 `signedTransactionInfo`、`signedRenewalInfo` 等嵌套 JWT |

### 购买验证安全

- **收据验证**：`VerifyPurchase` 使用 `appstore.Verify` 将收据发送到 Apple 服务器验证，结果来自 Apple 官方
- **交易验证（推荐）**：`VerifyTransaction` 通过 App Store Server API 的 `GetTransactionInfo` 获取交易详情，并用 `ParseSignedTransaction` 解析签名交易数据，确保数据未被篡改
- 校验 `bundleId` 与配置一致，防止跨应用伪造

### Webhook 安全验证

- **JWS 验签**：使用 go-iap 的 `ParseNotificationV2WithClaim` 验证 `signedPayload`，验签失败则返回 400
- **证书链**：从 JWS header 的 `x5c` 获取证书链，使用 Apple Root CA G3 验证证书有效性
- **嵌套 JWT**：`signedTransactionInfo`、`signedRenewalInfo` 等嵌套 JWT 在外层 JWS 验签通过后再解析，保证端到端可信
- **处理流程**：`HandleAppleWebhook` → `ParseNotification`（内含 `ParseNotificationV2WithClaim`）→ 验签失败则不处理并返回错误

## 最佳实践

### 1. 使用 App Store Server API

推荐使用新的 App Store Server API 而非旧的收据验证：

```go
// 推荐：使用 App Store Server API
response, err := appleService.VerifyTransaction(ctx, transactionID)

// 不推荐：旧的收据验证方式
// response, err := appleService.VerifyPurchase(ctx, receiptData, orderID)
```

### 2. 正确处理 StoreKit 2 交易

iOS 客户端代码示例：

```swift
// StoreKit 2 (iOS 15+)
func purchase(product: Product) async throws {
    // 1. 调用服务端创建订单
    let order = try await api.createOrder(productId: product.id)
    
    // 2. 发起购买
    let result = try await product.purchase()
    
    switch result {
    case .success(let verification):
        let transaction = try checkVerified(verification)
        
        // 3. 发送交易到服务端验证
        try await api.verifyTransaction(
            transactionId: String(transaction.id),
            orderId: order.id
        )
        
        // 4. 完成交易
        await transaction.finish()
        
    case .pending:
        // 等待授权（如家长批准）
        break
    case .userCancelled:
        // 用户取消
        break
    @unknown default:
        break
    }
}
```

### 3. 幂等性处理

```go
func (s *AppleService) HandleNotification(ctx context.Context, notification *AppleNotification) error {
    // 使用 notification_uuid 确保幂等性
    if s.isNotificationProcessed(notification.NotificationUUID) {
        s.logger.Info("Notification already processed",
            zap.String("notification_uuid", notification.NotificationUUID))
        return nil
    }
    
    // 处理通知...
    
    s.markNotificationProcessed(notification.NotificationUUID)
    return nil
}
```

### 4. 宽限期处理

```go
func (s *AppleService) handleDidFailToRenew(ctx context.Context, notification *AppleNotification) error {
    if notification.Subtype == "GRACE_PERIOD" {
        // 在宽限期内，用户仍可使用服务
        // 可以发送提醒通知用户更新支付方式
        s.sendPaymentReminderNotification(payment.UserID)
        
        payment.GracePeriodStatus = "IN_GRACE_PERIOD"
    } else {
        // 宽限期外，应停止服务
        payment.Status = "RENEWAL_FAILED"
    }
    
    return s.db.Save(&payment).Error
}
```

### 5. 退款处理

```go
func (s *AppleService) handleRefund(ctx context.Context, notification *AppleNotification) error {
    // 1. 创建退款记录
    refund := &models.AppleRefund{
        OrderID:             payment.OrderID,
        RefundTransactionID: transactionInfo.TransactionID,
        RefundStatus:        "REFUNDED",
        RefundDate:          transactionInfo.RevocationDate,
    }
    
    // 2. 撤销用户权益
    s.revokeUserEntitlement(payment.OrderID)
    
    // 3. 更新订单状态
    s.db.Model(&models.Order{}).
        Where("id = ?", payment.OrderID).
        Update("status", models.OrderStatusRefunded)
    
    return nil
}
```

## 相关文档

- [App Store Server API](https://developer.apple.com/documentation/appstoreserverapi)
- [App Store Server Notifications V2](https://developer.apple.com/documentation/appstoreservernotifications)
- [StoreKit 2](https://developer.apple.com/documentation/storekit/in-app_purchase)
- [验证购买](https://developer.apple.com/documentation/storekit/in-app_purchase/original_api_for_in-app_purchase/validating_receipts_with_the_app_store)

