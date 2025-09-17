# 支付中心服务 - 三种支付方式整合完成

## 概述

支付中心服务已成功整合三种支付方式：
1. **Google Play 支付** - 应用内购买和订阅
2. **Apple Store 支付** - iOS应用内购买和订阅
3. **支付宝支付** - 网页和移动端支付

## 架构设计

### 统一支付接口
- `PaymentService` - 统一支付服务接口，支持所有支付方式
- `SubscriptionService` - 统一订阅管理服务接口
- 统一的订单管理系统，支持多种支付方式混合使用

### 数据库模型
- `Order` - 统一订单模型，包含支付方式字段
- `PaymentTransaction` - 统一交易记录模型
- `GooglePayment` - Google Play支付详情
- `ApplePayment` - Apple Store支付详情
- `AlipayPayment` - 支付宝支付详情

## API端点

### 通用支付API
```
POST /api/v1/orders                    # 创建订单（支持所有支付方式）
GET  /api/v1/orders/:id                # 获取订单详情
POST /api/v1/payments/process          # 处理支付
```

### 专用支付API
```
# Google Play
POST /webhook/google-play              # Google Play Webhook

# Apple Store
POST /api/v1/apple/verify-receipt      # 验证Apple收据
POST /api/v1/apple/verify-transaction  # 验证Apple交易
POST /webhook/apple                    # Apple Webhook

# 支付宝
POST /api/v1/alipay/orders             # 创建支付宝订单
POST /api/v1/alipay/payments           # 创建支付宝支付
POST /webhook/alipay                   # 支付宝 Webhook
```

### 订阅管理API
```
POST /api/v1/subscriptions             # 创建订阅（支持所有支付方式）
GET  /api/v1/subscriptions/:id         # 获取订阅详情
GET  /api/v1/subscriptions/:id/validate # 验证订阅
```

## 配置说明

### 环境变量配置
所有支付方式都支持通过环境变量进行配置：

#### Google Play配置
```bash
GOOGLE_SERVICE_ACCOUNT_FILE=service-account.json
GOOGLE_PACKAGE_NAME=com.example.app
GOOGLE_WEBHOOK_SECRET=your_webhook_secret
```

#### Apple Store配置
```bash
APPLE_KEY_ID=your_key_id
APPLE_ISSUER_ID=your_issuer_id
APPLE_BUNDLE_ID=com.example.app
APPLE_PRIVATE_KEY=your_private_key_content
APPLE_SANDBOX=true
APPLE_WEBHOOK_SECRET=your_webhook_secret
```

#### 支付宝配置
```bash
ALIPAY_APP_ID=your_app_id
ALIPAY_PRIVATE_KEY=your_private_key
ALIPAY_IS_PRODUCTION=false
ALIPAY_NOTIFY_URL=https://your-domain.com/api/alipay/notify
ALIPAY_RETURN_URL=https://your-domain.com/payment/return
```

### 配置文件示例
详见 `configs/config.example.toml` 文件

## 使用方法

### 1. 创建订单
```json
POST /api/v1/orders
{
  "user_id": 1,
  "product_id": "product_123",
  "type": "PURCHASE",
  "title": "商品标题",
  "currency": "CNY",
  "total_amount": 1000,
  "payment_method": "ALIPAY"  // GOOGLE_PLAY, APPLE_STORE, ALIPAY
}
```

### 2. 处理支付
```json
POST /api/v1/payments/process
{
  "order_id": 1,
  "provider": "ALIPAY",  // GOOGLE_PLAY, APPLE_STORE, ALIPAY
  "purchase_token": "purchase_token_or_receipt_data"
}
```

### 3. 创建订阅
```json
POST /api/v1/subscriptions
{
  "user_id": 1,
  "product_id": "subscription_123",
  "title": "订阅服务",
  "currency": "CNY",
  "price": 9900,
  "period": "P1M",
  "payment_method": "GOOGLE_PLAY"  // GOOGLE_PLAY, APPLE_STORE, ALIPAY
}
```

## 支付提供商特定处理

### Google Play
- 支持应用内购买和订阅
- 自动处理购买确认和消费
- 支持订阅生命周期管理
- Webhook自动处理状态变更

### Apple Store
- 支持收据验证和交易验证
- 支持订阅状态查询
- 支持交易历史查询
- Webhook自动处理状态变更

### 支付宝
- 支持网页支付和扫码支付
- 支持订单查询和退款
- 支持异步通知处理
- Webhook自动处理支付状态

## 状态管理

### 订单状态
- `CREATED` - 订单已创建
- `PAID` - 订单已支付
- `DELIVERED` - 订单已交付
- `CANCELLED` - 订单已取消
- `REFUNDED` - 订单已退款

### 支付状态
- `PENDING` - 待支付
- `COMPLETED` - 支付完成
- `FAILED` - 支付失败
- `CANCELLED` - 支付取消
- `REFUNDED` - 已退款
- `EXPIRED` - 已过期

### 订阅状态
- `PENDING` - 待处理
- `ACTIVE` - 活跃中
- `CANCELLED` - 已取消
- `EXPIRED` - 已过期
- `IN_GRACE_PERIOD` - 宽限期中
- `ON_HOLD` - 暂停中
- `PAUSED` - 已暂停

## 错误处理

所有API都遵循统一的错误响应格式：
```json
{
  "code": 400,
  "message": "错误描述",
  "error": "详细错误信息"
}
```

## 安全特性

- JWT身份认证
- Webhook签名验证
- HTTPS强制使用
- 请求频率限制
- SQL注入防护
- 输入参数验证

## 监控和日志

- 结构化日志记录（Zap）
- 请求链路追踪
- 性能监控
- 错误告警
- 健康检查端点

## 部署说明

1. 配置环境变量或配置文件
2. 确保数据库和Redis正常运行
3. 运行数据库迁移
4. 启动服务

```bash
# 使用Docker Compose
docker-compose up -d

# 或直接运行
go run cmd/server/main.go
```

## 测试

```bash
# 运行单元测试
go test ./...

# 运行集成测试
go test ./test/...

# 代码质量检查
go fmt ./...
go vet ./...
```

## 扩展性

该架构支持轻松添加新的支付方式：
1. 创建新的支付服务实现
2. 添加相应的数据库模型
3. 更新配置支持
4. 添加API端点
5. 更新支付处理逻辑

## 总结

支付中心服务现已完整支持Google Play、Apple Store和支付宝三种主流支付方式，提供统一的API接口和一致的开发体验。所有支付方式都经过良好集成，支持完整的支付流程、订阅管理和Webhook处理。