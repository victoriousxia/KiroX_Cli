package email

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
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
			Raw     string `json:"raw"`
		} `json:"results"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if len(result.Results) == 0 {
		return "", nil
	}

	for _, m := range result.Results {
		log.Printf("[CfEmail] 收到邮件: id=%d, source=%s, subject=%s, message_len=%d, raw_len=%d",
			m.ID, m.Source, m.Subject, len(m.Message), len(m.Raw))

		// 先尝试从列表字段直接提取
		text := m.Subject + " " + m.Message
		if code := ExtractCode(text); code != "" {
			return code, nil
		}

		// 尝试从列表中的 raw 字段解析 MIME 提取
		if m.Raw != "" {
			if code := extractCodeFromRaw(m.Raw); code != "" {
				log.Printf("[CfEmail] 从列表 raw 字段提取到验证码 id=%d", m.ID)
				return code, nil
			}
		}

		// 请求单封邮件详情 (parsed_mail 端点返回已解析内容)
		detail, err := c.fetchParsedMail(m.ID)
		if err != nil {
			log.Printf("[CfEmail] parsed_mail 失败 id=%d: %v, 尝试 raw 端点", m.ID, err)
			detail, err = c.fetchRawMail(m.ID)
			if err != nil {
				log.Printf("[CfEmail] raw mail 也失败 id=%d: %v", m.ID, err)
				continue
			}
		}
		if code := ExtractCode(detail); code != "" {
			log.Printf("[CfEmail] 从邮件详情提取到验证码 id=%d", m.ID)
			return code, nil
		}
	}

	return "", nil
}

// fetchParsedMail 获取已解析的邮件内容 (/api/parsed_mail/:id)
func (c *CloudflareEmailProvider) fetchParsedMail(mailID int) (string, error) {
	url := fmt.Sprintf("%s/api/parsed_mail/%d", c.baseURL, mailID)

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

	var detail struct {
		Subject string `json:"subject"`
		Text    string `json:"text"`
		HTML    string `json:"html"`
		Message string `json:"message"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		return string(body), nil
	}

	candidates := []string{detail.Text, detail.HTML, detail.Message, detail.Content, detail.Subject}
	for _, candidate := range candidates {
		if candidate != "" {
			if code := ExtractCode(candidate); code != "" {
				return candidate, nil
			}
		}
	}
	return detail.Subject + " " + detail.Text + " " + detail.HTML + " " + detail.Message + " " + detail.Content, nil
}

// fetchRawMail 获取原始邮件内容 (/api/mail/:id 单数)
func (c *CloudflareEmailProvider) fetchRawMail(mailID int) (string, error) {
	url := fmt.Sprintf("%s/api/mail/%d", c.baseURL, mailID)

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

	var detail struct {
		Raw     string `json:"raw"`
		Message string `json:"message"`
		HTML    string `json:"html"`
		Text    string `json:"text"`
		Content string `json:"content"`
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		return extractTextFromMIME(string(body)), nil
	}

	// 如果有 raw 字段，解析 MIME
	if detail.Raw != "" {
		parsed := extractTextFromMIME(detail.Raw)
		if code := ExtractCode(parsed); code != "" {
			return parsed, nil
		}
		if code := ExtractCode(detail.Raw); code != "" {
			return detail.Raw, nil
		}
	}

	candidates := []string{detail.Text, detail.HTML, detail.Message, detail.Content}
	for _, candidate := range candidates {
		if candidate != "" {
			if code := ExtractCode(candidate); code != "" {
				return candidate, nil
			}
		}
	}
	return detail.Subject + " " + detail.Raw + " " + detail.Text + " " + detail.HTML + " " + detail.Message + " " + detail.Content, nil
}

// extractCodeFromRaw 从原始 MIME 邮件中提取验证码
func extractCodeFromRaw(raw string) string {
	if code := ExtractCode(raw); code != "" {
		return code
	}
	text := extractTextFromMIME(raw)
	if text != "" {
		if code := ExtractCode(text); code != "" {
			return code
		}
	}
	return ""
}

// extractTextFromMIME 解析 MIME 邮件，提取文本内容
func extractTextFromMIME(raw string) string {
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		if decoded, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(raw)); decErr == nil {
			return string(decoded)
		}
		return raw
	}

	contentType := msg.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain"
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		body, _ := io.ReadAll(msg.Body)
		return decodeBody(string(body), msg.Header.Get("Content-Transfer-Encoding"))
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		return parseMultipart(msg.Body, params["boundary"])
	}

	body, _ := io.ReadAll(msg.Body)
	return decodeBody(string(body), msg.Header.Get("Content-Transfer-Encoding"))
}

func parseMultipart(r io.Reader, boundary string) string {
	if boundary == "" {
		body, _ := io.ReadAll(r)
		return string(body)
	}

	mr := multipart.NewReader(r, boundary)
	var texts []string

	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}

		partType := part.Header.Get("Content-Type")
		partEncoding := part.Header.Get("Content-Transfer-Encoding")
		partBody, _ := io.ReadAll(part)
		decoded := decodeBody(string(partBody), partEncoding)

		if strings.Contains(partType, "text/plain") || strings.Contains(partType, "text/html") {
			texts = append(texts, decoded)
		} else if strings.HasPrefix(partType, "multipart/") {
			_, innerParams, _ := mime.ParseMediaType(partType)
			inner := parseMultipart(strings.NewReader(decoded), innerParams["boundary"])
			if inner != "" {
				texts = append(texts, inner)
			}
		}
	}

	return strings.Join(texts, " ")
}

func decodeBody(body, encoding string) string {
	encoding = strings.ToLower(strings.TrimSpace(encoding))
	switch encoding {
	case "base64":
		cleaned := strings.ReplaceAll(body, "\r\n", "")
		cleaned = strings.ReplaceAll(cleaned, "\n", "")
		decoded, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(cleaned)
			if err != nil {
				return body
			}
		}
		return string(decoded)
	case "quoted-printable":
		reader := quotedprintable.NewReader(strings.NewReader(body))
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return body
		}
		return string(decoded)
	default:
		return body
	}
}

func randomName(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
