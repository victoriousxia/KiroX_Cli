package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	httputil "reg_go/internal/http"
)

type AppConfig struct {
	Proxy          string `json:"proxy"`
	UpstreamProxy  string `json:"upstreamProxy"`
	MoEmailURL     string `json:"moEmailUrl"`
	MoEmailKey     string `json:"moEmailKey"`
	CfEmailURL     string `json:"cfEmailUrl"`
	CfEmailAuth    string `json:"cfEmailAuth"`
}

func HandleGetConfig(dataDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read from saved .env file first, then fall back to env vars
		saved := loadSavedEnv(filepath.Join(dataDir, ".env"))

		cfg := AppConfig{
			Proxy:         saved["PROXY"],
			UpstreamProxy: saved["UPSTREAM_PROXY"],
			MoEmailURL:    saved["MOEMAIL_BASE_URL"],
			MoEmailKey:    saved["MOEMAIL_API_KEY"],
			CfEmailURL:    saved["CF_EMAIL_BASE_URL"],
			CfEmailAuth:   saved["CF_EMAIL_AUTH"],
		}
		// Fall back to environment variables if not in saved file
		if cfg.Proxy == "" {
			cfg.Proxy = os.Getenv("PROXY")
		}
		if cfg.UpstreamProxy == "" {
			cfg.UpstreamProxy = os.Getenv("UPSTREAM_PROXY")
		}
		if cfg.MoEmailURL == "" {
			cfg.MoEmailURL = os.Getenv("MOEMAIL_BASE_URL")
		}
		if cfg.MoEmailKey == "" {
			cfg.MoEmailKey = os.Getenv("MOEMAIL_API_KEY")
		}
		if cfg.CfEmailURL == "" {
			cfg.CfEmailURL = os.Getenv("CF_EMAIL_BASE_URL")
		}
		if cfg.CfEmailAuth == "" {
			cfg.CfEmailAuth = os.Getenv("CF_EMAIL_AUTH")
		}
		if cfg.MoEmailURL == "" {
			cfg.MoEmailURL = "https://api.moemail.app"
		}
		c.JSON(http.StatusOK, gin.H{"config": cfg})
	}
}

func loadSavedEnv(path string) map[string]string {
	result := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

func HandleUpdateConfig(dataDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var cfg AppConfig
		if err := c.ShouldBindJSON(&cfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
			return
		}

		envPath := filepath.Join(dataDir, ".env")
		content := ""
		if cfg.Proxy != "" {
			content += "PROXY=" + cfg.Proxy + "\n"
		}
		if cfg.UpstreamProxy != "" {
			content += "UPSTREAM_PROXY=" + cfg.UpstreamProxy + "\n"
		}
		if cfg.MoEmailURL != "" {
			content += "MOEMAIL_BASE_URL=" + cfg.MoEmailURL + "\n"
		}
		if cfg.MoEmailKey != "" {
			content += "MOEMAIL_API_KEY=" + cfg.MoEmailKey + "\n"
		}
		if cfg.CfEmailURL != "" {
			content += "CF_EMAIL_BASE_URL=" + cfg.CfEmailURL + "\n"
		}
		if cfg.CfEmailAuth != "" {
			content += "CF_EMAIL_AUTH=" + cfg.CfEmailAuth + "\n"
		}

		if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config: " + err.Error()})
			return
		}

		if cfg.Proxy != "" {
			os.Setenv("PROXY", cfg.Proxy)
		} else {
			os.Unsetenv("PROXY")
		}
		if cfg.UpstreamProxy != "" {
			os.Setenv("UPSTREAM_PROXY", cfg.UpstreamProxy)
		} else {
			os.Unsetenv("UPSTREAM_PROXY")
		}
		if cfg.MoEmailURL != "" {
			os.Setenv("MOEMAIL_BASE_URL", cfg.MoEmailURL)
		} else {
			os.Unsetenv("MOEMAIL_BASE_URL")
		}
		if cfg.MoEmailKey != "" {
			os.Setenv("MOEMAIL_API_KEY", cfg.MoEmailKey)
		} else {
			os.Unsetenv("MOEMAIL_API_KEY")
		}
		if cfg.CfEmailURL != "" {
			os.Setenv("CF_EMAIL_BASE_URL", cfg.CfEmailURL)
		} else {
			os.Unsetenv("CF_EMAIL_BASE_URL")
		}
		if cfg.CfEmailAuth != "" {
			os.Setenv("CF_EMAIL_AUTH", cfg.CfEmailAuth)
		} else {
			os.Unsetenv("CF_EMAIL_AUTH")
		}

		c.JSON(http.StatusOK, gin.H{"message": "config updated"})
	}
}

func HandleUploadOutlook(dataDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, _, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no file uploaded: " + err.Error()})
			return
		}
		defer file.Close()

		savePath := filepath.Join(dataDir, "outlook.csv")
		out, err := os.Create(savePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file: " + err.Error()})
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write file: " + err.Error()})
			return
		}

		// Count valid accounts
		data, _ := os.ReadFile(savePath)
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		count := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "----", 4)
			if len(parts) == 4 {
				count++
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "上传成功", "count": count})
	}
}

func HandleGetOutlookAccounts(dataDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		csvPath := filepath.Join(dataDir, "outlook.csv")
		data, err := os.ReadFile(csvPath)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"accounts": []interface{}{}, "count": 0})
			return
		}

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		type OutlookItem struct {
			Email    string `json:"email"`
			ClientID string `json:"clientId"`
		}
		var accounts []OutlookItem
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "----", 4)
			if len(parts) == 4 {
				accounts = append(accounts, OutlookItem{
					Email:    parts[0],
					ClientID: parts[2],
				})
			}
		}
		c.JSON(http.StatusOK, gin.H{"accounts": accounts, "count": len(accounts)})
	}
}

func HandleTestProxy(dataDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Proxy      string `json:"proxy"`
			ChainProxy string `json:"chainProxy"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		proxy := req.Proxy
		if proxy == "" {
			saved := loadSavedEnv(filepath.Join(dataDir, ".env"))
			proxy = saved["PROXY"]
		}
		if proxy == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "未配置代理地址"})
			return
		}

		actualProxy := proxy
		var stopChain func()
		defer func() {
			if stopChain != nil {
				stopChain()
			}
		}()

		if req.ChainProxy != "" {
			localAddr, stop, err := httputil.ProxyChain(req.ChainProxy, proxy)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"error":   "代理链启动失败: " + err.Error(),
					"proxy":   proxy,
				})
				return
			}
			stopChain = stop
			actualProxy = "socks5://" + localAddr
		}

		client := httputil.NewTLSClient(actualProxy, true, "137")

		start := time.Now()
		httpReq, _ := http.NewRequest("GET", "https://api.ip.sb/geoip", nil)
		httpReq.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := client.Do(httpReq)
		elapsed := time.Since(start).Milliseconds()

		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "代理连接失败: " + err.Error(),
				"proxy":   proxy,
			})
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var geoData map[string]interface{}
		json.Unmarshal(body, &geoData)

		ip, _ := geoData["ip"].(string)
		country, _ := geoData["country"].(string)
		region, _ := geoData["region"].(string)
		city, _ := geoData["city"].(string)
		isp, _ := geoData["isp"].(string)
		org, _ := geoData["organization"].(string)
		if isp == "" {
			isp = org
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"proxy":   proxy,
			"ip":      ip,
			"country": country,
			"region":  region,
			"city":    city,
			"isp":     isp,
			"latency": elapsed,
		})
	}
}
