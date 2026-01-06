# 文档中心

欢迎使用Pay Gateway支付网关文档中心。

---

## 📂 文档结构

```
docs/
├── README.md (本文档)              # 文档导航
├── INDEX.md                        # 完整文档索引
├── guides/                         # 使用指南
│   ├── wechat/                     # 微信支付
│   ├── alipay/                     # 支付宝
│   ├── google-play/                # Google Play
│   └── apple/                      # Apple Store
├── references/                     # 参考文档
│   ├── code-map.md                 # 代码位置速查
│   ├── all-payments.md             # 所有支付功能总结
│   ├── checklist.md                # 功能检查清单
│   └── integration.md              # 支付集成文档
└── development/                    # 开发文档
    └── implementation.md           # 项目实施总结
```

---

## 🚀 快速开始

### 第一次使用？从这里开始 →

1. **了解项目** - 回到根目录查看 [README.md](../README.md)
2. **选择支付方式** - 查看下方的快速开始文档
3. **查看配置** - 参考 [配置示例](../configs/config.toml.example)
4. **开始集成** - 按照文档步骤操作

---

## 📖 按支付方式浏览

### 💳 微信支付 (WeChat Pay)

| 文档 | 说明 |
|------|------|
| [快速开始](guides/wechat/quick-start.md) | 5分钟快速上手 |
| [完整指南](guides/wechat/complete-guide.md) | 详细功能说明 |

**支持**: JSAPI、Native、APP、H5支付

---

### 💰 支付宝 (Alipay)

| 文档 | 说明 |
|------|------|
| [快速开始](guides/alipay/quick-start.md) | 5分钟快速上手 |
| [完整指南](guides/alipay/complete-guide.md) | 详细功能说明 |
| [实现说明](guides/alipay/implementation.md) | 代码实现细节 |

**支持**: WAP、PAGE支付、周期扣款（订阅）

---

### 🤖 Google Play

| 文档 | 说明 |
|------|------|
| [快速开始](guides/google-play/quick-start.md) | 5分钟快速上手 |
| [完整指南](guides/google-play/complete-guide.md) | 详细功能说明 |

**支持**: 内购、订阅、确认、消费

---

### 🍎 Apple Store

| 文档 | 说明 |
|------|------|
| [快速开始](guides/apple/quick-start.md) | 5分钟快速上手 |
| [完整指南](guides/apple/complete-guide.md) | 详细功能说明 |

**支持**: 内购、订阅、收据验证、交易验证

---

## 📋 参考文档

### 查找代码位置
👉 **[代码位置速查表](references/code-map.md)**

快速找到任何功能的代码位置，包括：
- 服务层方法
- HTTP处理器
- 路由配置
- 数据模型

### 功能对比和总结
👉 **[所有支付功能总结](references/all-payments.md)**

查看：
- 4种支付方式对比
- API端点汇总
- 代码统计
- 架构设计

### 验证功能完整性
👉 **[功能检查清单](references/checklist.md)**

检查：
- 每个支付方式的功能是否完整
- 代码质量
- 测试覆盖

### 技术集成详解
👉 **[支付集成文档](references/integration.md)**

了解：
- 统一支付接口设计
- 适配器模式应用
- 数据模型设计
- 最佳实践

---

## 🛠️ 开发文档

### 项目实施总结
👉 **[项目实施总结](development/implementation.md)**

包含：
- 实施过程
- 技术选型
- 代码组织
- 版本历史

---

## 🎯 常见任务导航

### 任务1：集成新的支付方式

1. 阅读 [支付集成文档](references/integration.md)
2. 参考现有支付方式的实现
3. 创建对应的 `xxx_service.go` 服务文件
4. 创建 `xxx_handler.go` 和 `xxx_webhook.go` 处理器
5. 添加路由配置
6. 更新文档

### 任务2：调试支付问题

1. 查看 [代码位置速查表](references/code-map.md) 找到相关代码
2. 检查日志输出
3. 参考对应支付方式的完整指南
4. 查看常见问题解答

### 任务3：添加新功能

1. 在对应的 service 文件中添加方法
2. 在对应的 handler 文件中添加HTTP处理
3. 在 routes.go 中注册路由
4. 更新对应的快速开始文档

### 任务4：部署到生产环境

1. 阅读根目录 [README.md](../README.md) 的部署章节
2. 配置生产环境参数（configs/config.toml）
3. 设置HTTPS和域名
4. 配置Webhook URL
5. 进行测试验证

---

## 📊 文档统计

- **总文档数**: 11份
- **快速开始**: 4份
- **完整指南**: 4份
- **参考文档**: 4份
- **开发文档**: 1份

---

## 💡 文档使用技巧

### 1. 善用搜索

在文档中搜索关键词：
- API端点名称
- 错误信息
- 配置项名称
- 方法名称

### 2. 查看示例

所有文档都包含实际可运行的示例代码，可以直接复制使用。

### 3. 跟随链接

文档间有交叉引用链接，可以深入了解相关内容。

### 4. 查看代码

文档中标注了代码位置，可以直接查看源码了解实现细节。

---

## 🔄 文档更新

本文档会随着项目更新而更新。

**最后更新**: 2024-12-05  
**文档版本**: v1.0.0  
**维护者**: 开发团队

---

## 📞 获取帮助

如果文档无法解决你的问题：

1. 查看 [功能检查清单](references/checklist.md)
2. 搜索项目Issues
3. 创建新Issue
4. 联系技术支持

---

**快速链接**:
- 🏠 [项目主页](../README.md)
- 📖 [完整文档索引](INDEX.md)
- 🗺️ [代码位置速查](references/code-map.md)
- ⚙️ [配置示例](../configs/config.toml.example)

