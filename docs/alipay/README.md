# 支付宝接入指南

本文档详细介绍支付宝支付能力，包括普通支付、周期扣款、免密代扣、退款及对账。

## 📋 目录

- [功能概述](#功能概述)
- [架构设计](#架构设计)
- [配置](#配置)
- [支付流程](#支付流程)
- [API 接口](#api-接口)
- [Webhook 处理](#webhook-处理)
- [最佳实践](#最佳实践)

## 功能概述

| 功能 | 说明 | 状态 |
|-----|------|-----|
| 手机网站支付 | WAP H5 支付 | ✅ |
| 电脑网站支付 | PC 网页支付 | ✅ |
| App 支付 | 原生 App 支付 | ✅ |
| 周期扣款 | 签约代扣/订阅 | ✅ |
| 免密支付 | 商户代扣，签约后单次扣款无需用户确认 | ✅ |
| 退款 | 原路退回，支持幂等 | ✅ |
| 对账 | 下载账单与本地订单比对 | ✅ |
| 异步通知 | 支付/签约/扣款结果通知 | ✅ |

## 架构设计

为保证**订单一致性**和**防资损**，采用三层保障机制：

| 层级 | 机制 | 说明 |
|-----|------|-----|
| 主通道 | Webhook 为主 | 支付宝异步通知，实时性最好 |
| 兜底一 | 主动查询 | 客户端轮询 + 服务端定时任务（每 2 分钟）向支付宝查询待支付订单并同步状态 |
| 兜底二 | 对账 | 每日下载官方账单，与本地订单逐笔比对，发现并记录差异 |

## 配置

### 1. 创建支付宝应用

1. 登录 [支付宝开放平台](https://open.alipay.com/)
2. 创建应用，获取 **AppID**
3. 配置应用公钥或上传证书
4. 开通所需产品（手机网站支付、电脑网站支付、周期扣款、代扣等）

### 2. 配置文件

```toml
[alipay]
app_id = "2021000000000000"
private_key = """
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA...
-----END RSA PRIVATE KEY-----
"""
is_production = false
notify_url = "https://your-domain.com/webhook/alipay/notify"
withhold_notify_url = "https://your-domain.com/webhook/alipay/withhold"  # 免密签约通知，可选
return_url = "https://your-domain.com/payment/return"

# 证书模式（推荐）
cert_mode = true
app_cert_path = "configs/alipay/appCertPublicKey.crt"
root_cert_path = "configs/alipay/alipayRootCert.crt"
alipay_cert_path = "configs/alipay/alipayCertPublicKey_RSA2.crt"

# 对账定时任务（可选）
reconciliation_cron_enable = true
reconciliation_cron_time = "02:00"   # 每日凌晨 2 点执行前一日对账
```

### 3. 周期扣款配置

使用周期扣款需额外开通：

1. 申请开通 **周期扣款** 产品
2. 获取 **个人签约产品码**（如 `CYCLE_PAY_AUTH_P`）
3. 确定 **签约场景**（如 `INDUSTRY|MEDICAL_INSURANCE`）

### 4. 免密支付配置

使用免密代扣需额外开通 **代扣** 产品，并配置 `withhold_notify_url` 接收签约通知。

## 支付流程

### 普通支付流程

```
    客户端                     服务端                      支付宝
      │                         │                            │
      │  1. 创建订单            │                            │
      │  POST /alipay/orders    │                            │
      │  {allow_duplicate}      │                            │
      │ ─────────────────────> │                            │
      │                         │                            │
      │  2. 创建支付            │                            │
      │  POST /alipay/payments  │  调用支付宝下单           │
      │  {order_no, pay_type}   │ ─────────────────────────> │
      │                         │                            │
      │  返回 payment_url       │                            │
      │ <───────────────────── │                            │
      │                         │                            │
      │  3. 跳转支付宝支付      │                            │
      │ ─────────────────────────────────────────────────> │
      │                         │                            │
      │                         │  4. 异步通知               │
      │                         │  POST /webhook/alipay/notify│
      │                         │ <───────────────────────── │
      │                         │  验证签名、幂等、金额校验  │
      │                         │  更新订单状态              │
      │                         │                            │
      │  5. 主动查询（可选）    │                            │
      │  GET /alipay/orders/query?order_no=xxx               │
      │ ─────────────────────> │  调用 TradeQuery 同步状态   │
      │                         │ ─────────────────────────> │
      │  返回最新状态           │                            │
      │ <───────────────────── │                            │
```

### 周期扣款流程

```
    客户端                     服务端                      支付宝
      │                         │                            │
      │  1. 创建周期扣款        │                            │
      │  POST /alipay/subscriptions                          │
      │ ─────────────────────> │  调用签约接口               │
      │                         │ ─────────────────────────> │
      │  返回 sign_url         │                            │
      │ <───────────────────── │                            │
      │                         │                            │
      │  2. 跳转签约页面        │                            │
      │ ─────────────────────────────────────────────────> │
      │                         │                            │
      │                         │  3. 签约成功通知           │
      │                         │  POST /webhook/alipay/subscription
      │                         │ <───────────────────────── │
      │                         │                            │
      │  4. 到期自动扣款       │                            │
      │                         │  POST /webhook/alipay/deduct
      │                         │ <───────────────────────── │
      │                         │  幂等、分布式锁处理         │
```

### 免密支付流程

```
    客户端                     服务端                      支付宝
      │                         │                            │
      │  1. 创建免密签约        │                            │
      │  POST /alipay/withhold/agreements                    │
      │ ─────────────────────> │  调用签约接口               │
      │                         │ ─────────────────────────> │
      │  返回 sign_url         │                            │
      │ <───────────────────── │                            │
      │                         │                            │
      │  2. 跳转签约            │                            │
      │ ─────────────────────────────────────────────────> │
      │                         │  签约通知                  │
      │                         │  POST /webhook/alipay/withhold
      │                         │ <───────────────────────── │
      │                         │                            │
      │  3. 执行单次代扣        │                            │
      │  POST /alipay/withhold/execute                       │
      │ ─────────────────────> │  调用代扣接口               │
      │                         │ ─────────────────────────> │
      │  返回扣款结果           │                            │
      │ <───────────────────── │                            │
```

## API 接口

### 创建订单

```http
POST /api/v1/alipay/orders
Content-Type: application/json

{
  "user_id": 1,
  "product_id": "premium_upgrade",
  "subject": "高级会员升级",
  "body": "解锁全部高级功能",
  "total_amount": 9900,
  "allow_duplicate": false
}
```

| 参数 | 类型 | 必填 | 说明 |
|-----|------|-----|------|
| `allow_duplicate` | bool | 否 | 默认 `false`。为 `false` 时，若存在同用户、同商品、未支付且未过期的订单，则复用该订单 |

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "order_id": 1,
    "order_no": "ORD20240101120000abcd1234",
    "total_amount": 9900,
    "subject": "高级会员升级",
    "description": "解锁全部高级功能"
  }
}
```

### 创建支付

```http
POST /api/v1/alipay/payments
Content-Type: application/json

{
  "order_no": "ORD20240101120000abcd1234",
  "pay_type": "WAP"
}
```

| pay_type | 说明 |
|----------|------|
| `WAP` | 手机网站支付 |
| `PAGE` | 电脑网站支付 |
| `APP` | App 支付 |

### 查询订单

```http
GET /api/v1/alipay/orders/query?order_no=ORD20240101120000abcd1234
```

查询时会向支付宝发起 `TradeQuery`，若支付宝已支付而本地未同步，则自动更新本地订单状态。

### 退款

```http
POST /api/v1/alipay/refunds
Content-Type: application/json

{
  "order_no": "ORD20240101120000abcd1234",
  "refund_amount": 9900,
  "refund_reason": "用户申请退款",
  "out_request_no": "REF001"
}
```

| 参数 | 说明 |
|-----|------|
| `out_request_no` | 可选。退款请求号，传入相同值可实现重试幂等 |

### 周期扣款

| 接口 | 方法 | 说明 |
|-----|------|------|
| 创建周期扣款 | `POST /api/v1/alipay/subscriptions` | 创建签约，返回签约 URL |
| 查询周期扣款 | `GET /api/v1/alipay/subscriptions/query?out_request_no=xxx` | 查询签约状态 |
| 取消周期扣款 | `POST /api/v1/alipay/subscriptions/cancel` | 解约 |

### 免密支付（商户代扣）

| 接口 | 方法 | 说明 |
|-----|------|------|
| 创建免密签约 | `POST /api/v1/alipay/withhold/agreements` | 创建签约，返回签约 URL |
| 查询免密签约 | `GET /api/v1/alipay/withhold/agreements/query?out_request_no=xxx` | 查询签约状态 |
| 执行单次代扣 | `POST /api/v1/alipay/withhold/execute` | 执行扣款 |

### 对账

| 接口 | 方法 | 说明 |
|-----|------|------|
| 执行对账 | `POST /api/v1/alipay/reconciliation/run?bill_date=2024-01-15` | 下载指定日期账单并与本地订单比对 |
| 列出对账报告 | `GET /api/v1/alipay/reconciliation/reports?bill_date=&limit=20` | 列出对账报告 |
| 获取报告详情 | `GET /api/v1/alipay/reconciliation/reports/:id` | 获取报告及差异明细 |

对账逻辑：通过 `BillDownloadURLQuery` 获取对账文件，自动下载并解析 ZIP/CSV（支持 GBK 编码），与本地订单逐笔比对金额与状态，记录差异（`alipay_only`、`local_only`、`amount_mismatch`）。

## Webhook 处理

| 路径 | 说明 |
|-----|------|
| `POST /webhook/alipay/notify` | 支付异步通知 |
| `POST /webhook/alipay/subscription` | 周期扣款签约通知 |
| `POST /webhook/alipay/deduct` | 周期扣款扣款通知 |
| `POST /webhook/alipay/withhold` | 免密签约通知 |

### 支付通知处理要点

- **签名验证**：所有通知必须验证签名
- **幂等**：订单已支付完成直接返回成功
- **金额校验**：通知金额与订单金额必须一致
- **分布式锁**：Redis 锁保证同一订单并发通知串行处理

### trade_status 状态

| 状态 | 说明 |
|-----|------|
| `WAIT_BUYER_PAY` | 等待买家付款 |
| `TRADE_SUCCESS` | 交易成功 |
| `TRADE_CLOSED` | 交易关闭 |
| `TRADE_FINISHED` | 交易结束 |

## 最佳实践

### 1. 防重复下单

创建订单时设置 `allow_duplicate: false`，系统会复用同用户、同商品、未支付且未过期的订单。

### 2. 订单超时取消

系统定时任务每分钟执行，自动取消已过期的待支付订单。也可手动调用 `POST /api/v1/orders/cancel-expired`。

### 3. 主动查询兜底

- **客户端**：支付完成后轮询 `GET /api/v1/alipay/orders/query` 获取最新状态
- **服务端**：定时任务每 2 分钟对最近 2 小时内创建的待支付订单调用 `TradeQuery` 同步状态

### 4. 对账兜底

启用 `reconciliation_cron_enable` 后，每日在指定时间自动执行前一日对账。也可手动调用 `POST /api/v1/alipay/reconciliation/run?bill_date=yyyy-MM-dd`。

### 5. 金额单位

- 内部存储使用**分**（int64）
- 与支付宝交互时使用**元**（字符串）

### 6. Webhook 响应

处理成功必须返回 `success`，否则支付宝会重试。

## 相关文档

- [支付宝开放平台](https://open.alipay.com/)
- [手机网站支付](https://opendocs.alipay.com/open/02ivbs)
- [电脑网站支付](https://opendocs.alipay.com/open/270)
- [周期扣款](https://opendocs.alipay.com/open/02fkar)
- [代扣](https://opendocs.alipay.com/open/02ekfg)
- [账单下载](https://opendocs.alipay.com/open/02e7go)
- [异步通知](https://opendocs.alipay.com/open/203/105286)
