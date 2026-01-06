# Pay Gateway - 多渠道支付中心

一个基于 Go 语言开发的高性能支付网关服务，支持国内外主流支付方式。

## ✨ 支持的支付方式

| 支付方式 | 一次性购买 | 订阅/周期扣款 | Webhook | 文档 |
|---------|----------|-------------|---------|-----|
| 🤖 Google Play | ✅ | ✅ | ✅ | [查看](docs/google-play/) |
| 🍎 Apple Store | ✅ | ✅ | ✅ | [查看](docs/apple/) |
| 💰 支付宝 | ✅ | ✅ | ✅ | [查看](docs/alipay/) |
| 💳 微信支付 | ✅ | ❌ | ✅ | [查看](docs/wechat/) |

## 🚀 快速开始

### 环境要求

- Go 1.21+
- PostgreSQL 12+
- Redis 6+

### 安装运行

```bash
# 克隆项目
git clone <repository-url>
cd pay-gateway

# 安装依赖
go mod download

# 配置环境
cp configs/config.toml.example configs/config.toml
# 编辑 config.toml 配置数据库和支付渠道

# 运行服务
make run
```

### Docker 部署

```bash
# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f pay-gateway
```

## 📁 项目结构

```
pay-gateway/
├── cmd/server/              # 应用入口
├── internal/
│   ├── config/              # 配置管理
│   ├── models/              # 数据模型
│   ├── services/            # 业务服务
│   │   ├── google_service.go    # Google Play 服务
│   │   ├── apple_service.go     # Apple Store 服务
│   │   ├── alipay_service.go    # 支付宝服务
│   │   ├── wechat_service.go    # 微信支付服务
│   │   └── payment_service.go   # 通用支付服务
│   ├── handlers/            # HTTP 处理器
│   │   ├── google_handler.go    # Google Play API
│   │   ├── google_webhook.go    # Google Play Webhook
│   │   ├── apple_handler.go     # Apple Store API
│   │   ├── apple_webhook.go     # Apple Webhook
│   │   ├── alipay_handler.go    # 支付宝 API
│   │   ├── alipay_webhook.go    # 支付宝回调
│   │   ├── wechat_handler.go    # 微信支付 API
│   │   ├── wechat_webhook.go    # 微信支付回调
│   │   └── common.go            # 通用处理器
│   ├── routes/              # 路由配置
│   ├── middleware/          # 中间件
│   └── database/            # 数据库连接
├── configs/                 # 配置文件
├── docs/                    # 文档目录
│   ├── google-play/         # Google Play 文档
│   ├── apple/               # Apple 文档
│   ├── alipay/              # 支付宝文档
│   └── wechat/              # 微信支付文档
└── scripts/                 # 脚本工具
```

## 🔌 API 概览

### 通用订单接口

| 方法 | 路径 | 说明 |
|-----|------|-----|
| POST | `/api/v1/orders` | 创建订单 |
| GET | `/api/v1/orders/:id` | 获取订单详情 |
| GET | `/api/v1/orders/no/:order_no` | 根据订单号查询 |
| POST | `/api/v1/orders/:id/cancel` | 取消订单 |
| GET | `/api/v1/users/:user_id/orders` | 获取用户订单 |

### Google Play

| 方法 | 路径 | 说明 |
|-----|------|-----|
| POST | `/api/v1/google/purchases` | 创建内购订单 |
| POST | `/api/v1/google/subscriptions` | 创建订阅订单 |
| POST | `/api/v1/google/verify-purchase` | 验证购买 |
| POST | `/api/v1/google/verify-subscription` | 验证订阅 |
| POST | `/api/v1/google/acknowledge-purchase` | 确认购买 |
| POST | `/api/v1/google/acknowledge-subscription` | 确认订阅 |
| POST | `/api/v1/google/consume-purchase` | 消费购买 |
| POST | `/webhook/google` | Webhook 回调 |

### Apple Store

| 方法 | 路径 | 说明 |
|-----|------|-----|
| POST | `/api/v1/apple/purchases` | 创建内购订单 |
| POST | `/api/v1/apple/subscriptions` | 创建订阅订单 |
| POST | `/api/v1/apple/verify-receipt` | 验证收据 |
| POST | `/api/v1/apple/verify-transaction` | 验证交易 |
| GET | `/api/v1/apple/transactions/:id/history` | 获取交易历史 |
| GET | `/api/v1/apple/subscriptions/:id/status` | 获取订阅状态 |
| POST | `/webhook/apple` | Webhook 回调 |

### 支付宝

| 方法 | 路径 | 说明 |
|-----|------|-----|
| POST | `/api/v1/alipay/orders` | 创建订单 |
| POST | `/api/v1/alipay/payments` | 创建支付 |
| GET | `/api/v1/alipay/orders/query` | 查询订单 |
| POST | `/api/v1/alipay/refunds` | 退款 |
| POST | `/api/v1/alipay/subscriptions` | 创建周期扣款 |
| GET | `/api/v1/alipay/subscriptions/query` | 查询周期扣款 |
| POST | `/api/v1/alipay/subscriptions/cancel` | 取消周期扣款 |
| POST | `/webhook/alipay/notify` | 支付通知 |
| POST | `/webhook/alipay/subscription` | 签约通知 |
| POST | `/webhook/alipay/deduct` | 扣款通知 |

### 微信支付

| 方法 | 路径 | 说明 |
|-----|------|-----|
| POST | `/api/v1/wechat/orders` | 创建订单 |
| GET | `/api/v1/wechat/orders/:order_no` | 查询订单 |
| POST | `/api/v1/wechat/orders/:order_no/close` | 关闭订单 |
| POST | `/api/v1/wechat/payments/jsapi/:order_no` | JSAPI支付 |
| POST | `/api/v1/wechat/payments/native/:order_no` | Native支付 |
| POST | `/api/v1/wechat/payments/app/:order_no` | APP支付 |
| POST | `/api/v1/wechat/payments/h5/:order_no` | H5支付 |
| POST | `/api/v1/wechat/refunds` | 退款 |
| POST | `/webhook/wechat/notify` | 支付通知 |
| POST | `/webhook/wechat/refund` | 退款通知 |

## ⚙️ 配置说明

### 主要环境变量

| 变量名 | 说明 | 默认值 |
|-------|------|-------|
| `SERVER_PORT` | 服务端口 | `8080` |
| `DB_HOST` | 数据库地址 | `localhost` |
| `DB_PORT` | 数据库端口 | `5432` |
| `DB_NAME` | 数据库名称 | `pay_gateway` |
| `REDIS_HOST` | Redis地址 | `localhost` |
| `REDIS_PORT` | Redis端口 | `6379` |

### 支付渠道配置

详细配置请参考各支付方式的文档：

- [Google Play 配置](docs/google-play/README.md#配置)
- [Apple Store 配置](docs/apple/README.md#配置)
- [支付宝配置](docs/alipay/README.md#配置)
- [微信支付配置](docs/wechat/README.md#配置)

## 🛠️ 开发命令

```bash
# 运行开发服务器
make dev

# 构建
make build

# 运行测试
make test

# 代码检查
make lint

# 生成 API 文档
make swagger
```

## 📖 详细文档

- [Google Play 接入指南](docs/google-play/README.md)
- [Apple Store 接入指南](docs/apple/README.md)
- [支付宝接入指南](docs/alipay/README.md)
- [微信支付接入指南](docs/wechat/README.md)

## 📄 许可证

MIT License
