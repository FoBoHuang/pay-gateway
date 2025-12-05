# 支付宝支付和周期扣款使用指南

本文档详细介绍支付宝支付和周期扣款（订阅）功能的使用方法。

## 目录

1. [功能概述](#功能概述)
2. [配置说明](#配置说明)
3. [支付宝支付](#支付宝支付)
4. [支付宝周期扣款（订阅）](#支付宝周期扣款订阅)
5. [Webhook处理](#webhook处理)
6. [测试指南](#测试指南)
7. [常见问题](#常见问题)

---

## 功能概述

### 支付宝支付

**已实现功能**：
- ✅ 创建订单
- ✅ 手机网站支付 (Wap)
- ✅ 电脑网站支付 (Page)
- ✅ 查询订单状态
- ✅ 退款
- ✅ 异步通知处理

**文件位置**：
- 服务层：`internal/services/alipay_service.go`
- 处理器：`internal/handlers/alipay_handler.go`
- 路由：`internal/routes/routes.go`

### 支付宝周期扣款（订阅）

**已实现功能**：
- ✅ 创建周期扣款协议（签约）
- ✅ 查询周期扣款状态
- ✅ 取消周期扣款（解约）
- ✅ 签约通知处理
- ✅ 扣款通知处理

**文件位置**：
- 服务层：`internal/services/alipay_service.go`（第486-730行）
- 处理器：`internal/handlers/alipay_handler.go`（第287-450行）
- 路由：`internal/routes/routes.go`
- 数据模型：`internal/models/payment_models.go`（AlipaySubscription）

---

## 配置说明

### 配置文件

编辑 `configs/config.toml`：

```toml
[alipay]
app_id = "2021001234567890"                      # 支付宝应用ID
private_key = """
-----BEGIN RSA PRIVATE KEY-----
Your Alipay Private Key Content Here
-----END RSA PRIVATE KEY-----
"""
is_production = false                             # 是否为生产环境
notify_url = "https://your-domain.com/webhook/alipay"
return_url = "https://your-domain.com/payment/return"
cert_mode = false                                 # 是否使用证书模式
app_cert_path = "configs/alipay/appCertPublicKey.crt"
root_cert_path = "configs/alipay/alipayRootCert.crt"
alipay_cert_path = "configs/alipay/alipayCertPublicKey_RSA2.crt"
```

### 环境变量

```bash
export ALIPAY_APP_ID="2021001234567890"
export ALIPAY_PRIVATE_KEY="your_private_key"
export ALIPAY_IS_PRODUCTION="false"
export ALIPAY_NOTIFY_URL="https://your-domain.com/webhook/alipay"
export ALIPAY_RETURN_URL="https://your-domain.com/payment/return"
```

---

## 支付宝支付

### API端点

```
POST   /api/v1/alipay/orders         # 创建订单
POST   /api/v1/alipay/payments       # 创建支付
GET    /api/v1/alipay/orders/query   # 查询订单
POST   /api/v1/alipay/refunds        # 退款
POST   /webhook/alipay               # 异步通知
```

### 1. 创建订单

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/v1/alipay/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "product_id": "premium_month",
    "subject": "Premium会员月卡",
    "body": "解锁所有高级功能",
    "total_amount": 2999
  }'
```

**响应示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "order_id": 123,
    "order_no": "ORD20240105120000abcdef12",
    "total_amount": 2999,
    "subject": "Premium会员月卡",
    "description": "解锁所有高级功能"
  }
}
```

### 2. 创建支付

创建订单后，需要创建支付获取支付URL。

#### 2.1 手机网站支付 (WAP)

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/v1/alipay/payments \
  -H "Content-Type: application/json" \
  -d '{
    "order_no": "ORD20240105120000abcdef12",
    "pay_type": "WAP"
  }'
```

#### 2.2 电脑网站支付 (PAGE)

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/v1/alipay/payments \
  -H "Content-Type: application/json" \
  -d '{
    "order_no": "ORD20240105120000abcdef12",
    "pay_type": "PAGE"
  }'
```

**响应示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "payment_url": "https://openapi.alipay.com/gateway.do?...",
    "order_no": "ORD20240105120000abcdef12"
  }
}
```

客户端需要跳转到 `payment_url` 进行支付。

### 3. 查询订单

**请求示例**：

```bash
curl -X GET "http://localhost:8080/api/v1/alipay/orders/query?order_no=ORD20240105120000abcdef12"
```

**响应示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "order_no": "ORD20240105120000abcdef12",
    "trade_no": "2024010522001234567890",
    "trade_status": "TRADE_SUCCESS",
    "total_amount": 2999,
    "payment_status": "COMPLETED",
    "paid_at": "2024-01-05 12:05:30"
  }
}
```

### 4. 退款

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/v1/alipay/refunds \
  -H "Content-Type: application/json" \
  -d '{
    "order_no": "ORD20240105120000abcdef12",
    "refund_amount": 2999,
    "refund_reason": "用户申请退款"
  }'
```

**响应示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "refund_request_no": "REFUND20240105120530",
    "refund_amount": 2999,
    "refund_status": "REFUND_SUCCESS",
    "refund_at": "2024-01-05 12:05:30"
  }
}
```

---

## 支付宝周期扣款（订阅）

### API端点

```
POST   /api/v1/alipay/subscriptions         # 创建周期扣款（签约）
GET    /api/v1/alipay/subscriptions/query   # 查询周期扣款状态
POST   /api/v1/alipay/subscriptions/cancel  # 取消周期扣款（解约）
```

### 1. 创建周期扣款（签约）

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/v1/alipay/subscriptions \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "product_id": "premium_subscription",
    "product_name": "Premium会员月度订阅",
    "product_desc": "每月自动续费，畅享所有高级功能",
    "period_type": "MONTH",
    "period": 1,
    "single_amount": 2999,
    "total_amount": 0,
    "total_payments": 0,
    "personal_product_code": "CYCLE_PAY_AUTH_P",
    "sign_scene": "INDUSTRY|MEDICAL_INSURANCE"
  }'
```

**参数说明**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user_id | uint | 是 | 用户ID |
| product_id | string | 是 | 商品ID |
| product_name | string | 是 | 商品名称 |
| product_desc | string | 否 | 商品描述 |
| period_type | string | 是 | 周期类型：DAY（日）或 MONTH（月） |
| period | int | 是 | 周期数，如：1表示每1个月 |
| execution_time | datetime | 否 | 首次执行时间，不传默认为明天 |
| single_amount | int64 | 是 | 单次扣款金额（分） |
| total_amount | int64 | 否 | 总金额限制（分），0表示不限制 |
| total_payments | int | 否 | 总扣款次数，0表示不限制 |
| personal_product_code | string | 是 | 个人签约产品码，如：CYCLE_PAY_AUTH_P |
| sign_scene | string | 是 | 签约场景，如：INDUSTRY\|MEDICAL_INSURANCE |

**响应示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "order_id": 124,
    "out_request_no": "SUB20240105120000abcdef12",
    "sign_url": "https://openapi.alipay.com/gateway.do?...",
    "status": "TEMP",
    "execution_time": "2024-01-06 12:00:00"
  }
}
```

客户端需要跳转到 `sign_url` 进行签约授权。

### 2. 查询周期扣款状态

**请求示例**：

```bash
curl -X GET "http://localhost:8080/api/v1/alipay/subscriptions/query?out_request_no=SUB20240105120000abcdef12"
```

**响应示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "out_request_no": "SUB20240105120000abcdef12",
    "agreement_no": "20240105000001234567",
    "status": "NORMAL",
    "sign_time": "2024-01-05 12:05:00",
    "valid_time": "2024-01-05 12:05:00",
    "invalid_time": "2099-12-31 23:59:59",
    "period_type": "MONTH",
    "period": 1,
    "execution_time": "2024-01-06 12:00:00",
    "single_amount": "29.99",
    "total_amount": "0.00",
    "total_payments": 0,
    "current_period": 3,
    "last_deduct_time": "2024-03-05 12:00:00",
    "next_deduct_time": "2024-04-05 12:00:00",
    "deduct_success_count": 3,
    "deduct_fail_count": 0
  }
}
```

**状态说明**：

- `TEMP`: 临时状态，等待用户签约
- `NORMAL`: 正常状态，协议生效
- `STOP`: 已停止，协议已解约

### 3. 取消周期扣款（解约）

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/v1/alipay/subscriptions/cancel \
  -H "Content-Type: application/json" \
  -d '{
    "out_request_no": "SUB20240105120000abcdef12",
    "cancel_reason": "用户主动取消订阅"
  }'
```

或者使用支付宝协议号：

```bash
curl -X POST http://localhost:8080/api/v1/alipay/subscriptions/cancel \
  -H "Content-Type: application/json" \
  -d '{
    "agreement_no": "20240105000001234567",
    "cancel_reason": "用户主动取消订阅"
  }'
```

**响应示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "周期扣款取消成功"
  }
}
```

---

## Webhook处理

### 异步通知端点

```
POST   /webhook/alipay
```

### 通知类型

1. **支付通知** - 用户完成支付后的通知
2. **签约通知** - 用户完成周期扣款签约后的通知
3. **扣款通知** - 周期扣款执行后的通知

### 处理流程

服务会自动处理支付宝的异步通知：

1. **验证签名** - 确保通知来自支付宝
2. **处理通知** - 更新订单/协议状态
3. **返回响应** - 返回 "success" 给支付宝

### 注意事项

- 确保 `notify_url` 配置正确且可被支付宝访问
- 必须使用HTTPS（生产环境）
- 确保服务器能够接收POST请求
- 处理通知时要做幂等性检查，避免重复处理

---

## 测试指南

### 1. 使用沙箱环境

**配置沙箱**：

```toml
[alipay]
is_production = false
app_id = "your_sandbox_app_id"
```

**获取沙箱账号**：

访问 [支付宝开放平台](https://open.alipay.com/) 获取沙箱账号和配置。

### 2. 测试支付流程

```bash
# 1. 创建订单
ORDER_NO=$(curl -s -X POST http://localhost:8080/api/v1/alipay/orders \
  -H "Content-Type: application/json" \
  -d '{...}' | jq -r '.data.order_no')

# 2. 创建支付
curl -X POST http://localhost:8080/api/v1/alipay/payments \
  -H "Content-Type: application/json" \
  -d "{\"order_no\": \"$ORDER_NO\", \"pay_type\": \"WAP\"}"

# 3. 使用沙箱买家账号完成支付

# 4. 查询订单状态
curl -X GET "http://localhost:8080/api/v1/alipay/orders/query?order_no=$ORDER_NO"
```

### 3. 测试周期扣款流程

```bash
# 1. 创建周期扣款
OUT_REQUEST_NO=$(curl -s -X POST http://localhost:8080/api/v1/alipay/subscriptions \
  -H "Content-Type: application/json" \
  -d '{...}' | jq -r '.data.out_request_no')

# 2. 用户完成签约（在支付宝沙箱）

# 3. 查询周期扣款状态
curl -X GET "http://localhost:8080/api/v1/alipay/subscriptions/query?out_request_no=$OUT_REQUEST_NO"

# 4. 取消周期扣款
curl -X POST http://localhost:8080/api/v1/alipay/subscriptions/cancel \
  -H "Content-Type: application/json" \
  -d "{\"out_request_no\": \"$OUT_REQUEST_NO\", \"cancel_reason\": \"测试取消\"}"
```

---

## 常见问题

### Q1: 支付通知没有收到？

**A**: 检查以下几点：
1. `notify_url` 配置是否正确
2. 服务器是否可以被外网访问
3. 是否使用HTTPS（生产环境必须）
4. 检查服务器日志是否有错误

### Q2: 签名验证失败？

**A**: 检查以下几点：
1. 私钥配置是否正确
2. 私钥格式是否正确（PKCS8或PKCS1）
3. 是否与支付宝公钥匹配

### Q3: 周期扣款何时开始？

**A**: 
- 首次扣款时间由 `execution_time` 参数决定
- 不传则默认为签约后的第二天
- 后续扣款按照 `period_type` 和 `period` 计算

### Q4: 如何限制周期扣款次数？

**A**: 
- 设置 `total_payments` 参数限制总次数
- 设置 `total_amount` 参数限制总金额
- 两者都为0或不传表示不限制

### Q5: 用户如何查看已签约的周期扣款？

**A**: 
- 用户可以在支付宝APP中查看"自动扣款"
- 服务端可以通过查询API获取状态
- 用户可以随时在支付宝中取消授权

---

## 代码位置速查

### 支付宝支付

| 功能 | 文件 | 位置 |
|------|------|------|
| 创建订单 | `alipay_service.go` | 第57-131行 |
| 手机网站支付 | `alipay_service.go` | 第134-164行 |
| 电脑网站支付 | `alipay_service.go` | 第167-197行 |
| 异步通知处理 | `alipay_service.go` | 第200-304行 |
| 查询订单 | `alipay_service.go` | 第307-361行 |
| 退款 | `alipay_service.go` | 第364-447行 |

### 支付宝周期扣款

| 功能 | 文件 | 位置 |
|------|------|------|
| 创建周期扣款 | `alipay_service.go` | 第492-578行 |
| 查询周期扣款 | `alipay_service.go` | 第581-617行 |
| 取消周期扣款 | `alipay_service.go` | 第620-646行 |
| 签约通知处理 | `alipay_service.go` | 第649-697行 |
| 扣款通知处理 | `alipay_service.go` | 第700-770行 |

### HTTP处理器

| 功能 | 文件 | 位置 |
|------|------|------|
| 支付相关处理器 | `alipay_handler.go` | 第83-263行 |
| 周期扣款处理器 | `alipay_handler.go` | 第289-388行 |

### 路由配置

| 功能 | 文件 | 位置 |
|------|------|------|
| 支付宝路由 | `routes.go` | 第67-79行 |

---

## 总结

支付宝支付和周期扣款功能已全部实现，包括：

✅ **支付功能**：创建订单、发起支付、查询、退款、通知处理  
✅ **周期扣款**：签约、查询、解约、签约通知、扣款通知

所有代码已经过整理，结构清晰，易于使用和维护。

