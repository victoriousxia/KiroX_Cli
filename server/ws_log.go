package server

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type LogMessage struct {
	TaskID    string `json:"taskId"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type LogHub struct {
	clients map[string]map[*websocket.Conn]bool
	mu      sync.RWMutex
}

func NewLogHub() *LogHub {
	return &LogHub{
		clients: make(map[string]map[*websocket.Conn]bool),
	}
}

func (h *LogHub) Subscribe(taskID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[taskID] == nil {
		h.clients[taskID] = make(map[*websocket.Conn]bool)
	}
	h.clients[taskID][conn] = true
}

func (h *LogHub) Unsubscribe(taskID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.clients[taskID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.clients, taskID)
		}
	}
}

func (h *LogHub) Send(taskID, message string) {
	h.mu.RLock()
	conns := h.clients[taskID]
	h.mu.RUnlock()

	msg := LogMessage{
		TaskID:    taskID,
		Message:   message,
		Timestamp: time.Now().Format("15:04:05"),
	}

	for conn := range conns {
		conn.WriteJSON(msg)
	}
}
