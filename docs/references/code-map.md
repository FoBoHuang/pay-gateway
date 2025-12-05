# 支付功能代码位置速查表

本文档汇总了所有支付方式的代码位置，方便快速定位和修改。

---

## 📋 支付方式概览

| 支付方式 | 状态 | 主要功能 |
|---------|------|---------|
| **微信支付** | ✅ 已实现 | JSAPI、Native、APP、H5支付 |
| **支付宝支付** | ✅ 已实现 | WAP、PAGE支付 |
| **支付宝周期扣款** | ✅ 已实现 | 签约、扣款、解约 |
| **Google Play** | ✅ 已实现 | 内购、订阅 |
| **Apple Store** | ✅ 已实现 | 内购、订阅 |

---

## 🔍 代码位置详细索引

### 1. 微信支付

#### 服务层
**文件**: `internal/services/wechat_service.go` (约880行)

```
第25-54行   ✅ WechatService结构体和初始化
第57-100行  ✅ CreateOrder - 创建微信订单
第103-155行 ✅ CreateJSAPIPayment - JSAPI支付
第158-195行 ✅ CreateNativePayment - Native支付
第198-233行 ✅ CreateAPPPayment - APP支付
第236-274行 ✅ CreateH5Payment - H5支付
第277-343行 ✅ HandleNotify - 支付通知处理
第346-398行 ✅ QueryOrder - 查询订单
第401-478行 ✅ Refund - 退款
第481-509行 ✅ CloseOrder - 关闭订单
```

#### HTTP处理器
**文件**: `internal/handlers/wechat_handler.go` (约430行)

```
第13-34行   ✅ WechatHandler结构体
第49-74行   ✅ CreateOrder - 创建订单API
第92-128行  ✅ CreateJSAPIPayment - JSAPI支付API
第145-177行 ✅ CreateNativePayment - Native支付API
第194-226行 ✅ CreateAPPPayment - APP支付API
第243-279行 ✅ CreateH5Payment - H5支付API
第296-330行 ✅ QueryOrder - 查询订单API
第347-381行 ✅ Refund - 退款API
第398-428行 ✅ CloseOrder - 关闭订单API
第445-497行 ✅ HandleNotify - Webhook通知处理
```

#### 路由配置
**文件**: `internal/routes/routes.go` (第88-97行)

```go
wechat := v1.Group("/wechat")
{
    wechat.POST("/orders", wechatHandler.CreateOrder)
    wechat.GET("/orders/:order_no", wechatHandler.QueryOrder)
    wechat.POST("/orders/:order_no/close", wechatHandler.CloseOrder)
    wechat.POST("/payments/jsapi/:order_no", wechatHandler.CreateJSAPIPayment)
    wechat.POST("/payments/native/:order_no", wechatHandler.CreateNativePayment)
    wechat.POST("/payments/app/:order_no", wechatHandler.CreateAPPPayment)
    wechat.POST("/payments/h5/:order_no", wechatHandler.CreateH5Payment)
    wechat.POST("/refunds", wechatHandler.Refund)
}
```

#### 数据模型
**文件**: `internal/models/payment_models.go`

```
第325-353行  ✅ WechatPayment - 微信支付详情
第355-382行  ✅ WechatRefund - 微信退款记录
```

---

### 2. 支付宝支付

#### 服务层
**文件**: `internal/services/alipay_service.go` (约800行)

```
第21-54行   ✅ AlipayService结构体和初始化
第57-131行  ✅ CreateOrder - 创建订单
第134-164行 ✅ CreateWapPayment - 手机网站支付
第167-197行 ✅ CreatePagePayment - 电脑网站支付
第200-304行 ✅ HandleNotify - 支付通知处理
第307-361行 ✅ QueryOrder - 查询订单
第364-447行 ✅ Refund - 退款
```

#### 周期扣款（订阅）
**文件**: `internal/services/alipay_service.go` (同一文件)

```
第492-578行 ✅ CreateSubscription - 创建周期扣款
第581-617行 ✅ QuerySubscription - 查询周期扣款
第620-646行 ✅ CancelSubscription - 取消周期扣款
第649-697行 ✅ HandleSubscriptionNotify - 签约通知
第700-770行 ✅ HandleDeductNotify - 扣款通知
```

#### HTTP处理器
**文件**: `internal/handlers/alipay_handler.go` (约500行)

```
// 支付相关
第94-128行  ✅ CreateAlipayOrder - 创建订单API
第141-176行 ✅ CreateAlipayPayment - 创建支付API
第189-217行 ✅ QueryAlipayOrder - 查询订单API
第230-263行 ✅ AlipayRefund - 退款API

// 周期扣款相关
第302-337行 ✅ CreateAlipaySubscription - 创建周期扣款API
第354-388行 ✅ QueryAlipaySubscription - 查询周期扣款API
第405-422行 ✅ CancelAlipaySubscription - 取消周期扣款API
```

#### 路由配置
**文件**: `internal/routes/routes.go` (第67-79行)

```go
alipay := v1.Group("/alipay")
{
    // 支付宝支付
    alipay.POST("/orders", alipayHandler.CreateAlipayOrder)
    alipay.POST("/payments", alipayHandler.CreateAlipayPayment)
    alipay.GET("/orders/query", alipayHandler.QueryAlipayOrder)
    alipay.POST("/refunds", alipayHandler.AlipayRefund)
    
    // 支付宝周期扣款（订阅）
    alipay.POST("/subscriptions", alipayHandler.CreateAlipaySubscription)
    alipay.GET("/subscriptions/query", alipayHandler.QueryAlipaySubscription)
    alipay.POST("/subscriptions/cancel", alipayHandler.CancelAlipaySubscription)
}
```

#### 数据模型
**文件**: `internal/models/payment_models.go`

```
第161-214行  ✅ AlipayPayment - 支付宝支付详情
第216-245行  ✅ AlipayRefund - 支付宝退款记录
第384-419行  ✅ AlipaySubscription - 支付宝周期扣款
```

---

### 3. Google Play内购和订阅

#### 服务层
**文件**: `internal/services/google_play_service.go` (392行)

```
第19-26行   ✅ GooglePlayService结构体
第28-69行   ✅ PurchaseResponse/SubscriptionResponse结构体
第71-107行  ✅ NewGooglePlayService - 初始化
第109-147行 ✅ VerifyPurchase - 验证购买
第149-206行 ✅ VerifySubscription - 验证订阅
第208-236行 ✅ AcknowledgePurchase - 确认购买
第238-266行 ✅ AcknowledgeSubscription - 确认订阅
第268-292行 ✅ ConsumePurchase - 消费购买
第302-323行 ✅ VerifyWebhookSignature - 验证签名
第340-377行 ✅ GetSubscriptionStatus - 获取订阅状态
第379-391行 ✅ ParseWebhookPayload - 解析Webhook
```

#### HTTP处理器
**文件**: `internal/handlers/handlers.go`

```
通过统一支付接口 ProcessPayment 处理
```

#### Webhook处理
**文件**: `internal/handlers/webhook.go`

```
✅ HandleGooglePlayWebhook - Google Play Webhook处理
```

#### 路由配置
**文件**: `internal/routes/routes.go`

```go
payments := v1.Group("/payments")
{
    payments.POST("/process", handler.ProcessPayment)  // 包含Google Play验证
}

webhooks := router.Group("/webhook")
{
    webhooks.POST("/google-play", webhookHandler.HandleGooglePlayWebhook)
}
```

#### 数据模型
**文件**: `internal/models/payment_models.go`

```
第93-124行   ✅ GooglePayment - Google Play支付详情
```

---

### 4. Apple内购和订阅

#### 服务层
**文件**: `internal/services/apple_service.go` (442行)

```
第21-30行   ✅ AppleService结构体
第32-60行   ✅ ApplePurchaseResponse/AppleSubscriptionResponse结构体
第62-119行  ✅ NewAppleService - 初始化
第121-222行 ✅ VerifyPurchase - 验证收据（旧版）
第224-286行 ✅ VerifyTransaction - 验证交易（推荐）
第288-354行 ✅ GetTransactionHistory - 获取交易历史
第356-381行 ✅ ParseNotification - 解析通知
第383-433行 ✅ SaveApplePayment - 保存支付信息
第435-441行 ✅ getEnvironment - 获取环境
```

#### HTTP处理器
**文件**: `internal/handlers/apple_handler.go` (304行)

```
第13-34行   ✅ AppleHandler结构体
第49-104行  ✅ VerifyReceipt - 验证收据API
第106-161行 ✅ VerifyTransaction - 验证交易API
第163-203行 ✅ GetTransactionHistory - 获取交易历史API
第205-255行 ✅ GetSubscriptionStatus - 获取订阅状态API
第257-303行 ✅ ValidateReceipt - 验证收据（简化版）API
```

#### Webhook处理
**文件**: `internal/handlers/apple_webhook.go`

```
✅ HandleAppleWebhook - Apple Webhook处理
```

#### 路由配置
**文件**: `internal/routes/routes.go` (第81-86行)

```go
apple := v1.Group("/apple")
{
    apple.POST("/verify-receipt", appleHandler.VerifyReceipt)
    apple.POST("/verify-transaction", appleHandler.VerifyTransaction)
    apple.POST("/validate-receipt", appleHandler.ValidateReceipt)
    apple.GET("/transactions/:original_transaction_id/history", appleHandler.GetTransactionHistory)
    apple.GET("/subscriptions/:original_transaction_id/status", appleHandler.GetSubscriptionStatus)
}

webhooks := router.Group("/webhook")
{
    webhooks.POST("/apple", appleWebhookHandler.HandleAppleWebhook)
}
```

#### 数据模型
**文件**: `internal/models/payment_models.go`

```
第255-300行  ✅ ApplePayment - Apple支付详情
第302-323行  ✅ AppleRefund - Apple退款记录
```

---

## 🔧 配置文件

### 配置定义
**文件**: `internal/config/config.go` (413行)

```
第13-21行   ✅ Config总结构
第23-30行   ✅ ServerConfig
第32-43行   ✅ DatabaseConfig
第45-54行   ✅ RedisConfig
第56-62行   ✅ GoogleConfig
第64-69行   ✅ JWTConfig
第71-82行   ✅ AlipayConfig
第84-93行   ✅ AppleConfig
第95-103行  ✅ WechatConfig
```

### 配置示例
**文件**: `configs/config.toml.example` (85行)

```toml
[server]  # 服务器配置
[database]  # 数据库配置
[redis]  # Redis配置
[jwt]  # JWT配置
[google]  # Google Play配置
[wechat]  # 微信支付配置
[alipay]  # 支付宝配置
[apple]  # Apple配置
```

---

## 📊 数据模型汇总

**文件**: `internal/models/payment_models.go` (420行)

### 核心模型

```
第60-91行    ✅ Order - 统一订单表
第93-124行   ✅ GooglePayment - Google Play支付详情
第126-145行  ✅ PaymentTransaction - 支付交易记录
第147-159行  ✅ UserBalance - 用户余额
第161-214行  ✅ AlipayPayment - 支付宝支付详情
第216-245行  ✅ AlipayRefund - 支付宝退款记录
第255-300行  ✅ ApplePayment - Apple支付详情
第302-323行  ✅ AppleRefund - Apple退款记录
第325-353行  ✅ WechatPayment - 微信支付详情
第355-382行  ✅ WechatRefund - 微信退款记录
第384-419行  ✅ AlipaySubscription - 支付宝周期扣款
```

---

## 🚀 统一支付接口

### 支付提供商抽象层
**文件**: `internal/services/payment_provider.go` (约850行)

```
第18-38行   ✅ PaymentProvider接口定义
第40-80行   ✅ 统一请求/响应结构体
第82-119行  ✅ PaymentProviderRegistry - 注册表
第121-234行 ✅ WechatPaymentAdapter - 微信适配器
第236-346行 ✅ AlipayPaymentAdapter - 支付宝适配器
第348-454行 ✅ ApplePaymentAdapter - Apple适配器
第456-562行 ✅ GooglePlayPaymentAdapter - Google Play适配器
```

---

## 📚 文档索引

| 文档 | 路径 | 说明 |
|------|------|------|
| **总README** | `README.md` | 项目总览 |
| **微信支付** | 无专门文档 | 查看总文档 |
| **支付宝快速开始** | `ALIPAY_QUICK_START.md` | 支付宝快速参考 |
| **支付宝详细指南** | `docs/ALIPAY_GUIDE.md` | 支付宝完整指南 |
| **支付宝总结** | `docs/ALIPAY_SUMMARY.md` | 支付宝实施总结 |
| **Google & Apple快速开始** | `GOOGLE_APPLE_QUICK_START.md` | 快速参考 |
| **Google & Apple详细指南** | `docs/GOOGLE_APPLE_GUIDE.md` | 完整指南 |
| **支付集成文档** | `docs/PAYMENT_INTEGRATION.md` | 所有支付方式集成 |
| **实施总结** | `docs/IMPLEMENTATION_SUMMARY.md` | 项目实施总结 |
| **本文档** | `PAYMENT_CODE_MAP.md` | 代码位置索引 |

---

## 🎯 快速定位指南

### 需要修改支付逻辑时

1. **服务层逻辑** → `internal/services/xxx_service.go`
2. **API接口** → `internal/handlers/xxx_handler.go`
3. **路由配置** → `internal/routes/routes.go`
4. **数据模型** → `internal/models/payment_models.go`
5. **配置** → `internal/config/config.go` 和 `configs/config.toml`

### 需要添加新功能时

1. 在相应的 `service` 文件中添加方法
2. 在相应的 `handler` 文件中添加HTTP处理
3. 在 `routes.go` 中添加路由
4. 必要时在 `models` 中添加数据结构

### 需要调试问题时

1. 查看日志（使用zap logger）
2. 检查数据库记录（Order、Payment表）
3. 验证配置是否正确
4. 检查Webhook通知处理

---

## ✅ 功能完整性检查

### 微信支付
- [x] 创建订单
- [x] JSAPI支付
- [x] Native支付
- [x] APP支付
- [x] H5支付
- [x] 查询订单
- [x] 退款
- [x] 关闭订单
- [x] Webhook通知

### 支付宝支付
- [x] 创建订单
- [x] WAP支付
- [x] PAGE支付
- [x] 查询订单
- [x] 退款
- [x] Webhook通知

### 支付宝周期扣款
- [x] 创建签约
- [x] 查询状态
- [x] 取消签约
- [x] 签约通知
- [x] 扣款通知

### Google Play
- [x] 验证购买
- [x] 验证订阅
- [x] 确认购买
- [x] 确认订阅
- [x] 消费购买
- [x] 获取订阅状态
- [x] Webhook通知

### Apple Store
- [x] 验证收据
- [x] 验证交易
- [x] 获取交易历史
- [x] 获取订阅状态
- [x] 保存支付信息
- [x] Webhook通知

---

## 🔍 代码搜索技巧

### 按功能搜索

```bash
# 查找所有支付相关的服务
find internal/services -name "*_service.go"

# 查找所有HTTP处理器
find internal/handlers -name "*_handler.go"

# 查找所有数据模型
grep -r "type.*Payment" internal/models/

# 查找特定API端点
grep -r "POST.*payments" internal/routes/
```

### 按支付方式搜索

```bash
# 微信支付
grep -r "Wechat" internal/

# 支付宝
grep -r "Alipay" internal/

# Google Play
grep -r "Google" internal/

# Apple
grep -r "Apple" internal/
```

---

**维护提示**: 此文档应在每次添加新支付方式或修改现有功能时更新。

