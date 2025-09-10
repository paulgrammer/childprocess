package jobs

import (
	"sync"

	"github.com/gorilla/websocket"
)

// LogStreamer manages log subscribers for jobs
type LogStreamer struct {
	mu          sync.RWMutex
	subscribers map[string][]*websocket.Conn
}

// NewLogStreamer creates a new LogStreamer
func NewLogStreamer() *LogStreamer {
	return &LogStreamer{
		subscribers: make(map[string][]*websocket.Conn),
	}
}

// Subscribe adds a new subscriber to a job's log stream
func (ls *LogStreamer) Subscribe(jobID string, conn *websocket.Conn) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.subscribers[jobID] = append(ls.subscribers[jobID], conn)
}

// Unsubscribe removes a subscriber from a job's log stream
func (ls *LogStreamer) Unsubscribe(jobID string, conn *websocket.Conn) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	subscribers := ls.subscribers[jobID]
	for i, s := range subscribers {
		if s == conn {
			ls.subscribers[jobID] = append(subscribers[:i], subscribers[i+1:]...)
			break
		}
	}
}

// Broadcast sends a log message to all subscribers of a job
func (ls *LogStreamer) Broadcast(jobID string, message []byte) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	subscribers := ls.subscribers[jobID]
	for _, conn := range subscribers {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			// Handle error, e.g., remove the connection
		}
	}
}

// Close closes all connections for a job
func (ls *LogStreamer) Close(jobID string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	subscribers := ls.subscribers[jobID]
	for _, conn := range subscribers {
		conn.Close()
	}
	delete(ls.subscribers, jobID)
}
