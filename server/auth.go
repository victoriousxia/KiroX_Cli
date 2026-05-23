package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var jwtSecret []byte

func init() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "kirox-default-secret-change-me"
	}
	jwtSecret = []byte(secret)
}

func getAdminPassword() string {
	pw := os.Getenv("ADMIN_PASSWORD")
	if pw == "" {
		pw = "admin"
	}
	return pw
}

type LoginRequest struct {
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func HandleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Password != getAdminPassword() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wrong password"})
		return
	}

	token, err := generateJWT()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{Token: token})
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if !validateJWT(token) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func generateJWT() (string, error) {
	header := base64url(mustJSON(map[string]string{"alg": "HS256", "typ": "JWT"}))
	payload := base64url(mustJSON(map[string]interface{}{
		"sub": "admin",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}))

	sigInput := header + "." + payload
	sig := signHMAC(sigInput)
	return sigInput + "." + sig, nil
}

func validateJWT(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}

	sigInput := parts[0] + "." + parts[1]
	expectedSig := signHMAC(sigInput)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return false
	}

	exp, ok := payload["exp"].(float64)
	if !ok || time.Now().Unix() > int64(exp) {
		return false
	}

	return true
}

func signHMAC(input string) string {
	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func base64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
