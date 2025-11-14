package livelog
package livelog

import (
	"sync"
	"time"
)

// LiveLog represents a live log entry
type LiveLog struct {
	FilePath  string
	Logs      string
	StartTime time.Time
	LastUpdate time.Time
}

// Manager manages live logs for running tasks
type Manager struct {
	mu   sync.RWMutex
	logs map[string]*LiveLog // key: file path
}

var globalManager = &Manager{
	logs: make(map[string]*LiveLog),
}

// GetManager returns the singleton live log manager
func GetManager() *Manager {
	return globalManager
}

// StartTask creates a new live log entry for a task
func (m *Manager) StartTask(filePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logs[filePath] = &LiveLog{
		FilePath:   filePath,
		Logs:       "",
		StartTime:  time.Now(),
		LastUpdate: time.Now(),
	}
}

// AppendLog appends log content to a task's live log
func (m *Manager) AppendLog(filePath, logContent string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if log, exists := m.logs[filePath]; exists {
		log.Logs += logContent
		log.LastUpdate = time.Now()
	}
}

// GetLog retrieves the live log for a task
func (m *Manager) GetLog(filePath string) (*LiveLog, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	log, exists := m.logs[filePath]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	return &LiveLog{
		FilePath:   log.FilePath,
		Logs:       log.Logs,
		StartTime:  log.StartTime,
		LastUpdate: log.LastUpdate,
	}, true
}

// EndTask removes a task's live log (called when task completes)
func (m *Manager) EndTask(filePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.logs, filePath)
}

// GetAllActiveLogs returns all active live logs
func (m *Manager) GetAllActiveLogs() map[string]*LiveLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*LiveLog, len(m.logs))
	for path, log := range m.logs {
		result[path] = &LiveLog{
			FilePath:   log.FilePath,
			Logs:       log.Logs,
			StartTime:  log.StartTime,
			LastUpdate: log.LastUpdate,
		}
	}
	return result
}

// CleanOldLogs removes logs that haven't been updated in a while (cleanup stale logs)
func (m *Manager) CleanOldLogs(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for path, log := range m.logs {
		if now.Sub(log.LastUpdate) > maxAge {
			delete(m.logs, path)
		}
	}
}
