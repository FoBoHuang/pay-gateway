#!/bin/bash

# 支付宝API测试脚本
# 使用前请确保服务已启动并配置正确

BASE_URL="http://localhost:8080"
echo "测试支付宝支付API..."

# 1. 创建订单
echo "1. 创建支付宝订单..."
CREATE_ORDER_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/alipay/orders" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "product_id": "test_product_123",
    "subject": "测试商品",
    "body": "这是一个测试商品的描述",
    "total_amount": 100
  }')

echo "创建订单响应: ${CREATE_ORDER_RESPONSE}"

# 提取订单号
ORDER_NO=$(echo "${CREATE_ORDER_RESPONSE}" | grep -o '"order_no":"[^"]*"' | cut -d'"' -f4)
echo "订单号: ${ORDER_NO}"

if [ -z "$ORDER_NO" ]; then
  echo "创建订单失败，无法继续测试"
  exit 1
fi

# 2. 创建WAP支付
echo -e "\n2. 创建WAP支付..."
WAP_PAY_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/alipay/payments" \
  -H "Content-Type: application/json" \
  -d "{
    \"order_no\": \"${ORDER_NO}\",
    \"pay_type\": \"WAP\"
  }")

echo "WAP支付响应: ${WAP_PAY_RESPONSE}"

# 3. 查询订单状态
echo -e "\n3. 查询订单状态..."
QUERY_RESPONSE=$(curl -s "${BASE_URL}/api/v1/alipay/orders/query?order_no=${ORDER_NO}")
echo "查询订单响应: ${QUERY_RESPONSE}"

echo -e "\n测试完成！"
echo "注意：实际支付需要在支付宝环境中完成，这里只测试API接口"