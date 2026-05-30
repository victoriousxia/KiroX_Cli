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

// Q Developer 公共 profileArn（所有 Builder ID 用户共用）
const qProfileArn = "arn%3Aaws%3Acodewhisperer%3Aus-east-1%3A638616132270%3Aprofile%2FAAAACCCCXXXX"

// 真实 Kiro IDE User-Agent
const kiroUA = "aws-sdk-js/1.0.0 ua/2.1 os/windows lang/js md/nodejs#22.22.0 api/codewhispererruntime#1.0.0 m/N,E KiroIDE-0.12.263"
const kiroXAmzUA = "aws-sdk-js/1.0.0 KiroIDE-0.12.263"

type endpointResult struct {
	body      []byte
	ok        bool
	suspended bool
}

func checkEndpointResponse(url string, statusCode int, body []byte) endpointResult {
	if statusCode == 403 {
		log.Printf("账号已被封禁 (403) [%s]: %s", url, string(body))
		return endpointResult{suspended: true}
	}
	if statusCode != 200 {
		log.Printf("端点查询失败 [%s]: %d %s", url, statusCode, string(body))
		return endpointResult{}
	}
	return endpointResult{body: body, ok: true}
}

func queryGetEndpoint(client interface{ Do(req *fhttp.Request) (*fhttp.Response, error) }, access, url string) endpointResult {
	req, _ := fhttp.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("User-Agent", kiroUA)
	req.Header.Set("x-amz-user-agent", kiroXAmzUA)
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
	req.Header.Set("amz-sdk-invocation-id", NewUUID())

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("端点查询异常 [%s]: %v", url, err)
		return endpointResult{}
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return checkEndpointResponse(url, resp.StatusCode, body)
}

// VerifyAlive 验活: 刷新 Token + 查用量 + 查模型
func (r *Registrar) VerifyAlive(awsToken map[string]interface{}) map[string]interface{} {
	log.Println("[验活] 刷新 Token + 查用量 + 查模型")
	client := httputil.NewTLSClient(r.Cfg.Proxy, true, r.Identity.ChromeVer)

	refreshToken, _ := awsToken["refreshToken"].(string)

	tokenBody, _ := json.Marshal(map[string]string{
		"clientId":     r.ClientID,
		"clientSecret": r.ClientSecret,
		"refreshToken": refreshToken,
		"grantType":    "refresh_token",
	})
	req, _ := fhttp.NewRequest("POST", "https://oidc.us-east-1.amazonaws.com/token",
		bytes.NewReader(tokenBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("验活异常: %v", err)
		return map[string]interface{}{"alive": false, "error": err.Error()}
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("Token 刷新失败: %d", resp.StatusCode)
		return map[string]interface{}{"alive": false, "error": fmt.Sprintf("refresh failed: %d", resp.StatusCode)}
	}

	var tok map[string]interface{}
	json.Unmarshal(body, &tok)
	access, _ := tok["accessToken"].(string)
	expiresIn, _ := tok["expiresIn"].(float64)
	log.Printf("Token 刷新成功, expiresIn=%ds", int(expiresIn))

	endpoints := []string{
		"https://q.us-east-1.amazonaws.com/getUsageLimits?origin=AI_EDITOR&profileArn=" + qProfileArn + "&resourceType=AGENTIC_REQUEST&isEmailRequired=true",
		"https://q.us-east-1.amazonaws.com/ListAvailableModels?origin=AI_EDITOR&profileArn=" + qProfileArn,
	}

	var usageRes endpointResult
	for i, ep := range endpoints {
		res := queryGetEndpoint(client, access, ep)
		if res.suspended {
			return map[string]interface{}{"alive": false, "suspended": true, "error": "suspended"}
		}
		if i == 0 {
			usageRes = res
		}
	}

	if !usageRes.ok {
		return map[string]interface{}{"alive": false, "error": "usage query failed"}
	}

	return r.parseUsage(usageRes.body)
}

func (r *Registrar) parseUsage(body []byte) map[string]interface{} {
	var usage map[string]interface{}
	json.Unmarshal(body, &usage)

	userInfo, _ := usage["userInfo"].(map[string]interface{})
	emailAddr, _ := userInfo["email"].(string)
	subInfo, _ := usage["subscriptionInfo"].(map[string]interface{})
	sub, _ := subInfo["subscriptionTitle"].(string)
	if sub == "" {
		sub = "Free"
	}

	var totalLimit, totalUsed float64
	if breakdown, ok := usage["usageBreakdownList"].([]interface{}); ok {
		for _, item := range breakdown {
			b, _ := item.(map[string]interface{})
			rt, _ := b["resourceType"].(string)
			dn, _ := b["displayName"].(string)
			if rt == "CREDIT" || dn == "Credits" {
				baseLimit, _ := b["usageLimitWithPrecision"].(float64)
				if baseLimit == 0 {
					baseLimit, _ = b["usageLimit"].(float64)
				}
				baseUsed, _ := b["currentUsageWithPrecision"].(float64)
				if baseUsed == 0 {
					baseUsed, _ = b["currentUsage"].(float64)
				}
				totalLimit = baseLimit
				totalUsed = baseUsed

				if ft, ok := b["freeTrialInfo"].(map[string]interface{}); ok {
					if ftStatus, _ := ft["freeTrialStatus"].(string); ftStatus == "ACTIVE" {
						ftLimit, _ := ft["usageLimitWithPrecision"].(float64)
						ftUsed, _ := ft["currentUsageWithPrecision"].(float64)
						totalLimit += ftLimit
						totalUsed += ftUsed
					}
				}
				break
			}
		}
	}

	log.Printf("验活成功! 邮箱=%s 订阅=%s Credit=%.1f/%.1f", emailAddr, sub, totalUsed, totalLimit)
	return map[string]interface{}{
		"alive": true, "email": emailAddr, "subscription": sub,
		"credit_used": totalUsed, "credit_limit": totalLimit,
	}
}

// VerifyAccountResult 独立验证结果
type VerifyAccountResult struct {
	Alive        bool    `json:"alive"`
	Email        string  `json:"email"`
	Subscription string  `json:"subscription"`
	CreditUsed   float64 `json:"creditUsed"`
	CreditLimit  float64 `json:"creditLimit"`
	Suspended    bool    `json:"suspended"`
	Error        string  `json:"error,omitempty"`
}

func queryEndpointStandalone(client interface{ Do(req *fhttp.Request) (*fhttp.Response, error) }, access, url string) endpointResult {
	return queryGetEndpoint(client, access, url)
}

// VerifyAccount 独立验证函数，不依赖 Registrar 实例
func VerifyAccount(clientID, clientSecret, refreshToken, proxy string) VerifyAccountResult {
	client := httputil.NewTLSClient(proxy, true, "137.0.0.0")

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
		return VerifyAccountResult{Error: err.Error()}
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		return VerifyAccountResult{Error: fmt.Sprintf("refresh failed: %d", resp.StatusCode)}
	}

	var tok map[string]interface{}
	json.Unmarshal(body, &tok)
	access, _ := tok["accessToken"].(string)

	endpoints := []string{
		"https://q.us-east-1.amazonaws.com/getUsageLimits?origin=AI_EDITOR&profileArn=" + qProfileArn + "&resourceType=AGENTIC_REQUEST&isEmailRequired=true",
		"https://q.us-east-1.amazonaws.com/ListAvailableModels?origin=AI_EDITOR&profileArn=" + qProfileArn,
	}

	for _, ep := range endpoints {
		res := queryEndpointStandalone(client, access, ep)
		if res.suspended {
			return VerifyAccountResult{Suspended: true, Error: "suspended"}
		}
	}

	res := queryEndpointStandalone(client, access, endpoints[0])
	if !res.ok {
		return VerifyAccountResult{Error: "usage query failed"}
	}

	return parseUsageStandalone(res.body)
}

func parseUsageStandalone(body []byte) VerifyAccountResult {
	var usage map[string]interface{}
	json.Unmarshal(body, &usage)

	userInfo, _ := usage["userInfo"].(map[string]interface{})
	emailAddr, _ := userInfo["email"].(string)
	subInfo, _ := usage["subscriptionInfo"].(map[string]interface{})
	sub, _ := subInfo["subscriptionTitle"].(string)
	if sub == "" {
		sub = "Free"
	}

	var totalLimit, totalUsed float64
	if breakdown, ok := usage["usageBreakdownList"].([]interface{}); ok {
		for _, item := range breakdown {
			b, _ := item.(map[string]interface{})
			rt, _ := b["resourceType"].(string)
			dn, _ := b["displayName"].(string)
			if rt == "CREDIT" || dn == "Credits" {
				baseLimit, _ := b["usageLimitWithPrecision"].(float64)
				if baseLimit == 0 {
					baseLimit, _ = b["usageLimit"].(float64)
				}
				baseUsed, _ := b["currentUsageWithPrecision"].(float64)
				if baseUsed == 0 {
					baseUsed, _ = b["currentUsage"].(float64)
				}
				totalLimit = baseLimit
				totalUsed = baseUsed

				if ft, ok := b["freeTrialInfo"].(map[string]interface{}); ok {
					if ftStatus, _ := ft["freeTrialStatus"].(string); ftStatus == "ACTIVE" {
						ftLimit, _ := ft["usageLimitWithPrecision"].(float64)
						ftUsed, _ := ft["currentUsageWithPrecision"].(float64)
						totalLimit += ftLimit
						totalUsed += ftUsed
					}
				}
				break
			}
		}
	}

	return VerifyAccountResult{
		Alive:        true,
		Email:        emailAddr,
		Subscription: sub,
		CreditUsed:   totalUsed,
		CreditLimit:  totalLimit,
	}
}
