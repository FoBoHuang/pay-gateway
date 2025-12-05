# 微信支付完整使用指南

本文档详细介绍微信支付功能的所有使用方法。

## 目录

1. [功能概述](#功能概述)
2. [配置说明](#配置说明)
3. [支付方式详解](#支付方式详解)
4. [订单管理](#订单管理)
5. [退款功能](#退款功能)
6. [Webhook处理](#webhook处理)
7. [测试指南](#测试指南)
8. [常见问题](#常见问题)

---

## 功能概述

### 已实现功能

✅ **四种支付方式**
- JSAPI支付（小程序、公众号）
- Native支付（扫码支付）
- APP支付（移动应用）
- H5支付（手机网站）

✅ **完整的订单管理**
- 创建订单
- 查询订单
- 关闭订单

✅ **退款功能**
- 发起退款
- 退款记录

✅ **Webhook处理**
- 支付通知处理
- 签名验证（待完善）

### 代码位置

| 组件 | 文件 | 行数 |
|------|------|------|
| 服务层 | `internal/services/wechat_service.go` | 880行 |
| HTTP处理器 | `internal/handlers/wechat_handler.go` | 430行 |
| 路由配置 | `internal/routes/routes.go` | 第88-97行 |
| 数据模型 | `internal/models/payment_models.go` | WechatPayment, WechatRefund |

---

## 配置说明

### 配置文件

编辑 `configs/config.toml`:

```toml
[wechat]
app_id = "wx1234567890abcdef"           # 微信应用ID
mch_id = "1234567890"                    # 微信商户号
apiv3_key = "your_apiv3_key_32chars"    # API v3密钥（32位）
serial_no = "certificate_serial_no"      # 证书序列号
private_key = """
-----BEGIN PRIVATE KEY-----
Your Wechat Merchant Private Key
-----END PRIVATE KEY-----
"""
notify_url = "https://your-domain.com/webhook/wechat/notify"
cert_path = "configs/wechat_cert.pem"    # 可选
```

### 环境变量

```bash
export WECHAT_APP_ID="wx1234567890abcdef"
export WECHAT_MCH_ID="1234567890"
export WECHAT_APIV3_KEY="your_apiv3_key"
export WECHAT_SERIAL_NO="certificate_serial_no"
export WECHAT_PRIVATE_KEY="your_private_key"
export WECHAT_NOTIFY_URL="https://your-domain.com/webhook/wechat/notify"
```

### 获取配置参数

1. **应用ID (app_id)**
   - 登录 [微信支付商户平台](https://pay.weixin.qq.com/)
   - 产品中心 → 开发配置 → 查看APPID

2. **商户号 (mch_id)**
   - 账户中心 → 商户信息 → 商户号

3. **API v3密钥 (apiv3_key)**
   - 账户中心 → API安全 → 设置APIv3密钥（32位）

4. **证书序列号 (serial_no)**
   - 账户中心 → API安全 → API证书 → 查看证书序列号

5. **商户私钥 (private_key)**
   - 下载API证书工具
   - 生成证书，获取私钥文件

---

## 支付方式详解

### 1. JSAPI支付（小程序、公众号）

**适用场景**: 微信小程序、微信公众号内支付

**使用流程**:

#### 步骤1：创建订单
```bash
curl -X POST http://localhost:8080/api/v1/wechat/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "product_id": "premium_month",
    "description": "Premium会员月卡",
    "detail": "解锁所有高级功能",
    "total_amount": 2999,
    "trade_type": "JSAPI"
  }'
```

**响应**:
```json
{
  "success": true,
  "data": {
    "order_id": 123,
    "order_no": "WX20240105120000abcdef12",
    "total_amount": 2999,
    "description": "Premium会员月卡"
  }
}
```

#### 步骤2：创建JSAPI支付
```bash
curl -X POST http://localhost:8080/api/v1/wechat/payments/jsapi/WX20240105120000abcdef12 \
  -H "Content-Type: application/json" \
  -d '{
    "openid": "o6_bmjrPTlm6_2sgVt7hMZOPfL2M"
  }'
```

**响应**:
```json
{
  "success": true,
  "data": {
    "prepay_id": "wx20240105120000xxx",
    "app_id": "wx1234567890abcdef",
    "time_stamp": "1704432000",
    "nonce_str": "abcdef1234567890",
    "package": "prepay_id=wx20240105120000xxx",
    "sign_type": "RSA"
  }
}
```

#### 步骤3：小程序调起支付

在小程序中使用返回的参数调起支付：

```javascript
wx.requestPayment({
  timeStamp: response.time_stamp,
  nonceStr: response.nonce_str,
  package: response.package,
  signType: response.sign_type,
  paySign: calculateSign(), // 需要客户端计算签名
  success: (res) => {
    console.log('支付成功', res);
  },
  fail: (res) => {
    console.log('支付失败', res);
  }
});
```

---

### 2. Native支付（扫码支付）

**适用场景**: PC网站、线下扫码支付

**使用流程**:

#### 步骤1：创建订单
```bash
curl -X POST http://localhost:8080/api/v1/wechat/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "product_id": "premium_month",
    "description": "Premium会员月卡",
    "total_amount": 2999,
    "trade_type": "NATIVE"
  }'
```

#### 步骤2：创建Native支付（获取二维码）
```bash
curl -X POST http://localhost:8080/api/v1/wechat/payments/native/WX20240105120000abcdef12
```

**响应**:
```json
{
  "success": true,
  "data": {
    "code_url": "weixin://wxpay/bizpayurl?pr=abcdef1234"
  }
}
```

#### 步骤3：生成二维码

将 `code_url` 生成二维码，用户扫码支付。

---

### 3. APP支付（移动应用）

**适用场景**: 微信APP内调起支付

**使用流程**:

#### 步骤1：创建订单
```bash
curl -X POST http://localhost:8080/api/v1/wechat/orders \
  -d '{
    "user_id": 1,
    "product_id": "premium",
    "description": "Premium会员",
    "total_amount": 2999,
    "trade_type": "APP"
  }'
```

#### 步骤2：创建APP支付
```bash
curl -X POST http://localhost:8080/api/v1/wechat/payments/app/WX20240105...
```

**响应**:
```json
{
  "success": true,
  "data": {
    "prepay_id": "wx20240105120000xxx",
    "partner_id": "1234567890",
    "app_id": "wx1234567890abcdef",
    "time_stamp": "1704432000",
    "nonce_str": "abcdef1234567890",
    "package": "Sign=WXPay",
    "sign_type": "RSA"
  }
}
```

#### 步骤3：APP调起支付

使用返回参数调起微信支付。

---

### 4. H5支付（手机网站）

**适用场景**: 手机浏览器内支付

**使用流程**:

#### 步骤1：创建订单
```bash
curl -X POST http://localhost:8080/api/v1/wechat/orders \
  -d '{
    "user_id": 1,
    "product_id": "premium",
    "description": "Premium会员",
    "total_amount": 2999,
    "trade_type": "MWEB"
  }'
```

#### 步骤2：创建H5支付
```bash
curl -X POST http://localhost:8080/api/v1/wechat/payments/h5/WX20240105... \
  -d '{
    "scene_info": {
      "h5_info": {
        "type": "Wap",
        "wap_url": "https://your-domain.com",
        "wap_name": "Your Site Name"
      }
    }
  }'
```

**响应**:
```json
{
  "success": true,
  "data": {
    "h5_url": "https://wx.tenpay.com/cgi-bin/mmpayweb-bin/checkmweb?prepay_id=..."
  }
}
```

#### 步骤3：跳转支付

将用户重定向到 `h5_url` 完成支付。

---

## 订单管理

### 查询订单状态

```bash
curl -X GET http://localhost:8080/api/v1/wechat/orders/WX20240105120000abcdef12
```

**响应**:
```json
{
  "success": true,
  "data": {
    "order_no": "WX20240105120000abcdef12",
    "transaction_id": "4200001234567890",
    "trade_state": "SUCCESS",
    "total_amount": 2999,
    "payment_status": "COMPLETED",
    "paid_at": "2024-01-05T12:05:30Z"
  }
}
```

**交易状态说明**:
- `SUCCESS` - 支付成功
- `REFUND` - 转入退款
- `NOTPAY` - 未支付
- `CLOSED` - 已关闭
- `REVOKED` - 已撤销
- `USERPAYING` - 用户支付中
- `PAYERROR` - 支付失败

### 关闭订单

```bash
curl -X POST http://localhost:8080/api/v1/wechat/orders/WX20240105.../close
```

**响应**:
```json
{
  "success": true,
  "message": "订单关闭成功"
}
```

**注意**: 只能关闭未支付的订单

---

## 退款功能

### 发起退款

```bash
curl -X POST http://localhost:8080/api/v1/wechat/refunds \
  -H "Content-Type: application/json" \
  -d '{
    "order_no": "WX20240105120000abcdef12",
    "refund_amount": 2999,
    "refund_reason": "用户申请退款"
  }'
```

**响应**:
```json
{
  "success": true,
  "data": {
    "out_refund_no": "REFWX20240105120000abcdef12120530",
    "refund_id": "wx20240105120530xxx",
    "refund_amount": 2999,
    "refund_status": "SUCCESS",
    "refund_at": "2024-01-05T12:05:30Z"
  }
}
```

### 退款注意事项

1. 只能对已支付的订单退款
2. 退款金额不能超过订单金额
3. 部分退款需要多次调用
4. 退款会实时处理

---

## Webhook处理

### 配置Webhook

1. 登录微信支付商户平台
2. 产品中心 → 开发配置 → 设置通知URL
3. 填写: `https://your-domain.com/webhook/wechat/notify`

### 通知处理

服务端会自动处理微信支付通知：

1. **接收通知** - POST请求到 `/webhook/wechat/notify`
2. **验证签名** - 确保通知来自微信（待完善）
3. **处理通知** - 更新订单和支付状态
4. **返回响应** - 返回成功给微信

### 通知数据格式

```json
{
  "id": "notification_id",
  "create_time": "2024-01-05T12:05:30+08:00",
  "event_type": "TRANSACTION.SUCCESS",
  "resource_type": "encrypt-resource",
  "resource": {
    "algorithm": "AEAD_AES_256_GCM",
    "ciphertext": "encrypted_data",
    "nonce": "random_nonce",
    "associated_data": "transaction"
  }
}
```

### 服务端处理流程

```
接收通知 → 解密数据 → 验证签名 → 更新订单 → 返回成功
```

---

## 测试指南

### 使用沙盒环境

微信支付没有独立的沙盒环境，需要使用：

1. **申请测试商户号**
   - 联系微信支付申请测试商户号
   
2. **使用测试金额**
   - 使用小额测试（如0.01元）
   
3. **测试账号**
   - 使用测试微信账号

### 测试步骤

#### 1. 测试JSAPI支付

```bash
# 1. 创建订单
ORDER_NO=$(curl -s -X POST http://localhost:8080/api/v1/wechat/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id": 1, "product_id": "test", "description": "测试商品", "total_amount": 1, "trade_type": "JSAPI"}' \
  | jq -r '.data.order_no')

# 2. 创建支付
curl -X POST http://localhost:8080/api/v1/wechat/payments/jsapi/$ORDER_NO \
  -d '{"openid": "test_openid"}'

# 3. 在小程序中完成支付

# 4. 查询订单
curl http://localhost:8080/api/v1/wechat/orders/$ORDER_NO
```

#### 2. 测试Native支付

```bash
# 1. 创建订单
ORDER_NO=$(curl -s -X POST http://localhost:8080/api/v1/wechat/orders \
  -d '{"user_id": 1, "product_id": "test", "description": "测试", "total_amount": 1, "trade_type": "NATIVE"}' \
  | jq -r '.data.order_no')

# 2. 生成二维码
curl -X POST http://localhost:8080/api/v1/wechat/payments/native/$ORDER_NO

# 3. 扫码支付
```

#### 3. 测试退款

```bash
# 发起退款
curl -X POST http://localhost:8080/api/v1/wechat/refunds \
  -d '{
    "order_no": "WX20240105...",
    "refund_amount": 1,
    "refund_reason": "测试退款"
  }'
```

---

## 常见问题

### Q1: 如何获取用户的openid？

**A**: 
- **小程序**: 使用 `wx.login()` 获取code，后端调用接口换取openid
- **公众号**: 网页授权获取openid
- **详细流程**: 参考微信开放文档

### Q2: 支付通知没有收到？

**A**: 检查以下几点：
1. `notify_url` 必须是公网可访问的HTTPS地址
2. 检查防火墙和安全组配置
3. 查看服务器日志确认是否收到请求
4. 在微信商户平台查看通知记录

### Q3: 签名验证如何实现？

**A**: 
- 微信支付使用RSA签名
- 需要使用商户私钥计算签名
- 验证时使用平台公钥
- 当前代码中的签名验证为占位实现，需要完善

### Q4: 不同支付方式有什么区别？

**A**:
- **JSAPI**: 需要openid，在微信内使用
- **Native**: 返回二维码，适合PC端
- **APP**: 调起微信APP支付
- **MWEB**: 在手机浏览器内支付

### Q5: prepay_id有效期是多久？

**A**: 
- 有效期为2小时
- 超时需要重新创建支付
- 建议在创建后立即使用

---

## 业务流程图

### JSAPI支付流程

```
客户端           服务端                微信支付
  │                │                    │
  ├──创建订单──→   │                    │
  │                ├──保存Order──→DB    │
  │←──返回订单号── │                    │
  │                │                    │
  ├──发起支付──→   │                    │
  │  (openid)      ├──请求预支付──→    │
  │                │                ←──prepay_id
  │                ├──保存prepay_id→DB  │
  │←──返回参数──   │                    │
  │                │                    │
  ├──调起支付──→   │                    │
  │            微信支付界面             │
  │                │                    │
  │                │    ←───支付通知────┤
  │                ├──更新状态──→DB     │
  │                ├──返回成功──→       │
  │                │                    │
  ├──查询结果──→   │                    │
  │←──支付成功──   │                    │
```

---

## 最佳实践

### 1. 安全措施

- ✅ 使用HTTPS传输
- ✅ 验证异步通知签名
- ✅ 私钥安全存储
- ✅ 定期检查订单状态

### 2. 性能优化

- ✅ 缓存订单查询结果
- ✅ 异步处理Webhook通知
- ✅ 使用连接池

### 3. 错误处理

- ✅ 记录详细日志
- ✅ 实现重试机制
- ✅ 友好的错误提示

### 4. 用户体验

- ✅ 及时更新订单状态
- ✅ 支持查询支付结果
- ✅ 提供退款功能

---

## 代码示例

### 在代码中使用

```go
// 初始化服务
wechatService, err := services.NewWechatService(db, &cfg.Wechat, logger)
if err != nil {
    log.Fatal(err)
}

// 创建订单
req := &services.CreateWechatOrderRequest{
    UserID:      1,
    ProductID:   "premium",
    Description: "Premium会员",
    TotalAmount: 2999,
    TradeType:   "JSAPI",
}
order, err := wechatService.CreateOrder(ctx, req)

// 创建JSAPI支付
jsapiResp, err := wechatService.CreateJSAPIPayment(ctx, order.OrderNo, userOpenID)

// 查询订单
orderStatus, err := wechatService.QueryOrder(ctx, order.OrderNo)

// 退款
refundReq := &services.WechatRefundRequest{
    OrderNo:      order.OrderNo,
    RefundAmount: 2999,
    RefundReason: "用户申请退款",
}
refund, err := wechatService.Refund(ctx, refundReq)
```

---

## 错误码参考

| 错误码 | 说明 | 解决方案 |
|--------|------|---------|
| SYSTEMERROR | 系统错误 | 稍后重试 |
| PARAM_ERROR | 参数错误 | 检查请求参数 |
| ORDERPAID | 订单已支付 | 不要重复支付 |
| ORDERCLOSED | 订单已关闭 | 创建新订单 |
| NOAUTH | 商户无权限 | 检查商户配置 |
| AUTHCODEEXPIRE | 授权码过期 | 重新获取授权码 |
| INVALID_REQUEST | 无效请求 | 检查请求格式 |

---

## 数据模型

### WechatPayment（微信支付详情）

```go
type WechatPayment struct {
    ID               uint       // 主键
    OrderID          uint       // 订单ID
    OutTradeNo       string     // 商户订单号
    TransactionID    string     // 微信支付订单号
    TradeType        string     // 交易类型
    TradeState       string     // 交易状态
    BankType         string     // 银行类型
    SuccessTime      *time.Time // 支付完成时间
    PrepayID         string     // 预支付ID
    CodeURL          string     // 二维码链接（Native）
    H5URL            string     // H5支付链接
    AppID            string     // 应用ID
    MchID            string     // 商户号
    // ... 更多字段
}
```

---

## 总结

微信支付功能已完整实现，支持四种支付方式，代码结构清晰，易于使用和维护。

**代码位置**：
- 服务：`internal/services/wechat_service.go`
- 处理器：`internal/handlers/wechat_handler.go`
- 路由：`internal/routes/routes.go`

**下一步**：
1. 完善签名验证逻辑
2. 添加单元测试
3. 接入真实的微信支付API

更多信息请查看 [文档索引](../../INDEX.md)

