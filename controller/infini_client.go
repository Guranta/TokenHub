package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
)

// InfiniClient is the API client for Infini payment gateway.
// Doc: https://developer.infini.money/docs/zh/6-api-ducumentation
type InfiniClient struct {
	KeyID     string
	SecretKey string
	BaseURL   string
}

type InfiniCreateOrderRequest struct {
	Amount          string `json:"amount"`
	RequestID       string `json:"request_id"`
	ClientReference string `json:"client_reference,omitempty"`
	OrderDesc       string `json:"order_desc,omitempty"`
	ExpiresIn       int64  `json:"expires_in,omitempty"`
	MerchantAlias   string `json:"merchant_alias,omitempty"`
	SuccessURL      string `json:"success_url,omitempty"`
	FailureURL      string `json:"failure_url,omitempty"`
	PayMethods      []int  `json:"pay_methods,omitempty"`
}

type InfiniCreateOrderResponse struct {
	OrderID         string `json:"order_id"`
	RequestID       string `json:"request_id"`
	CheckoutURL     string `json:"checkout_url"`
	ClientReference string `json:"client_reference"`
}

type InfiniWithdrawRequest struct {
	Chain         string `json:"chain"`
	TokenType     string `json:"token_type"`
	Amount        string `json:"amount"`
	WalletAddress string `json:"wallet_address"`
	Note          string `json:"note,omitempty"`
}

type InfiniWithdrawResponse struct {
	RequestID string `json:"request_id"`
}

type InfiniAPIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func getInfiniClient() *InfiniClient {
	if setting.InfiniKeyID == "" || setting.InfiniSecretKey == "" {
		return nil
	}
	baseURL := setting.InfiniBaseURL
	if baseURL == "" {
		baseURL = "https://openapi-sandbox.infini.money"
	}
	return &InfiniClient{
		KeyID:     setting.InfiniKeyID,
		SecretKey: setting.InfiniSecretKey,
		BaseURL:   baseURL,
	}
}

// Sign generates HMAC-SHA256 signature for Infini API requests.
// signing_string = "{keyId}\n{METHOD} {path}\ndate: {GMT_time}\n"
func (c *InfiniClient) Sign(method, path, gmtTime string) string {
	signingString := fmt.Sprintf("%s\n%s %s\ndate: %s\n", c.KeyID, strings.ToUpper(method), path, gmtTime)
	mac := hmac.New(sha256.New, []byte(c.SecretKey))
	mac.Write([]byte(signingString))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// BuildHeaders constructs the authentication headers for Infini API requests.
func (c *InfiniClient) BuildHeaders(method, path string, body []byte) map[string]string {
	gmtTime := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	signature := c.Sign(method, path, gmtTime)

	headers := map[string]string{
		"Date": gmtTime,
		"Authorization": fmt.Sprintf(
			`Signature keyId="%s",algorithm="hmac-sha256",headers="@request-target date",signature="%s"`,
			c.KeyID, signature,
		),
	}

	if len(body) > 0 {
		digest := sha256.Sum256(body)
		headers["Digest"] = "SHA-256=" + base64.StdEncoding.EncodeToString(digest[:])
		headers["Content-Type"] = "application/json"
	}

	return headers
}

// doRequest sends an HTTP request to Infini API and returns the response body.
func (c *InfiniClient) doRequest(method, path string, body []byte) ([]byte, error) {
	url := c.BaseURL + path
	headers := c.BuildHeaders(method, path, body)

	var req *http.Request
	var err error
	if len(body) > 0 {
		req, err = http.NewRequest(method, url, strings.NewReader(string(body)))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("infini: create request failed: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("infini: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("infini: read response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("infini: API error status=%d body=%s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// CreateOrder creates a payment order via Infini and returns the checkout URL.
func (c *InfiniClient) CreateOrder(req InfiniCreateOrderRequest) (*InfiniCreateOrderResponse, error) {
	body, err := common.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("infini: marshal request failed: %w", err)
	}

	respBody, err := c.doRequest("POST", "/v1/acquiring/order", body)
	if err != nil {
		return nil, err
	}

	var apiResp InfiniAPIResponse
	if err := common.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("infini: unmarshal response failed: %w", err)
	}

	if apiResp.Code != 0 {
		return nil, fmt.Errorf("infini: API error code=%d message=%s", apiResp.Code, apiResp.Message)
	}

	var result InfiniCreateOrderResponse
	if err := common.Unmarshal(apiResp.Data, &result); err != nil {
		return nil, fmt.Errorf("infini: unmarshal order data failed: %w", err)
	}

	return &result, nil
}

// QueryOrder queries an order status by order_id.
func (c *InfiniClient) QueryOrder(orderID string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/v1/acquiring/order?order_id=%s", orderID)
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var apiResp InfiniAPIResponse
	if err := common.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("infini: unmarshal response failed: %w", err)
	}

	if apiResp.Code != 0 {
		return nil, fmt.Errorf("infini: API error code=%d message=%s", apiResp.Code, apiResp.Message)
	}

	var result map[string]interface{}
	if err := common.Unmarshal(apiResp.Data, &result); err != nil {
		return nil, fmt.Errorf("infini: unmarshal order data failed: %w", err)
	}

	return result, nil
}

// Withdraw requests a fund withdrawal from Infini account.
func (c *InfiniClient) Withdraw(req InfiniWithdrawRequest) (*InfiniWithdrawResponse, error) {
	body, err := common.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("infini: marshal request failed: %w", err)
	}

	respBody, err := c.doRequest("POST", "/v1/acquiring/fund/withdraw", body)
	if err != nil {
		return nil, err
	}

	var apiResp InfiniAPIResponse
	if err := common.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("infini: unmarshal response failed: %w", err)
	}

	if apiResp.Code != 0 {
		return nil, fmt.Errorf("infini: API error code=%d message=%s", apiResp.Code, apiResp.Message)
	}

	var result InfiniWithdrawResponse
	if err := common.Unmarshal(apiResp.Data, &result); err != nil {
		return nil, fmt.Errorf("infini: unmarshal withdraw data failed: %w", err)
	}

	return &result, nil
}

// VerifyWebhook verifies the HMAC-SHA256 signature of an Infini webhook notification.
// signed_content = "{timestamp}.{event_id}.{payload}"
func (c *InfiniClient) VerifyWebhook(timestamp, eventID, payload, signature string) bool {
	webhookSecret := setting.InfiniWebhookSecret
	if webhookSecret == "" {
		return false
	}

	signedContent := fmt.Sprintf("%s.%s.%s", timestamp, eventID, payload)
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write([]byte(signedContent))
	expectedSig := fmt.Sprintf("%x", mac.Sum(nil))

	return hmac.Equal([]byte(expectedSig), []byte(signature))
}
