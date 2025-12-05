# Google Play 快速开始

本文档提供Google Play内购和订阅功能的快速参考。

## API端点

```
POST   /api/v1/payments/process      # 统一支付处理（验证）
POST   /webhook/google-play          # Webhook通知
```

## 使用示例

### 验证购买
```bash
curl -X POST http://localhost:8080/api/v1/payments/process \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": 123,
    "provider": "GOOGLE_PLAY",
    "purchase_token": "purchase_token_from_google",
    "product_id": "premium_upgrade",
    "developer_payload": "user_123"
  }'
```

### 验证订阅
```bash
curl -X POST http://localhost:8080/api/v1/payments/process \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": 124,
    "provider": "GOOGLE_PLAY",
    "purchase_token": "subscription_token",
    "subscription_id": "premium_monthly",
    "developer_payload": "user_123"
  }'
```

## 配置示例
```toml
[google]
service_account_file = "configs/google-service-account.json"
package_name = "com.example.app"
webhook_secret = "your_webhook_secret"
```

## 核心功能
- ✅ 验证购买 (`VerifyPurchase`)
- ✅ 验证订阅 (`VerifySubscription`)
- ✅ 确认购买 (`AcknowledgePurchase`)
- ✅ 确认订阅 (`AcknowledgeSubscription`)
- ✅ 消费购买 (`ConsumePurchase`)

## 代码位置
- 服务：`internal/services/google_play_service.go` (392行)
- Webhook：`internal/handlers/webhook.go`

## 重要提示
⚠️ **购买后3天内必须确认，否则会自动退款！**

详细文档请查看 [Google Play完整指南](complete-guide.md)

