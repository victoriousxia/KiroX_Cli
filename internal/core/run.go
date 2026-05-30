package core

import (
	"log"
	"math/rand"
	"time"

	"reg_go/internal/crypto"
)

// Run 执行完整注册流程
func (r *Registrar) Run() map[string]interface{} {
	log.Printf("[代理] 当前使用代理: %s", r.Cfg.Proxy)
	crypto.RefreshAppJSConfig(r.Cfg.Proxy)

	steps := []struct {
		name string
		fn   func() error
	}{
		{"OIDC", r.Step1OIDC},
		{"Device", r.Step2Device},
		{"Email", r.Step3Email},
		{"Portal", r.Step4Portal},
		{"WorkflowInit", r.Step5WorkflowInit},
	}

	for _, s := range steps {
		if err := s.fn(); err != nil {
			log.Printf("注册失败 [%s]: %v", s.name, err)
			return map[string]interface{}{"status": "failed", "error": err.Error(), "email": r.Email}
		}
		humanDelay()
	}

	// 步骤 6: 提交邮箱
	longHumanDelay()
	status, err := r.Step6SubmitEmail()
	if err != nil {
		log.Printf("注册失败: %v", err)
		return map[string]interface{}{"status": "failed", "error": err.Error(), "email": r.Email}
	}

	if status == "signup" {
		signupSteps := []struct {
			name string
			fn   func() error
		}{
			{"Signup", r.Step7Signup},
			{"SignupInit", r.Step7_5SignupInit},
			{"ProfileInit", r.Step7_8ProfileInit},
			{"ProfileStart", r.Step8ProfileStart},
			{"SendOTP", r.Step9SendOTP},
		}
		for _, s := range signupSteps {
			if err := s.fn(); err != nil {
				log.Printf("注册失败 [%s]: %v", s.name, err)
				return map[string]interface{}{"status": "failed", "error": err.Error(), "email": r.Email}
			}
			if s.name == "ProfileStart" || s.name == "SendOTP" {
				longHumanDelay()
			} else {
				humanDelay()
			}
		}

		otp, err := r.Step10GetOTP()
		if err != nil {
			log.Printf("注册失败: %v", err)
			return map[string]interface{}{"status": "failed", "error": err.Error(), "email": r.Email}
		}

		otpSteps := []struct {
			name string
			fn   func() error
		}{
			{"CreateIdentity", func() error { return r.Step11CreateIdentity(otp) }},
			{"SetPassword", r.Step12SetPassword},
		}
		for _, s := range otpSteps {
			if err := s.fn(); err != nil {
				log.Printf("注册失败 [%s]: %v", s.name, err)
				return map[string]interface{}{"status": "failed", "error": err.Error(), "email": r.Email}
			}
		}
	} else {
		if r.Cfg.EmailMode == "outlook" {
			return map[string]interface{}{"status": "failed", "error": "邮箱已注册过，跳过", "email": r.Email}
		}
		return map[string]interface{}{"status": "failed", "error": "临时邮箱不可能已存在", "email": r.Email}
	}

	finalSteps := []struct {
		name string
		fn   func() error
	}{
		{"SSOWorkflow", r.Step12_8SSOWorkflow},
	}
	for _, s := range finalSteps {
		if err := s.fn(); err != nil {
			log.Printf("注册失败 [%s]: %v", s.name, err)
			return map[string]interface{}{"status": "failed", "error": err.Error(), "email": r.Email}
		}
	}

	// 注册完成后等待一段时间再验活，模拟真人行为
	time.Sleep(time.Duration(5000+rand.Intn(5001)) * time.Millisecond)

	awsToken, err := r.Step13SSOToken()
	if err != nil {
		log.Printf("注册失败: %v", err)
		return map[string]interface{}{"status": "failed", "error": err.Error(), "email": r.Email}
	}

	kiroCode, err := r.Step14KiroAuthorize()
	if err != nil {
		log.Printf("注册失败: %v", err)
		return map[string]interface{}{"status": "failed", "error": err.Error(), "email": r.Email}
	}

	humanDelay()

	kiroTokens, err := r.Step15KiroExchange(kiroCode)
	if err != nil {
		log.Printf("注册失败: %v", err)
		return map[string]interface{}{"status": "failed", "error": err.Error(), "email": r.Email}
	}

	verify := r.VerifyAlive(awsToken)
	if suspended, _ := verify["suspended"].(bool); suspended {
		log.Println("注册失败! 账号已被封禁 (suspended)")
		return map[string]interface{}{"status": "failed", "error": "suspended", "email": r.Email}
	}

	alive, _ := verify["alive"].(bool)
	if alive {
		log.Println("注册成功! (已验活)")
	} else {
		log.Println("注册完成 (验活失败，但账号可能可用)")
	}

	return map[string]interface{}{
		"email":         r.Email,
		"password":      r.Cfg.Password,
		"status":        "success",
		"client_id":     r.ClientID,
		"client_secret": r.ClientSecret,
		"device_code":   r.DeviceCode,
		"aws_token":     awsToken,
		"kiro_tokens":   kiroTokens,
		"verify":        verify,
	}
}

// humanDelay 模拟人类操作延迟 (1.5-4秒)
func humanDelay() {
	d := time.Duration(1500+rand.Intn(2501)) * time.Millisecond
	log.Printf("[延迟] 等待 %.1f..", d.Seconds())
	time.Sleep(d)
}

// longHumanDelay 模拟较长的人类思考延迟 (3-7秒)
func longHumanDelay() {
	d := time.Duration(3000+rand.Intn(4001)) * time.Millisecond
	log.Printf("[延迟] 长等待 %.1f 秒...", d.Seconds())
	time.Sleep(d)
}
