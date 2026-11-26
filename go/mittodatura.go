package mittodatura

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s (%d)", e.Code, e.Message, e.StatusCode)
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) GetCatalog(ctx context.Context) (map[string]any, error) {
	return c.do(ctx, http.MethodGet, c.baseURL+"/api/v1/catalog", nil, nil)
}

func (c *Client) CreateOrder(ctx context.Context, externalOrderID, productSKU string, payload map[string]any) (map[string]any, error) {
	body := map[string]any{
		"productSku":      productSKU,
		"customerPayload": payload,
	}
	headers := map[string]string{
		"Idempotency-Key": externalOrderID,
	}
	return c.do(ctx, http.MethodPost, c.baseURL+"/api/v1/orders", body, headers)
}

func (c *Client) GetOrder(ctx context.Context, orderID string) (map[string]any, error) {
	return c.do(ctx, http.MethodGet, c.baseURL+"/api/v1/orders/"+orderID, nil, nil)
}

func (c *Client) FulfillAndWait(ctx context.Context, externalOrderID, productSKU string, payload map[string]any, pollInterval, timeout time.Duration) (map[string]any, error) {
	created, err := c.CreateOrder(ctx, externalOrderID, productSKU, payload)
	if err != nil {
		return nil, err
	}

	orderID, _ := created["orderId"].(string)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		state, err := c.GetOrder(ctx, orderID)
		if err != nil {
			return nil, err
		}
		status, _ := state["status"].(string)
		if status == "COMPLETED" || status == "FAILED" || status == "REFUNDED" {
			return state, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}

	return nil, fmt.Errorf("order %s did not finish within %s", orderID, timeout)
}

func VerifyWebhook(secret, rawBody, timestamp, signature string, maxSkewSec int64) bool {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if int64(math.Abs(float64(time.Now().Unix()-ts))) > maxSkewSec {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", ts, rawBody)))
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(strings.ToLower(strings.TrimSpace(signature))))
}

func (c *Client) do(ctx context.Context, method, targetURL string, body any, extraHeaders map[string]string) (map[string]any, error) {
	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if len(respBytes) > 0 {
		_ = json.Unmarshal(respBytes, &data)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return data, nil
	}

	code := fmt.Sprintf("HTTP_%d", resp.StatusCode)
	msg := string(respBytes)
	if errObj, ok := data["error"].(map[string]any); ok {
		if cStr, ok := errObj["code"].(string); ok && cStr != "" {
			code = cStr
		}
		if mStr, ok := errObj["message"].(string); ok && mStr != "" {
			msg = mStr
		}
	}

	return nil, &APIError{Code: code, Message: msg, StatusCode: resp.StatusCode}
}
