# 新手入门指南

欢迎使用Pay Gateway支付网关！本文档将帮助你快速上手。

---

## 🎯 三步快速开始

### 步骤 1: 选择支付方式

根据你的业务需求，选择需要集成的支付方式：

| 支付方式 | 适用场景 | 快速开始文档 |
|---------|---------|-------------|
| **微信支付** | 国内用户，微信生态 | [点击查看](guides/wechat/quick-start.md) |
| **支付宝** | 国内用户，网页支付 | [点击查看](guides/alipay/quick-start.md) |
| **Google Play** | Android应用内购买 | [点击查看](guides/google-play/quick-start.md) |
| **Apple Store** | iOS应用内购买 | [点击查看](guides/apple/quick-start.md) |

### 步骤 2: 配置服务

1. **复制配置文件**
   ```bash
   cp configs/config.toml.example configs/config.toml
   ```

2. **编辑配置**
   - 填写对应支付方式的配置参数
   - 参考: [配置示例](../configs/config.toml.example)

3. **获取密钥**
   - 微信支付：[商户平台](https://pay.weixin.qq.com/)
   - 支付宝：[开放平台](https://open.alipay.com/)
   - Google Play：[Google Cloud Console](https://console.cloud.google.com/)
   - Apple：[App Store Connect](https://appstoreconnect.apple.com/)

### 步骤 3: 启动服务

```bash
# 方式1：直接运行
go run cmd/server/main.go

# 方式2：编译后运行
go build -o build/pay-gateway ./cmd/server/
./build/pay-gateway

# 方式3：使用Docker
docker-compose up -d
```

**验证服务**:
```bash
curl http://localhost:8080/health
```

---

## 📚 学习路径

### 路径1: 快速集成（推荐给着急的你）

**时间**: 30分钟

1. ⏱️ 5分钟：阅读 [README.md](../README.md) 了解项目
2. ⏱️ 10分钟：选择支付方式，阅读对应的 `quick-start.md`
3. ⏱️ 10分钟：配置服务，启动测试
4. ⏱️ 5分钟：测试API调用

### 路径2: 深入学习（推荐给认真的你）

**时间**: 2小时

1. ⏱️ 15分钟：阅读 [README.md](../README.md)
2. ⏱️ 30分钟：阅读 [所有支付功能总结](references/all-payments.md)
3. ⏱️ 45分钟：阅读需要的支付方式的完整指南
4. ⏱️ 30分钟：阅读 [支付集成文档](references/integration.md)

### 路径3: 全面掌握（推荐给完美主义者）

**时间**: 半天

1. 阅读所有快速开始文档
2. 阅读所有完整指南
3. 查看代码实现
4. 运行所有测试场景
5. 阅读开发文档

---

## 🎓 按角色推荐

### 前端开发者

**关注点**: API如何调用

**推荐文档**:
1. 对应支付方式的 [快速开始](guides/)
2. API请求示例
3. 响应数据格式

**学习时间**: 15-30分钟

### 后端开发者

**关注点**: 如何实现和扩展

**推荐文档**:
1. [代码位置速查](references/code-map.md)
2. [支付集成文档](references/integration.md)
3. [项目实施总结](development/implementation.md)
4. 各支付方式的完整指南

**学习时间**: 1-2小时

### 测试人员

**关注点**: 测试场景和验证

**推荐文档**:
1. [功能检查清单](references/checklist.md)
2. 各支付方式完整指南中的"测试指南"章节
3. 快速开始文档中的示例

**学习时间**: 30分钟-1小时

### 项目经理/产品经理

**关注点**: 功能概览和对比

**推荐文档**:
1. [README.md](../README.md)
2. [所有支付功能总结](references/all-payments.md)
3. [功能检查清单](references/checklist.md)

**学习时间**: 15-30分钟

---

## 💡 常见使用场景

### 场景1: 我想集成微信支付

```
1. 阅读 guides/wechat/quick-start.md
2. 配置微信支付参数
3. 按照示例创建订单和发起支付
4. 测试支付流程
```

### 场景2: 我想查找某个功能的代码在哪里

```
1. 打开 references/code-map.md
2. 按支付方式或功能查找
3. 找到文件和行号
4. 打开对应的代码文件
```

### 场景3: 我想对比不同支付方式

```
1. 打开 references/all-payments.md
2. 查看功能对比表
3. 了解各支付方式的特点
4. 选择适合的支付方式
```

### 场景4: 我想验证功能是否完整

```
1. 打开 references/checklist.md
2. 逐项检查功能
3. 查看实现状态
4. 运行测试验证
```

---

## 🔍 文档搜索技巧

### 按关键词搜索

```bash
# 搜索所有文档中包含"退款"的内容
grep -r "退款" docs/

# 搜索API端点
grep -r "POST /api" docs/

# 搜索配置项
grep -r "config.toml" docs/
```

### 使用IDE搜索

在VSCode等IDE中：
1. 按 `Cmd/Ctrl + Shift + F` 打开全局搜索
2. 限定搜索范围为 `docs/` 目录
3. 输入关键词搜索

---

## 📖 推荐阅读顺序

### 第一次使用（必读）

1. 📄 [../README.md](../README.md) - 3分钟
2. 📄 [README.md](README.md) - 2分钟
3. 📄 选择支付方式的 quick-start.md - 5分钟

**总时间**: 10分钟

### 深入学习（推荐）

1. 📄 [references/all-payments.md](references/all-payments.md) - 10分钟
2. 📄 对应支付方式的 complete-guide.md - 20-30分钟
3. 📄 [references/integration.md](references/integration.md) - 15分钟

**总时间**: 45-55分钟

### 开发维护（高级）

1. 📄 [references/code-map.md](references/code-map.md) - 10分钟
2. 📄 [development/implementation.md](development/implementation.md) - 15分钟
3. 📄 查看实际代码 - 30分钟+

**总时间**: 55分钟+

---

## 🎯 学习成果

### 完成基础学习后，你应该能够：

- ✅ 了解项目支持哪些支付方式
- ✅ 知道如何配置服务
- ✅ 能够调用API创建订单和发起支付
- ✅ 了解基本的业务流程

### 完成深入学习后，你应该能够：

- ✅ 理解各支付方式的技术细节
- ✅ 知道如何处理Webhook通知
- ✅ 能够调试和解决常见问题
- ✅ 了解最佳实践

### 完成全面学习后，你应该能够：

- ✅ 独立开发和扩展新功能
- ✅ 优化性能和安全性
- ✅ 解决复杂的技术问题
- ✅ 指导他人使用

---

## 📞 获取帮助

如果文档无法解决你的问题：

1. **搜索文档** - 使用关键词搜索
2. **查看FAQ** - 各完整指南都有常见问题
3. **查看代码** - 代码即文档
4. **提交Issue** - 在GitHub上提问
5. **联系团队** - 寻求技术支持

---

## 🔗 快速链接

- 🏠 [项目主页](../README.md)
- 📖 [文档索引](INDEX.md)
- 🗺️ [代码速查](references/code-map.md)
- ⚙️ [配置示例](../configs/config.toml.example)

---

**祝你使用愉快！** 🎉

如有任何问题，欢迎查看文档或联系我们。

