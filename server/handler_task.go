package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleCreateTask(tm *TaskManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var cfg TaskConfig
		if err := c.ShouldBindJSON(&cfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
			return
		}
		if cfg.Count < 1 {
			cfg.Count = 1
		}
		if cfg.Concurrency < 1 {
			cfg.Concurrency = 1
		}
		task := tm.CreateTask(cfg)
		c.JSON(http.StatusOK, gin.H{"task": task})
	}
}

func HandleListTasks(tm *TaskManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tasks := tm.ListTasks()
		c.JSON(http.StatusOK, gin.H{"tasks": tasks})
	}
}

func HandleGetTask(tm *TaskManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		task := tm.GetTask(id)
		if task == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"task": task})
	}
}

func HandleStopTask(tm *TaskManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ok := tm.StopTask(id)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "task not found or not running"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "task stopped"})
	}
}
