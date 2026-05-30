package core

import (
	"bytes"
	"fmt"
	"io"
	"log"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/fxamacker/cbor/v2"

	httputil "reg_go/internal/http"
)

// StepKiroPortalLogin 调用 Kiro Web Portal 的 InitiateLogin
// 这一步在 Kiro 后端注册用户，授予订阅管理权限
func (r *Registrar) StepKiroPortalLogin() error {
	log.Println("[Kiro Portal] InitiateLogin")
	client := httputil.NewTLSClient(r.Cfg.Proxy, true, r.Identity.ChromeVer)

	// 生成 PKCE
	verifier, challenge := httputil.PKCE()
	_ = verifier // 不需要后续使用 verifier，只需要 challenge
	state := NewUUID()
	visitorID := httputil.KiroVisitorID()

	// 构造 redirectUrl（模拟 Kiro IDE 本地监听端口）
	redirectURL := "http://localhost:3128/signin/callback?login_option=builderid"

	// 构造 CBOR body
	payload := map[string]interface{}{
		"idp":                 "BuilderId",
		"redirectUrl":         redirectURL,
		"state":               state,
		"codeChallenge":       challenge,
		"codeChallengeMethod": "S256",
		"redirectFrom":        "KiroIDE",
	}

	body, err := cbor.Marshal(payload)
	if err != nil {
		return fmt.Errorf("CBOR 编码失败: %v", err)
	}

	// 构造请求
	apiURL := r.Cfg.KiroBase + "/service/KiroWebPortalService/operation/InitiateLogin"
	req, _ := fhttp.NewRequest("POST", apiURL, bytes.NewReader(body))

	// 设置 headers（按抓包顺序）
	referer := fmt.Sprintf("%s/signin?state=%s&code_challenge=%s&code_challenge_method=S256&redirect_uri=http://localhost:3128&redirect_from=KiroIDE",
		r.Cfg.KiroBase, state, challenge)

	req.Header.Set("Accept", "application/cbor")
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set("smithy-protocol", "rpc-v2-cbor")
	req.Header.Set("x-amz-user-agent", "aws-sdk-js/1.0.0 ua/2.1 os/macOS lang/js md/browser#Not-A-Brand_99 m/N,M,E")
	req.Header.Set("x-kiro-visitorid", visitorID)
	req.Header.Set("amz-sdk-invocation-id", NewUUID())
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
	req.Header.Set("User-Agent", r.Identity.UA)
	req.Header.Set("Origin", r.Cfg.KiroBase)
	req.Header.Set("Referer", referer)
	req.Header.Set("sec-ch-ua", r.Identity.SecUA)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")

	// 设置 cookie
	req.Header.Set("Cookie", fmt.Sprintf("kiro-visitor-id=%s", visitorID))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("InitiateLogin 请求失败: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("InitiateLogin 失败: %d %s", resp.StatusCode, string(respBody))
	}

	// 解码 CBOR 响应
	var result map[string]interface{}
	if err := cbor.Unmarshal(respBody, &result); err != nil {
		// 即使解码失败，只要 200 就算成功
		log.Printf("[Kiro Portal] 响应解码失败（但状态 200）: %v", err)
		return nil
	}

	if rURL, ok := result["redirectUrl"].(string); ok && len(rURL) > 0 {
		if len(rURL) > 60 {
			rURL = rURL[:60]
		}
		log.Printf("[Kiro Portal] InitiateLogin 成功, redirectUrl=%s...", rURL)
	} else {
		log.Println("[Kiro Portal] InitiateLogin 成功 (200)")
	}

	return nil
}
