# Google Play 和 Apple 内购及订阅完整指南

本文档详细介绍Google Play和Apple的内购（IAP）及订阅功能的实现和使用方法。

## 目录

1. [功能概述](#功能概述)
2. [Google Play内购和订阅](#google-play内购和订阅)
3. [Apple内购和订阅](#apple内购和订阅)
4. [代码位置索引](#代码位置索引)
5. [使用示例](#使用示例)
6. [测试指南](#测试指南)

---

## 功能概述

### ✅ Google Play内购和订阅 - 已完整实现

**功能列表**：
1. ✅ 验证单次购买
2. ✅ 验证订阅
3. ✅ 确认购买（防止重复发放）
4. ✅ 确认订阅
5. ✅ 消费购买（消耗型商品）
6. ✅ 获取订阅状态
7. ✅ Webhook通知处理

**文件位置**：
- 服务层：`internal/services/google_play_service.go` (392行)
- 处理器：`internal/handlers/handlers.go` 和 `internal/handlers/webhook.go`
- 数据模型：`internal/models/payment_models.go` (GooglePayment)

### ✅ Apple内购和订阅 - 已完整实现

**功能列表**：
1. ✅ 验证收据（旧版API）
2. ✅ 验证交易（App Store Server API，推荐）
3. ✅ 获取交易历史
4. ✅ 获取订阅状态
5. ✅ 保存支付信息
6. ✅ Server-to-Server通知处理

**文件位置**：
- 服务层：`internal/services/apple_service.go` (442行)
- 处理器：`internal/handlers/apple_handler.go` (304行)
- Webhook：`internal/handlers/apple_webhook.go`
- 数据模型：`internal/models/payment_models.go` (ApplePayment)

---

## Google Play内购和订阅

### 核心方法

#### 服务层 (`google_play_service.go`)

```go
// 验证购买
VerifyPurchase(ctx, productID, purchaseToken) (*PurchaseResponse, error)
  位置：第117-147行

// 验证订阅
VerifySubscription(ctx, subscriptionID, purchaseToken) (*SubscriptionResponse, error)
  位置：第157-206行

// 确认购买（防止重复发放）
AcknowledgePurchase(ctx, productID, purchaseToken, developerPayload) error
  位置：第217-236行

// 确认订阅
AcknowledgeSubscription(ctx, subscriptionID, purchaseToken, developerPayload) error
  位置：第247-266行

// 消费购买（消耗型商品）
ConsumePurchase(ctx, productID, purchaseToken) error
  位置：第276-292行

// 获取订阅状态
GetSubscriptionStatus(subscription, currentTime) SubscriptionState
  位置：第347-377行

// 验证Webhook签名
VerifyWebhookSignature(payload, signature) error
  位置：第309-323行
```

### API端点

```
POST   /api/v1/payments/process      # 统一支付处理（包含Google Play验证）
POST   /webhook/google-play          # Google Play Webhook通知
```

### 使用流程

#### 1. 验证购买（一次性商品）

```bash
curl -X POST http://localhost:8080/api/v1/payments/process \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": 123,
    "provider": "GOOGLE_PLAY",
    "purchase_token": "purchase_token_from_client",
    "product_id": "premium_upgrade",
    "developer_payload": "user_123"
  }'
```

**流程说明**：
1. 客户端完成购买后获得 `purchaseToken`
2. 将 `purchaseToken` 发送到服务端
3. 服务端调用 `VerifyPurchase` 验证购买
4. 验证成功后调用 `AcknowledgePurchase` 确认
5. 保存支付记录到数据库
6. 返回验证结果给客户端

#### 2. 验证订阅

```bash
curl -X POST http://localhost:8080/api/v1/payments/process \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": 124,
    "provider": "GOOGLE_PLAY",
    "purchase_token": "subscription_token_from_client",
    "subscription_id": "premium_monthly",
    "developer_payload": "user_123"
  }'
```

**流程说明**：
1. 客户端完成订阅后获得 `purchaseToken`
2. 将 `purchaseToken` 发送到服务端
3. 服务端调用 `VerifySubscription` 验证订阅
4. 验证成功后调用 `AcknowledgeSubscription` 确认
5. 保存订阅记录到数据库
6. 返回验证结果，包含到期时间、自动续费状态等

#### 3. 消费购买（消耗型商品）

对于消耗型商品（如游戏内金币），需要调用消费接口：

```go
// 在代码中调用
err := googleService.ConsumePurchase(ctx, productID, purchaseToken)
```

**说明**：
- 消费后该商品可以再次购买
- 适用于游戏内货币、道具等消耗型商品
- 必须在发放商品后立即调用

### 数据结构

#### PurchaseResponse（购买响应）

```go
type PurchaseResponse struct {
    Kind                        string  // 类型
    PurchaseTimeMillis          string  // 购买时间（毫秒）
    PurchaseState               int     // 购买状态：0-已购买，1-取消
    ConsumptionState            int     // 消费状态：0-未消费，1-已消费
    DeveloperPayload            string  // 开发者自定义数据
    OrderId                     string  // 订单ID
    AcknowledgementState        int     // 确认状态：0-未确认，1-已确认
    RegionCode                  string  // 地区代码
}
```

#### SubscriptionResponse（订阅响应）

```go
type SubscriptionResponse struct {
    Kind                  string  // 类型
    StartTimeMillis       string  // 开始时间
    ExpiryTimeMillis      string  // 到期时间
    AutoRenewing          bool    // 是否自动续费
    PriceCurrencyCode     string  // 货币代码
    PriceAmountMicros     string  // 价格（微单位）
    PaymentState          int     // 支付状态：0-待支付，1-已支付，2-免费试用
    CancelReason          int     // 取消原因
    OrderId               string  // 订单ID
    AcknowledgementState  int     // 确认状态
}
```

### Webhook处理

Google Play会发送实时开发者通知(Real-time Developer Notifications)：

**通知类型**：
1. `SUBSCRIPTION_RECOVERED` - 订阅恢复
2. `SUBSCRIPTION_RENEWED` - 订阅续费
3. `SUBSCRIPTION_CANCELED` - 订阅取消
4. `SUBSCRIPTION_PURCHASED` - 订阅购买
5. `SUBSCRIPTION_ON_HOLD` - 订阅暂停
6. `SUBSCRIPTION_IN_GRACE_PERIOD` - 宽限期
7. `SUBSCRIPTION_RESTARTED` - 订阅重启
8. `SUBSCRIPTION_PRICE_CHANGE_CONFIRMED` - 价格变更确认
9. `SUBSCRIPTION_DEFERRED` - 订阅延期
10. `SUBSCRIPTION_PAUSED` - 订阅暂停
11. `SUBSCRIPTION_PAUSE_SCHEDULE_CHANGED` - 暂停计划变更
12. `SUBSCRIPTION_REVOKED` - 订阅撤销
13. `SUBSCRIPTION_EXPIRED` - 订阅过期

---

## Apple内购和订阅

### 核心方法

#### 服务层 (`apple_service.go`)

```go
// 验证收据（旧版API）
VerifyPurchase(ctx, receiptData, orderID) (*ApplePurchaseResponse, error)
  位置：第128-222行

// 验证交易（App Store Server API，推荐）
VerifyTransaction(ctx, transactionID) (*ApplePurchaseResponse, error)
  位置：第231-286行

// 获取交易历史
GetTransactionHistory(ctx, originalTransactionID) ([]*ApplePurchaseResponse, error)
  位置：第294-354行

// 解析通知
ParseNotification(signedPayload) (*jwt.Token, error)
  位置：第361-381行

// 保存支付信息
SaveApplePayment(ctx, orderID, response) error
  位置：第390-433行
```

### API端点

```
POST   /api/v1/apple/verify-receipt                        # 验证收据
POST   /api/v1/apple/verify-transaction                    # 验证交易（推荐）
POST   /api/v1/apple/validate-receipt                      # 验证收据（简化版）
GET    /api/v1/apple/transactions/:id/history              # 获取交易历史
GET    /api/v1/apple/subscriptions/:id/status              # 获取订阅状态
POST   /webhook/apple                                      # Apple Webhook通知
```

### 使用流程

#### 1. 验证收据（旧版API）

```bash
curl -X POST http://localhost:8080/api/v1/apple/verify-receipt \
  -H "Content-Type: application/json" \
  -d '{
    "receipt_data": "base64_encoded_receipt_data",
    "order_id": 123,
    "is_sandbox": false
  }'
```

**流程说明**：
1. 客户端完成购买后获得收据（receipt）
2. 将收据Base64编码后发送到服务端
3. 服务端调用 `VerifyPurchase` 验证收据
4. 验证成功后保存支付信息到数据库
5. 返回验证结果，包含交易ID、商品信息等

#### 2. 验证交易（推荐，App Store Server API）

```bash
curl -X POST http://localhost:8080/api/v1/apple/verify-transaction \
  -H "Content-Type: application/json" \
  -d '{
    "transaction_id": "1000000123456789",
    "order_id": 123
  }'
```

**流程说明**：
1. 客户端完成购买后获得 `transactionID`
2. 将 `transactionID` 发送到服务端
3. 服务端调用 `VerifyTransaction` 使用App Store Server API验证
4. 验证成功后保存支付信息到数据库
5. 返回验证结果

**推荐使用这个方法的原因**：
- 不需要处理收据数据
- 更准确的验证结果
- 支持最新的Apple功能
- 性能更好

#### 3. 获取交易历史

```bash
curl -X GET http://localhost:8080/api/v1/apple/transactions/1000000123456789/history
```

**用途**：
- 查看用户所有的交易记录
- 检查订阅续费历史
- 审计和对账

#### 4. 获取订阅状态

```bash
curl -X GET http://localhost:8080/api/v1/apple/subscriptions/1000000123456789/status
```

**返回信息**：
- 订阅是否活跃
- 到期时间
- 自动续费状态
- 试用期信息
- 优惠信息

### 数据结构

#### ApplePurchaseResponse（购买响应）

```go
type ApplePurchaseResponse struct {
    TransactionID         string     // 交易ID
    OriginalTransactionID string     // 原始交易ID（订阅）
    ProductID             string     // 商品ID
    BundleID              string     // Bundle ID
    PurchaseDate          time.Time  // 购买时间
    OriginalPurchaseDate  time.Time  // 原始购买时间
    Quantity              int        // 数量
    IsTrialPeriod         bool       // 是否试用期
    IsInIntroOfferPeriod  bool       // 是否优惠期
    ExpiresDate           *time.Time // 到期时间（订阅）
    CancellationDate      *time.Time // 取消时间
    ProductType           string     // 商品类型
    Environment           string     // 环境（Sandbox/Production）
    Status                string     // 状态
}
```

#### AppleSubscriptionResponse（订阅响应）

```go
type AppleSubscriptionResponse struct {
    ApplePurchaseResponse            // 包含购买响应的所有字段
    AutoRenewStatus       bool       // 自动续费状态
    AutoRenewProductID    string     // 自动续费商品ID
    GracePeriodStatus     string     // 宽限期状态
    ExpirationIntent      string     // 到期意图
}
```

### Webhook处理

Apple会发送Server-to-Server通知：

**通知类型**：
1. `DID_CHANGE_RENEWAL_PREF` - 用户更改续费偏好
2. `DID_CHANGE_RENEWAL_STATUS` - 续费状态变更
3. `DID_FAIL_TO_RENEW` - 续费失败
4. `DID_RENEW` - 续费成功
5. `EXPIRED` - 订阅过期
6. `GRACE_PERIOD_EXPIRED` - 宽限期结束
7. `OFFER_REDEEMED` - 优惠已兑换
8. `PRICE_INCREASE_CONSENT` - 价格上涨同意
9. `REFUND` - 退款
10. `REVOKE` - 撤销
11. `SUBSCRIBED` - 新订阅

---

## 代码位置索引

### Google Play

| 功能 | 文件 | 行数 |
|------|------|------|
| 服务实例化 | `google_play_service.go` | 71-107 |
| 验证购买 | `google_play_service.go` | 117-147 |
| 验证订阅 | `google_play_service.go` | 157-206 |
| 确认购买 | `google_play_service.go` | 217-236 |
| 确认订阅 | `google_play_service.go` | 247-266 |
| 消费购买 | `google_play_service.go` | 276-292 |
| 获取订阅状态 | `google_play_service.go` | 347-377 |
| Webhook处理 | `webhook.go` | - |

### Apple

| 功能 | 文件 | 行数 |
|------|------|------|
| 服务实例化 | `apple_service.go` | 70-119 |
| 验证收据 | `apple_service.go` | 128-222 |
| 验证交易 | `apple_service.go` | 231-286 |
| 获取交易历史 | `apple_service.go` | 294-354 |
| 解析通知 | `apple_service.go` | 361-381 |
| 保存支付信息 | `apple_service.go` | 390-433 |
| HTTP处理器 | `apple_handler.go` | 全文304行 |
| Webhook处理器 | `apple_webhook.go` | - |

---

## 使用示例

### Google Play完整示例

```go
// 1. 初始化服务
googleService, err := services.NewGooglePlayService(cfg, logger)
if err != nil {
    log.Fatal(err)
}

// 2. 验证购买
purchase, err := googleService.VerifyPurchase(ctx, "product_id", "purchase_token")
if err != nil {
    log.Fatal(err)
}

// 3. 确认购买（防止重复发放）
err = googleService.AcknowledgePurchase(ctx, "product_id", "purchase_token", "user_123")
if err != nil {
    log.Fatal(err)
}

// 4. 消费购买（消耗型商品）
err = googleService.ConsumePurchase(ctx, "product_id", "purchase_token")
if err != nil {
    log.Fatal(err)
}

// 5. 验证订阅
subscription, err := googleService.VerifySubscription(ctx, "subscription_id", "purchase_token")
if err != nil {
    log.Fatal(err)
}

// 6. 确认订阅
err = googleService.AcknowledgeSubscription(ctx, "subscription_id", "purchase_token", "user_123")
if err != nil {
    log.Fatal(err)
}

// 7. 获取订阅状态
status := services.GetSubscriptionStatus(subscription, time.Now())
```

### Apple完整示例

```go
// 1. 初始化服务
appleService, err := services.NewAppleService(cfg, logger, db)
if err != nil {
    log.Fatal(err)
}

// 2. 验证收据（旧版）
purchase, err := appleService.VerifyPurchase(ctx, receiptData, orderID)
if err != nil {
    log.Fatal(err)
}

// 3. 验证交易（推荐）
purchase, err := appleService.VerifyTransaction(ctx, transactionID)
if err != nil {
    log.Fatal(err)
}

// 4. 保存支付信息
err = appleService.SaveApplePayment(ctx, orderID, purchase)
if err != nil {
    log.Fatal(err)
}

// 5. 获取交易历史
history, err := appleService.GetTransactionHistory(ctx, originalTransactionID)
if err != nil {
    log.Fatal(err)
}

// 6. 解析Apple通知
token, err := appleService.ParseNotification(signedPayload)
if err != nil {
    log.Fatal(err)
}
```

---

## 测试指南

### Google Play测试

1. **使用测试账号**
   - 在Google Play Console创建测试账号
   - 使用测试账号进行购买和订阅

2. **使用静态测试响应**
   - Google提供固定的测试商品ID
   - `android.test.purchased` - 成功购买
   - `android.test.canceled` - 取消购买
   - `android.test.refunded` - 退款
   - `android.test.item_unavailable` - 商品不可用

3. **测试Webhook**
   - 在Google Play Console配置Webhook URL
   - 使用Google Play的测试工具发送测试通知

### Apple测试

1. **使用沙盒环境**
   - 在App Store Connect创建沙盒测试账号
   - 配置 `Apple.Sandbox = true`

2. **测试收据验证**
   ```bash
   # 沙盒环境
   https://sandbox.itunes.apple.com/verifyReceipt
   
   # 生产环境
   https://buy.itunes.apple.com/verifyReceipt
   ```

3. **测试订阅**
   - 沙盒环境下订阅周期被压缩
   - 3天订阅 → 3分钟
   - 1周订阅 → 3分钟
   - 1月订阅 → 5分钟
   - 6个月订阅 → 10分钟
   - 1年订阅 → 1小时

4. **测试Webhook**
   - 配置Server-to-Server通知URL
   - Apple会发送测试通知验证URL有效性

---

## 最佳实践

### 1. 安全性

- ✅ 始终在服务端验证购买
- ✅ 不要相信客户端传来的验证结果
- ✅ 使用HTTPS传输
- ✅ 验证Webhook签名

### 2. 性能优化

- ✅ 缓存验证结果
- ✅ 使用异步处理Webhook
- ✅ 批量处理订阅更新

### 3. 错误处理

- ✅ 实现重试机制
- ✅ 记录详细日志
- ✅ 友好的错误提示

### 4. 订阅管理

- ✅ 定期查询订阅状态
- ✅ 处理订阅暂停、恢复
- ✅ 支持宽限期
- ✅ 处理价格变更

---

## 常见问题

### Q1: Google Play和Apple有什么区别？

**A**: 主要区别：
- **验证方式**：Google使用purchaseToken，Apple使用receipt或transactionID
- **API风格**：Google是RESTful API，Apple有旧版收据验证和新版Server API
- **订阅周期**：不同的订阅周期选项
- **通知机制**：都支持Server-to-Server通知，但格式不同

### Q2: 什么时候使用Acknowledge？

**A**: 
- Google Play要求在购买后3天内确认(Acknowledge)
- 未确认的购买会被退款
- 订阅也需要确认
- 消耗型商品需要消费(Consume)后才能再次购买

### Q3: Apple应该使用哪个API？

**A**: 
- **推荐**：使用App Store Server API (`VerifyTransaction`)
- **旧版**：收据验证API (`VerifyPurchase`)
- 新API更准确、更快、功能更强

### Q4: 如何处理订阅续费？

**A**: 
- 监听Webhook通知
- 定期轮询订阅状态
- 处理续费失败情况
- 支持宽限期机制

### Q5: 如何测试？

**A**: 
- **Google**: 使用测试账号和静态测试商品
- **Apple**: 使用沙盒环境和测试账号
- 两者都支持Webhook测试

---

## 总结

✅ **Google Play和Apple内购及订阅功能已完整实现**

**实现内容**：
- Google Play：购买验证、订阅验证、确认、消费、Webhook
- Apple：收据验证、交易验证、历史查询、订阅状态、Webhook

**代码位置**：
- Google服务：`internal/services/google_play_service.go`
- Apple服务：`internal/services/apple_service.go`
- Apple处理器：`internal/handlers/apple_handler.go`
- Webhook：`internal/handlers/webhook.go` 和 `internal/handlers/apple_webhook.go`

**使用方法**：
- 参考本文档的API端点和示例代码
- 查看详细的流程说明
- 按照最佳实践实施

所有功能已经过验证，代码结构清晰，可以直接使用！🎉

