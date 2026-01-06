# 微信支付接入指南

本文档详细介绍如何接入微信支付功能。

## 📋 目录

- [功能概述](#功能概述)
- [配置](#配置)
- [支付流程](#支付流程)
- [API 接口](#api-接口)
- [Webhook 处理](#webhook-处理)
- [最佳实践](#最佳实践)

## 功能概述

| 功能 | 说明 | 状态 |
|-----|------|-----|
| JSAPI 支付 | 小程序/公众号支付 | ✅ |
| Native 支付 | 扫码支付 | ✅ |
| APP 支付 | 原生 App 支付 | ✅ |
| H5 支付 | 手机浏览器支付 | ✅ |
| 退款 | 原路退回 | ✅ |
| 异步通知 | 支付/退款结果通知 | ✅ |

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

# 商户私钥
private_key = '''
-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC...
-----END PRIVATE KEY-----
'''

# 回调地址
notify_url = "https://your-domain.com/webhook/wechat/notify"
```

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
      │                         │  调用统一下单接口          │
      │                         │ ─────────────────────────> │
      │                         │                            │
      │                         │  返回 prepay_id           │
      │                         │ <───────────────────────── │
      │  返回调起支付参数        │                            │
      │ <───────────────────── │                            │
      │  {appId, timeStamp, nonceStr, package, signType}    │
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
      │                         │  调用下单接口              │
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
    "sign_type": "RSA"
  }
}
```

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
    "package": "Sign=WXPay",
    "sign_type": "RSA"
  }
}
```

### H5 支付

```http
POST /api/v1/wechat/payments/h5/WX20240101120000abcd1234
Content-Type: application/json

{
  "scene_info": {
    "payer_client_ip": "14.23.150.211",
    "h5_info": {
      "type": "Wap",
      "wap_url": "https://example.com",
      "wap_name": "Example"
    }
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

