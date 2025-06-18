package watcher

import (
	"context"
	"sync"
	"time"

	"github.com/eternnoir/gollmscribe/pkg/transcriber"
)

// FileWatcher defines the interface for watching directories for new files
type FileWatcher interface {
	// Start begins watching the specified directory
	Start(ctx context.Context) error

	// Stop gracefully shuts down the watcher
	Stop() error

	// SetProgressCallback sets a callback for progress updates
	SetProgressCallback(callback ProgressCallback)

	// GetStats returns statistics about processed files
	GetStats() *WatchStats

	// WaitForInitialProcessing returns a WaitGroup that completes when initial file processing is done
	WaitForInitialProcessing() *sync.WaitGroup
}

// ProcessingTracker manages the state of files being processed
type ProcessingTracker interface {
	// TryLock attempts to acquire a lock for processing a file
	TryLock(filepath string) bool

	// Unlock releases the lock for a file
	Unlock(filepath string)

	// IsLocked checks if a file is currently locked
	IsLocked(filepath string) bool

	// CleanupStale removes locks older than the specified duration
	CleanupStale(timeout time.Duration) int

	// GetLocked returns all currently locked files
	GetLocked() []string
}

// ProcessingHistory manages the persistent history of processed files
type ProcessingHistory interface {
	// IsProcessed checks if a file hash has been processed
	IsProcessed(fileHash string) (bool, error)

	// RecordProcessed records a successfully processed file
	RecordProcessed(fileHash string, info *ProcessedInfo) error

	// RecordFailed records a failed processing attempt
	RecordFailed(fileHash string, info *FailedInfo) error

	// GetProcessedInfo retrieves information about a processed file
	GetProcessedInfo(fileHash string) (*ProcessedInfo, error)

	// GetFailedInfo retrieves information about a failed file
	GetFailedInfo(fileHash string) (*FailedInfo, error)

	// Close closes the underlying database
	Close() error
}

// FileProcessor handles the processing of individual files
type FileProcessor interface {
	// ProcessFile processes a single file
	ProcessFile(ctx context.Context, filepath string) error

	// CanProcess checks if a file can be processed
	CanProcess(filepath string) bool
}

// ProgressCallback is called to report progress
type ProgressCallback func(event *ProgressEvent)

// ProgressEvent represents a progress update
type ProgressEvent struct {
	Type      string // "found", "processing", "completed", "failed", "skipped"
	FilePath  string
	Message   string
	Error     error
	Timestamp time.Time
}

// ProcessedInfo contains information about a successfully processed file
type ProcessedInfo struct {
	FileHash    string        `json:"hash"`
	FilePath    string        `json:"filepath"`
	ProcessedAt time.Time     `json:"processed_at"`
	OutputPath  string        `json:"output_path"`
	Duration    time.Duration `json:"duration"`
	FileSize    int64         `json:"file_size"`
}

// FailedInfo contains information about a failed processing attempt
type FailedInfo struct {
	FileHash   string    `json:"hash"`
	FilePath   string    `json:"filepath"`
	FailedAt   time.Time `json:"failed_at"`
	Error      string    `json:"error"`
	RetryCount int       `json:"retry_count"`
}

// WatchStats contains statistics about the watcher
type WatchStats struct {
	StartTime      time.Time
	ProcessedCount int
	FailedCount    int
	SkippedCount   int
	InProgress     int
	TotalSize      int64
}

// WatchConfig contains configuration for the file watcher
type WatchConfig struct {
	// Directory to watch
	WatchDir string

	// File patterns to match (e.g., "*.mp3", "*.wav")
	Patterns []string

	// Whether to watch subdirectories recursively
	Recursive bool

	// Polling interval for checking new files
	Interval time.Duration

	// Time to wait for file stability before processing
	StabilityWait time.Duration

	// Maximum time allowed for processing a single file
	ProcessingTimeout time.Duration

	// Directory to move processed files to (optional)
	MoveToDir string

	// Directory to output transcriptions to
	OutputDir string

	// Shared prompt for all transcriptions
	SharedPrompt string

	// Path to the BoltDB history database
	HistoryDB string

	// Whether to process existing files on startup
	ProcessExisting bool

	// Whether to retry failed files
	RetryFailed bool

	// Maximum number of concurrent processing workers
	MaxWorkers int

	// Transcription options for all files
	TranscribeOptions transcriber.TranscribeOptions
}

// DefaultWatchConfig returns default configuration
func DefaultWatchConfig() *WatchConfig {
	return &WatchConfig{
		Patterns:          []string{"*.mp3", "*.wav", "*.mp4", "*.m4a"},
		Recursive:         false,
		Interval:          5 * time.Second,
		StabilityWait:     2 * time.Second,
		ProcessingTimeout: 30 * time.Minute,
		HistoryDB:         ".gollmscribe-watch.db",
		ProcessExisting:   true,
		RetryFailed:       false,
		MaxWorkers:        3,
		TranscribeOptions: transcriber.TranscribeOptions{
			ChunkMinutes:   15,
			OverlapSeconds: 30,
			Workers:        3,
			Temperature:    0.1,
			PreserveAudio:  false,
		},
	}
}
