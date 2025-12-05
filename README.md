# Pay Gateway - 支付中心服务

一个基于Go语言开发的高性能、多渠道支付中心服务，支持国内外主流支付方式和应用内购买。

## 📖 文档导航

### 🎓 新手入门

👉 **[新手入门指南](docs/GETTING_STARTED.md)** - 三步快速开始

👉 **[文档中心](docs/README.md)** - 查看所有文档

### 🚀 快速开始（按支付方式）

- 💳 [微信支付](docs/guides/wechat/quick-start.md) - JSAPI、Native、APP、H5
- 💰 [支付宝](docs/guides/alipay/quick-start.md) - WAP、PAGE、周期扣款
- 🤖 [Google Play](docs/guides/google-play/quick-start.md) - 内购和订阅
- 🍎 [Apple](docs/guides/apple/quick-start.md) - 内购和订阅

### 📚 常用文档

- 🗺️ [代码位置速查](docs/references/code-map.md) - 快速找代码
- 📊 [功能总结](docs/references/all-payments.md) - 完整功能概览
- 📝 [配置示例](configs/config.toml.example) - 配置模板

## 🚀 项目特性

- **多支付渠道**: 支持微信支付、支付宝、Google Play、Apple Store
- **完整的支付流程**: 支持一次性购买和订阅管理
- **统一的接口设计**: 提供统一的API接口，屏蔽不同支付渠道的差异
- **国内支付**: 
  - 微信支付（JSAPI、Native、APP、H5）
  - 支付宝（手机网站支付、电脑网站支付、周期扣款）
- **海外支付**: 
  - Google Play（应用内购买、订阅）
  - Apple Store（应用内购买、订阅）
- **Webhook处理**: 实时处理各支付渠道的异步通知
- **高可用架构**: 支持分布式部署和负载均衡
- **完善的监控**: 内置健康检查和日志记录
- **容器化部署**: 支持Docker和Kubernetes部署
- **RESTful API**: 完整的REST API接口
- **数据库支持**: PostgreSQL + Redis缓存
- **安全可靠**: 完整的错误处理和事务管理

## 🏗️ 技术架构

### 技术栈
- **语言**: Go 1.21
- **Web框架**: Gin
- **数据库**: PostgreSQL + GORM
- **缓存**: Redis
- **认证**: JWT
- **日志**: Zap
- **容器化**: Docker + Docker Compose

### 系统架构
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Mobile App    │    │   Web Client    │    │  Admin Panel    │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │
                    ┌─────────────▼─────────────┐
                    │      Pay Gateway          │
                    │   (Gin HTTP Server)       │
                    └─────────────┬─────────────┘
                                  │
                    ┌─────────────▼─────────────┐
                    │     Business Logic        │
                    │  ┌─────────────────────┐  │
                    │  │  Payment Service    │  │
                    │  │ Subscription Service│  │
                    │  │  Wechat Service     │  │
                    │  │  Alipay Service     │  │
                    │  │  Apple Service      │  │
                    │  │ Google Play Service │  │
                    │  └─────────────────────┘  │
                    └─────────────┬─────────────┘
                                  │
                    ┌─────────────▼─────────────┐
                    │      Data Layer          │
                    │  ┌─────────────────────┐  │
                    │  │    PostgreSQL       │  │
                    │  │      Redis          │  │
                    │  └─────────────────────┘  │
                    └───────────────────────────┘
                                  │
                    ┌─────────────▼─────────────┐
                    │   External Services       │
                    │  ┌─────────────────────┐  │
                    │  │   Wechat Pay API    │  │
                    │  │   Alipay API        │  │
                    │  │   Apple Store API   │  │
                    │  │   Google Play API   │  │
                    │  │   Webhook Endpoints │  │
                    │  └─────────────────────┘  │
                    └───────────────────────────┘
```

## 📁 项目结构

```
pay-gateway/
├── cmd/
│   └── server/
│       └── main.go              # 应用程序入口
├── internal/
│   ├── config/
│   │   └── config.go            # 配置管理
│   ├── models/
│   │   ├── models.go            # 数据模型
│   │   └── payment_models.go    # 支付相关模型
│   ├── services/
│   │   ├── wechat_service.go         # 微信支付服务
│   │   ├── alipay_service.go         # 支付宝服务
│   │   ├── apple_service.go          # Apple Store服务
│   │   ├── google_play_service.go    # Google Play服务
│   │   ├── payment_service.go        # 支付服务
│   │   ├── subscription_service.go   # 订阅服务
│   │   └── payment_provider.go       # 统一支付接口
│   ├── handlers/
│   │   ├── handlers.go          # HTTP处理器
│   │   ├── wechat_handler.go    # 微信支付处理器
│   │   ├── alipay_handler.go    # 支付宝处理器
│   │   ├── apple_handler.go     # Apple处理器
│   │   ├── webhook.go           # Webhook处理器
│   │   └── apple_webhook.go     # Apple Webhook处理器
│   ├── middleware/
│   │   └── middleware.go        # 中间件
│   ├── routes/
│   │   └── routes.go            # 路由配置
│   ├── database/
│   │   └── database.go          # 数据库连接
│   └── cache/
│       └── redis.go             # Redis缓存
├── config/                      # 配置文件目录
├── docs/                        # API文档
├── scripts/                     # 脚本文件
├── docker-compose.yml           # Docker Compose配置
├── Dockerfile                   # Docker镜像构建
├── Makefile                     # 构建脚本
├── go.mod                       # Go模块文件
└── README.md                    # 项目文档
```

## 🚀 快速开始

### 环境要求

- Go 1.21+
- PostgreSQL 12+
- Redis 6+
- Docker & Docker Compose (可选)

### 本地开发

1. **克隆项目**
```bash
git clone <repository-url>
cd pay-gateway
```

2. **安装依赖**
```bash
go mod download
```

3. **配置环境变量**
```bash
cp .env.example .env
# 编辑 .env 文件，配置数据库和Redis连接信息
```

4. **启动依赖服务**
```bash
# 使用Docker Compose启动PostgreSQL和Redis
docker-compose up -d postgres redis
```

5. **运行应用**
```bash
# 开发模式运行
make dev

# 或者直接运行
go run cmd/server/main.go
```

### Docker部署

1. **构建并启动所有服务**
```bash
make compose-up
```

2. **查看服务状态**
```bash
make compose-logs
```

3. **停止服务**
```bash
make compose-down
```

## 📚 API文档

完整的API使用指南请查看：
- 📖 微信支付：[快速开始](docs/guides/wechat/quick-start.md)
- 📖 支付宝：[快速开始](docs/guides/alipay/quick-start.md) 和 [完整指南](docs/guides/alipay/complete-guide.md)
- 📖 Google Play：[快速开始](docs/guides/google-play/quick-start.md) 和 [完整指南](docs/guides/google-play/complete-guide.md)
- 📖 Apple：[快速开始](docs/guides/apple/quick-start.md) 和 [完整指南](docs/guides/apple/complete-guide.md)
- 📖 代码位置索引：[代码速查表](docs/references/code-map.md)

### 订单管理

#### 创建订单
```http
POST /api/v1/orders
Content-Type: application/json

{
  "user_id": 1,
  "product_id": "premium_upgrade",
  "type": "PURCHASE",
  "title": "Premium Upgrade",
  "description": "Unlock premium features",
  "quantity": 1,
  "currency": "USD",
  "total_amount": 999,
  "payment_method": "GOOGLE_PLAY",
  "developer_payload": "user_123"
}
```

#### 获取订单详情
```http
GET /api/v1/orders/{id}
```

#### 取消订单
```http
POST /api/v1/orders/{id}/cancel?reason=用户取消
```

### 支付管理

#### 处理支付
```http
POST /api/v1/payments/process
Content-Type: application/json

{
  "order_id": 1,
  "provider": "GOOGLE_PLAY",
  "purchase_token": "purchase_token_here",
  "developer_payload": "user_123"
}
```

### 支付宝周期扣款（订阅）

#### 创建周期扣款
```http
POST /api/v1/alipay/subscriptions
Content-Type: application/json

{
  "user_id": 1,
  "product_id": "premium_subscription",
  "product_name": "Premium会员月度订阅",
  "product_desc": "每月自动续费",
  "period_type": "MONTH",
  "period": 1,
  "single_amount": 2999,
  "personal_product_code": "CYCLE_PAY_AUTH_P",
  "sign_scene": "INDUSTRY|MEDICAL_INSURANCE"
}
```

#### 查询周期扣款
```http
GET /api/v1/alipay/subscriptions/query?out_request_no={out_request_no}
```

#### 取消周期扣款
```http
POST /api/v1/alipay/subscriptions/cancel
Content-Type: application/json

{
  "out_request_no": "SUB20240105...",
  "cancel_reason": "用户取消"
}
```

### 订阅管理

#### 创建订阅
```http
POST /api/v1/subscriptions
Content-Type: application/json

{
  "user_id": 1,
  "product_id": "monthly_subscription",
  "title": "Monthly Subscription",
  "description": "Monthly premium subscription",
  "currency": "USD",
  "price": 999,
  "period": "P1M",
  "developer_payload": "user_123"
}
```

#### 验证订阅
```http
GET /api/v1/subscriptions/{id}/validate
```

#### 取消订阅
```http
POST /api/v1/subscriptions/{id}/cancel?reason=用户取消
```

### Webhook接口

#### Google Play Webhook
```http
POST /webhook/google-play
Content-Type: application/json

{
  "message": {
    "data": "base64_encoded_data",
    "messageId": "message_id",
    "publishTime": "2023-01-01T00:00:00Z"
  },
  "subscription": "subscription_name"
}
```

### 微信支付接口

#### 创建微信订单
```http
POST /api/v1/wechat/orders
Content-Type: application/json

{
  "user_id": 1,
  "product_id": "premium_upgrade",
  "description": "Premium Upgrade",
  "detail": "Unlock premium features",
  "total_amount": 999,
  "trade_type": "JSAPI"
}
```

#### 创建JSAPI支付
```http
POST /api/v1/wechat/payments/jsapi/{order_no}
Content-Type: application/json

{
  "openid": "user_openid"
}
```

#### 创建Native支付
```http
POST /api/v1/wechat/payments/native/{order_no}
```

#### 查询订单
```http
GET /api/v1/wechat/orders/{order_no}
```

#### 退款
```http
POST /api/v1/wechat/refunds
Content-Type: application/json

{
  "order_no": "WX20240101120000xxxxx",
  "refund_amount": 999,
  "refund_reason": "用户申请退款"
}
```

### 系统接口

#### 健康检查
```http
GET /health
```

## 🔧 配置说明

### 环境变量

| 变量名 | 描述 | 默认值 |
|--------|------|--------|
| `SERVER_PORT` | 服务器端口 | `8080` |
| `SERVER_MODE` | 运行模式 | `release` |
| `DB_HOST` | 数据库主机 | `localhost` |
| `DB_PORT` | 数据库端口 | `5432` |
| `DB_USER` | 数据库用户 | `postgres` |
| `DB_PASSWORD` | 数据库密码 | - |
| `DB_NAME` | 数据库名称 | `billing` |
| `REDIS_HOST` | Redis主机 | `localhost` |
| `REDIS_PORT` | Redis端口 | `6379` |
| `GOOGLE_SERVICE_ACCOUNT_FILE` | Google服务账户文件路径 | `service-account.json` |
| `GOOGLE_PACKAGE_NAME` | Google Play包名 | `com.example.app` |
| `WECHAT_APP_ID` | 微信应用ID | - |
| `WECHAT_MCH_ID` | 微信商户号 | - |
| `WECHAT_APIV3_KEY` | 微信API v3密钥 | - |
| `WECHAT_SERIAL_NO` | 微信证书序列号 | - |
| `WECHAT_PRIVATE_KEY` | 微信商户私钥 | - |
| `WECHAT_NOTIFY_URL` | 微信支付通知URL | `https://your-domain.com/api/wechat/notify` |
| `ALIPAY_APP_ID` | 支付宝应用ID | - |
| `ALIPAY_PRIVATE_KEY` | 支付宝应用私钥 | - |
| `ALIPAY_NOTIFY_URL` | 支付宝通知URL | `https://your-domain.com/api/alipay/notify` |
| `APPLE_KEY_ID` | Apple私钥ID | - |
| `APPLE_ISSUER_ID` | Apple发行者ID | - |
| `APPLE_BUNDLE_ID` | iOS应用Bundle ID | `com.example.app` |
| `APPLE_PRIVATE_KEY` | Apple私钥内容 | - |
| `JWT_SECRET` | JWT密钥 | - |

### Google Play配置

1. **创建Google Play服务账户**
   - 访问 [Google Cloud Console](https://console.cloud.google.com/)
   - 创建新项目或选择现有项目
   - 启用 Android Developer API
   - 创建服务账户并下载JSON密钥文件

2. **配置服务账户文件**
   ```bash
   # 将下载的JSON文件放到config目录
   cp ~/Downloads/service-account.json config/
   ```

3. **设置包名**
   ```bash
   export GOOGLE_PACKAGE_NAME="com.yourcompany.yourapp"
   ```

## 🧪 测试

### 运行测试
```bash
# 运行所有测试
make test

# 运行测试并生成覆盖率报告
make test-coverage

# 运行性能测试
make bench
```

### 测试覆盖率
项目目标测试覆盖率达到80%以上。

## 📊 监控和日志

### 健康检查
服务提供健康检查端点：
```bash
curl http://localhost:8080/health
```

### 日志级别
- `DEBUG`: 开发环境详细日志
- `INFO`: 一般信息日志
- `WARN`: 警告日志
- `ERROR`: 错误日志

### 监控指标
- HTTP请求响应时间
- 数据库连接池状态
- Redis连接状态
- Google Play API调用成功率
- Webhook处理成功率

## 🔒 安全考虑

### 数据安全
- 所有敏感数据加密存储
- 使用HTTPS传输
- 实现请求限流
- 输入验证和SQL注入防护

### 认证授权
- JWT令牌认证
- API密钥验证
- 角色基础访问控制

### 隐私保护
- 用户数据匿名化
- 符合GDPR要求
- 数据保留策略

## 🚀 部署指南

### 生产环境部署

1. **准备环境**
```bash
# 创建生产环境配置
cp .env.example .env.production
# 编辑生产环境配置
```

2. **构建应用**
```bash
make build-linux
```

3. **部署到服务器**
```bash
# 上传构建文件到服务器
scp build/pay-gateway-linux-amd64 user@server:/opt/pay-gateway/

# 在服务器上启动服务
./pay-gateway-linux-amd64
```

### Kubernetes部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pay-gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: pay-gateway
  template:
    metadata:
      labels:
        app: pay-gateway
    spec:
      containers:
      - name: pay-gateway
        image: pay-gateway:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          value: "postgres-service"
        - name: REDIS_HOST
          value: "redis-service"
```

## 🤝 贡献指南

### 开发流程
1. Fork项目
2. 创建功能分支
3. 提交代码
4. 创建Pull Request

### 代码规范
- 遵循Go官方代码规范
- 使用gofmt格式化代码
- 编写单元测试
- 添加必要的注释

### 提交信息规范
```
type(scope): description

[optional body]

[optional footer]
```

类型包括：
- `feat`: 新功能
- `fix`: 修复bug
- `docs`: 文档更新
- `style`: 代码格式调整
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建过程或辅助工具的变动

## 📄 许可证

本项目采用 MIT 许可证。详情请参阅 [LICENSE](LICENSE) 文件。

## 🆘 支持

如果您遇到问题或有任何疑问，请：

1. 查看 [FAQ](docs/FAQ.md)
2. 搜索 [Issues](https://github.com/your-repo/issues)
3. 创建新的 Issue
4. 联系维护者

## 🔄 更新日志

### v1.0.0 (2024-01-01)
- 初始版本发布
- 支持Google Play支付
- 完整的订阅管理
- Webhook处理
- Docker支持

---

**注意**: 这是一个示例项目，实际使用时请根据具体需求进行调整和优化。
