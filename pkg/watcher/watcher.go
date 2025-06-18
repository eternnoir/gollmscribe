package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/eternnoir/gollmscribe/pkg/logger"
	"github.com/eternnoir/gollmscribe/pkg/transcriber"
)

// fileWatcher implements FileWatcher interface
type fileWatcher struct {
	config      *WatchConfig
	transcriber transcriber.Transcriber
	tracker     ProcessingTracker
	history     ProcessingHistory
	processor   FileProcessor
	watcher     *fsnotify.Watcher
	progress    ProgressCallback
	stats       *WatchStats
	statsLock   sync.RWMutex

	// Event deduplication
	recentEvents    map[string]time.Time
	recentEventsMux sync.RWMutex

	// Initial processing tracking
	initialProcessing    sync.WaitGroup
	initialProcessingMap map[string]bool
	initialProcessingMux sync.Mutex

	// Control channels
	stopCh      chan struct{}
	workerQueue chan string
	wg          sync.WaitGroup
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(config *WatchConfig, trans transcriber.Transcriber) (FileWatcher, error) {
	// Validate config
	if config.WatchDir == "" {
		return nil, fmt.Errorf("watch directory is required")
	}

	// Create processing history
	history, err := NewProcessingHistory(config.HistoryDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create processing history: %w", err)
	}

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		_ = history.Close()
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Create components
	tracker := NewProcessingTracker()

	fw := &fileWatcher{
		config:               config,
		transcriber:          trans,
		tracker:              tracker,
		history:              history,
		watcher:              watcher,
		recentEvents:         make(map[string]time.Time),
		initialProcessingMap: make(map[string]bool),
		stopCh:               make(chan struct{}),
		workerQueue:          make(chan string, config.MaxWorkers*2),
		stats: &WatchStats{
			StartTime: time.Now(),
		},
	}

	// Create processor
	processor := NewFileProcessor(config, trans, tracker, history)
	fw.processor = processor

	// Set processor callback to update stats
	if fp, ok := processor.(*fileProcessor); ok {
		fp.SetProgressCallback(fw.handleProgressEvent)
	}

	return fw, nil
}

// Start begins watching the specified directory
func (fw *fileWatcher) Start(ctx context.Context) error {
	log := logger.WithComponent("watcher")

	// Add watch directory
	if err := fw.addWatchDir(fw.config.WatchDir); err != nil {
		return fmt.Errorf("failed to add watch directory: %w", err)
	}

	// Start workers
	for i := 0; i < fw.config.MaxWorkers; i++ {
		fw.wg.Add(1)
		go fw.processWorker(ctx)
	}

	// Start cleanup routine
	fw.wg.Add(1)
	go fw.cleanupRoutine()

	// Clean up stale processing markers first
	log.Info().Msg("Cleaning up stale processing markers")
	if err := fw.cleanupStaleMarkers(); err != nil {
		log.Warn().Err(err).Msg("Failed to clean up stale processing markers")
	}

	// Process existing files if configured
	if fw.config.ProcessExisting {
		log.Info().Msg("Processing existing files")
		if err := fw.processExistingFiles(); err != nil {
			log.Warn().Err(err).Msg("Failed to process some existing files")
		}
	}

	// Start watching
	fw.wg.Add(1)
	go fw.watchLoop(ctx)

	log.Info().
		Str("directory", fw.config.WatchDir).
		Bool("recursive", fw.config.Recursive).
		Strs("patterns", fw.config.Patterns).
		Msg("File watcher started")

	return nil
}

// Stop gracefully shuts down the watcher
func (fw *fileWatcher) Stop() error {
	log := logger.WithComponent("watcher")
	log.Info().Msg("Stopping file watcher")

	// Signal stop
	close(fw.stopCh)

	// Close watcher
	if err := fw.watcher.Close(); err != nil {
		log.Warn().Err(err).Msg("Error closing watcher")
	}

	// Close worker queue
	close(fw.workerQueue)

	// Wait for all workers to finish
	fw.wg.Wait()

	// Close history database
	if err := fw.history.Close(); err != nil {
		log.Warn().Err(err).Msg("Error closing history database")
	}

	log.Info().Msg("File watcher stopped")
	return nil
}

// SetProgressCallback sets a callback for progress updates
func (fw *fileWatcher) SetProgressCallback(callback ProgressCallback) {
	fw.progress = callback
}

// GetStats returns statistics about processed files
func (fw *fileWatcher) GetStats() *WatchStats {
	fw.statsLock.RLock()
	defer fw.statsLock.RUnlock()

	// Create a copy to avoid race conditions
	stats := *fw.stats
	stats.InProgress = len(fw.tracker.GetLocked())
	return &stats
}

// WaitForInitialProcessing returns a WaitGroup that completes when initial file processing is done
func (fw *fileWatcher) WaitForInitialProcessing() *sync.WaitGroup {
	return &fw.initialProcessing
}

// addWatchDir adds a directory to watch
func (fw *fileWatcher) addWatchDir(dir string) error {
	// Add the directory
	if err := fw.watcher.Add(dir); err != nil {
		return err
	}

	// If recursive, add subdirectories
	if fw.config.Recursive {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && path != dir {
				if err := fw.watcher.Add(path); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// processExistingFiles processes files that already exist in the watch directory
func (fw *fileWatcher) processExistingFiles() error {
	log := logger.WithComponent("watcher")

	err := filepath.Walk(fw.config.WatchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Check if we should recurse
			if !fw.config.Recursive && path != fw.config.WatchDir {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file can be processed
		if fw.processor.CanProcess(path) {
			log.Debug().Str("file", path).Msg("Queueing existing file")

			// Add to initial processing tracking
			fw.initialProcessingMux.Lock()
			fw.initialProcessingMap[path] = true
			fw.initialProcessing.Add(1)
			fw.initialProcessingMux.Unlock()

			select {
			case fw.workerQueue <- path:
			case <-fw.stopCh:
				// Clean up if we're stopping
				fw.initialProcessingMux.Lock()
				delete(fw.initialProcessingMap, path)
				fw.initialProcessing.Done()
				fw.initialProcessingMux.Unlock()
				return fmt.Errorf("watcher stopped")
			}
		}

		return nil
	})

	return err
}

// watchLoop is the main watch loop
func (fw *fileWatcher) watchLoop(ctx context.Context) {
	defer fw.wg.Done()
	log := logger.WithComponent("watcher")

	// Also use a ticker for periodic scans
	ticker := time.NewTicker(fw.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-fw.stopCh:
			return
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleFileEvent(event)
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msg("Watcher error")
		case <-ticker.C:
			// Periodic scan for missed files
			fw.periodicScan()
		}
	}
}

// handleFileEvent handles a file system event
func (fw *fileWatcher) handleFileEvent(event fsnotify.Event) {
	log := logger.WithComponent("watcher").WithField("file", event.Name)

	// Check for duplicate events (debouncing)
	if fw.isDuplicateEvent(event.Name) {
		log.Debug().Msg("Duplicate event ignored")
		return
	}

	// Handle different event types
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		log.Debug().Msg("File created")
		if !fw.tracker.IsLocked(event.Name) && fw.processor.CanProcess(event.Name) {
			fw.queueFile(event.Name)
		}
	case event.Op&fsnotify.Write == fsnotify.Write:
		log.Debug().Msg("File modified")
		// Wait a bit for write to complete
		time.Sleep(fw.config.StabilityWait)
		if !fw.tracker.IsLocked(event.Name) && fw.processor.CanProcess(event.Name) {
			fw.queueFile(event.Name)
		}
	}
}

// periodicScan performs a periodic scan for new files
func (fw *fileWatcher) periodicScan() {
	// This helps catch files that might have been missed by fsnotify
	_ = filepath.Walk(fw.config.WatchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if fw.processor.CanProcess(path) {
			fw.queueFile(path)
		}

		return nil
	})
}

// queueFile queues a file for processing
func (fw *fileWatcher) queueFile(filepath string) {
	select {
	case fw.workerQueue <- filepath:
		fw.reportProgress(&ProgressEvent{
			Type:      "found",
			FilePath:  filepath,
			Message:   "File queued for processing",
			Timestamp: time.Now(),
		})
	default:
		// Queue is full, skip this file for now
		logger.WithComponent("watcher").
			Warn().
			Str("file", filepath).
			Msg("Worker queue is full, skipping file")
	}
}

// processWorker is a worker that processes files from the queue
func (fw *fileWatcher) processWorker(ctx context.Context) {
	defer fw.wg.Done()
	log := logger.WithComponent("worker")

	for filepath := range fw.workerQueue {
		select {
		case <-ctx.Done():
			return
		case <-fw.stopCh:
			return
		default:
			log.Debug().Str("file", filepath).Msg("Processing file")

			// Process the file
			if err := fw.processor.ProcessFile(ctx, filepath); err != nil {
				log.Error().Err(err).Str("file", filepath).Msg("Failed to process file")
			}

			// Mark this file as done from initial processing (if it was part of it)
			fw.initialProcessingMux.Lock()
			if fw.initialProcessingMap[filepath] {
				delete(fw.initialProcessingMap, filepath)
				fw.initialProcessing.Done()
			}
			fw.initialProcessingMux.Unlock()
		}
	}
}

// cleanupRoutine periodically cleans up stale locks and recent events
func (fw *fileWatcher) cleanupRoutine() {
	defer fw.wg.Done()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-fw.stopCh:
			return
		case <-ticker.C:
			// Clean up stale processing locks
			cleaned := fw.tracker.CleanupStale(fw.config.ProcessingTimeout)
			if cleaned > 0 {
				logger.WithComponent("watcher").
					Info().
					Int("cleaned", cleaned).
					Msg("Cleaned up stale locks")
			}

			// Clean up old event cache
			fw.cleanupRecentEvents()
		}
	}
}

// handleProgressEvent handles progress events from the processor
func (fw *fileWatcher) handleProgressEvent(event *ProgressEvent) {
	// Update stats
	fw.statsLock.Lock()
	switch event.Type {
	case "completed":
		fw.stats.ProcessedCount++
	case "failed":
		fw.stats.FailedCount++
	case "skipped":
		fw.stats.SkippedCount++
	}
	fw.statsLock.Unlock()

	// Forward to external callback
	fw.reportProgress(event)
}

// cleanupStaleMarkers removes stale .processing marker files left by crashed processes
func (fw *fileWatcher) cleanupStaleMarkers() error {
	log := logger.WithComponent("watcher")

	cleaned := 0
	err := filepath.Walk(fw.config.WatchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Check if we should recurse
			if !fw.config.Recursive && path != fw.config.WatchDir {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if this is a processing marker file
		if !strings.HasSuffix(info.Name(), ".processing") {
			return nil
		}

		// Check if the marker is stale (older than processing timeout)
		if time.Since(info.ModTime()) > fw.config.ProcessingTimeout {
			log.Info().
				Str("marker_file", path).
				Dur("age", time.Since(info.ModTime())).
				Msg("Removing stale processing marker")

			if err := os.Remove(path); err != nil {
				log.Warn().Err(err).Str("marker_file", path).Msg("Failed to remove stale marker")
				return nil // Continue processing other files
			}
			cleaned++
		}

		return nil
	})

	if cleaned > 0 {
		log.Info().Int("cleaned_markers", cleaned).Msg("Cleaned up stale processing markers")
	}

	return err
}

// isDuplicateEvent checks if we've seen this file event recently (debouncing)
func (fw *fileWatcher) isDuplicateEvent(filePath string) bool {
	fw.recentEventsMux.Lock()
	defer fw.recentEventsMux.Unlock()

	now := time.Now()

	// Check if we've seen this file recently
	if lastSeen, exists := fw.recentEvents[filePath]; exists {
		// If seen within the last 5 seconds, consider it a duplicate
		if now.Sub(lastSeen) < 5*time.Second {
			return true
		}
	}

	// Record this event
	fw.recentEvents[filePath] = now
	return false
}

// cleanupRecentEvents removes old entries from the recent events cache
func (fw *fileWatcher) cleanupRecentEvents() {
	fw.recentEventsMux.Lock()
	defer fw.recentEventsMux.Unlock()

	now := time.Now()
	cutoff := 30 * time.Second

	for filePath, timestamp := range fw.recentEvents {
		if now.Sub(timestamp) > cutoff {
			delete(fw.recentEvents, filePath)
		}
	}
}

// reportProgress reports progress if callback is set
func (fw *fileWatcher) reportProgress(event *ProgressEvent) {
	if fw.progress != nil {
		fw.progress(event)
	}
}
