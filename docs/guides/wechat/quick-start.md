# 微信支付快速开始

本文档提供微信支付功能的快速参考。

## API端点

```
POST   /api/v1/wechat/orders                     # 创建订单
POST   /api/v1/wechat/payments/jsapi/:order_no   # JSAPI支付
POST   /api/v1/wechat/payments/native/:order_no  # Native支付
POST   /api/v1/wechat/payments/app/:order_no     # APP支付
POST   /api/v1/wechat/payments/h5/:order_no      # H5支付
GET    /api/v1/wechat/orders/:order_no           # 查询订单
POST   /api/v1/wechat/refunds                    # 退款
POST   /api/v1/wechat/orders/:order_no/close     # 关闭订单
POST   /webhook/wechat/notify                    # Webhook通知
```

## 使用示例

### JSAPI支付（小程序）
```bash
# 1. 创建订单
curl -X POST http://localhost:8080/api/v1/wechat/orders \
  -d '{
    "user_id": 1,
    "product_id": "premium",
    "description": "Premium会员",
    "total_amount": 2999,
    "trade_type": "JSAPI"
  }'

# 2. 创建JSAPI支付
curl -X POST http://localhost:8080/api/v1/wechat/payments/jsapi/WX20240105... \
  -d '{"openid": "user_openid"}'
```

### Native支付（扫码）
```bash
# 1. 创建订单
curl -X POST http://localhost:8080/api/v1/wechat/orders \
  -d '{"user_id": 1, "product_id": "premium", "description": "Premium", "total_amount": 2999, "trade_type": "NATIVE"}'

# 2. 创建Native支付（获取二维码）
curl -X POST http://localhost:8080/api/v1/wechat/payments/native/WX20240105...
```

## 配置示例
```toml
[wechat]
app_id = "wx1234567890abcdef"
mch_id = "1234567890"
apiv3_key = "your_apiv3_key"
serial_no = "certificate_serial_number"
private_key = "your_private_key"
notify_url = "https://your-domain.com/webhook/wechat/notify"
```

## 代码位置
- 服务：`internal/services/wechat_service.go` (880行)
- 处理器：`internal/handlers/wechat_handler.go` (430行)
- 路由：`internal/routes/routes.go` (第88-97行)

## 支持的支付方式
- ✅ JSAPI（小程序、公众号）
- ✅ Native（扫码支付）
- ✅ APP（移动应用）
- ✅ MWEB（H5支付）

