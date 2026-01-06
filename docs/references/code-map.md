# 支付功能代码位置速查表

本文档汇总了所有支付方式的代码位置，方便快速定位和修改。

---

## 📋 支付方式概览

| 支付方式 | 状态 | 主要功能 |
|---------|------|---------|
| **微信支付** | ✅ 已实现 | JSAPI、Native、APP、H5支付 |
| **支付宝支付** | ✅ 已实现 | WAP、PAGE、APP支付 |
| **支付宝周期扣款** | ✅ 已实现 | 签约、扣款、解约 |
| **Google Play** | ✅ 已实现 | 内购、订阅 |
| **Apple Store** | ✅ 已实现 | 内购、订阅 |

---

## 📂 代码结构

```
internal/
├── handlers/                          # HTTP处理器
│   ├── common.go                      # 通用处理器（订单、支付、健康检查）
│   ├── alipay_handler.go              # 支付宝支付处理器
│   ├── alipay_webhook.go              # 支付宝Webhook处理器
│   ├── apple_handler.go               # Apple支付处理器
│   ├── apple_webhook.go               # Apple Webhook处理器
│   ├── google_handler.go              # Google Play处理器
│   ├── google_webhook.go              # Google Play Webhook处理器
│   ├── wechat_handler.go              # 微信支付处理器
│   └── wechat_webhook.go              # 微信支付Webhook处理器
│
├── services/                          # 业务服务
│   ├── payment_service.go             # 通用支付服务
│   ├── alipay_service.go              # 支付宝服务
│   ├── apple_service.go               # Apple服务
│   ├── google_service.go              # Google Play服务
│   └── wechat_service.go              # 微信支付服务
│
├── routes/
│   └── routes.go                      # 路由配置
│
├── models/
│   └── payment_models.go              # 数据模型
│
└── config/
    └── config.go                      # 配置定义
```

---

## 🔍 代码位置详细索引

### 1. 微信支付

#### 服务层
**文件**: `internal/services/wechat_service.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第25-54行 | WechatService | 结构体和初始化 |
| 第50-126行 | CreateOrder | 创建微信订单 |
| 第128-195行 | CreateJSAPIPayment | JSAPI支付（小程序、公众号） |
| 第197-228行 | CreateNativePayment | Native支付（扫码） |
| 第230-272行 | CreateAPPPayment | APP支付 |
| 第274-308行 | CreateH5Payment | H5支付 |
| 第310-411行 | HandleNotify | 支付通知处理 |
| 第413-450行 | QueryOrder | 查询订单 |
| 第452-531行 | Refund | 退款 |
| 第533-560行 | CloseOrder | 关闭订单 |

#### HTTP处理器
**文件**: `internal/handlers/wechat_handler.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第12-24行 | WechatHandler | 结构体 |
| 第36-55行 | CreateOrder | 创建订单API |
| 第68-95行 | CreateJSAPIPayment | JSAPI支付API |
| 第107-125行 | CreateNativePayment | Native支付API |
| 第137-155行 | CreateAPPPayment | APP支付API |
| 第168-194行 | CreateH5Payment | H5支付API |
| 第206-224行 | QueryOrder | 查询订单API |
| 第236-255行 | Refund | 退款API |
| 第267-285行 | CloseOrder | 关闭订单API |

#### Webhook处理器
**文件**: `internal/handlers/wechat_webhook.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第33-72行 | HandleWechatNotify | 支付通知处理 |
| 第75-113行 | HandleWechatRefundNotify | 退款通知处理 |

---

### 2. 支付宝支付

#### 服务层
**文件**: `internal/services/alipay_service.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第21-54行 | AlipayService | 结构体和初始化 |
| 第56-131行 | CreateOrder | 创建订单 |
| 第133-164行 | CreateWapPayment | 手机网站支付 |
| 第166-197行 | CreatePagePayment | 电脑网站支付 |
| 第199-229行 | CreateAppPayment | APP支付 |
| 第231-336行 | HandleNotify | 支付通知处理 |
| 第338-393行 | QueryOrder | 查询订单 |
| 第395-479行 | Refund | 退款 |
| 第520-593行 | CreateSubscription | 创建周期扣款 |
| 第595-624行 | QuerySubscription | 查询周期扣款 |
| 第626-654行 | CancelSubscription | 取消周期扣款 |
| 第656-711行 | HandleSubscriptionNotify | 签约通知 |
| 第713-783行 | HandleDeductNotify | 扣款通知 |

#### HTTP处理器
**文件**: `internal/handlers/alipay_handler.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第14-27行 | AlipayHandler | 结构体 |
| 第95-129行 | CreateAlipayOrder | 创建订单API |
| 第142-179行 | CreateAlipayPayment | 创建支付API |
| 第192-220行 | QueryAlipayOrder | 查询订单API |
| 第233-266行 | AlipayRefund | 退款API |
| 第302-343行 | CreateAlipaySubscription | 创建周期扣款API |
| 第356-412行 | QueryAlipaySubscription | 查询周期扣款API |
| 第425-448行 | CancelAlipaySubscription | 取消周期扣款API |

#### Webhook处理器
**文件**: `internal/handlers/alipay_webhook.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第31-57行 | HandleAlipayNotify | 支付通知处理 |
| 第70-99行 | HandleAlipaySubscriptionNotify | 签约通知处理 |
| 第112-141行 | HandleAlipayDeductNotify | 扣款通知处理 |

---

### 3. Google Play

#### 服务层
**文件**: `internal/services/google_service.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第19-26行 | GooglePlayService | 结构体 |
| 第71-107行 | NewGooglePlayService | 初始化 |
| 第117-147行 | VerifyPurchase | 验证购买 |
| 第157-206行 | VerifySubscription | 验证订阅 |
| 第217-236行 | AcknowledgePurchase | 确认购买 |
| 第247-266行 | AcknowledgeSubscription | 确认订阅 |
| 第276-292行 | ConsumePurchase | 消费购买 |
| 第309-323行 | VerifyWebhookSignature | 验证签名 |
| 第347-377行 | GetSubscriptionStatus | 获取订阅状态 |
| 第385-391行 | ParseWebhookPayload | 解析Webhook |

#### HTTP处理器
**文件**: `internal/handlers/google_handler.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第16-26行 | GoogleHandler | 结构体 |
| 第79-100行 | VerifyPurchase | 验证购买API |
| 第115-135行 | VerifySubscription | 验证订阅API |
| 第149-168行 | AcknowledgePurchase | 确认购买API |
| 第182-201行 | AcknowledgeSubscription | 确认订阅API |
| 第214-233行 | ConsumePurchase | 消费购买API |
| 第248-276行 | CreateSubscription | 创建订阅订单API |
| 第290-310行 | GetSubscriptionStatus | 获取订阅状态API |
| 第325-349行 | GetUserSubscriptions | 获取用户订阅API |

#### Webhook处理器
**文件**: `internal/handlers/google_webhook.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第30-41行 | GoogleWebhookHandler | 结构体 |
| 第87-157行 | HandleGooglePlayWebhook | Webhook处理 |
| 第160-186行 | processWebhookEvent | 处理Webhook事件 |
| 第188-198行 | processTestEvent | 测试事件 |
| 第200-217行 | processOneTimeProductEvent | 一次性产品事件 |
| 第219-250行 | processSubscriptionEvent | 订阅事件 |

---

### 4. Apple Store

#### 服务层
**文件**: `internal/services/apple_service.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第21-30行 | AppleService | 结构体 |
| 第62-119行 | NewAppleService | 初始化 |
| 第128-222行 | VerifyPurchase | 验证收据（旧版） |
| 第231-286行 | VerifyTransaction | 验证交易（推荐） |
| 第294-354行 | GetTransactionHistory | 获取交易历史 |
| 第361-381行 | ParseNotification | 解析通知 |
| 第390-433行 | SaveApplePayment | 保存支付信息 |

#### HTTP处理器
**文件**: `internal/handlers/apple_handler.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第13-21行 | AppleHandler | 结构体 |
| 第60-104行 | VerifyReceipt | 验证收据API |
| 第117-161行 | VerifyTransaction | 验证交易API |
| 第174-203行 | GetTransactionHistory | 获取交易历史API |
| 第216-255行 | GetSubscriptionStatus | 获取订阅状态API |
| 第268-303行 | ValidateReceipt | 验证收据（简化版）API |

#### Webhook处理器
**文件**: `internal/handlers/apple_webhook.go`

| 行号 | 方法 | 功能 |
|------|------|------|
| 第17-28行 | AppleWebhookHandler | 结构体 |
| 第59-125行 | HandleAppleWebhook | Webhook处理 |
| 第128-137行 | validateSignature | 验证签名 |
| 第140-153行 | processNotification | 处理通知 |

---

## 🔧 配置文件

### 配置定义
**文件**: `internal/config/config.go`

| 行号 | 配置项 | 说明 |
|------|--------|------|
| 第13-21行 | Config | 总配置结构 |
| 第71-82行 | AlipayConfig | 支付宝配置 |
| 第84-93行 | AppleConfig | Apple配置 |
| 第56-62行 | GoogleConfig | Google Play配置 |
| 第95-103行 | WechatConfig | 微信支付配置 |

### 配置示例
**文件**: `configs/config.toml.example`

---

## 📊 数据模型汇总

**文件**: `internal/models/payment_models.go`

| 行号 | 模型 | 说明 |
|------|------|------|
| 第60-91行 | Order | 统一订单表 |
| 第93-124行 | GooglePayment | Google Play支付详情 |
| 第126-145行 | PaymentTransaction | 支付交易记录 |
| 第161-214行 | AlipayPayment | 支付宝支付详情 |
| 第216-245行 | AlipayRefund | 支付宝退款记录 |
| 第255-300行 | ApplePayment | Apple支付详情 |
| 第302-323行 | AppleRefund | Apple退款记录 |
| 第325-353行 | WechatPayment | 微信支付详情 |
| 第355-382行 | WechatRefund | 微信退款记录 |
| 第384-419行 | AlipaySubscription | 支付宝周期扣款 |

---

## 🚀 路由配置

**文件**: `internal/routes/routes.go`

### API路由结构

```
/api/v1
├── /orders                            # 通用订单
│   ├── POST /                         # 创建订单
│   ├── GET /:id                       # 获取订单详情
│   ├── GET /no/:order_no              # 根据订单号获取
│   └── POST /:id/cancel               # 取消订单
│
├── /payments
│   └── POST /process                  # 处理支付
│
├── /users
│   └── GET /:user_id/orders           # 获取用户订单
│
├── /google                            # Google Play
│   ├── POST /verify-purchase          # 验证购买
│   ├── POST /verify-subscription      # 验证订阅
│   ├── POST /acknowledge-purchase     # 确认购买
│   ├── POST /acknowledge-subscription # 确认订阅
│   ├── POST /consume-purchase         # 消费购买
│   ├── POST /subscriptions            # 创建订阅订单
│   ├── GET /subscriptions/status      # 获取订阅状态
│   └── GET /users/:user_id/subscriptions
│
├── /alipay                            # 支付宝
│   ├── POST /orders                   # 创建订单
│   ├── POST /payments                 # 创建支付
│   ├── GET /orders/query              # 查询订单
│   ├── POST /refunds                  # 退款
│   ├── POST /subscriptions            # 创建周期扣款
│   ├── GET /subscriptions/query       # 查询周期扣款
│   └── POST /subscriptions/cancel     # 取消周期扣款
│
├── /apple                             # Apple
│   ├── POST /verify-receipt           # 验证收据
│   ├── POST /verify-transaction       # 验证交易
│   ├── POST /validate-receipt         # 验证收据（简化）
│   ├── GET /transactions/:id/history  # 获取交易历史
│   └── GET /subscriptions/:id/status  # 获取订阅状态
│
└── /wechat                            # 微信支付
    ├── POST /orders                   # 创建订单
    ├── GET /orders/:order_no          # 查询订单
    ├── POST /orders/:order_no/close   # 关闭订单
    ├── POST /payments/jsapi/:order_no # JSAPI支付
    ├── POST /payments/native/:order_no# Native支付
    ├── POST /payments/app/:order_no   # APP支付
    ├── POST /payments/h5/:order_no    # H5支付
    └── POST /refunds                  # 退款
```

### Webhook路由

```
/webhook
├── POST /google                       # Google Play Webhook
├── POST /alipay/notify               # 支付宝支付通知
├── POST /alipay/subscription         # 支付宝签约通知
├── POST /alipay/deduct               # 支付宝扣款通知
├── POST /apple                        # Apple Webhook
├── POST /wechat/notify               # 微信支付通知
└── POST /wechat/refund               # 微信退款通知
```

---

## 🎯 快速定位指南

### 需要修改支付逻辑时

1. **服务层逻辑** → `internal/services/xxx_service.go`
2. **API接口** → `internal/handlers/xxx_handler.go`
3. **Webhook处理** → `internal/handlers/xxx_webhook.go`
4. **路由配置** → `internal/routes/routes.go`
5. **数据模型** → `internal/models/payment_models.go`
6. **配置** → `internal/config/config.go`

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

## 🔍 代码搜索技巧

### 按功能搜索

```bash
# 查找所有支付相关的服务
find internal/services -name "*_service.go"

# 查找所有HTTP处理器
find internal/handlers -name "*_handler.go"

# 查找所有Webhook处理器
find internal/handlers -name "*_webhook.go"

# 查找所有数据模型
grep -r "type.*Payment" internal/models/

# 查找特定API端点
grep -r "POST.*payments" internal/routes/
```

### 按支付方式搜索

```bash
# 微信支付
grep -r "wechat\|Wechat" internal/

# 支付宝
grep -r "alipay\|Alipay" internal/

# Google Play
grep -r "google\|Google" internal/

# Apple
grep -r "apple\|Apple" internal/
```

---

**维护提示**: 此文档应在每次添加新支付方式或修改现有功能时更新。

**最后更新**: 2026-01-06
