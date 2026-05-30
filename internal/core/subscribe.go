package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"

	fhttp "github.com/bogdanfinn/fhttp"
	httputil "reg_go/internal/http"
)

// CreateSubscriptionToken 调用 Q API 获取 Stripe 订阅支付链接
// 需要 Outlook 邮箱注册的账号才有权限
func CreateSubscriptionToken(clientID, clientSecret, refreshToken, proxy, subscriptionType string) (string, error) {
	if subscriptionType == "" {
		subscriptionType = "Q_DEVELOPER_STANDALONE_PRO_PLUS"
	}

	client := httputil.NewTLSClient(proxy, true, "137.0.0.0")

	// 1. 刷新 token
	tokenBody, _ := json.Marshal(map[string]string{
		"clientId":     clientID,
		"clientSecret": clientSecret,
		"refreshToken": refreshToken,
		"grantType":    "refresh_token",
	})
	req, _ := fhttp.NewRequest("POST", "https://oidc.us-east-1.amazonaws.com/token",
		bytes.NewReader(tokenBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token 刷新失败: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("token 刷新失败: %d", resp.StatusCode)
	}

	var tok map[string]interface{}
	json.Unmarshal(body, &tok)
	accessToken, _ := tok["accessToken"].(string)
	if accessToken == "" {
		return "", fmt.Errorf("无 accessToken")
	}

	// 2. 调用 CreateSubscriptionToken
	clientToken := NewUUID()
	payload, _ := json.Marshal(map[string]string{
		"clientToken":      clientToken,
		"profileArn":       "arn:aws:codewhisperer:us-east-1:638616132270:profile/AAAACCCCXXXX",
		"provider":         "STRIPE",
		"subscriptionType": subscriptionType,
	})

	apiURL := "https://q.us-east-1.amazonaws.com/CreateSubscriptionToken"
	req, _ = fhttp.NewRequest("POST", apiURL, bytes.NewReader(payload))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", kiroUA)
	req.Header.Set("x-amz-user-agent", kiroXAmzUA)
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
	req.Header.Set("amz-sdk-invocation-id", NewUUID())

	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("CreateSubscriptionToken 请求失败: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("CreateSubscriptionToken 失败: %d %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	// 提取 checkout URL
	checkoutURL, _ := result["checkoutUrl"].(string)
	if checkoutURL == "" {
		checkoutURL, _ = result["encodedVerificationUrl"].(string)
	}
	if checkoutURL == "" {
		checkoutURL, _ = result["url"].(string)
	}
	if checkoutURL == "" {
		checkoutURL, _ = result["redirectUrl"].(string)
	}

	if checkoutURL == "" {
		log.Printf("[订阅] CreateSubscriptionToken 响应: %s", string(respBody))
		return "", fmt.Errorf("未找到 checkout URL，响应: %s", string(respBody))
	}

	log.Printf("[订阅] 获取到 checkout URL: %s", checkoutURL)
	return checkoutURL, nil
}
