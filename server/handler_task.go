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
		type TaskSummary struct {
			ID        string     `json:"id"`
			Status    TaskStatus `json:"status"`
			Config    TaskConfig `json:"config"`
			Success   int        `json:"success"`
			Failed    int        `json:"failed"`
			Total     int        `json:"total"`
			CreatedAt string     `json:"createdAt"`
			StartedAt string     `json:"startedAt,omitempty"`
			EndedAt   string     `json:"endedAt,omitempty"`
		}
		summaries := make([]TaskSummary, 0, len(tasks))
		for _, t := range tasks {
			t.mu.Lock()
			summaries = append(summaries, TaskSummary{
				ID: t.ID, Status: t.Status, Config: t.Config,
				Success: t.Success, Failed: t.Failed, Total: t.Total,
				CreatedAt: t.CreatedAt, StartedAt: t.StartedAt, EndedAt: t.EndedAt,
			})
			t.mu.Unlock()
		}
		c.JSON(http.StatusOK, gin.H{"tasks": summaries})
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
		task.mu.Lock()
		snapshot := struct {
			ID        string       `json:"id"`
			Status    TaskStatus   `json:"status"`
			Config    TaskConfig   `json:"config"`
			Results   []TaskResult `json:"results"`
			Logs      []TaskLog    `json:"logs,omitempty"`
			Success   int          `json:"success"`
			Failed    int          `json:"failed"`
			Total     int          `json:"total"`
			CreatedAt string       `json:"createdAt"`
			StartedAt string       `json:"startedAt,omitempty"`
			EndedAt   string       `json:"endedAt,omitempty"`
		}{
			ID: task.ID, Status: task.Status, Config: task.Config,
			Results: task.Results, Logs: task.Logs,
			Success: task.Success, Failed: task.Failed, Total: task.Total,
			CreatedAt: task.CreatedAt, StartedAt: task.StartedAt, EndedAt: task.EndedAt,
		}
		task.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{"task": snapshot})
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
