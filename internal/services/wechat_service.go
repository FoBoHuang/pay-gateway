package services

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"pay-gateway/internal/config"
	"pay-gateway/internal/models"
)

const wechatAPIBaseURL = "https://api.mch.weixin.qq.com"

// WechatService 微信支付服务
type WechatService struct {
	db         *gorm.DB
	config     *config.WechatConfig
	logger     *zap.Logger
	privateKey *rsa.PrivateKey
}

// NewWechatService 创建微信支付服务实例
func NewWechatService(db *gorm.DB, cfg *config.WechatConfig, logger *zap.Logger) (*WechatService, error) {
	if cfg.MchID == "" || cfg.AppID == "" {
		return nil, errors.New("微信支付配置不完整")
	}

	// 获取私钥内容：优先使用 PrivateKey，否则从 PrivateKeyPath 读取
	privateKeyStr := cfg.PrivateKey
	if privateKeyStr == "" && cfg.PrivateKeyPath != "" {
		keyContent, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("读取微信私钥文件失败: %w", err)
		}
		privateKeyStr = string(keyContent)
	}
	if privateKeyStr == "" {
		return nil, errors.New("微信商户私钥未配置（需配置 private_key 或 private_key_path）")
	}

	// 解析私钥
	privateKey, err := parseWechatPrivateKey(privateKeyStr)
	if err != nil {
		return nil, fmt.Errorf("解析微信私钥失败: %v", err)
	}

	return &WechatService{
		db:         db,
		config:     cfg,
		logger:     logger,
		privateKey: privateKey,
	}, nil
}

// CreateOrder 创建微信支付订单
func (s *WechatService) CreateOrder(ctx context.Context, req *CreateWechatOrderRequest) (*CreateWechatOrderResponse, error) {
	// 生成系统订单号
	orderNo := generateWechatOrderNo()

	// 创建订单
	order := &models.Order{
		OrderNo:       orderNo,
		UserID:        req.UserID,
		ProductID:     req.ProductID,
		Type:          models.OrderTypePurchase,
		Title:         req.Description,
		Description:   req.Detail,
		Quantity:      1,
		Currency:      "CNY",
		TotalAmount:   req.TotalAmount,
		Status:        models.OrderStatusCreated,
		PaymentMethod: models.PaymentMethodWeChat,
		PaymentStatus: models.PaymentStatusPending,
		ExpiredAt:     &[]time.Time{time.Now().Add(30 * time.Minute)}[0],
	}

	// 开启事务
	tx := s.db.Begin()
	if err := tx.Create(order).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建订单失败: %v", err)
	}

	// 创建微信支付记录
	wechatPayment := &models.WechatPayment{
		OrderID:    order.ID,
		OutTradeNo: orderNo,
		TradeType:  req.TradeType,
		AppID:      s.config.AppID,
		MchID:      s.config.MchID,
	}

	if err := tx.Create(wechatPayment).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建微信支付记录失败: %v", err)
	}

	// 创建支付交易记录
	transaction := &models.PaymentTransaction{
		OrderID:       order.ID,
		TransactionID: orderNo,
		Provider:      models.PaymentProviderWeChat,
		Type:          "PAYMENT",
		Amount:        req.TotalAmount,
		Currency:      "CNY",
		Status:        models.PaymentStatusPending,
		ProviderData:  models.JSON{},
	}

	if err := tx.Create(transaction).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建交易记录失败: %v", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %v", err)
	}

	s.logger.Info("微信订单创建成功",
		zap.String("order_no", orderNo),
		zap.Uint("order_id", order.ID),
		zap.String("trade_type", req.TradeType),
	)

	return &CreateWechatOrderResponse{
		OrderID:     order.ID,
		OrderNo:     orderNo,
		TotalAmount: req.TotalAmount,
		Description: req.Description,
	}, nil
}

// CreateJSAPIPayment 创建JSAPI支付（小程序、公众号）
func (s *WechatService) CreateJSAPIPayment(ctx context.Context, orderNo, openID string) (*JSAPIPaymentResponse, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 调用微信支付 API 创建预支付单
	timeExpire := ""
	if order.ExpiredAt != nil {
		timeExpire = order.ExpiredAt.Format(time.RFC3339)
	}
	reqBody := wechatJSAPIReq{
		AppID:       s.config.AppID,
		MchID:       s.config.MchID,
		Description: order.Title,
		OutTradeNo:  orderNo,
		TimeExpire:  timeExpire,
		NotifyURL:   s.config.NotifyURL,
		Amount:      wechatAmount{Total: order.TotalAmount, Currency: "CNY"},
		Payer:       wechatPayer{OpenID: openID},
	}

	respBody, _, err := s.wechatAPIRequest(ctx, "POST", "/v3/pay/transactions/jsapi", reqBody)
	if err != nil {
		return nil, fmt.Errorf("调用微信JSAPI下单失败: %w", err)
	}

	var prepayResp wechatPrepayResp
	if err := json.Unmarshal(respBody, &prepayResp); err != nil {
		return nil, fmt.Errorf("解析微信响应失败: %w", err)
	}
	prepayID := prepayResp.PrepayID
	if prepayID == "" {
		return nil, fmt.Errorf("微信未返回 prepay_id")
	}

	// 更新微信支付记录
	wechatPayment.PrepayID = prepayID
	wechatPayment.Payer = models.JSON{"openid": openID}
	wechatPayment.Amount = models.JSON{"total": order.TotalAmount, "currency": "CNY"}
	if err := s.db.Save(&wechatPayment).Error; err != nil {
		s.logger.Error("更新微信支付记录失败", zap.Error(err))
	}

	s.logger.Info("JSAPI支付创建成功",
		zap.String("order_no", orderNo),
		zap.String("prepay_id", prepayID),
		zap.String("openid", openID),
	)

	// 生成小程序/公众号调起支付所需参数
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonceStr := generateNonceStr()
	packageStr := fmt.Sprintf("prepay_id=%s", prepayID)

	paySign, err := s.signPayParams(s.config.AppID, timestamp, nonceStr, packageStr)
	if err != nil {
		return nil, fmt.Errorf("生成pay_sign失败: %w", err)
	}

	return &JSAPIPaymentResponse{
		PrepayID:  prepayID,
		AppID:     s.config.AppID,
		TimeStamp: timestamp,
		NonceStr:  nonceStr,
		Package:   packageStr,
		SignType:  "RSA",
		PaySign:   paySign,
	}, nil
}

// CreateNativePayment 创建Native支付（扫码支付）
func (s *WechatService) CreateNativePayment(ctx context.Context, orderNo string) (*NativePaymentResponse, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 调用微信支付 API 创建 Native 预支付单
	timeExpire := ""
	if order.ExpiredAt != nil {
		timeExpire = order.ExpiredAt.Format(time.RFC3339)
	}
	reqBody := wechatNativeReq{
		AppID:       s.config.AppID,
		MchID:       s.config.MchID,
		Description: order.Title,
		OutTradeNo:  orderNo,
		TimeExpire:  timeExpire,
		NotifyURL:   s.config.NotifyURL,
		Amount:      wechatAmount{Total: order.TotalAmount, Currency: "CNY"},
	}

	respBody, _, err := s.wechatAPIRequest(ctx, "POST", "/v3/pay/transactions/native", reqBody)
	if err != nil {
		return nil, fmt.Errorf("调用微信Native下单失败: %w", err)
	}

	var prepayResp wechatPrepayResp
	if err := json.Unmarshal(respBody, &prepayResp); err != nil {
		return nil, fmt.Errorf("解析微信响应失败: %w", err)
	}
	codeURL := prepayResp.CodeURL
	if codeURL == "" {
		return nil, fmt.Errorf("微信未返回 code_url")
	}

	// 更新微信支付记录
	wechatPayment.CodeURL = codeURL
	wechatPayment.Amount = models.JSON{"total": order.TotalAmount, "currency": "CNY"}
	if err := s.db.Save(&wechatPayment).Error; err != nil {
		s.logger.Error("更新微信支付记录失败", zap.Error(err))
	}

	s.logger.Info("Native支付创建成功",
		zap.String("order_no", orderNo),
		zap.String("code_url", codeURL),
	)

	return &NativePaymentResponse{
		CodeURL: codeURL,
	}, nil
}

// CreateAPPPayment 创建APP支付
func (s *WechatService) CreateAPPPayment(ctx context.Context, orderNo string) (*APPPaymentResponse, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 调用微信支付 API 创建 APP 预支付单
	timeExpire := ""
	if order.ExpiredAt != nil {
		timeExpire = order.ExpiredAt.Format(time.RFC3339)
	}
	reqBody := wechatAPPReq{
		AppID:       s.config.AppID,
		MchID:       s.config.MchID,
		Description: order.Title,
		OutTradeNo:  orderNo,
		TimeExpire:  timeExpire,
		NotifyURL:   s.config.NotifyURL,
		Amount:      wechatAmount{Total: order.TotalAmount, Currency: "CNY"},
	}

	respBody, _, err := s.wechatAPIRequest(ctx, "POST", "/v3/pay/transactions/app", reqBody)
	if err != nil {
		return nil, fmt.Errorf("调用微信APP下单失败: %w", err)
	}

	var prepayResp wechatPrepayResp
	if err := json.Unmarshal(respBody, &prepayResp); err != nil {
		return nil, fmt.Errorf("解析微信响应失败: %w", err)
	}
	prepayID := prepayResp.PrepayID
	if prepayID == "" {
		return nil, fmt.Errorf("微信未返回 prepay_id")
	}

	// 更新微信支付记录
	wechatPayment.PrepayID = prepayID
	wechatPayment.Amount = models.JSON{"total": order.TotalAmount, "currency": "CNY"}
	if err := s.db.Save(&wechatPayment).Error; err != nil {
		s.logger.Error("更新微信支付记录失败", zap.Error(err))
	}

	s.logger.Info("APP支付创建成功",
		zap.String("order_no", orderNo),
		zap.String("prepay_id", prepayID),
	)

	// 生成APP调起支付所需参数（package 固定为 prepay_id=xxx）
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonceStr := generateNonceStr()
	packageStr := fmt.Sprintf("prepay_id=%s", prepayID)

	sign, err := s.signPayParams(s.config.AppID, timestamp, nonceStr, packageStr)
	if err != nil {
		return nil, fmt.Errorf("生成sign失败: %w", err)
	}

	return &APPPaymentResponse{
		PrepayID:  prepayID,
		PartnerID: s.config.MchID,
		AppID:     s.config.AppID,
		TimeStamp: timestamp,
		NonceStr:  nonceStr,
		Package:   packageStr,
		SignType:  "RSA",
		Sign:      sign,
	}, nil
}

// CreateH5Payment 创建H5支付
func (s *WechatService) CreateH5Payment(ctx context.Context, orderNo string, sceneInfo map[string]interface{}) (*H5PaymentResponse, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 构建 scene_info，H5 支付可选传，建议传 payer_client_ip 和 h5_info
	scene := wechatH5SceneInfo{
		H5Info: wechatH5Info{Type: "Wap", AppName: "PayGateway"},
	}
	if sceneInfo != nil {
		if ip, ok := sceneInfo["payer_client_ip"].(string); ok && ip != "" {
			scene.PayerClientIP = ip
		}
		if t, ok := sceneInfo["type"].(string); ok && t != "" {
			scene.H5Info.Type = t
		}
		if name, ok := sceneInfo["app_name"].(string); ok && name != "" {
			scene.H5Info.AppName = name
		}
		if url, ok := sceneInfo["app_url"].(string); ok && url != "" {
			scene.H5Info.AppURL = url
		}
	}

	// 调用微信支付 API 创建 H5 预支付单
	timeExpire := ""
	if order.ExpiredAt != nil {
		timeExpire = order.ExpiredAt.Format(time.RFC3339)
	}
	reqBody := wechatH5Req{
		AppID:       s.config.AppID,
		MchID:       s.config.MchID,
		Description: order.Title,
		OutTradeNo:  orderNo,
		TimeExpire:  timeExpire,
		NotifyURL:   s.config.NotifyURL,
		Amount:      wechatAmount{Total: order.TotalAmount, Currency: "CNY"},
		SceneInfo:   scene,
	}

	respBody, _, err := s.wechatAPIRequest(ctx, "POST", "/v3/pay/transactions/h5", reqBody)
	if err != nil {
		return nil, fmt.Errorf("调用微信H5下单失败: %w", err)
	}

	var prepayResp wechatPrepayResp
	if err := json.Unmarshal(respBody, &prepayResp); err != nil {
		return nil, fmt.Errorf("解析微信响应失败: %w", err)
	}
	h5URL := prepayResp.H5URL
	if h5URL == "" {
		return nil, fmt.Errorf("微信未返回 h5_url")
	}

	// 更新微信支付记录
	wechatPayment.H5URL = h5URL
	wechatPayment.Amount = models.JSON{"total": order.TotalAmount, "currency": "CNY"}
	if sceneInfo != nil {
		wechatPayment.SceneInfo = models.JSON(sceneInfo)
	}
	if err := s.db.Save(&wechatPayment).Error; err != nil {
		s.logger.Error("更新微信支付记录失败", zap.Error(err))
	}

	s.logger.Info("H5支付创建成功",
		zap.String("order_no", orderNo),
		zap.String("h5_url", h5URL),
	)

	return &H5PaymentResponse{
		H5URL: h5URL,
	}, nil
}

// WechatNotifyResource 微信回调加密资源结构
type WechatNotifyResource struct {
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	Nonce          string `json:"nonce"`
	AssociatedData string `json:"associated_data"`
}

// WechatNotifyRequest 微信回调请求结构
type WechatNotifyRequest struct {
	ID           string               `json:"id"`
	CreateTime   string               `json:"create_time"`
	ResourceType string               `json:"resource_type"`
	EventType    string               `json:"event_type"`
	Resource     WechatNotifyResource `json:"resource"`
}

// VerifyAndDecryptNotify 验签并解密微信回调
// 返回解密后的业务数据，供 HandleNotify 处理
func (s *WechatService) VerifyAndDecryptNotify(headers map[string]string, body []byte) (map[string]interface{}, error) {
	// 1. 验签
	if s.config.PlatformCertPath != "" {
		if err := s.verifyWechatSignature(headers, body); err != nil {
			return nil, fmt.Errorf("验签失败: %w", err)
		}
	} else {
		s.logger.Warn("未配置微信平台证书，跳过回调验签", zap.String("platform_cert_path", s.config.PlatformCertPath))
	}

	// 2. 解析请求体
	var req WechatNotifyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("解析回调失败: %w", err)
	}

	// 3. 解密 resource
	if req.Resource.Ciphertext == "" {
		return nil, errors.New("回调无加密内容")
	}

	plaintext, err := s.decryptWechatResource(req.Resource)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	var decrypted map[string]interface{}
	if err := json.Unmarshal(plaintext, &decrypted); err != nil {
		return nil, fmt.Errorf("解析解密内容失败: %w", err)
	}

	return decrypted, nil
}

// verifyWechatSignature 验证微信回调签名
func (s *WechatService) verifyWechatSignature(headers map[string]string, body []byte) error {
	timestamp := headers["Wechatpay-Timestamp"]
	nonce := headers["Wechatpay-Nonce"]
	signatureHeader := headers["Wechatpay-Signature"]

	if timestamp == "" || nonce == "" || signatureHeader == "" {
		return errors.New("缺少验签必要请求头")
	}

	// 解析 Wechatpay-Signature: nonce="xxx",timestamp="xxx",signature="xxx",serial="xxx"
	signature, err := parseWechatpaySignature(signatureHeader)
	if err != nil {
		return err
	}

	// 构建验签串
	message := fmt.Sprintf("%s\n%s\n%s\n", timestamp, nonce, string(body))

	// 加载平台证书
	certPEM, err := os.ReadFile(s.config.PlatformCertPath)
	if err != nil {
		return fmt.Errorf("读取平台证书失败: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return errors.New("解析平台证书失败")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("解析证书失败: %w", err)
	}

	pubKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("平台证书非RSA公钥")
	}

	// 验签：SHA256WithRSA
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("解码签名失败: %w", err)
	}

	hashed := sha256.Sum256([]byte(message))
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed[:], signatureBytes); err != nil {
		return fmt.Errorf("签名验证失败: %w", err)
	}

	return nil
}

func parseWechatpaySignature(header string) (string, error) {
	re := regexp.MustCompile(`signature="([^"]+)"`)
	matches := re.FindStringSubmatch(header)
	if len(matches) < 2 {
		return "", errors.New("无法解析 Wechatpay-Signature")
	}
	return strings.TrimSpace(matches[1]), nil
}

// buildWechatAuthHeader 构建微信支付 API v3 请求签名头
// 签名串格式：HTTP方法\nURL路径\n时间戳\n随机串\n请求体\n
// 使用商户私钥 RSA-SHA256 签名后 Base64 编码
func (s *WechatService) buildWechatAuthHeader(method, urlPath, body string) (string, error) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonceStr := generateNonceStr()

	// 构造待签名字符串
	signStr := method + "\n" + urlPath + "\n" + timestamp + "\n" + nonceStr + "\n" + body + "\n"

	// RSA-SHA256 签名
	hashed := sha256.Sum256([]byte(signStr))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("签名失败: %w", err)
	}
	signatureBase64 := base64.StdEncoding.EncodeToString(signature)

	// Authorization: WECHATPAY2-SHA256-RSA2048 mchid="xxx",nonce_str="xxx",signature="xxx",timestamp="xxx",serial_no="xxx"
	auth := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		s.config.MchID, nonceStr, signatureBase64, timestamp, s.config.SerialNo)
	return auth, nil
}

// signPayParams 对调起支付参数签名（用于 JSAPI/APP 的 pay_sign/sign）
// 待签名字符串：appId\ntimeStamp\nnonceStr\npackage\n\n
func (s *WechatService) signPayParams(appID, timestamp, nonceStr, packageStr string) (string, error) {
	signStr := appID + "\n" + timestamp + "\n" + nonceStr + "\n" + packageStr + "\n\n"
	hashed := sha256.Sum256([]byte(signStr))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// wechatAPIRequest 发起带签名的微信支付 API 请求
func (s *WechatService) wechatAPIRequest(ctx context.Context, method, urlPath string, reqBody interface{}) ([]byte, int, error) {
	var bodyBytes []byte
	if reqBody != nil {
		var err error
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return nil, 0, fmt.Errorf("序列化请求体失败: %w", err)
		}
	}
	bodyStr := string(bodyBytes)

	auth, err := s.buildWechatAuthHeader(method, urlPath, bodyStr)
	if err != nil {
		return nil, 0, err
	}

	url := wechatAPIBaseURL + urlPath
	var req *http.Request
	if len(bodyBytes) > 0 {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", auth)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("请求微信API失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		s.logger.Error("微信API请求失败",
			zap.Int("status", resp.StatusCode),
			zap.String("url", url),
			zap.String("response", string(respBody)),
		)
		return respBody, resp.StatusCode, fmt.Errorf("微信API返回错误: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}

// 微信支付 API 请求/响应结构
type wechatJSAPIReq struct {
	AppID       string         `json:"appid"`
	MchID       string         `json:"mchid"`
	Description string         `json:"description"`
	OutTradeNo  string         `json:"out_trade_no"`
	TimeExpire  string         `json:"time_expire,omitempty"`
	NotifyURL   string         `json:"notify_url"`
	Amount      wechatAmount   `json:"amount"`
	Payer       wechatPayer    `json:"payer"`
}
type wechatNativeReq struct {
	AppID       string       `json:"appid"`
	MchID       string       `json:"mchid"`
	Description string       `json:"description"`
	OutTradeNo  string       `json:"out_trade_no"`
	TimeExpire  string       `json:"time_expire,omitempty"`
	NotifyURL   string       `json:"notify_url"`
	Amount      wechatAmount `json:"amount"`
}
type wechatAPPReq struct {
	AppID       string       `json:"appid"`
	MchID       string       `json:"mchid"`
	Description string       `json:"description"`
	OutTradeNo  string       `json:"out_trade_no"`
	TimeExpire  string       `json:"time_expire,omitempty"`
	NotifyURL   string       `json:"notify_url"`
	Amount      wechatAmount `json:"amount"`
}
type wechatH5Req struct {
	AppID       string              `json:"appid"`
	MchID       string              `json:"mchid"`
	Description string              `json:"description"`
	OutTradeNo  string              `json:"out_trade_no"`
	TimeExpire  string              `json:"time_expire,omitempty"`
	NotifyURL   string              `json:"notify_url"`
	Amount      wechatAmount        `json:"amount"`
	SceneInfo   wechatH5SceneInfo    `json:"scene_info"`
}
type wechatAmount struct {
	Total    int64  `json:"total"`
	Currency string `json:"currency"`
}
type wechatPayer struct {
	OpenID string `json:"openid"`
}
type wechatH5SceneInfo struct {
	PayerClientIP string       `json:"payer_client_ip,omitempty"`
	DeviceID      string       `json:"device_id,omitempty"`
	H5Info        wechatH5Info `json:"h5_info"`
}
type wechatH5Info struct {
	Type    string `json:"type"` // "Wap" 或 "iOS"/"Android"
	AppName string `json:"app_name,omitempty"`
	AppURL  string `json:"app_url,omitempty"`
}
type wechatPrepayResp struct {
	PrepayID string `json:"prepay_id"`
	CodeURL  string `json:"code_url"`
	H5URL    string `json:"h5_url"`
}

// decryptWechatResource 使用 API v3 密钥解密回调资源
func (s *WechatService) decryptWechatResource(res WechatNotifyResource) ([]byte, error) {
	if s.config.APIv3Key == "" || len(s.config.APIv3Key) != 32 {
		return nil, errors.New("API v3 密钥未配置或长度非32位")
	}

	key := []byte(s.config.APIv3Key)
	ciphertext, err := base64.StdEncoding.DecodeString(res.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("解码密文失败: %w", err)
	}

	nonce := []byte(res.Nonce)
	associatedData := []byte(res.AssociatedData)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, associatedData)
	if err != nil {
		return nil, fmt.Errorf("AES-GCM 解密失败: %w", err)
	}

	return plaintext, nil
}

// HandleNotify 处理微信支付异步通知
func (s *WechatService) HandleNotify(ctx context.Context, notifyData map[string]interface{}) error {
	// 提取关键参数
	outTradeNo, _ := notifyData["out_trade_no"].(string)
	transactionID, _ := notifyData["transaction_id"].(string)
	tradeState, _ := notifyData["trade_state"].(string)

	if outTradeNo == "" {
		return errors.New("缺少商户订单号")
	}

	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", outTradeNo).First(&order).Error; err != nil {
		return fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 开启事务
	tx := s.db.Begin()

	// 更新微信支付记录
	wechatPayment.TransactionID = transactionID
	wechatPayment.TradeState = tradeState
	if tradeStateDesc, ok := notifyData["trade_state_desc"].(string); ok {
		wechatPayment.TradeStateDesc = tradeStateDesc
	}
	if bankType, ok := notifyData["bank_type"].(string); ok {
		wechatPayment.BankType = bankType
	}

	// 解析支付完成时间
	if successTimeStr, ok := notifyData["success_time"].(string); ok {
		if successTime, err := time.Parse(time.RFC3339, successTimeStr); err == nil {
			wechatPayment.SuccessTime = &successTime
		}
	}

	// 保存原始通知数据
	wechatPayment.RawNotifyData = models.JSON(notifyData)
	now := time.Now()
	wechatPayment.NotifyTime = &now

	if err := tx.Save(&wechatPayment).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新微信支付记录失败: %v", err)
	}

	// 根据交易状态更新订单
	switch tradeState {
	case "SUCCESS":
		order.Status = models.OrderStatusPaid
		order.PaymentStatus = models.PaymentStatusCompleted
		order.PaidAt = &now
	case "CLOSED", "REVOKED", "PAYERROR":
		order.PaymentStatus = models.PaymentStatusFailed
	}

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新订单状态失败: %v", err)
	}

	// 更新交易记录
	var transaction models.PaymentTransaction
	if err := tx.Where("order_id = ? AND transaction_id = ?", order.ID, outTradeNo).First(&transaction).Error; err == nil {
		switch tradeState {
		case "SUCCESS":
			transaction.Status = models.PaymentStatusCompleted
			transaction.ProcessedAt = &now
			transaction.ProviderData = models.JSON(notifyData)
		case "CLOSED", "REVOKED", "PAYERROR":
			transaction.Status = models.PaymentStatusFailed
			transaction.ProcessedAt = &now
			if errorMsg, ok := notifyData["error_message"].(string); ok {
				transaction.ErrorMessage = &errorMsg
			}
		}
		if err := tx.Save(&transaction).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("更新交易记录失败: %v", err)
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交事务失败: %v", err)
	}

	s.logger.Info("微信支付通知处理成功",
		zap.String("out_trade_no", outTradeNo),
		zap.String("transaction_id", transactionID),
		zap.String("trade_state", tradeState),
	)

	return nil
}

// QueryOrder 查询订单状态
func (s *WechatService) QueryOrder(ctx context.Context, orderNo string) (*QueryWechatOrderResponse, error) {
	// 查询本地订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 如果订单已完成，直接返回
	if order.Status == models.OrderStatusPaid {
		return &QueryWechatOrderResponse{
			OrderNo:       orderNo,
			TransactionID: wechatPayment.TransactionID,
			TradeState:    wechatPayment.TradeState,
			TotalAmount:   order.TotalAmount,
			PaymentStatus: order.PaymentStatus,
			PaidAt:        order.PaidAt,
		}, nil
	}

	// 实际应用中，这里应该调用微信支付查询接口
	// 为演示目的，直接返回本地数据

	return &QueryWechatOrderResponse{
		OrderNo:       orderNo,
		TransactionID: wechatPayment.TransactionID,
		TradeState:    wechatPayment.TradeState,
		TotalAmount:   order.TotalAmount,
		PaymentStatus: order.PaymentStatus,
		PaidAt:        order.PaidAt,
	}, nil
}

// Refund 退款
func (s *WechatService) Refund(ctx context.Context, req *WechatRefundRequest) (*WechatRefundResponse, error) {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", req.OrderNo).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 检查订单状态
	if order.Status != models.OrderStatusPaid {
		return nil, errors.New("订单未支付，无法退款")
	}

	// 查询微信支付记录
	var wechatPayment models.WechatPayment
	if err := s.db.Where("order_id = ?", order.ID).First(&wechatPayment).Error; err != nil {
		return nil, fmt.Errorf("微信支付记录不存在: %v", err)
	}

	// 生成退款单号
	outRefundNo := generateWechatRefundNo(req.OrderNo)
	refundID := fmt.Sprintf("wx%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])

	// 开启事务
	tx := s.db.Begin()

	// 创建退款记录
	refund := &models.WechatRefund{
		OrderID:         order.ID,
		WechatPaymentID: wechatPayment.ID,
		OutRefundNo:     outRefundNo,
		RefundID:        refundID,
		OutTradeNo:      req.OrderNo,
		TransactionID:   wechatPayment.TransactionID,
		RefundAmount:    req.RefundAmount,
		TotalAmount:     order.TotalAmount,
		Currency:        "CNY",
		RefundReason:    req.RefundReason,
		RefundStatus:    "SUCCESS",
	}

	now := time.Now()
	refund.SuccessTime = &now

	if err := tx.Create(refund).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建退款记录失败: %v", err)
	}

	// 更新订单状态
	order.Status = models.OrderStatusRefunded
	order.RefundAt = &now
	order.RefundReason = req.RefundReason
	order.RefundAmount = req.RefundAmount

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("更新订单状态失败: %v", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %v", err)
	}

	s.logger.Info("微信退款成功",
		zap.String("out_trade_no", req.OrderNo),
		zap.String("out_refund_no", outRefundNo),
		zap.String("refund_id", refundID),
		zap.Int64("refund_amount", req.RefundAmount),
	)

	return &WechatRefundResponse{
		OutRefundNo:  outRefundNo,
		RefundID:     refundID,
		RefundAmount: req.RefundAmount,
		RefundStatus: "SUCCESS",
		RefundAt:     &now,
	}, nil
}

// CloseOrder 关闭订单
func (s *WechatService) CloseOrder(ctx context.Context, orderNo string) error {
	// 查询订单
	var order models.Order
	if err := s.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return fmt.Errorf("订单不存在: %v", err)
	}

	// 检查订单状态
	if order.Status == models.OrderStatusPaid {
		return errors.New("订单已支付，无法关闭")
	}

	// 更新订单状态
	order.Status = models.OrderStatusCancelled
	order.PaymentStatus = models.PaymentStatusCancelled

	if err := s.db.Save(&order).Error; err != nil {
		return fmt.Errorf("关闭订单失败: %v", err)
	}

	s.logger.Info("订单关闭成功",
		zap.String("order_no", orderNo),
		zap.Uint("order_id", order.ID),
	)

	return nil
}

// 辅助函数

func parseWechatPrivateKey(privateKeyStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyStr))
	if block == nil {
		return nil, errors.New("无法解析私钥")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// 尝试PKCS1格式
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
	}

	privateKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("不是RSA私钥")
	}

	return privateKey, nil
}

func generateWechatOrderNo() string {
	return fmt.Sprintf("WX%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
}

func generateWechatRefundNo(orderNo string) string {
	return fmt.Sprintf("REF%s%s", orderNo, time.Now().Format("150405"))
}

func generateNonceStr() string {
	return uuid.New().String()[:32]
}

func generateCodeURL() string {
	return uuid.New().String()[:16]
}

// 请求和响应结构体

// CreateWechatOrderRequest 创建微信订单请求
type CreateWechatOrderRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	ProductID   string `json:"product_id" binding:"required"`
	Description string `json:"description" binding:"required"`
	Detail      string `json:"detail"`
	TotalAmount int64  `json:"total_amount" binding:"required,min=1"`
	TradeType   string `json:"trade_type" binding:"required,oneof=JSAPI NATIVE APP MWEB"`
}

// CreateWechatOrderResponse 创建微信订单响应
type CreateWechatOrderResponse struct {
	OrderID     uint   `json:"order_id"`
	OrderNo     string `json:"order_no"`
	TotalAmount int64  `json:"total_amount"`
	Description string `json:"description"`
}

// JSAPIPaymentResponse JSAPI支付响应
type JSAPIPaymentResponse struct {
	PrepayID  string `json:"prepay_id"`
	AppID     string `json:"app_id"`
	TimeStamp string `json:"time_stamp"`
	NonceStr  string `json:"nonce_str"`
	Package   string `json:"package"`
	SignType  string `json:"sign_type"`
	PaySign   string `json:"pay_sign,omitempty"` // 需要客户端计算
}

// NativePaymentResponse Native支付响应
type NativePaymentResponse struct {
	CodeURL string `json:"code_url"`
}

// APPPaymentResponse APP支付响应
type APPPaymentResponse struct {
	PrepayID  string `json:"prepay_id"`
	PartnerID string `json:"partner_id"`
	AppID     string `json:"app_id"`
	TimeStamp string `json:"time_stamp"`
	NonceStr  string `json:"nonce_str"`
	Package   string `json:"package"`
	SignType  string `json:"sign_type"`
	Sign      string `json:"sign,omitempty"` // 需要客户端计算
}

// H5PaymentResponse H5支付响应
type H5PaymentResponse struct {
	H5URL string `json:"h5_url"`
}

// QueryWechatOrderResponse 查询微信订单响应
type QueryWechatOrderResponse struct {
	OrderNo       string               `json:"order_no"`
	TransactionID string               `json:"transaction_id"`
	TradeState    string               `json:"trade_state"`
	TotalAmount   int64                `json:"total_amount"`
	PaymentStatus models.PaymentStatus `json:"payment_status"`
	PaidAt        *time.Time           `json:"paid_at,omitempty"`
}

// WechatRefundRequest 微信退款请求
type WechatRefundRequest struct {
	OrderNo      string `json:"order_no" binding:"required"`
	RefundAmount int64  `json:"refund_amount" binding:"required,min=1"`
	RefundReason string `json:"refund_reason" binding:"required"`
}

// WechatRefundResponse 微信退款响应
type WechatRefundResponse struct {
	OutRefundNo  string     `json:"out_refund_no"`
	RefundID     string     `json:"refund_id"`
	RefundAmount int64      `json:"refund_amount"`
	RefundStatus string     `json:"refund_status"`
	RefundAt     *time.Time `json:"refund_at,omitempty"`
}
