#!/bin/bash

# 支付宝App支付测试脚本
# 用于测试支付宝App支付功能

set -e

# 配置
BASE_URL="http://localhost:8080"
USER_ID=1
PRODUCT_ID="test_app_payment"
SUBJECT="测试App支付"
BODY="这是一个测试App支付的订单"
TOTAL_AMOUNT=100  # 单位：分（1元）

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印分隔线
print_separator() {
    echo -e "${BLUE}================================================${NC}"
}

# 打印标题
print_title() {
    echo -e "${GREEN}$1${NC}"
}

# 打印错误
print_error() {
    echo -e "${RED}错误: $1${NC}"
}

# 打印警告
print_warning() {
    echo -e "${YELLOW}警告: $1${NC}"
}

# 打印信息
print_info() {
    echo -e "${BLUE}$1${NC}"
}

# 检查响应状态
check_response() {
    local response=$1
    local code=$(echo "$response" | jq -r '.code' 2>/dev/null || echo "error")
    
    if [ "$code" != "0" ]; then
        print_error "API调用失败"
        echo "$response" | jq '.' 2>/dev/null || echo "$response"
        return 1
    fi
    return 0
}

# 主流程
main() {
    print_separator
    print_title "支付宝App支付功能测试"
    print_separator
    echo ""

    # 步骤1: 创建订单
    print_title "步骤1: 创建订单"
    print_separator
    
    CREATE_ORDER_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/alipay/orders" \
        -H "Content-Type: application/json" \
        -d "{
            \"user_id\": ${USER_ID},
            \"product_id\": \"${PRODUCT_ID}\",
            \"subject\": \"${SUBJECT}\",
            \"body\": \"${BODY}\",
            \"total_amount\": ${TOTAL_AMOUNT}
        }")
    
    if ! check_response "$CREATE_ORDER_RESPONSE"; then
        exit 1
    fi
    
    echo "$CREATE_ORDER_RESPONSE" | jq '.'
    
    ORDER_NO=$(echo "$CREATE_ORDER_RESPONSE" | jq -r '.data.order_no')
    ORDER_ID=$(echo "$CREATE_ORDER_RESPONSE" | jq -r '.data.order_id')
    
    if [ -z "$ORDER_NO" ] || [ "$ORDER_NO" = "null" ]; then
        print_error "未能获取订单号"
        exit 1
    fi
    
    print_info "✓ 订单创建成功"
    print_info "  订单ID: $ORDER_ID"
    print_info "  订单号: $ORDER_NO"
    echo ""
    
    # 步骤2: 创建App支付
    print_title "步骤2: 创建App支付"
    print_separator
    
    CREATE_PAYMENT_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/alipay/payments" \
        -H "Content-Type: application/json" \
        -d "{
            \"order_no\": \"${ORDER_NO}\",
            \"pay_type\": \"APP\"
        }")
    
    if ! check_response "$CREATE_PAYMENT_RESPONSE"; then
        exit 1
    fi
    
    echo "$CREATE_PAYMENT_RESPONSE" | jq '.'
    
    PAYMENT_PARAM=$(echo "$CREATE_PAYMENT_RESPONSE" | jq -r '.data.payment_url')
    
    if [ -z "$PAYMENT_PARAM" ] || [ "$PAYMENT_PARAM" = "null" ]; then
        print_error "未能获取支付参数"
        exit 1
    fi
    
    print_info "✓ App支付参数创建成功"
    echo ""
    
    # 显示支付参数
    print_title "支付参数信息"
    print_separator
    print_info "支付参数长度: ${#PAYMENT_PARAM} 字符"
    print_info "支付参数预览: ${PAYMENT_PARAM:0:100}..."
    echo ""
    
    # 保存支付参数到文件
    echo "$PAYMENT_PARAM" > /tmp/alipay_app_payment_param.txt
    print_info "✓ 支付参数已保存到: /tmp/alipay_app_payment_param.txt"
    echo ""
    
    # 步骤3: 客户端集成说明
    print_title "步骤3: 客户端集成说明"
    print_separator
    echo ""
    
    print_info "【iOS集成】"
    echo "将支付参数传递给支付宝SDK："
    echo ""
    echo "Swift:"
    echo "-------"
    cat << 'EOF'
let orderString = "支付参数字符串"
AlipaySDK.defaultService()?.payOrder(
    orderString,
    fromScheme: "yourapp",
    callback: { resultDic in
        if let resultStatus = resultDic?["resultStatus"] as? String {
            switch resultStatus {
            case "9000":
                print("支付成功")
            case "6001":
                print("用户取消")
            default:
                print("支付失败")
            }
        }
    }
)
EOF
    echo ""
    
    print_info "【Android集成】"
    echo "将支付参数传递给支付宝SDK："
    echo ""
    echo "Kotlin:"
    echo "-------"
    cat << 'EOF'
Thread {
    val alipay = PayTask(activity)
    val result = alipay.payV2(orderInfo, true)
    
    runOnUiThread {
        when (result["resultStatus"]) {
            "9000" -> Log.d("Alipay", "支付成功")
            "6001" -> Log.d("Alipay", "用户取消")
            else -> Log.d("Alipay", "支付失败")
        }
    }
}.start()
EOF
    echo ""
    
    # 步骤4: 查询订单状态
    print_title "步骤4: 查询订单状态"
    print_separator
    
    QUERY_ORDER_RESPONSE=$(curl -s -X GET "${BASE_URL}/api/v1/alipay/orders/query?order_no=${ORDER_NO}")
    
    if ! check_response "$QUERY_ORDER_RESPONSE"; then
        exit 1
    fi
    
    echo "$QUERY_ORDER_RESPONSE" | jq '.'
    
    TRADE_STATUS=$(echo "$QUERY_ORDER_RESPONSE" | jq -r '.data.trade_status')
    PAYMENT_STATUS=$(echo "$QUERY_ORDER_RESPONSE" | jq -r '.data.payment_status')
    
    print_info "✓ 订单查询成功"
    print_info "  交易状态: $TRADE_STATUS"
    print_info "  支付状态: $PAYMENT_STATUS"
    echo ""
    
    # 测试总结
    print_separator
    print_title "测试总结"
    print_separator
    echo ""
    
    print_info "✓ 所有API调用成功"
    print_info "✓ 订单创建成功: $ORDER_NO"
    print_info "✓ App支付参数生成成功"
    print_info "✓ 订单查询成功"
    echo ""
    
    print_warning "注意事项:"
    echo "  1. 支付参数已保存到 /tmp/alipay_app_payment_param.txt"
    echo "  2. 需要在移动App中集成支付宝SDK才能完成实际支付"
    echo "  3. 完成支付后，服务端会收到异步通知"
    echo "  4. 建议在支付成功后调用查询接口确认最终状态"
    echo ""
    
    print_separator
    print_info "详细文档请查看: docs/guides/alipay/app-payment.md"
    print_separator
    echo ""
}

# 运行主流程
main

exit 0

