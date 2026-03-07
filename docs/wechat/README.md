# 微信支付接入指南

本文档详细介绍如何接入微信支付功能。

## 📋 目录

- [功能概述](#功能概述)
- [配置](#配置)
- [密钥机制说明](#密钥机制说明)
- [双向签名校验](#双向签名校验)
- [支付流程](#支付流程)
- [API 接口](#api-接口)
- [Webhook 处理](#webhook-处理)
- [回调处理流程与防篡改机制](#回调处理流程与防篡改机制)
- [最佳实践](#最佳实践)

## 功能概述

| 功能 | 说明 | 状态 |
|-----|------|-----|
| JSAPI 支付 | 小程序/公众号支付 | ✅ 真实 API |
| Native 支付 | 扫码支付 | ✅ 真实 API |
| APP 支付 | 原生 App 支付 | ✅ 真实 API |
| H5 支付 | 手机浏览器支付 | ✅ 真实 API |
| 退款 | 原路退回 | ✅ |
| 异步通知 | 支付/退款结果通知 | ✅ 验签+解密 |

## 配置

### 1. 获取商户信息

1. 登录 [微信支付商户平台](https://pay.weixin.qq.com/)
2. 获取 **商户号 (MchID)**
3. 获取 **API v3 密钥**
4. 下载 **商户证书**

### 2. 配置 config.toml

```toml
[wechat]
# 应用 ID (公众号/小程序/开放平台应用)
app_id = "wx1234567890abcdef"

# 商户号
mch_id = "1234567890"

# API v3 密钥
api_v3_key = "your-api-v3-key-32-chars-long"

# 证书序列号
serial_no = "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

# 商户私钥（二选一：内容或文件路径）
private_key = '''
-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC...
-----END PRIVATE KEY-----
'''
# 或使用私钥文件路径（当 private_key 为空时从文件读取）
# private_key_path = "configs/wechat_private_key.pem"

# 回调地址
notify_url = "https://your-domain.com/webhook/wechat/notify"

# 微信平台证书路径（用于验签回调，从商户平台或 /v3/certificates 接口下载）
platform_cert_path = "configs/wechat_platform_cert.pem"
```

### 3. 密钥机制说明

微信支付 API v3 采用**混合密钥机制**，与支付宝 RSA2 不同：

| 密钥类型 | 加密方式 | 来源 | 用途 |
|----------|----------|------|------|
| **API v3 密钥** | 对称加密 | 商户在微信商户平台**设置**（32 位） | 解密回调通知内容（AEAD_AES_256_GCM） |
| **商户私钥** | 非对称加密 (RSA) | 商户**自己生成** | 签名请求、调用微信 API |
| **微信平台证书** | 非对称加密 (RSA) | 微信提供 | 验签回调请求 |

#### 与支付宝 RSA2 的区别

| 对比项 | 支付宝 RSA2 | 微信 API v3 |
|--------|-------------|-------------|
| **签名** | 商户 RSA 私钥签名 | 商户 RSA 私钥签名（相同） |
| **验签** | 支付宝公钥验签 | 微信平台证书公钥验签（相同） |
| **通知内容** | 明文 + 签名 | **加密**（AES-256-GCM）+ 签名 |
| **API v3 密钥** | 无 | **对称密钥**，用于解密通知，在商户平台设置 |

#### API v3 密钥说明

- **32 个字符**，支持数字和大小写字母
- **由商户在微信商户平台设置**，非本地生成
- **仅用于解密回调**中的 `resource.ciphertext`（AEAD_AES_256_GCM 算法）
- **不参与**商户发往微信的 API 请求签名（请求签名使用商户私钥）
- 与支付宝「商户自己生成密钥对」不同，此密钥为商户与微信**共享**

#### 商户私钥说明

- **由商户自己生成**，与支付宝相同
- RSA 私钥，用于：
  1. **签名 API 请求**：调用微信支付接口时，请求头带 `Authorization: WECHATPAY2-SHA256-RSA2048` 签名
  2. **签名调起支付参数**：JSAPI/APP 的 `pay_sign`/`sign` 由服务端用商户私钥计算后返回给客户端
- 公钥通过商户证书上传至微信

#### 双向签名校验

微信支付与支付宝一样，对**请求和回调**均进行签名校验：

| 方向 | 谁签名 | 谁验签 | 说明 |
|------|--------|--------|------|
| **商户 → 微信** | 商户私钥 | 微信用商户证书公钥 | 商户调用 API 时，请求头带 `Authorization: WECHATPAY2-SHA256-RSA2048` 签名 |
| **微信 → 商户** | 微信私钥 | 商户用微信平台证书公钥 | 回调通知请求头带 `Wechatpay-Signature`，商户验签 |

商户请求签名覆盖：HTTP 方法、URL、时间戳、随机串、请求体等，微信用商户证书验签，确保请求来自该商户且未被篡改。

**本实现**：JSAPI/Native/APP/H5 下单均已接入真实微信支付 API，请求自动携带 `Authorization: WECHATPAY2-SHA256-RSA2048` 签名；JSAPI/APP 的 `pay_sign`/`sign` 由服务端用商户私钥计算后返回，客户端无需本地签名。

## 支付流程

### JSAPI 支付流程（小程序/公众号）

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          JSAPI 支付流程                                       │
└─────────────────────────────────────────────────────────────────────────────┘

    客户端                     服务端                      微信支付
      │                         │                            │
      │  1. 创建订单            │                            │
      │ ─────────────────────> │                            │
      │  POST /wechat/orders    │                            │
      │                         │                            │
      │  返回 order_no          │                            │
      │ <───────────────────── │                            │
      │                         │                            │
      │  2. 创建 JSAPI 支付     │                            │
      │ ─────────────────────> │                            │
      │  POST /wechat/payments/jsapi/{order_no}             │
      │  {openid}               │                            │
      │                         │  商户私钥签名，调用微信 API │
      │                         │ ─────────────────────────> │
      │                         │                            │
      │                         │  返回 prepay_id           │
      │                         │ <───────────────────────── │
      │  返回调起支付参数        │                            │
      │ <───────────────────── │                            │
      │  {appId, timeStamp, nonceStr, package, signType, paySign} │
      │                         │                            │
      │  3. 调起微信支付         │                            │
      │ ─────────────────────────────────────────────────> │
      │  wx.requestPayment()    │                            │
      │                         │                            │
      │  支付完成               │                            │
      │ <───────────────────────────────────────────────── │
      │                         │                            │
      │                         │  4. 支付结果通知           │
      │                         │ <───────────────────────── │
      │                         │  POST /webhook/wechat/notify│
      │                         │                            │
      │                         │  验签，更新订单状态        │
      │                         │  返回成功                  │
      │                         │ ─────────────────────────> │
      └                         └                            └
```

### Native 支付流程（扫码）

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Native 支付流程                                      │
└─────────────────────────────────────────────────────────────────────────────┘

    客户端                     服务端                      微信支付
      │                         │                            │
      │  1. 创建订单            │                            │
      │ ─────────────────────> │                            │
      │  POST /wechat/orders    │                            │
      │                         │                            │
      │  返回 order_no          │                            │
      │ <───────────────────── │                            │
      │                         │                            │
      │  2. 创建 Native 支付    │                            │
      │ ─────────────────────> │                            │
      │  POST /wechat/payments/native/{order_no}            │
      │                         │  商户私钥签名，调用微信 API │
      │                         │ ─────────────────────────> │
      │                         │  返回 code_url            │
      │                         │ <───────────────────────── │
      │  返回二维码链接          │                            │
      │ <───────────────────── │                            │
      │  {code_url}             │                            │
      │                         │                            │
      │  3. 展示二维码           │                            │
      │  用户扫码支付           │                            │
      │ ─────────────────────────────────────────────────> │
      │                         │                            │
      │                         │  4. 支付结果通知           │
      │                         │ <───────────────────────── │
      │                         │                            │
      └                         └                            └
```

## API 接口

### 创建订单

```http
POST /api/v1/wechat/orders
Content-Type: application/json

{
  "user_id": 1,
  "product_id": "premium_upgrade",
  "description": "高级会员升级",
  "detail": "解锁所有高级功能",
  "total_amount": 9900,
  "trade_type": "JSAPI"
}
```

**trade_type 可选值：**

| 类型 | 说明 |
|-----|------|
| `JSAPI` | 公众号/小程序支付 |
| `NATIVE` | 扫码支付 |
| `APP` | APP 支付 |
| `MWEB` | H5 支付 |

**响应：**

```json
{
  "success": true,
  "data": {
    "order_id": 1,
    "order_no": "WX20240101120000abcd1234",
    "total_amount": 9900,
    "description": "高级会员升级"
  }
}
```

### JSAPI 支付

```http
POST /api/v1/wechat/payments/jsapi/WX20240101120000abcd1234
Content-Type: application/json

{
  "openid": "oUpF8uMuAJO_M2pxb1Q9zNjWeS6o"
}
```

**响应：**

```json
{
  "success": true,
  "data": {
    "prepay_id": "wx20240101120000xxxxx",
    "app_id": "wx1234567890abcdef",
    "time_stamp": "1704067200",
    "nonce_str": "a1b2c3d4e5f6g7h8",
    "package": "prepay_id=wx20240101120000xxxxx",
    "sign_type": "RSA",
    "pay_sign": "Base64编码的RSA签名，服务端已用商户私钥计算"
  }
}
```

> `pay_sign` 由服务端计算，客户端可直接传给 `wx.requestPayment`，无需本地签名。

### Native 支付

```http
POST /api/v1/wechat/payments/native/WX20240101120000abcd1234
```

**响应：**

```json
{
  "success": true,
  "data": {
    "code_url": "weixin://wxpay/bizpayurl?pr=xxxxx"
  }
}
```

### APP 支付

```http
POST /api/v1/wechat/payments/app/WX20240101120000abcd1234
```

**响应：**

```json
{
  "success": true,
  "data": {
    "prepay_id": "wx20240101120000xxxxx",
    "partner_id": "1234567890",
    "app_id": "wx1234567890abcdef",
    "time_stamp": "1704067200",
    "nonce_str": "a1b2c3d4e5f6g7h8",
    "package": "prepay_id=wx20240101120000xxxxx",
    "sign_type": "RSA",
    "sign": "Base64编码的RSA签名，服务端已用商户私钥计算"
  }
}
```

> `sign` 由服务端计算，APP 端可直接用于调起微信支付 SDK。

### H5 支付

```http
POST /api/v1/wechat/payments/h5/WX20240101120000abcd1234
Content-Type: application/json

{
  "scene_info": {
    "payer_client_ip": "14.23.150.211",
    "type": "Wap",
    "app_name": "Example",
    "app_url": "https://example.com"
  }
}
```

**响应：**

```json
{
  "success": true,
  "data": {
    "h5_url": "https://wx.tenpay.com/cgi-bin/mmpayweb-bin/checkmweb?..."
  }
}
```

### 查询订单

```http
GET /api/v1/wechat/orders/WX20240101120000abcd1234
```

**响应：**

```json
{
  "success": true,
  "data": {
    "order_no": "WX20240101120000abcd1234",
    "transaction_id": "4200001234567890123456789012",
    "trade_state": "SUCCESS",
    "total_amount": 9900,
    "payment_status": "COMPLETED",
    "paid_at": "2024-01-01T12:05:30Z"
  }
}
```

### 关闭订单

```http
POST /api/v1/wechat/orders/WX20240101120000abcd1234/close
```

### 退款

```http
POST /api/v1/wechat/refunds
Content-Type: application/json

{
  "order_no": "WX20240101120000abcd1234",
  "refund_amount": 9900,
  "refund_reason": "用户申请退款"
}
```

**响应：**

```json
{
  "success": true,
  "data": {
    "out_refund_no": "REFWX20240101120000abcd1234130530",
    "refund_id": "wx20240101130530xxxxx",
    "refund_amount": 9900,
    "refund_status": "SUCCESS",
    "refund_at": "2024-01-01T13:05:30Z"
  }
}
```

## Webhook 处理

### 支付通知

```
POST /webhook/wechat/notify
Content-Type: application/json

{
  "id": "xxxxx-xxxxx-xxxxx",
  "create_time": "2024-01-01T12:05:30+08:00",
  "resource_type": "encrypt-resource",
  "event_type": "TRANSACTION.SUCCESS",
  "resource": {
    "algorithm": "AEAD_AES_256_GCM",
    "ciphertext": "...",
    "nonce": "...",
    "associated_data": "..."
  }
}
```

**trade_state 状态说明：**

| 状态 | 说明 |
|-----|------|
| `SUCCESS` | 支付成功 |
| `REFUND` | 转入退款 |
| `NOTPAY` | 未支付 |
| `CLOSED` | 已关闭 |
| `REVOKED` | 已撤销 |
| `USERPAYING` | 用户支付中 |
| `PAYERROR` | 支付失败 |

### 退款通知

```
POST /webhook/wechat/refund
```

### 回调处理流程与防篡改机制

微信回调通知内容经过加密，防篡改依赖**两层机制**。

#### 处理流程（必须按顺序执行）

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        微信回调处理流程                                       │
└─────────────────────────────────────────────────────────────────────────────┘

  1. 验签                    2. 解密                    3. 业务处理
  ┌─────────────┐           ┌─────────────┐           ┌─────────────┐
  │ 校验请求头   │   通过    │ 用 API v3   │   成功    │ 更新订单    │
  │ Signature   │ ───────> │ 密钥解密    │ ───────> │ 返回成功    │
  │ 微信平台公钥 │           │ ciphertext  │           │             │
  └─────────────┘           └─────────────┘           └─────────────┘
        │                          │
        │ 失败：拒绝请求             │ 失败：密文被篡改
        ▼                          ▼
  ┌─────────────┐           ┌─────────────┐
  │ 直接返回    │           │ 解密失败    │
  │ 不处理      │           │ 拒绝请求    │
  └─────────────┘           └─────────────┘
```

| 步骤 | 操作 | 失败处理 |
|------|------|----------|
| 1. 验签 | 用微信平台证书公钥验证 `Wechatpay-Signature` | 拒绝请求，不解密 |
| 2. 解密 | 用 API v3 密钥解密 `resource.ciphertext` | 解密失败说明密文被篡改，拒绝 |
| 3. 业务处理 | 解析明文，更新订单，幂等校验 | 返回 `success` 或 `fail` |

#### 防篡改机制

**机制一：AEAD_AES_256_GCM 认证加密**

| 特性 | 说明 |
|------|------|
| **AEAD** | Authenticated Encryption with Associated Data，认证加密 |
| **认证标签** | 密文附带认证标签，解密时自动校验完整性 |
| **防篡改** | 密文被篡改 → 解密失败，无法得到明文 |

**机制二：请求头签名验证**

```
Wechatpay-Timestamp: 时间戳
Wechatpay-Nonce: 随机串
Wechatpay-Signature: 微信用私钥对 body 的签名
```

- 验签字符串：`timestamp + "\n" + nonce + "\n" + body + "\n"`
- 使用**微信平台证书公钥**验证签名
- 攻击者无法伪造有效签名（无微信私钥）

**双重保障示意：**

```
  请求体 (body)                          请求头
  ┌─────────────────────┐               ┌─────────────────────┐
  │ resource:           │               │ Wechatpay-Signature │
  │  ciphertext (加密)   │  ──验签──>   │ Wechatpay-Timestamp │
  │  nonce              │               │ Wechatpay-Nonce     │
  │  associated_data    │               └─────────────────────┘
  └─────────────────────┘                        │
           │                                     │ 用微信平台公钥
           │ 解密时校验认证标签                    │ 验证签名
           ▼                                     ▼
  ┌─────────────────────┐               ┌─────────────────────┐
  │ 密文被篡改→解密失败   │               │ 签名无效→拒绝请求    │
  └─────────────────────┘               └─────────────────────┘
```

## 最佳实践

### 1. 签名验证

```go
func VerifyWechatSignature(headers http.Header, body []byte, apiKey string) error {
    timestamp := headers.Get("Wechatpay-Timestamp")
    nonce := headers.Get("Wechatpay-Nonce")
    signature := headers.Get("Wechatpay-Signature")
    
    // 构建验签字符串
    message := fmt.Sprintf("%s\n%s\n%s\n", timestamp, nonce, string(body))
    
    // 使用微信支付平台公钥验证签名
    // ...
}
```

### 2. 解密通知内容

```go
func DecryptNotification(ciphertext, nonce, associatedData, apiKey string) ([]byte, error) {
    key := []byte(apiKey)
    
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    
    aesgcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    ciphertextBytes, _ := base64.StdEncoding.DecodeString(ciphertext)
    nonceBytes := []byte(nonce)
    
    return aesgcm.Open(nil, nonceBytes, ciphertextBytes, []byte(associatedData))
}
```

### 3. 小程序调起支付

```javascript
// 小程序端代码
wx.requestPayment({
  timeStamp: res.data.time_stamp,
  nonceStr: res.data.nonce_str,
  package: res.data.package,
  signType: res.data.sign_type,
  paySign: res.data.pay_sign,
  success: function(res) {
    console.log('支付成功');
  },
  fail: function(res) {
    console.log('支付失败', res);
  }
});
```

### 4. 幂等性处理

```go
func HandleWechatNotify(notifyData map[string]interface{}) error {
    outTradeNo := notifyData["out_trade_no"].(string)
    
    var order models.Order
    err := db.Where("order_no = ?", outTradeNo).First(&order).Error
    if err != nil {
        return err
    }
    
    // 已处理则直接返回成功
    if order.PaymentStatus == models.PaymentStatusCompleted {
        return nil
    }
    
    // 处理通知...
}
```

## 相关文档

- [微信支付开发文档](https://pay.weixin.qq.com/wiki/doc/apiv3/apis/index.shtml)
- [JSAPI 支付](https://pay.weixin.qq.com/wiki/doc/apiv3/apis/chapter3_1_1.shtml)
- [Native 支付](https://pay.weixin.qq.com/wiki/doc/apiv3/apis/chapter3_4_1.shtml)
- [APP 支付](https://pay.weixin.qq.com/wiki/doc/apiv3/apis/chapter3_2_1.shtml)
- [H5 支付](https://pay.weixin.qq.com/wiki/doc/apiv3/apis/chapter3_3_1.shtml)

