package server

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewServer(dataDir string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	logHub := NewLogHub()
	tm := NewTaskManager(logHub, dataDir)

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Public routes
	r.POST("/api/login", HandleLogin)
	r.GET("/api/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "running",
			"version": "1.0.0",
			"time":    time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	// Protected routes
	api := r.Group("/api")
	api.Use(AuthMiddleware())
	{
		api.POST("/tasks", HandleCreateTask(tm))
		api.GET("/tasks", HandleListTasks(tm))
		api.GET("/tasks/:id", HandleGetTask(tm))
		api.POST("/tasks/:id/stop", HandleStopTask(tm))

		api.GET("/accounts", HandleListAccounts(tm))
		api.POST("/accounts/verify", HandleVerifyAccounts(tm))

		api.GET("/config", HandleGetConfig(dataDir))
		api.POST("/config", HandleUpdateConfig(dataDir))
		api.POST("/config/outlook", HandleUploadOutlook(dataDir))
		api.GET("/config/outlook", HandleGetOutlookAccounts(dataDir))
	}

	// WebSocket endpoint
	r.GET("/ws/logs/:taskId", func(c *gin.Context) {
		taskID := c.Param("taskId")
		conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		entry := logHub.Subscribe(taskID, conn)
		defer logHub.Unsubscribe(taskID, entry)

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	})

	// Serve embedded frontend
	distFS, err := fs.Sub(FrontendFS, "dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(distFS))
		r.NoRoute(func(c *gin.Context) {
			// Try to serve the file directly
			path := c.Request.URL.Path
			f, err := distFS.Open(path[1:]) // strip leading /
			if err == nil {
				f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			// SPA fallback: serve index.html for all unmatched routes
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	return r
}
