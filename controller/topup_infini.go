package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func isInfiniTopUpEnabled() bool {
	return setting.InfiniKeyID != "" && setting.InfiniSecretKey != ""
}

type InfiniPayRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
	SuccessURL    string `json:"success_url,omitempty"`
	FailureURL    string `json:"failure_url,omitempty"`
}

func getInfiniMinTopup() int64 {
	minTopup := setting.InfiniMinTopUp
	if minTopup <= 0 {
		minTopup = 1
	}
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		minTopup = minTopup * int(common.QuotaPerUnit)
	}
	return int64(minTopup)
}

func RequestInfiniPay(c *gin.Context) {
	var req InfiniPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	if req.Amount < getInfiniMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getInfiniMinTopup())})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	client := getInfiniClient()
	if client == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Infini 支付未配置"})
		return
	}

	tradeNo := fmt.Sprintf("INF%dNO%s%d", id, common.GetRandomString(6), time.Now().Unix())

	successURL := system_setting.ServerAddress + "/console/log"
	failureURL := system_setting.ServerAddress + "/console/topup"
	if req.SuccessURL != "" && common.ValidateRedirectURL(req.SuccessURL) == nil {
		successURL = req.SuccessURL
	}
	if req.FailureURL != "" && common.ValidateRedirectURL(req.FailureURL) == nil {
		failureURL = req.FailureURL
	}

	infiniOrder, err := client.CreateOrder(InfiniCreateOrderRequest{
		Amount:          strconv.FormatFloat(payMoney, 'f', 2, 64),
		RequestID:       tradeNo,
		ClientReference: tradeNo,
		OrderDesc:       fmt.Sprintf("TokenHub TopUp %d", req.Amount),
		ExpiresIn:       3600,
		SuccessURL:      successURL,
		FailureURL:      failureURL,
		PayMethods:      []int{1},
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Infini 创建订单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(int64(amount))
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = int(dAmount.Div(dQuotaPerUnit).IntPart())
	}

	topUp := &model.TopUp{
		UserId:        id,
		Amount:        amount,
		Money:         payMoney,
		TradeNo:       tradeNo,
		PaymentMethod: model.PaymentMethodInfini,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Infini 创建充值订单失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Infini 充值订单创建成功 user_id=%d trade_no=%s infini_order_id=%s checkout_url=%s amount=%d money=%.2f", id, tradeNo, infiniOrder.OrderID, infiniOrder.CheckoutURL, req.Amount, payMoney))

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"checkout_url": infiniOrder.CheckoutURL,
		},
	})
}

func InfiniWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	if !isInfiniTopUpEnabled() {
		logger.LogWarn(ctx, fmt.Sprintf("Infini webhook 被拒绝 reason=not_configured path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.JSON(http.StatusForbidden, gin.H{"error": "webhook disabled"})
		return
	}

	signature := c.GetHeader("X-Webhook-Signature")
	timestamp := c.GetHeader("X-Webhook-Timestamp")
	eventID := c.GetHeader("X-Webhook-Event-Id")

	if signature == "" || timestamp == "" || eventID == "" {
		logger.LogWarn(ctx, fmt.Sprintf("Infini webhook 缺少必要 headers path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing headers"})
		return
	}

	payload, err := c.GetRawData()
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Infini webhook 读取请求体失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body failed"})
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("Infini webhook 收到请求 path=%q client_ip=%s event_id=%s body=%q", c.Request.RequestURI, c.ClientIP(), eventID, string(payload)))

	client := getInfiniClient()
	if !client.VerifyWebhook(timestamp, eventID, string(payload), signature) {
		logger.LogWarn(ctx, fmt.Sprintf("Infini webhook 验签失败 path=%q client_ip=%s event_id=%s", c.Request.RequestURI, c.ClientIP(), eventID))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	var event map[string]interface{}
	if err := common.Unmarshal(payload, &event); err != nil {
		logger.LogError(ctx, fmt.Sprintf("Infini webhook 解析事件失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	eventType, _ := event["event"].(string)
	clientRef, _ := event["client_reference"].(string)
	status, _ := event["status"].(string)

	callerIP := c.ClientIP()

	switch {
	case eventType == "order.completed" && status == "paid":
		handleInfiniPaymentCompleted(ctx, clientRef, callerIP)
	case eventType == "order.expired":
		handleInfiniPaymentExpired(ctx, clientRef, callerIP)
	default:
		logger.LogInfo(ctx, fmt.Sprintf("Infini webhook 忽略事件 event=%s status=%s client_ref=%s client_ip=%s", eventType, status, clientRef, callerIP))
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func handleInfiniPaymentCompleted(ctx context.Context, tradeNo string, callerIP string) {
	if tradeNo == "" {
		logger.LogWarn(ctx, "Infini webhook order.completed 缺少 client_reference")
		return
	}

	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	if err := model.RechargeInfini(tradeNo, callerIP); err != nil {
		logger.LogError(ctx, fmt.Sprintf("Infini 充值处理失败 trade_no=%s client_ip=%s error=%q", tradeNo, callerIP, err.Error()))
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("Infini 充值成功 trade_no=%s client_ip=%s", tradeNo, callerIP))
}

func handleInfiniPaymentExpired(ctx context.Context, tradeNo string, callerIP string) {
	if tradeNo == "" {
		logger.LogWarn(ctx, "Infini webhook order.expired 缺少 client_reference")
		return
	}

	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	if err := model.UpdatePendingTopUpStatus(tradeNo, model.PaymentMethodInfini, common.TopUpStatusExpired); err != nil {
		logger.LogError(ctx, fmt.Sprintf("Infini 订单过期处理失败 trade_no=%s client_ip=%s error=%q", tradeNo, callerIP, err.Error()))
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("Infini 订单已过期 trade_no=%s client_ip=%s", tradeNo, callerIP))
}

func AdminInfiniWithdraw(c *gin.Context) {
	var req InfiniWithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	client := getInfiniClient()
	if client == nil {
		common.ApiErrorMsg(c, "Infini 支付未配置")
		return
	}

	result, err := client.Withdraw(req)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Infini 提现失败 chain=%s amount=%s error=%q", req.Chain, req.Amount, err.Error()))
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, result)
}

func GetInfiniWithdrawStatus(c *gin.Context) {
	requestID := c.Query("request_id")
	if requestID == "" {
		common.ApiErrorMsg(c, "缺少 request_id")
		return
	}

	client := getInfiniClient()
	if client == nil {
		common.ApiErrorMsg(c, "Infini 支付未配置")
		return
	}

	result, err := client.QueryOrder(requestID)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, result)
}
