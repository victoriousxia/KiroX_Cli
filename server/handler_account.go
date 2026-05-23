package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"reg_go/internal/core"
)

func HandleListAccounts(tm *TaskManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		results := tm.GetAllResults()
		if results == nil {
			results = []map[string]interface{}{}
		}
		c.JSON(http.StatusOK, gin.H{"accounts": results})
	}
}

type VerifyRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RefreshToken string `json:"refreshToken"`
	Proxy        string `json:"proxy"`
}

func HandleVerifyAccounts(tm *TaskManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var accounts []VerifyRequest
		if err := c.ShouldBindJSON(&accounts); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
			return
		}

		var results []core.VerifyAccountResult
		for _, acc := range accounts {
			result := core.VerifyAccount(acc.ClientID, acc.ClientSecret, acc.RefreshToken, acc.Proxy)
			results = append(results, result)
		}

		c.JSON(http.StatusOK, gin.H{"results": results})
	}
}
