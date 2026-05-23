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

type connEntry struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type LogHub struct {
	clients map[string]map[*connEntry]bool
	mu      sync.RWMutex
}

func NewLogHub() *LogHub {
	return &LogHub{
		clients: make(map[string]map[*connEntry]bool),
	}
}

func (h *LogHub) Subscribe(taskID string, conn *websocket.Conn) *connEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	entry := &connEntry{conn: conn}
	if h.clients[taskID] == nil {
		h.clients[taskID] = make(map[*connEntry]bool)
	}
	h.clients[taskID][entry] = true
	return entry
}

func (h *LogHub) Unsubscribe(taskID string, entry *connEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.clients[taskID]; ok {
		delete(conns, entry)
		if len(conns) == 0 {
			delete(h.clients, taskID)
		}
	}
}

func (h *LogHub) Send(taskID, message string) {
	h.mu.RLock()
	snapshot := make([]*connEntry, 0, len(h.clients[taskID]))
	for entry := range h.clients[taskID] {
		snapshot = append(snapshot, entry)
	}
	h.mu.RUnlock()

	msg := LogMessage{
		TaskID:    taskID,
		Message:   message,
		Timestamp: time.Now().Format("15:04:05"),
	}

	for _, entry := range snapshot {
		entry.mu.Lock()
		entry.conn.WriteJSON(msg)
		entry.mu.Unlock()
	}
}
