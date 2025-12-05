# Apple 快速开始

本文档提供Apple内购和订阅功能的快速参考。

## API端点

```
POST   /api/v1/apple/verify-receipt              # 验证收据（旧版）
POST   /api/v1/apple/verify-transaction          # 验证交易（推荐）
POST   /api/v1/apple/validate-receipt            # 验证收据（简化）
GET    /api/v1/apple/transactions/:id/history    # 获取交易历史
GET    /api/v1/apple/subscriptions/:id/status    # 获取订阅状态
POST   /webhook/apple                            # Webhook通知
```

## 使用示例

### 验证交易（推荐）
```bash
curl -X POST http://localhost:8080/api/v1/apple/verify-transaction \
  -H "Content-Type: application/json" \
  -d '{
    "transaction_id": "1000000123456789",
    "order_id": 123
  }'
```

### 验证收据（旧版）
```bash
curl -X POST http://localhost:8080/api/v1/apple/verify-receipt \
  -H "Content-Type: application/json" \
  -d '{
    "receipt_data": "base64_encoded_receipt_data",
    "order_id": 123
  }'
```

### 获取订阅状态
```bash
curl -X GET http://localhost:8080/api/v1/apple/subscriptions/1000000123456789/status
```

## 配置示例
```toml
[apple]
key_id = "ABC123DEFG"
issuer_id = "12345678-1234-1234-1234-123456789012"
bundle_id = "com.example.app"
private_key = "your_p8_key_content"
sandbox = false
```

## 核心功能
- ✅ 验证收据 (`VerifyPurchase`)
- ✅ 验证交易 (`VerifyTransaction`) - **推荐**
- ✅ 获取交易历史 (`GetTransactionHistory`)
- ✅ 保存支付信息 (`SaveApplePayment`)

## 代码位置
- 服务：`internal/services/apple_service.go` (442行)
- 处理器：`internal/handlers/apple_handler.go` (304行)
- Webhook：`internal/handlers/apple_webhook.go`

## 重要提示
💡 **推荐使用 App Store Server API (`VerifyTransaction`)，更准确更快！**

详细文档请查看 [Apple完整指南](complete-guide.md)

