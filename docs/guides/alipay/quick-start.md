# 支付宝快速开始

本文档提供支付宝支付和周期扣款功能的快速参考。

## API端点

### 支付宝支付
```
POST   /api/v1/alipay/orders          # 创建订单
POST   /api/v1/alipay/payments        # 创建支付
GET    /api/v1/alipay/orders/query    # 查询订单
POST   /api/v1/alipay/refunds         # 退款
POST   /webhook/alipay                # 异步通知
```

### 支付宝周期扣款（订阅）
```
POST   /api/v1/alipay/subscriptions         # 创建周期扣款
GET    /api/v1/alipay/subscriptions/query   # 查询状态
POST   /api/v1/alipay/subscriptions/cancel  # 取消签约
```

## 使用示例

### 手机网站支付
```bash
# 1. 创建订单
curl -X POST http://localhost:8080/api/v1/alipay/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id": 1, "product_id": "premium_month", "subject": "Premium会员", "total_amount": 2999}'

# 2. 创建支付
curl -X POST http://localhost:8080/api/v1/alipay/payments \
  -d '{"order_no": "ORD...", "pay_type": "WAP"}'
```

### 周期扣款（订阅）
```bash
# 1. 创建签约
curl -X POST http://localhost:8080/api/v1/alipay/subscriptions \
  -d '{
    "user_id": 1,
    "product_id": "premium_sub",
    "product_name": "月度订阅",
    "period_type": "MONTH",
    "period": 1,
    "single_amount": 2999,
    "personal_product_code": "CYCLE_PAY_AUTH_P",
    "sign_scene": "INDUSTRY|MEDICAL_INSURANCE"
  }'

# 2. 查询状态
curl "http://localhost:8080/api/v1/alipay/subscriptions/query?out_request_no=SUB..."
```

## 配置示例
```toml
[alipay]
app_id = "2021001234567890"
private_key = "your_private_key"
is_production = false
notify_url = "https://your-domain.com/webhook/alipay"
```

## 代码位置
- 服务：`internal/services/alipay_service.go`
- 处理器：`internal/handlers/alipay_handler.go`
- 路由：`internal/routes/routes.go` (第67-79行)

详细文档请查看 [支付宝完整指南](complete-guide.md)

