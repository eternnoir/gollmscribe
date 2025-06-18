package watcher

import (
	"sync"
	"time"
)

// processingTracker implements ProcessingTracker interface
type processingTracker struct {
	mu         sync.RWMutex
	processing map[string]time.Time
}

// NewProcessingTracker creates a new processing tracker
func NewProcessingTracker() ProcessingTracker {
	return &processingTracker{
		processing: make(map[string]time.Time),
	}
}

// TryLock attempts to acquire a lock for processing a file
func (pt *processingTracker) TryLock(filepath string) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if _, exists := pt.processing[filepath]; exists {
		return false
	}

	pt.processing[filepath] = time.Now()
	return true
}

// Unlock releases the lock for a file
func (pt *processingTracker) Unlock(filepath string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	delete(pt.processing, filepath)
}

// IsLocked checks if a file is currently locked
func (pt *processingTracker) IsLocked(filepath string) bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	_, exists := pt.processing[filepath]
	return exists
}

// CleanupStale removes locks older than the specified duration
func (pt *processingTracker) CleanupStale(timeout time.Duration) int {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for filepath, startTime := range pt.processing {
		if now.Sub(startTime) > timeout {
			delete(pt.processing, filepath)
			cleaned++
		}
	}

	return cleaned
}

// GetLocked returns all currently locked files
func (pt *processingTracker) GetLocked() []string {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	files := make([]string, 0, len(pt.processing))
	for filepath := range pt.processing {
		files = append(files, filepath)
	}

	return files
}
