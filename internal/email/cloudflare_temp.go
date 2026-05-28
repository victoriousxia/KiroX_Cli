package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"time"

	http "github.com/bogdanfinn/fhttp"
	httputil "reg_go/internal/http"
	tls_client "github.com/bogdanfinn/tls-client"
)

type CloudflareEmailProvider struct {
	baseURL   string
	adminAuth string
	address   string
	jwt       string
	client    tls_client.HttpClient
}

func NewCloudflareEmailProvider(baseURL, adminAuth, proxy, chromeVer string) *CloudflareEmailProvider {
	return &CloudflareEmailProvider{
		baseURL:   baseURL,
		adminAuth: adminAuth,
		client:    httputil.NewTLSClient(proxy, true, chromeVer),
	}
}

func (c *CloudflareEmailProvider) getDomains() ([]string, error) {
	url := c.baseURL + "/open_api/settings"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		DefaultDomains []string `json:"defaultDomains"`
		Domains        []string `json:"domains"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	domains := result.DefaultDomains
	if len(domains) == 0 {
		domains = result.Domains
	}
	if len(domains) == 0 {
		return nil, fmt.Errorf("没有可用的域名")
	}
	return domains, nil
}

func (c *CloudflareEmailProvider) Create() string {
	domains, err := c.getDomains()
	if err != nil {
		log.Printf("[CfEmail] 获取域名列表失败: %v", err)
		return ""
	}

	domain := domains[0]
	name := randomName(10)
	log.Printf("[CfEmail] 使用域名: %s, 地址: %s", domain, name)

	url := c.baseURL + "/api/new_address"
	payload := map[string]interface{}{
		"name":   name,
		"domain": domain,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[CfEmail] 创建请求失败: %v", err)
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	if c.adminAuth != "" {
		req.Header.Set("x-admin-auth", c.adminAuth)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("[CfEmail] 请求失败: %v", err)
		return ""
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Printf("[CfEmail] 创建失败 HTTP %d: %s", resp.StatusCode, string(respBody))
		return ""
	}

	var result struct {
		JWT     string `json:"jwt"`
		Address string `json:"address"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Printf("[CfEmail] 解析响应失败: %v, body: %s", err, string(respBody))
		return ""
	}

	if result.Address == "" || result.JWT == "" {
		log.Printf("[CfEmail] 响应缺少必要字段, body: %s", string(respBody))
		return ""
	}

	c.jwt = result.JWT
	c.address = result.Address
	log.Printf("[CfEmail] 创建成功: %s", c.address)
	return c.address
}

func (c *CloudflareEmailProvider) GetAddress() string {
	return c.address
}

func (c *CloudflareEmailProvider) WaitForCode(timeout, interval int) (string, error) {
	if c.jwt == "" {
		return "", fmt.Errorf("邮箱未创建")
	}

	maxRetries := timeout / interval
	for attempt := 1; attempt <= maxRetries; attempt++ {
		code, err := c.fetchCode()
		if err != nil {
			if attempt%6 == 0 {
				log.Printf("[CfEmail] [%d/%d] 请求异常: %v", attempt, maxRetries, err)
			}
		} else if code != "" {
			return code, nil
		}

		if attempt%6 == 0 && err == nil {
			log.Printf("[CfEmail] [%d/%d] 等待邮件...", attempt, maxRetries)
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}

	return "", fmt.Errorf("等待验证码超时 (%ds)", timeout)
}

func (c *CloudflareEmailProvider) fetchCode() (string, error) {
	url := c.baseURL + "/api/mails?limit=10&offset=0"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.jwt)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Results []struct {
			ID      int    `json:"id"`
			Source  string `json:"source"`
			Subject string `json:"subject"`
			Message string `json:"message"`
		} `json:"results"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if len(result.Results) == 0 {
		return "", nil
	}

	for _, mail := range result.Results {
		log.Printf("[CfEmail] 收到邮件: id=%d, source=%s, subject=%s, message_len=%d", mail.ID, mail.Source, mail.Subject, len(mail.Message))

		text := mail.Subject + " " + mail.Message
		if code := ExtractCode(text); code != "" {
			return code, nil
		}

		detail, err := c.fetchMailDetail(mail.ID)
		if err != nil {
			log.Printf("[CfEmail] 获取邮件详情失败 id=%d: %v", mail.ID, err)
			continue
		}
		if code := ExtractCode(detail); code != "" {
			log.Printf("[CfEmail] 从邮件详情提取到验证码 id=%d", mail.ID)
			return code, nil
		}
	}

	return "", nil
}

func (c *CloudflareEmailProvider) fetchMailDetail(mailID int) (string, error) {
	url := fmt.Sprintf("%s/api/mails/%d", c.baseURL, mailID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.jwt)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("[CfEmail] 邮件详情 id=%d: body_len=%d, body_preview=%.200s", mailID, len(body), string(body))

	var detail struct {
		Raw     string `json:"raw"`
		Message string `json:"message"`
		HTML    string `json:"html"`
		Text    string `json:"text"`
		Content string `json:"content"`
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		// 可能直接返回纯文本
		return string(body), nil
	}

	// 尝试所有可能的内容字段
	candidates := []string{detail.Raw, detail.Message, detail.HTML, detail.Text, detail.Content}
	for _, candidate := range candidates {
		if candidate != "" {
			if code := ExtractCode(candidate); code != "" {
				return candidate, nil
			}
		}
	}
	// 返回所有内容拼接，让上层再尝试提取
	return detail.Subject + " " + detail.Raw + " " + detail.Message + " " + detail.HTML + " " + detail.Text + " " + detail.Content, nil
}

func randomName(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
