# 支付集成文档

本文档描述Pay Gateway支持的所有支付方式的集成情况和使用说明。

## 支持的支付方式

### 1. 微信支付 (WeChat Pay)

**状态**: ✅ 已实现

**支持的支付方式**:
- JSAPI支付（小程序、公众号）
- Native支付（扫码支付）
- APP支付
- H5支付（手机网站支付）

**功能特性**:
- ✅ 创建订单
- ✅ 发起支付
- ✅ 查询订单
- ✅ 退款
- ✅ 关闭订单
- ✅ 异步通知处理

**API端点**:
```
POST   /api/v1/wechat/orders                      # 创建订单
POST   /api/v1/wechat/payments/jsapi/:order_no    # JSAPI支付
POST   /api/v1/wechat/payments/native/:order_no   # Native支付
POST   /api/v1/wechat/payments/app/:order_no      # APP支付
POST   /api/v1/wechat/payments/h5/:order_no       # H5支付
GET    /api/v1/wechat/orders/:order_no            # 查询订单
POST   /api/v1/wechat/refunds                     # 退款
POST   /api/v1/wechat/orders/:order_no/close      # 关闭订单
POST   /webhook/wechat/notify                     # 异步通知
```

**配置说明**:
```toml
[wechat]
app_id = "wx1234567890abcdef"
mch_id = "1234567890"
apiv3_key = "your_apiv3_key"
serial_no = "certificate_serial_number"
private_key = "merchant_private_key_content"
notify_url = "https://your-domain.com/webhook/wechat/notify"
```

**使用示例**:
```bash
# 1. 创建订单
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

# 2. JSAPI支付
curl -X POST http://localhost:8080/api/v1/wechat/payments/jsapi/WX20240101120000xxxxx \
  -H "Content-Type: application/json" \
  -d '{
    "openid": "o6_bmjrPTlm6_2sgVt7hMZOPfL2M"
  }'
```

---

### 2. 支付宝 (Alipay)

**状态**: ✅ 已实现

**支持的支付方式**:
- 手机网站支付 (Wap)
- 电脑网站支付 (Page)
- 周期扣款（订阅）

**功能特性**:
- ✅ 创建订单
- ✅ 发起支付
- ✅ 查询订单
- ✅ 退款
- ✅ 异步通知处理
- ✅ 周期扣款（订阅）

**API端点**:
```
POST   /api/v1/alipay/orders         # 创建订单
POST   /api/v1/alipay/payments       # 创建支付
GET    /api/v1/alipay/orders/query   # 查询订单
POST   /api/v1/alipay/refunds        # 退款
POST   /webhook/alipay               # 异步通知
```

**配置说明**:
```toml
[alipay]
app_id = "2021001234567890"
private_key = "your_alipay_private_key"
is_production = false
notify_url = "https://your-domain.com/webhook/alipay"
return_url = "https://your-domain.com/payment/return"
cert_mode = false
```

**使用示例**:
```bash
# 1. 创建订单
curl -X POST http://localhost:8080/api/v1/alipay/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "product_id": "premium_month",
    "subject": "Premium会员月卡",
    "body": "解锁所有高级功能",
    "total_amount": 2999
  }'

# 2. 创建手机网站支付
curl -X POST http://localhost:8080/api/v1/alipay/payments \
  -H "Content-Type: application/json" \
  -d '{
    "order_no": "ORD20240101120000xxxxx",
    "payment_type": "wap"
  }'
```

---

### 3. Apple Store (Apple In-App Purchase)

**状态**: ✅ 已实现

**支持的支付方式**:
- 应用内购买（消耗型、非消耗型、非续期订阅）
- 自动续期订阅

**功能特性**:
- ✅ 收据验证
- ✅ 交易验证（App Store Server API）
- ✅ 交易历史查询
- ✅ 订阅状态查询
- ✅ Server-to-Server通知处理

**API端点**:
```
POST   /api/v1/apple/verify-receipt                        # 验证收据
POST   /api/v1/apple/verify-transaction                    # 验证交易
POST   /api/v1/apple/validate-receipt                      # 验证收据（简化）
GET    /api/v1/apple/transactions/:id/history              # 交易历史
GET    /api/v1/apple/subscriptions/:id/status              # 订阅状态
POST   /webhook/apple                                      # Server通知
```

**配置说明**:
```toml
[apple]
key_id = "ABC123DEFG"
issuer_id = "12345678-1234-1234-1234-123456789012"
bundle_id = "com.example.app"
private_key = "your_apple_private_key_p8_content"
sandbox = false
webhook_secret = "your_apple_webhook_secret"
```

**使用示例**:
```bash
# 验证收据
curl -X POST http://localhost:8080/api/v1/apple/verify-receipt \
  -H "Content-Type: application/json" \
  -d '{
    "receipt_data": "base64_encoded_receipt_data",
    "order_id": 123
  }'

# 验证交易
curl -X POST http://localhost:8080/api/v1/apple/verify-transaction \
  -H "Content-Type: application/json" \
  -d '{
    "transaction_id": "1000000123456789",
    "order_id": 123
  }'
```

---

### 4. Google Play (Google In-App Billing)

**状态**: ✅ 已实现

**支持的支付方式**:
- 应用内购买（一次性产品、消耗型产品）
- 订阅

**功能特性**:
- ✅ 购买验证
- ✅ 订阅验证
- ✅ 确认购买
- ✅ 消费购买
- ✅ Real-time Developer Notifications处理

**API端点**:
```
POST   /api/v1/payments/process           # 处理支付（包含验证）
POST   /webhook/google-play                # 实时通知
```

**配置说明**:
```toml
[google]
service_account_file = "configs/google-service-account.json"
package_name = "com.example.app"
webhook_secret = "your_google_webhook_secret"
```

**使用示例**:
```bash
# 处理购买验证
curl -X POST http://localhost:8080/api/v1/payments/process \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": 123,
    "provider": "GOOGLE_PLAY",
    "purchase_token": "purchase_token_from_client",
    "developer_payload": "user_123"
  }'
```

---

## 统一支付接口设计

为了更好地整合不同的支付方式，我们设计了统一的支付服务抽象层。

### 核心接口

```go
type PaymentProvider interface {
    // 获取支付提供商名称
    GetProviderName() string
    
    // 创建订单
    CreateOrder(ctx context.Context, req *UnifiedOrderRequest) (*UnifiedOrderResponse, error)
    
    // 创建支付
    CreatePayment(ctx context.Context, orderNo string, paymentReq interface{}) (interface{}, error)
    
    // 查询订单状态
    QueryOrder(ctx context.Context, orderNo string) (*UnifiedOrderQueryResponse, error)
    
    // 退款
    Refund(ctx context.Context, req *UnifiedRefundRequest) (*UnifiedRefundResponse, error)
    
    // 关闭订单
    CloseOrder(ctx context.Context, orderNo string) error
    
    // 处理支付通知
    HandleNotify(ctx context.Context, notifyData interface{}) error
    
    // 验证支付（用于客户端验证）
    VerifyPayment(ctx context.Context, verifyReq interface{}) (interface{}, error)
}
```

### 适配器模式

每个支付方式都通过适配器模式实现统一接口：

- `WechatPaymentAdapter` - 微信支付适配器
- `AlipayPaymentAdapter` - 支付宝适配器
- `ApplePaymentAdapter` - Apple支付适配器
- `GooglePlayPaymentAdapter` - Google Play适配器

### 支付提供商注册表

使用注册表模式管理所有支付提供商：

```go
registry := NewPaymentProviderRegistry()

// 注册支付提供商
registry.Register(models.PaymentProviderWeChat, NewWechatPaymentAdapter(wechatService))
registry.Register(models.PaymentProviderAlipay, NewAlipayPaymentAdapter(alipayService))
registry.Register(models.PaymentProviderAppleStore, NewApplePaymentAdapter(appleService))
registry.Register(models.PaymentProviderGooglePlay, NewGooglePlayPaymentAdapter(googleService))

// 获取支付提供商
provider, err := registry.GetProvider(models.PaymentProviderWeChat)
```

---

## 数据模型

### 订单模型

所有支付方式共享统一的订单模型：

```go
type Order struct {
    ID            uint
    OrderNo       string           // 系统订单号
    UserID        uint             // 用户ID
    ProductID     string           // 商品ID
    Type          OrderType        // 订单类型（购买/订阅）
    Title         string           // 商品标题
    Description   string           // 商品描述
    Quantity      int              // 数量
    Currency      string           // 货币代码
    TotalAmount   int64            // 总金额（微单位）
    Status        OrderStatus      // 订单状态
    PaymentMethod PaymentMethod    // 支付方式
    PaymentStatus PaymentStatus    // 支付状态
    PaidAt        *time.Time       // 支付时间
    // ... 其他字段
}
```

### 支付详情模型

每个支付方式有自己的详情模型：

- `WechatPayment` - 微信支付详情
- `AlipayPayment` - 支付宝支付详情
- `ApplePayment` - Apple支付详情
- `GooglePayment` - Google支付详情

---

## 最佳实践

### 1. 环境隔离

- 开发环境使用沙盒/测试环境
- 生产环境使用正式环境
- 通过配置文件或环境变量区分

### 2. 安全措施

- 私钥和密钥使用环境变量或安全存储
- 验证所有异步通知的签名
- 使用HTTPS传输
- 实现请求限流和防重放攻击

### 3. 错误处理

- 记录所有支付相关的错误日志
- 实现重试机制
- 提供友好的错误提示

### 4. 监控告警

- 监控支付成功率
- 监控异步通知处理成功率
- 设置异常告警

### 5. 测试

- 单元测试覆盖核心逻辑
- 集成测试验证支付流程
- 使用沙盒环境进行端到端测试

---

## 常见问题

### Q1: 如何选择支付方式？

**A**: 根据目标用户和应用场景选择：
- 国内用户：微信支付、支付宝
- iOS应用内购买：Apple In-App Purchase
- Android应用内购买：Google Play Billing
- 海外用户：考虑Stripe、PayPal等（待实现）

### Q2: 如何处理支付通知？

**A**: 所有支付方式都提供异步通知接口：
1. 验证通知签名
2. 检查订单是否已处理（防重复）
3. 更新订单状态
4. 返回成功响应

### Q3: 订单号如何生成？

**A**: 不同支付方式使用不同前缀：
- 微信支付：WX + 时间戳 + 随机字符串
- 支付宝：ORD + 时间戳 + 随机字符串
- 系统内部统一使用`OrderNo`字段

### Q4: 如何测试支付功能？

**A**: 
1. 使用各支付平台的沙盒/测试环境
2. 配置测试商户号和应用
3. 使用测试账号进行支付
4. 验证异步通知处理

---

## 更新日志

### v1.0.0 (2024-12-05)
- ✅ 实现微信支付（JSAPI、Native、APP、H5）
- ✅ 实现支付宝支付（Wap、Page、周期扣款）
- ✅ 实现Apple In-App Purchase（收据验证、交易验证）
- ✅ 实现Google Play Billing（购买验证、订阅验证）
- ✅ 创建统一支付接口抽象层
- ✅ 实现支付提供商注册表
- ✅ 完善数据模型和数据库迁移

---

## 贡献指南

欢迎贡献代码，请遵循以下步骤：

1. Fork项目
2. 创建功能分支
3. 实现新功能或修复bug
4. 编写测试
5. 提交Pull Request

---

## 技术支持

如有问题，请：
1. 查看本文档
2. 查看[FAQ](FAQ.md)
3. 提交Issue
4. 联系技术支持团队
