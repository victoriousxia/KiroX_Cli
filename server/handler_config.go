package server

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

type AppConfig struct {
	Proxy      string `json:"proxy"`
	MoEmailURL string `json:"moEmailUrl"`
	MoEmailKey string `json:"moEmailKey"`
}

func HandleGetConfig(dataDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := AppConfig{
			Proxy:      os.Getenv("PROXY"),
			MoEmailURL: os.Getenv("MOEMAIL_BASE_URL"),
			MoEmailKey: os.Getenv("MOEMAIL_API_KEY"),
		}
		if cfg.MoEmailURL == "" {
			cfg.MoEmailURL = "https://api.moemail.app"
		}
		c.JSON(http.StatusOK, gin.H{"config": cfg})
	}
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
		if cfg.MoEmailURL != "" {
			content += "MOEMAIL_BASE_URL=" + cfg.MoEmailURL + "\n"
		}
		if cfg.MoEmailKey != "" {
			content += "MOEMAIL_API_KEY=" + cfg.MoEmailKey + "\n"
		}

		if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config: " + err.Error()})
			return
		}

		if cfg.Proxy != "" {
			os.Setenv("PROXY", cfg.Proxy)
		}
		if cfg.MoEmailURL != "" {
			os.Setenv("MOEMAIL_BASE_URL", cfg.MoEmailURL)
		}
		if cfg.MoEmailKey != "" {
			os.Setenv("MOEMAIL_API_KEY", cfg.MoEmailKey)
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
