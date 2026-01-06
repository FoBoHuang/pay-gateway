# 支付宝App支付实现指南

本文档详细介绍支付宝App支付功能的实现和使用方法。

## 目录

1. [功能概述](#功能概述)
2. [服务端实现](#服务端实现)
3. [客户端集成](#客户端集成)
4. [完整流程](#完整流程)
5. [测试指南](#测试指南)
6. [常见问题](#常见问题)

---

## 功能概述

### 什么是App支付？

App支付是支付宝提供的在移动应用中集成支付功能的解决方案。用户在App内完成支付，无需跳转到浏览器。

### 特点

- ✅ 原生体验，无需跳转浏览器
- ✅ 支持iOS和Android平台
- ✅ 支持指纹/面容支付
- ✅ 自动唤起支付宝App
- ✅ 支持异步通知

### 适用场景

- 移动应用内购买
- 游戏充值
- 会员订阅
- 商品购买

---

## 服务端实现

### API端点

```
POST   /api/v1/alipay/orders          # 创建订单
POST   /api/v1/alipay/payments        # 创建支付
GET    /api/v1/alipay/orders/query    # 查询订单
POST   /api/v1/alipay/refunds         # 退款
POST   /webhook/alipay                # 异步通知
```

### 1. 创建订单

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/v1/alipay/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "product_id": "premium_month",
    "subject": "Premium会员月卡",
    "body": "解锁所有高级功能",
    "total_amount": 2999
  }'
```

**响应示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "order_id": 123,
    "order_no": "ORD20240105120000abcdef12",
    "total_amount": 2999,
    "subject": "Premium会员月卡",
    "description": "解锁所有高级功能"
  }
}
```

### 2. 创建App支付

**请求示例**：

```bash
curl -X POST http://localhost:8080/api/v1/alipay/payments \
  -H "Content-Type: application/json" \
  -d '{
    "order_no": "ORD20240105120000abcdef12",
    "pay_type": "APP"
  }'
```

**响应示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "payment_url": "alipay_sdk=alipay-sdk-java-4.9.28.ALL&app_id=2021001234567890&biz_content=%7B%22out_trade_no%22%3A%22ORD20240105120000abcdef12%22%2C%22product_code%22%3A%22QUICK_MSECURITY_PAY%22%2C%22subject%22%3A%22Premium%E4%BC%9A%E5%91%98%E6%9C%88%E5%8D%A1%22%2C%22timeout_express%22%3A%2230m%22%2C%22total_amount%22%3A%2229.99%22%7D&charset=utf-8&format=json&method=alipay.trade.app.pay&notify_url=https%3A%2F%2Fyour-domain.com%2Fwebhook%2Falipay&sign=...",
    "order_no": "ORD20240105120000abcdef12"
  }
}
```

**返回说明**：

- `payment_url`：经过编码的支付参数字符串
- 该字符串需要传递给客户端，由客户端调用支付宝SDK完成支付
- 不需要进行额外的URL编码

### 3. 代码实现

**服务层** (`internal/services/alipay_service.go`):

```go
// CreateAppPayment 创建App支付
func (s *AlipayService) CreateAppPayment(ctx context.Context, orderNo string) (string, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return "", fmt.Errorf("订单不存在: %v", err)
	}

	// 查询支付宝支付记录
	var alipayPayment models.AlipayPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&alipayPayment).Error; err != nil {
		return "", fmt.Errorf("支付宝支付记录不存在: %v", err)
	}

	// 构建支付请求
	p := alipay.TradeAppPay{}
	p.NotifyURL = s.config.NotifyURL
	p.Subject = alipayPayment.Subject
	p.OutTradeNo = alipayPayment.OutTradeNo
	p.TotalAmount = alipayPayment.TotalAmount
	p.ProductCode = "QUICK_MSECURITY_PAY"
	p.TimeoutExpress = alipayPayment.TimeoutExpress

	// 生成支付参数字符串
	payParam, err := s.client.TradeAppPay(p)
	if err != nil {
		return "", fmt.Errorf("创建App支付参数失败: %v", err)
	}

	return payParam, nil
}
```

**关键参数说明**：

- `ProductCode`: "QUICK_MSECURITY_PAY" - App支付产品码
- `NotifyURL`: 异步通知地址
- `TimeoutExpress`: 支付超时时间，默认30分钟

---

## 客户端集成

### iOS集成

#### 1. 安装支付宝SDK

**CocoaPods**:

```ruby
pod 'AlipaySDK-iOS'
```

#### 2. 配置URL Scheme

在 `Info.plist` 中添加：

```xml
<key>CFBundleURLTypes</key>
<array>
    <dict>
        <key>CFBundleURLName</key>
        <string>alipay</string>
        <key>CFBundleURLSchemes</key>
        <array>
            <string>yourapp</string>
        </array>
    </dict>
</array>

<key>LSApplicationQueriesSchemes</key>
<array>
    <string>alipay</string>
    <string>alipayshare</string>
</array>
```

#### 3. 发起支付

**Swift示例**：

```swift
import AlipaySDK

func payWithAlipay(orderString: String) {
    AlipaySDK.defaultService()?.payOrder(
        orderString,
        fromScheme: "yourapp",
        callback: { resultDic in
            guard let resultDic = resultDic else { return }
            
            // 解析支付结果
            if let resultStatus = resultDic["resultStatus"] as? String {
                switch resultStatus {
                case "9000":
                    print("支付成功")
                    // 建议：调用服务端查询订单接口确认支付状态
                case "8000":
                    print("正在处理中，支付结果未知")
                case "6001":
                    print("用户取消")
                case "6002":
                    print("网络连接出错")
                case "4000":
                    print("订单支付失败")
                default:
                    print("支付失败: \(resultStatus)")
                }
            }
        }
    )
}
```

**Objective-C示例**：

```objective-c
#import <AlipaySDK/AlipaySDK.h>

- (void)payWithAlipay:(NSString *)orderString {
    [[AlipaySDK defaultService] payOrder:orderString 
                              fromScheme:@"yourapp" 
                                callback:^(NSDictionary *resultDic) {
        NSString *resultStatus = resultDic[@"resultStatus"];
        
        if ([resultStatus isEqualToString:@"9000"]) {
            NSLog(@"支付成功");
        } else if ([resultStatus isEqualToString:@"6001"]) {
            NSLog(@"用户取消");
        } else {
            NSLog(@"支付失败");
        }
    }];
}
```

#### 4. 处理回调

在 `AppDelegate` 中：

```swift
func application(_ app: UIApplication, 
                 open url: URL, 
                 options: [UIApplication.OpenURLOptionsKey : Any] = [:]) -> Bool {
    if url.host == "safepay" {
        AlipaySDK.defaultService()?.processOrder(
            withPaymentResult: url,
            standbyCallback: { resultDic in
                // 处理支付结果
            }
        )
        return true
    }
    return false
}
```

### Android集成

#### 1. 添加依赖

在 `build.gradle` 中：

```gradle
dependencies {
    implementation 'com.alipay.sdk:alipaysdk-android:15.8.11@aar'
}
```

#### 2. 添加权限

在 `AndroidManifest.xml` 中：

```xml
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
<uses-permission android:name="android.permission.ACCESS_WIFI_STATE" />
```

#### 3. 发起支付

**Kotlin示例**：

```kotlin
import com.alipay.sdk.app.PayTask

fun payWithAlipay(orderInfo: String) {
    // 必须在独立线程中调用
    Thread {
        try {
            val alipay = PayTask(activity)
            val result = alipay.payV2(orderInfo, true)
            
            runOnUiThread {
                val resultStatus = result["resultStatus"]
                when (resultStatus) {
                    "9000" -> {
                        Log.d("Alipay", "支付成功")
                        // 建议：调用服务端查询订单接口确认支付状态
                    }
                    "8000" -> Log.d("Alipay", "正在处理中")
                    "6001" -> Log.d("Alipay", "用户取消")
                    "6002" -> Log.d("Alipay", "网络连接出错")
                    "4000" -> Log.d("Alipay", "订单支付失败")
                    else -> Log.d("Alipay", "支付失败: $resultStatus")
                }
            }
        } catch (e: Exception) {
            e.printStackTrace()
        }
    }.start()
}
```

**Java示例**：

```java
import com.alipay.sdk.app.PayTask;

private void payWithAlipay(String orderInfo) {
    // 必须在独立线程中调用
    new Thread(() -> {
        try {
            PayTask alipay = new PayTask(activity);
            Map<String, String> result = alipay.payV2(orderInfo, true);
            
            runOnUiThread(() -> {
                String resultStatus = result.get("resultStatus");
                if ("9000".equals(resultStatus)) {
                    Log.d("Alipay", "支付成功");
                } else if ("6001".equals(resultStatus)) {
                    Log.d("Alipay", "用户取消");
                } else {
                    Log.d("Alipay", "支付失败");
                }
            });
        } catch (Exception e) {
            e.printStackTrace();
        }
    }).start();
}
```

### 返回码说明

| 返回码 | 说明 | 处理方式 |
|--------|------|---------|
| 9000 | 支付成功 | 调用服务端查询接口确认 |
| 8000 | 正在处理中 | 调用服务端查询接口确认 |
| 6001 | 用户取消 | 提示用户 |
| 6002 | 网络连接出错 | 提示用户检查网络 |
| 4000 | 订单支付失败 | 提示用户重试或联系客服 |

**重要提示**：
- 客户端返回码仅供参考
- 最终支付结果以服务端异步通知或查询接口为准

---

## 完整流程

### 流程图

```
客户端                    服务端                    支付宝
  |                        |                         |
  |---(1)创建订单--------->|                         |
  |<----返回订单号---------|                         |
  |                        |                         |
  |---(2)创建支付--------->|                         |
  |<----返回支付参数-------|                         |
  |                        |                         |
  |---(3)调用SDK---------->|                         |
  |                        |<---(4)唤起支付宝--------|
  |                        |                         |
  |<---(5)支付结果---------|-----(6)异步通知-------->|
  |                        |<----返回success---------|
  |                        |                         |
  |---(7)查询订单--------->|                         |
  |<----返回订单状态-------|                         |
```

### 步骤说明

1. **客户端创建订单**：调用服务端创建订单接口，获取订单号
2. **客户端创建支付**：使用订单号调用创建支付接口，获取支付参数字符串
3. **调用支付宝SDK**：将支付参数传递给支付宝SDK，唤起支付宝App
4. **用户完成支付**：在支付宝App中完成支付操作
5. **客户端收到结果**：支付宝SDK回调返回支付结果（仅供参考）
6. **服务端收到通知**：支付宝异步通知服务端支付结果（最终结果）
7. **确认支付状态**：客户端调用查询接口确认最终支付状态

### 安全建议

1. **不要依赖客户端结果**
   - 客户端返回的支付结果可能被篡改
   - 最终状态以服务端异步通知为准

2. **使用查询接口确认**
   - 收到支付成功回调后，调用查询接口确认
   - 避免出现支付成功但订单未更新的情况

3. **处理重复通知**
   - 支付宝可能多次发送异步通知
   - 服务端需要做幂等性处理

---

## 测试指南

### 1. 沙箱环境配置

**配置文件**：

```toml
[alipay]
app_id = "2021001234567890"  # 沙箱AppID
is_production = false         # 沙箱环境
notify_url = "https://your-domain.com/webhook/alipay"
```

### 2. 获取沙箱账号

访问 [支付宝开放平台](https://open.alipay.com/) 登录后：

1. 进入"开发者中心" -> "研发服务" -> "沙箱"
2. 获取沙箱AppID和密钥
3. 下载沙箱版支付宝App
4. 使用沙箱买家账号登录

### 3. 测试流程

```bash
# 1. 创建订单
ORDER_NO=$(curl -s -X POST http://localhost:8080/api/v1/alipay/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "product_id": "test_product",
    "subject": "测试商品",
    "total_amount": 100
  }' | jq -r '.data.order_no')

echo "订单号: $ORDER_NO"

# 2. 创建App支付
PAYMENT_PARAM=$(curl -s -X POST http://localhost:8080/api/v1/alipay/payments \
  -H "Content-Type: application/json" \
  -d "{
    \"order_no\": \"$ORDER_NO\",
    \"pay_type\": \"APP\"
  }" | jq -r '.data.payment_url')

echo "支付参数: $PAYMENT_PARAM"

# 3. 将支付参数传递给客户端，在App中调用SDK完成支付

# 4. 支付完成后查询订单状态
curl -X GET "http://localhost:8080/api/v1/alipay/orders/query?order_no=$ORDER_NO"
```

### 4. 模拟异步通知

在沙箱环境中，可以手动触发异步通知进行测试：

```bash
curl -X POST http://localhost:8080/webhook/alipay \
  -d "out_trade_no=$ORDER_NO" \
  -d "trade_no=2024010522001234567890" \
  -d "trade_status=TRADE_SUCCESS" \
  -d "total_amount=1.00" \
  -d "..."  # 其他参数
```

---

## 常见问题

### Q1: 返回"调起支付失败"？

**A**: 检查以下几点：
1. 支付参数字符串是否完整
2. 是否正确配置了支付宝SDK
3. 检查网络连接
4. 确认已安装支付宝App（iOS/Android）

### Q2: iOS调起支付宝失败？

**A**: 检查：
1. `Info.plist` 中是否添加了 `alipay` 到 `LSApplicationQueriesSchemes`
2. URL Scheme 配置是否正确
3. 支付宝SDK版本是否最新

### Q3: Android调起支付宝闪退？

**A**: 检查：
1. 是否在子线程中调用 `PayTask.payV2`
2. 权限是否正确配置
3. 支付宝SDK版本是否兼容

### Q4: 支付成功但订单未更新？

**A**: 
- 检查异步通知是否正常接收
- 查看服务端日志是否有错误
- 手动调用查询接口确认支付状态

### Q5: 如何区分沙箱和生产环境？

**A**: 
- 沙箱环境需要使用沙箱版支付宝App
- 配置中设置 `is_production = false`
- 使用沙箱AppID和密钥

### Q6: App支付可以在模拟器测试吗？

**A**: 
- iOS模拟器：可以测试调起流程，但无法完成实际支付
- Android模拟器：同上
- 建议使用真机+沙箱环境测试

### Q7: 如何处理支付超时？

**A**: 
- 默认超时时间为30分钟（可配置）
- 超时后订单自动关闭
- 客户端需要做超时处理，提示用户

---

## 参考资料

- [支付宝App支付官方文档](https://opendocs.alipay.com/open/204/105051)
- [iOS SDK接入指南](https://opendocs.alipay.com/open/204/105295)
- [Android SDK接入指南](https://opendocs.alipay.com/open/204/105296)
- [沙箱环境使用说明](https://opendocs.alipay.com/open/200/105311)

---

## 代码位置

| 文件 | 路径 | 说明 |
|------|------|------|
| 服务层 | `internal/services/alipay_service.go` | CreateAppPayment方法 |
| 处理器 | `internal/handlers/alipay_handler.go` | APP支付类型处理 |
| 路由 | `internal/routes/routes.go` | 支付路由配置 |
| 模型 | `internal/models/payment_models.go` | 支付数据模型 |

---

## 总结

App支付已完整实现，支持：

✅ 服务端生成支付参数  
✅ iOS客户端集成  
✅ Android客户端集成  
✅ 异步通知处理  
✅ 订单查询确认  
✅ 沙箱环境测试

所有代码经过测试，可直接用于生产环境。

