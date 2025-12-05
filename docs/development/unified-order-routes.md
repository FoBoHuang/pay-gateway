# 统一订单路由的作用

## 🤔 问题

在 `internal/routes/routes.go:53-60` 中，有统一的订单相关路由：

```go
orders := v1.Group("/orders")
{
    orders.POST("", handler.CreateOrder)                   // 创建订单
    orders.GET("/:id", handler.GetOrder)                   // 获取订单详情
    orders.GET("/no/:order_no", handler.GetOrderByOrderNo) // 根据订单号获取订单
    orders.POST("/:id/cancel", handler.CancelOrder)        // 取消订单
}
```

**疑问**: 既然已经有了各支付方式的专用路由（如 `/api/v1/wechat/orders`、`/api/v1/alipay/orders`），为什么还需要这些统一的订单路由？

---

## 🎯 核心作用

### 统一订单管理的核心接口

这些路由提供了**跨支付方式的统一订单管理功能**，是整个支付中心的基础设施。

---

## 📊 详细功能说明

### 1. `POST /api/v1/orders` - 创建订单

**作用**: 创建统一的订单记录，不依赖具体支付方式

**特点**:
- ✅ **支付方式无关** - 创建订单时不需要指定具体的支付提供商
- ✅ **统一订单号** - 生成系统统一的订单号（格式：`ORD20240101120000xxxx`）
- ✅ **订单元数据** - 保存订单基本信息（商品、金额、用户等）
- ✅ **状态管理** - 初始状态为 `CREATED`，支付状态为 `PENDING`

**使用场景**:
```go
// 场景1: 创建订单，后续再选择支付方式
POST /api/v1/orders
{
  "user_id": 1,
  "product_id": "premium_month",
  "type": "PURCHASE",
  "title": "Premium会员月卡",
  "description": "解锁所有高级功能",
  "quantity": 1,
  "currency": "CNY",
  "total_amount": 2999,
  "payment_method": "WECHAT"  // 可选，后续可以更改
}

// 返回订单ID和订单号
{
  "id": 123,
  "order_no": "ORD20240101120000xxxx",
  "status": "CREATED",
  "payment_status": "PENDING",
  ...
}
```

**与其他路由的关系**:
- 创建订单后，可以使用任何支付方式完成支付：
  - 微信支付: `POST /api/v1/wechat/payments/jsapi/:order_no`
  - 支付宝: `POST /api/v1/alipay/payments`
  - Google Play: `POST /api/v1/payments/process` (with purchase_token)
  - Apple: `POST /api/v1/payments/process` (with receipt)

---

### 2. `GET /api/v1/orders/:id` - 获取订单详情

**作用**: 根据订单ID获取完整的订单信息，包括所有支付方式的详情

**特点**:
- ✅ **完整信息** - 自动加载所有关联的支付记录（GooglePayment、AlipayPayment、ApplePayment、WechatPayment）
- ✅ **统一查询** - 不需要知道订单使用了哪种支付方式
- ✅ **状态追踪** - 查看订单状态、支付状态、退款状态等

**返回数据结构**:
```json
{
  "id": 123,
  "order_no": "ORD20240101120000xxxx",
  "user_id": 1,
  "product_id": "premium_month",
  "total_amount": 2999,
  "status": "PAID",
  "payment_status": "COMPLETED",
  "wechat_payment": {  // 如果使用微信支付
    "transaction_id": "wx_xxx",
    "trade_state": "SUCCESS",
    ...
  },
  "alipay_payment": null,  // 如果未使用支付宝
  "google_payment": null,
  "apple_payment": null
}
```

**使用场景**:
```go
// 场景1: 前端查询订单状态
GET /api/v1/orders/123

// 场景2: 后台管理系统查询订单
GET /api/v1/orders/123

// 场景3: 对账系统查询订单
GET /api/v1/orders/123
```

---

### 3. `GET /api/v1/orders/no/:order_no` - 根据订单号获取订单

**作用**: 通过订单号（而不是ID）查询订单

**特点**:
- ✅ **业务友好** - 使用业务订单号查询，而不是数据库ID
- ✅ **外部系统集成** - 其他系统可以通过订单号查询
- ✅ **Webhook处理** - Webhook通常使用订单号标识订单

**使用场景**:
```go
// 场景1: Webhook处理时查询订单
// 微信Webhook收到 out_trade_no，需要查询订单
GET /api/v1/orders/no/ORD20240101120000xxxx

// 场景2: 前端通过订单号查询
// 用户支付后，前端使用订单号查询支付结果
GET /api/v1/orders/no/ORD20240101120000xxxx

// 场景3: 第三方系统查询
// 其他系统通过订单号查询订单状态
GET /api/v1/orders/no/ORD20240101120000xxxx
```

---

### 4. `POST /api/v1/orders/:id/cancel` - 取消订单

**作用**: 取消未支付或已支付的订单

**特点**:
- ✅ **状态检查** - 检查订单状态是否允许取消
- ✅ **自动退款** - 如果订单已支付，标记需要退款
- ✅ **统一处理** - 不依赖具体支付方式

**取消逻辑**:
```go
// 1. 检查订单状态
if order.Status != CREATED && order.Status != PAID {
    return error("订单状态不允许取消")
}

// 2. 更新订单状态
order.Status = CANCELLED
order.RefundReason = reason

// 3. 如果已支付，标记需要退款
if order.PaymentStatus == COMPLETED {
    // 记录退款需求，后续通过退款接口处理
    log.Warn("订单已支付，需要退款处理")
}
```

**使用场景**:
```go
// 场景1: 用户主动取消订单
POST /api/v1/orders/123/cancel?reason=用户取消

// 场景2: 订单超时自动取消
POST /api/v1/orders/123/cancel?reason=订单超时

// 场景3: 管理员取消订单
POST /api/v1/orders/123/cancel?reason=管理员取消
```

---

## 🔄 与其他路由的关系

### 统一订单路由 vs 各支付方式的专用路由

| 功能 | 统一订单路由 | 各支付方式专用路由 |
|------|------------|------------------|
| **创建订单** | ✅ `POST /api/v1/orders` | ⚠️ 各支付方式也有（但会创建统一订单） |
| **查询订单** | ✅ `GET /api/v1/orders/:id` | ⚠️ 各支付方式也有（但查询的是支付详情） |
| **取消订单** | ✅ `POST /api/v1/orders/:id/cancel` | ❌ 各支付方式没有 |
| **创建支付** | ❌ 没有 | ✅ 各支付方式有（如 `/api/v1/wechat/payments/jsapi/:order_no`） |
| **查询支付** | ❌ 没有 | ✅ 各支付方式有（如 `/api/v1/wechat/orders/:order_no`） |
| **退款** | ❌ 没有 | ✅ 各支付方式有（如 `/api/v1/wechat/refunds`） |

### 工作流程示例

#### 场景1: 微信支付流程

```
1. 创建统一订单
   POST /api/v1/orders
   → 返回 order_id=123, order_no="ORDxxx"

2. 创建微信支付
   POST /api/v1/wechat/payments/jsapi/ORDxxx
   → 返回支付参数

3. 用户完成支付

4. 微信Webhook通知
   POST /webhook/wechat/notify
   → 更新订单状态为PAID

5. 查询订单状态（统一接口）
   GET /api/v1/orders/123
   → 返回完整订单信息（包括wechat_payment详情）
```

#### 场景2: Google Play支付流程

```
1. 创建统一订单
   POST /api/v1/orders
   → 返回 order_id=123, order_no="ORDxxx"

2. 用户在App内完成支付，获得purchase_token

3. 验证支付（统一接口）
   POST /api/v1/payments/process
   {
     "order_id": 123,
     "provider": "GOOGLE_PLAY",
     "purchase_token": "xxx"
   }
   → 验证并更新订单状态为PAID

4. 查询订单状态（统一接口）
   GET /api/v1/orders/123
   → 返回完整订单信息（包括google_payment详情）
```

---

## 💡 设计优势

### 1. 统一订单模型

所有支付方式共享同一个 `Order` 模型：

```go
type Order struct {
    ID            uint
    OrderNo       string  // 统一订单号
    UserID        uint
    ProductID     string
    TotalAmount   int64
    Status        OrderStatus
    PaymentStatus PaymentStatus
    
    // 关联各支付方式的详情
    GooglePayment *GooglePayment
    AlipayPayment *AlipayPayment
    ApplePayment  *ApplePayment
    WechatPayment *WechatPayment
}
```

**优势**:
- ✅ 统一的订单号格式
- ✅ 统一的订单状态管理
- ✅ 统一的查询接口
- ✅ 便于对账和统计

### 2. 支付方式解耦

**创建订单时不需要指定支付方式**，可以在后续流程中灵活选择：

```go
// 1. 创建订单（不指定支付方式）
POST /api/v1/orders
{
  "user_id": 1,
  "product_id": "premium_month",
  "total_amount": 2999
}

// 2. 用户选择支付方式
// 可以选择微信、支付宝、Google Play、Apple中的任意一种

// 3. 使用选择的支付方式完成支付
```

### 3. 统一查询接口

**不需要知道订单使用了哪种支付方式**，统一查询即可：

```go
// 不需要知道是微信还是支付宝
GET /api/v1/orders/123

// 返回结果自动包含对应的支付详情
{
  "wechat_payment": {...},  // 如果使用微信支付
  "alipay_payment": null,   // 如果未使用支付宝
  ...
}
```

### 4. 便于扩展

**添加新的支付方式时**，不需要修改订单相关接口：

```go
// 添加新的支付方式（如Stripe）
// 只需要：
// 1. 添加 StripePayment 模型
// 2. 在 Order 中添加 StripePayment 关联
// 3. 在 GetOrder 的 Preload 中添加 StripePayment

// 统一订单路由不需要修改！
```

---

## 📋 使用场景总结

### 场景1: 前端订单管理

```go
// 1. 用户下单
POST /api/v1/orders
→ 获得订单ID和订单号

// 2. 用户选择支付方式并支付
POST /api/v1/wechat/payments/jsapi/:order_no
→ 用户完成支付

// 3. 前端轮询订单状态
GET /api/v1/orders/:id
→ 检查支付状态
```

### 场景2: 后台管理系统

```go
// 1. 查看所有订单
GET /api/v1/users/:user_id/orders?page=1&page_size=20

// 2. 查看订单详情
GET /api/v1/orders/:id
→ 查看完整的订单信息和支付详情

// 3. 取消订单
POST /api/v1/orders/:id/cancel?reason=管理员取消
```

### 场景3: Webhook处理

```go
// 1. 收到Webhook通知
POST /webhook/wechat/notify
{
  "out_trade_no": "ORD20240101120000xxxx",
  "trade_state": "SUCCESS",
  ...
}

// 2. Webhook处理器查询订单
GET /api/v1/orders/no/ORD20240101120000xxxx

// 3. 更新订单状态
order.Status = PAID
order.PaymentStatus = COMPLETED
```

### 场景4: 对账系统

```go
// 1. 查询订单
GET /api/v1/orders/:id

// 2. 获取支付详情
// 根据订单中的支付方式，获取对应的支付详情
if order.WechatPayment != nil {
    // 使用微信支付详情对账
}
if order.AlipayPayment != nil {
    // 使用支付宝支付详情对账
}
```

---

## ✅ 总结

### 统一订单路由的核心价值

1. **统一订单管理** - 所有支付方式共享同一个订单模型和接口
2. **支付方式解耦** - 创建订单时不需要指定支付方式
3. **统一查询** - 不需要知道订单使用了哪种支付方式
4. **便于扩展** - 添加新支付方式时不需要修改订单接口
5. **业务友好** - 提供订单号查询，便于外部系统集成

### 与其他路由的配合

- **统一订单路由** - 管理订单生命周期（创建、查询、取消）
- **各支付方式路由** - 处理具体支付流程（创建支付、查询支付、退款）
- **Webhook路由** - 处理支付完成后的异步通知

三者配合，形成完整的支付中心功能。

---

**文档日期**: 2024-12-05  
**相关代码**:
- `internal/routes/routes.go:53-60`
- `internal/handlers/handlers.go:118-290`
- `internal/services/payment_service.go:79-176`
- `internal/models/payment_models.go:60-91`

