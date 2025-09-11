# 支付宝支付接入完整实现方案

## 概述

本项目实现了完整的支付宝支付接入功能，支持多种支付方式和完整的支付流程，包括订单创建、支付处理、异步通知、退款等核心功能。

## 技术栈

- **后端框架**: Go + Gin
- **数据库**: PostgreSQL + GORM
- **缓存**: Redis
- **支付宝SDK**: smartwalle/alipay/v3
- **日志**: Zap

## 功能特性

### 核心功能
- ✅ 支付宝订单创建
- ✅ 手机网站支付（WAP）
- ✅ 电脑网站支付（PC Page）
- ✅ 异步通知处理
- ✅ 订单状态查询
- ✅ 退款处理
- ✅ 签名验证
- ✅ 证书模式支持

### 安全特性
- ✅ RSA签名验证
- ✅ 证书模式认证
- ✅ 异步通知签名验证
- ✅ 请求参数校验
- ✅ 数据库事务处理

## 项目结构

```
/Users/huangfobo/workspace/pay-gateway/
├── cmd/server/           # 应用入口
├── internal/             # 核心应用逻辑
│   ├── cache/           # Redis缓存实现
│   ├── config/          # 配置管理
│   ├── database/        # 数据库连接和迁移
│   ├── handlers/        # HTTP请求处理器
│   │   ├── handlers.go  # 通用处理器
│   │   ├── alipay_handler.go  # 支付宝专用处理器
│   │   └── webhook.go   # Webhook处理器
│   ├── middleware/      # HTTP中间件
│   ├── models/          # 数据模型和实体
│   ├── routes/          # 路由定义
│   └── services/        # 业务逻辑服务
│       ├── payment_service.go    # 支付服务
│       ├── alipay_service.go     # 支付宝服务
│       └── google_play_service.go # Google Play服务
├── docker-compose.yml   # 容器编排
├── Dockerfile          # 容器构建配置
├── go.mod              # Go模块依赖
├── .env.example        # 环境变量示例
└── README.md           # 项目文档
```

## API接口

### 支付宝订单管理

#### 1. 创建支付宝订单
```http
POST /api/v1/alipay/orders
Content-Type: application/json

{
  "user_id": 1,
  "product_id": "product_123",
  "subject": "商品标题",
  "body": "商品描述",
  "total_amount": 1000  // 单位为分，即10元
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "order_id": 123,
    "order_no": "ORD2024010112000012345678",
    "total_amount": 1000,
    "subject": "商品标题",
    "description": "商品描述"
  }
}
```

#### 2. 创建支付宝支付
```http
POST /api/v1/alipay/payments
Content-Type: application/json

{
  "order_no": "ORD2024010112000012345678",
  "pay_type": "WAP"  // WAP: 手机网站支付，PAGE: 电脑网站支付
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "payment_url": "https://openapi.alipay.com/gateway.do?...",
    "order_no": "ORD2024010112000012345678"
  }
}
```

#### 3. 查询支付宝订单
```http
GET /api/v1/alipay/orders/query?order_no=ORD2024010112000012345678
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "order_no": "ORD2024010112000012345678",
    "trade_no": "2024010122001234567890",
    "trade_status": "TRADE_SUCCESS",
    "total_amount": 1000,
    "payment_status": "COMPLETED",
    "paid_at": "2024-01-01 12:05:30"
  }
}
```

#### 4. 支付宝退款
```http
POST /api/v1/alipay/refunds
Content-Type: application/json

{
  "order_no": "ORD2024010112000012345678",
  "refund_amount": 500,  // 单位为分，即5元
  "refund_reason": "商品质量问题"
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "refund_request_no": "REFORD2024010112000012345678123456",
    "refund_amount": 500,
    "refund_status": "REFUND_SUCCESS",
    "refund_at": "2024-01-02 10:30:15"
  }
}
```

### Webhook通知

#### 支付宝异步通知
```http
POST /webhook/alipay
Content-Type: application/x-www-form-urlencoded

# 支付宝会将支付结果以表单形式POST到该接口
# 重要：必须返回 "success"，否则支付宝会持续重试
```

## 配置说明

### 环境变量配置

复制 `.env.example` 为 `.env` 并配置以下参数：

```bash
# 支付宝配置
ALIPAY_APP_ID=your_alipay_app_id
ALIPAY_PRIVATE_KEY=-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA...
-----END RSA PRIVATE KEY-----
ALIPAY_IS_PRODUCTION=false  # 是否生产环境
ALIPAY_NOTIFY_URL=https://your-domain.com/webhook/alipay
ALIPAY_RETURN_URL=https://your-domain.com/payment/return
ALIPAY_CERT_MODE=false  # 是否使用证书模式
ALIPAY_APP_CERT_PATH=certs/appCertPublicKey.crt
ALIPAY_ROOT_CERT_PATH=certs/alipayRootCert.crt
ALIPAY_PUBLIC_CERT_PATH=certs/alipayCertPublicKey_RSA2.crt
```

### 支付宝开放平台配置步骤

1. **创建应用**
   - 登录 [支付宝开放平台](https://open.alipay.com/)
   - 创建新的应用并获取 `AppID`

2. **配置密钥**
   - 使用支付宝提供的密钥生成工具生成RSA密钥对
   - 将公钥上传到支付宝开放平台
   - 配置私钥到环境变量 `ALIPAY_PRIVATE_KEY`

3. **配置接口加签方式**
   - 选择RSA2加签方式
   - 上传应用公钥

4. **配置授权回调地址**
   - 设置支付成功后的回调地址

5. **开通产品功能**
   - 开通「电脑网站支付」和「手机网站支付」产品

## 支付流程

### 1. 手机网站支付流程（WAP）

```mermaid
sequenceDiagram
    participant 用户
    participant 前端
    participant 后端
    participant 支付宝
    
    用户->>前端: 选择商品并下单
    前端->>后端: POST /api/v1/alipay/orders
    后端->>数据库: 创建订单
    后端->>前端: 返回订单信息
    前端->>后端: POST /api/v1/alipay/payments (pay_type=WAP)
    后端->>支付宝: 创建WAP支付订单
    后端->>前端: 返回支付URL
    前端->>用户: 跳转到支付宝支付页面
    用户->>支付宝: 完成支付
    支付宝->>后端: POST /webhook/alipay (异步通知)
    后端->>数据库: 更新订单状态
    后端->>支付宝: 返回 "success"
    支付宝->>前端: 重定向到return_url
```

### 2. 电脑网站支付流程（PAGE）

```mermaid
sequenceDiagram
    participant 用户
    participant 前端
    participant 后端
    participant 支付宝
    
    用户->>前端: 选择商品并下单
    前端->>后端: POST /api/v1/alipay/orders
    后端->>数据库: 创建订单
    后端->>前端: 返回订单信息
    前端->>后端: POST /api/v1/alipay/payments (pay_type=PAGE)
    后端->>支付宝: 创建PC支付订单
    后端->>前端: 返回支付URL
    前端->>用户: 在新标签页打开支付页面
    用户->>支付宝: 扫码或登录完成支付
    支付宝->>后端: POST /webhook/alipay (异步通知)
    后端->>数据库: 更新订单状态
    后端->>支付宝: 返回 "success"
    支付宝->>前端: 重定向到return_url
```

## 数据库设计

### 订单表 (orders)
```sql
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    order_no VARCHAR(32) UNIQUE NOT NULL,
    user_id BIGINT NOT NULL,
    product_id VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL,
    title VARCHAR(200) NOT NULL,
    description VARCHAR(500),
    quantity INTEGER NOT NULL DEFAULT 1,
    currency VARCHAR(3) NOT NULL,
    total_amount BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL,
    payment_method VARCHAR(20) NOT NULL,
    payment_status VARCHAR(20) NOT NULL,
    paid_at TIMESTAMP,
    expired_at TIMESTAMP,
    refund_at TIMESTAMP,
    refund_reason VARCHAR(500),
    refund_amount BIGINT,
    developer_payload VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
```

### 支付宝支付记录表 (alipay_payments)
```sql
CREATE TABLE alipay_payments (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT UNIQUE NOT NULL REFERENCES orders(id),
    out_trade_no VARCHAR(64) UNIQUE NOT NULL,
    trade_no VARCHAR(64),
    buyer_user_id VARCHAR(64),
    buyer_logon_id VARCHAR(100),
    total_amount VARCHAR(20) NOT NULL,
    subject VARCHAR(256) NOT NULL,
    body VARCHAR(400),
    trade_status VARCHAR(32),
    app_id VARCHAR(32),
    timeout_express VARCHAR(32),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 支付交易记录表 (payment_transactions)
```sql
CREATE TABLE payment_transactions (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    transaction_id VARCHAR(100) UNIQUE NOT NULL,
    provider VARCHAR(20) NOT NULL,
    type VARCHAR(50) NOT NULL,
    amount BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(20) NOT NULL,
    provider_data JSONB,
    error_code VARCHAR(50),
    error_message VARCHAR(500),
    processed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## 安全考虑

### 1. 签名验证
- 所有支付宝异步通知都进行签名验证
- 使用RSA2算法验证签名有效性
- 防止伪造通知

### 2. 证书模式
- 生产环境建议使用证书模式
- 提供更高的安全保障
- 支持公钥证书验证

### 3. 幂等性处理
- 订单号全局唯一
- 重复通知处理
- 事务保证数据一致性

### 4. 数据加密
- 敏感数据加密存储
- 传输过程使用HTTPS
- 私钥安全存储

## 错误处理

### 常见错误码

| 错误码 | 描述 | 处理方式 |
|--------|------|----------|
| 40001 | 请求参数错误 | 检查请求参数格式 |
| 40002 | 订单不存在 | 确认订单号是否正确 |
| 40003 | 订单状态错误 | 检查订单当前状态 |
| 50001 | 支付宝服务错误 | 稍后重试或联系支付宝技术支持 |
| 50002 | 签名验证失败 | 检查密钥配置和签名算法 |

## 测试

### 沙箱环境测试

1. 设置 `ALIPAY_IS_PRODUCTION=false`
2. 使用支付宝沙箱环境AppID
3. 使用沙箱环境密钥
4. 使用沙箱支付宝账号进行测试

### 测试用例

```bash
# 1. 创建订单
curl -X POST http://localhost:8080/api/v1/alipay/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "product_id": "test_product",
    "subject": "测试商品",
    "body": "这是一个测试商品",
    "total_amount": 100
  }'

# 2. 创建支付
curl -X POST http://localhost:8080/api/v1/alipay/payments \
  -H "Content-Type: application/json" \
  -d '{
    "order_no": "返回的订单号",
    "pay_type": "WAP"
  }'

# 3. 查询订单
curl "http://localhost:8080/api/v1/alipay/orders/query?order_no=返回的订单号"
```

## 部署建议

### 1. 环境分离
- 开发环境使用沙箱配置
- 生产环境使用正式配置
- 密钥分开管理

### 2. 高可用部署
- 多实例部署
- 数据库主从配置
- Redis集群配置

### 3. 监控告警
- 支付成功率监控
- 异常订单告警
- 系统性能监控

### 4. 日志管理
- 详细日志记录
- 日志收集分析
- 异常日志告警

## 常见问题

### Q: 支付成功后订单状态没有更新？
A: 检查以下几点：
1. Webhook地址是否正确配置
2. 服务器是否能接收外部POST请求
3. 签名验证是否通过
4. 返回格式是否为纯文本 "success"

### Q: 签名验证失败？
A: 检查以下几点：
1. 私钥是否正确配置
2. 支付宝公钥是否正确
3. 参数排序是否正确
4. 编码格式是否为UTF-8

### Q: 支付页面打不开？
A: 检查以下几点：
1. 支付URL是否完整
2. 订单参数是否正确
3. 支付宝服务是否正常
4. 网络连接是否正常

## 更新日志

### v1.0.0 (2024-01-01)
- 初始版本发布
- 支持WAP和PC网页支付
- 完整的支付流程实现
- 异步通知处理
- 退款功能

## 技术支持

如有问题，请检查：
1. [支付宝开放平台文档](https://opendocs.alipay.com/)
2. [支付宝SDK文档](https://github.com/smartwalle/alipay)
3. 查看应用日志获取详细信息

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件