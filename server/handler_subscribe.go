package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"reg_go/internal/core"
)

type SubscribeRequest struct {
	ClientID         string `json:"clientId" binding:"required"`
	ClientSecret     string `json:"clientSecret" binding:"required"`
	RefreshToken     string `json:"refreshToken" binding:"required"`
	Proxy            string `json:"proxy"`
	SubscriptionType string `json:"subscriptionType"`
}

func HandleSubscribe(dataDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SubscribeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
			return
		}

		// 如果没指定代理，从配置文件读取
		proxy := req.Proxy
		if proxy == "" {
			if data, err := os.ReadFile(filepath.Join(dataDir, "config.json")); err == nil {
				var cfg map[string]interface{}
				if json.Unmarshal(data, &cfg) == nil {
					proxy, _ = cfg["proxy"].(string)
				}
			}
		}

		checkoutURL, err := core.CreateSubscriptionToken(
			req.ClientID, req.ClientSecret, req.RefreshToken, proxy, req.SubscriptionType,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"checkoutUrl": checkoutURL})
	}
}
