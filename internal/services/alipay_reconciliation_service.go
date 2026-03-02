package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	alipay "github.com/smartwalle/alipay/v3"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorm.io/gorm"

	"pay-gateway/internal/config"
	"pay-gateway/internal/models"
)

// AlipayReconciliationService 支付宝对账服务
type AlipayReconciliationService struct {
	client *alipay.Client
	db     *gorm.DB
	config *config.AlipayConfig
	logger *zap.Logger
}

// NewAlipayReconciliationService 创建对账服务
func NewAlipayReconciliationService(db *gorm.DB, cfg *config.AlipayConfig, logger *zap.Logger) (*AlipayReconciliationService, error) {
	client, err := alipay.New(cfg.AppID, cfg.PrivateKey, cfg.IsProduction)
	if err != nil {
		return nil, fmt.Errorf("创建支付宝客户端失败: %v", err)
	}
	if cfg.CertMode {
		if err := client.LoadAppCertPublicKeyFromFile(cfg.AppCertPath); err != nil {
			return nil, fmt.Errorf("加载应用公钥证书失败: %v", err)
		}
		if err := client.LoadAliPayRootCertFromFile(cfg.RootCertPath); err != nil {
			return nil, fmt.Errorf("加载支付宝根证书失败: %v", err)
		}
		if err := client.LoadAlipayCertPublicKeyFromFile(cfg.AlipayCertPath); err != nil {
			return nil, fmt.Errorf("加载支付宝公钥证书失败: %v", err)
		}
	}
	return &AlipayReconciliationService{
		client: client,
		db:     db,
		config: cfg,
		logger: logger,
	}, nil
}

// BillRecord 对账文件中的单笔记录
type BillRecord struct {
	OutTradeNo    string // 商户订单号
	AlipayTradeNo string // 支付宝交易号
	TradeType     string // 业务类型：交易、退款等
	Amount        string // 订单金额（元）
	ReceivedAmount string // 商家实收（元）
}

// RunReconciliation 执行对账
func (s *AlipayReconciliationService) RunReconciliation(ctx context.Context, billDate string) (*models.AlipayReconciliationReport, error) {
	// 创建对账任务
	report := &models.AlipayReconciliationReport{
		BillDate: billDate,
		BillType: "trade",
		Status:   "processing",
		StartedAt: func() *time.Time { t := time.Now(); return &t }(),
	}
	if err := s.db.Create(report).Error; err != nil {
		return nil, fmt.Errorf("创建对账任务失败: %v", err)
	}

	// 1. 获取对账文件下载地址
	req := alipay.BillDownloadURLQuery{
		BillType: "trade",
		BillDate: billDate,
	}
	result, err := s.client.BillDownloadURLQuery(ctx, req)
	if err != nil {
		s.failReport(report, fmt.Sprintf("获取对账文件URL失败: %v", err))
		return report, err
	}
	if !result.Code.IsSuccess() {
		s.failReport(report, fmt.Sprintf("支付宝返回错误: %s - %s", result.Code, result.Msg))
		return report, fmt.Errorf("支付宝返回错误: %s", result.Msg)
	}

	report.DownloadURL = result.BillDownloadURL

	// 2. 下载对账文件
	body, err := s.downloadFile(ctx, result.BillDownloadURL)
	if err != nil {
		s.failReport(report, fmt.Sprintf("下载对账文件失败: %v", err))
		return report, err
	}

	// 3. 解析对账文件（支持 ZIP 和 CSV）
	records, err := s.parseBillFile(body, result.BillDownloadURL)
	if err != nil {
		s.failReport(report, fmt.Sprintf("解析对账文件失败: %v", err))
		return report, err
	}

	report.TotalCount = len(records)
	s.db.Save(report)

	// 4. 逐笔比对
	matchCount, diffCount, localOnlyCount, details := s.compareWithLocal(records, billDate)

	report.MatchCount = matchCount
	report.DiffCount = diffCount
	report.LocalOnlyCount = localOnlyCount
	report.Status = "completed"
	now := time.Now()
	report.CompletedAt = &now

	// 保存差异明细
	for _, d := range details {
		d.ReportID = report.ID
		s.db.Create(&d)
	}

	if err := s.db.Save(report).Error; err != nil {
		return report, fmt.Errorf("保存对账结果失败: %v", err)
	}

	s.logger.Info("对账完成",
		zap.String("bill_date", billDate),
		zap.Int("total", report.TotalCount),
		zap.Int("match", matchCount),
		zap.Int("diff", diffCount),
		zap.Int("local_only", localOnlyCount))

	return report, nil
}

func (s *AlipayReconciliationService) failReport(report *models.AlipayReconciliationReport, msg string) {
	report.Status = "failed"
	report.ErrorMessage = msg
	now := time.Now()
	report.CompletedAt = &now
	s.db.Save(report)
}

func (s *AlipayReconciliationService) downloadFile(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载返回状态码: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (s *AlipayReconciliationService) parseBillFile(body []byte, url string) ([]BillRecord, error) {
	// 尝试按 ZIP 解析
	if len(body) >= 4 && body[0] == 0x50 && body[1] == 0x4B {
		return s.parseZipBill(body)
	}
	// 按 CSV 解析
	return s.parseCSVBill(body)
}

func (s *AlipayReconciliationService) parseZipBill(body []byte) ([]BillRecord, error) {
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("解压ZIP失败: %w", err)
	}
	for _, f := range reader.File {
		if strings.HasSuffix(f.Name, ".csv") && !strings.Contains(f.Name, "汇总") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			return s.parseCSVBill(data)
		}
	}
	return nil, fmt.Errorf("ZIP中未找到业务明细CSV")
}

func (s *AlipayReconciliationService) parseCSVBill(body []byte) ([]BillRecord, error) {
	// 支付宝对账文件可能为 GBK 或 UTF-8，非 UTF-8 时尝试 GBK 解码
	utf8Body := body
	if !utf8.Valid(body) {
		if decoded, err := gbkToUTF8(body); err == nil {
			utf8Body = decoded
		}
	}
	r := csv.NewReader(strings.NewReader(string(utf8Body)))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析CSV失败: %w", err)
	}

	// 查找表头行，确定列索引
	var outTradeNoIdx, tradeNoIdx, tradeTypeIdx, amountIdx, receivedIdx int = -1, -1, -1, -1, -1
	var startRow int
	for i, row := range rows {
		if len(row) < 3 {
			continue
		}
		for j, col := range row {
			col = strings.TrimSpace(col)
			if col == "商户订单号" {
				outTradeNoIdx = j
			} else if col == "支付宝交易号" {
				tradeNoIdx = j
			} else if col == "业务类型" {
				tradeTypeIdx = j
			} else if col == "订单金额（元）" {
				amountIdx = j
			} else if col == "商家实收（元）" {
				receivedIdx = j
			}
		}
		if outTradeNoIdx >= 0 {
			startRow = i + 1
			break
		}
	}

	if outTradeNoIdx < 0 {
		return nil, fmt.Errorf("未找到对账文件表头，请检查文件格式")
	}
	if amountIdx < 0 {
		amountIdx = outTradeNoIdx + 11
	}
	if receivedIdx < 0 {
		receivedIdx = amountIdx + 1
	}

	var records []BillRecord
	for i := startRow; i < len(rows); i++ {
		row := rows[i]
		if len(row) <= outTradeNoIdx {
			continue
		}
		outTradeNo := strings.TrimSpace(row[outTradeNoIdx])
		if outTradeNo == "" || strings.HasPrefix(outTradeNo, "#") {
			continue
		}
		rec := BillRecord{OutTradeNo: outTradeNo}
		if tradeNoIdx >= 0 && len(row) > tradeNoIdx {
			rec.AlipayTradeNo = strings.TrimSpace(row[tradeNoIdx])
		}
		if tradeTypeIdx >= 0 && len(row) > tradeTypeIdx {
			rec.TradeType = strings.TrimSpace(row[tradeTypeIdx])
		}
		if amountIdx >= 0 && len(row) > amountIdx {
			rec.Amount = strings.TrimSpace(row[amountIdx])
		}
		if receivedIdx >= 0 && len(row) > receivedIdx {
			rec.ReceivedAmount = strings.TrimSpace(row[receivedIdx])
		}
		records = append(records, rec)
	}
	return records, nil
}

func (s *AlipayReconciliationService) compareWithLocal(records []BillRecord, billDate string) (matchCount, diffCount, localOnlyCount int, details []models.AlipayReconciliationDetail) {
	alipayOrderMap := make(map[string]BillRecord)
	for _, r := range records {
		if r.OutTradeNo != "" {
			alipayOrderMap[r.OutTradeNo] = r
		}
	}

	var localOrders []struct {
		OrderNo       string
		TotalAmount   int64
		PaymentStatus string
	}
	s.db.Model(&models.Order{}).
		Where("payment_method = ? AND DATE(created_at) = ?", models.PaymentMethodAlipay, billDate).
		Select("order_no, total_amount, payment_status").
		Find(&localOrders)

	localOrderMap := make(map[string]struct {
		TotalAmount   int64
		PaymentStatus string
	})
	for _, o := range localOrders {
		localOrderMap[o.OrderNo] = struct {
			TotalAmount   int64
			PaymentStatus string
		}{o.TotalAmount, string(o.PaymentStatus)}
	}

	matchedLocal := make(map[string]bool)

	for _, rec := range records {
		if rec.OutTradeNo == "" {
			continue
		}
		local, ok := localOrderMap[rec.OutTradeNo]
		if !ok {
			details = append(details, models.AlipayReconciliationDetail{
				OutTradeNo:      rec.OutTradeNo,
				AlipayTradeNo:   rec.AlipayTradeNo,
				DiffType:        "alipay_only",
				AlipayAmount:    rec.Amount,
				AlipayTradeType: rec.TradeType,
			})
			diffCount++
			continue
		}
		matchedLocal[rec.OutTradeNo] = true

		localAmountFen := local.TotalAmount
		alipayAmountFen, _ := parseAmountFromYuan(rec.Amount)
		if rec.ReceivedAmount != "" {
			alipayAmountFen, _ = parseAmountFromYuan(rec.ReceivedAmount)
		}

		if alipayAmountFen != localAmountFen {
			details = append(details, models.AlipayReconciliationDetail{
				OutTradeNo:    rec.OutTradeNo,
				AlipayTradeNo: rec.AlipayTradeNo,
				DiffType:      "amount_mismatch",
				AlipayAmount:  rec.Amount,
				LocalAmount:   localAmountFen,
				AlipayStatus:  rec.TradeType,
				LocalStatus:   local.PaymentStatus,
			})
			diffCount++
		} else {
			matchCount++
		}
	}

	for orderNo := range localOrderMap {
		if !matchedLocal[orderNo] {
			local := localOrderMap[orderNo]
			details = append(details, models.AlipayReconciliationDetail{
				OutTradeNo:   orderNo,
				DiffType:     "local_only",
				LocalAmount:  local.TotalAmount,
				LocalStatus:  local.PaymentStatus,
			})
			localOnlyCount++
		}
	}

	return matchCount, diffCount, localOnlyCount, details
}

// gbkToUTF8 将 GBK 编码转为 UTF-8，失败时返回原字节
func gbkToUTF8(b []byte) ([]byte, error) {
	r := transform.NewReader(bytes.NewReader(b), simplifiedchinese.GBK.NewDecoder())
	return io.ReadAll(r)
}

// GetReconciliationReport 查询对账报告
func (s *AlipayReconciliationService) GetReconciliationReport(ctx context.Context, reportID uint) (*models.AlipayReconciliationReport, []models.AlipayReconciliationDetail, error) {
	var report models.AlipayReconciliationReport
	if err := s.db.First(&report, reportID).Error; err != nil {
		return nil, nil, fmt.Errorf("对账报告不存在: %v", err)
	}
	var details []models.AlipayReconciliationDetail
	s.db.Where("report_id = ?", reportID).Find(&details)
	return &report, details, nil
}

// ListReconciliationReports 列出对账报告
func (s *AlipayReconciliationService) ListReconciliationReports(ctx context.Context, billDate string, limit int) ([]models.AlipayReconciliationReport, error) {
	if limit <= 0 {
		limit = 20
	}
	var reports []models.AlipayReconciliationReport
	query := s.db.Order("created_at DESC").Limit(limit)
	if billDate != "" {
		query = query.Where("bill_date = ?", billDate)
	}
	if err := query.Find(&reports).Error; err != nil {
		return nil, err
	}
	return reports, nil
}
