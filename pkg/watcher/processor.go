package watcher

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/eternnoir/gollmscribe/pkg/logger"
	"github.com/eternnoir/gollmscribe/pkg/transcriber"
)

// fileProcessor implements FileProcessor interface
type fileProcessor struct {
	config      *WatchConfig
	transcriber transcriber.Transcriber
	tracker     ProcessingTracker
	history     ProcessingHistory
	progress    ProgressCallback
}

// NewFileProcessor creates a new file processor
func NewFileProcessor(
	config *WatchConfig,
	transcriber transcriber.Transcriber,
	tracker ProcessingTracker,
	history ProcessingHistory,
) FileProcessor {
	return &fileProcessor{
		config:      config,
		transcriber: transcriber,
		tracker:     tracker,
		history:     history,
	}
}

// SetProgressCallback sets the progress callback
func (fp *fileProcessor) SetProgressCallback(callback ProgressCallback) {
	fp.progress = callback
}

// ProcessFile processes a single file
func (fp *fileProcessor) ProcessFile(ctx context.Context, filePath string) error {
	log := logger.WithComponent("processor").WithField("file", filePath)

	// Report progress
	fp.reportProgress(&ProgressEvent{
		Type:      "processing",
		FilePath:  filePath,
		Message:   "Starting processing",
		Timestamp: time.Now(),
	})

	// Check if we can process this file
	if !fp.CanProcess(filePath) {
		fp.reportProgress(&ProgressEvent{
			Type:      "skipped",
			FilePath:  filePath,
			Message:   "File cannot be processed",
			Timestamp: time.Now(),
		})
		return nil
	}

	// Try to acquire lock
	if !fp.tracker.TryLock(filePath) {
		fp.reportProgress(&ProgressEvent{
			Type:      "skipped",
			FilePath:  filePath,
			Message:   "File is already being processed",
			Timestamp: time.Now(),
		})
		return nil
	}
	defer fp.tracker.Unlock(filePath)

	// Create processing marker file
	markerFile := filePath + ".processing"
	if err := os.WriteFile(markerFile, []byte(time.Now().Format(time.RFC3339)), 0o644); err != nil {
		log.Warn().Err(err).Msg("Failed to create processing marker")
	}
	defer func() { _ = os.Remove(markerFile) }()

	// Calculate file hash
	hash, err := fp.getFileHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Check if already processed
	processed, err := fp.history.IsProcessed(hash)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to check processing history")
	} else if processed {
		fp.reportProgress(&ProgressEvent{
			Type:      "skipped",
			FilePath:  filePath,
			Message:   "File already processed",
			Timestamp: time.Now(),
		})
		return nil
	}

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Determine output path
	outputPath := fp.getOutputPath(filePath)

	// Create output directory if needed
	outputDir := fp.config.OutputDir
	if outputDir == "" {
		outputDir = filepath.Dir(filePath)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create transcription request
	req := &transcriber.TranscribeRequest{
		FilePath:     filePath,
		OutputPath:   outputPath,
		CustomPrompt: fp.config.SharedPrompt,
		Options:      fp.config.TranscribeOptions,
	}

	// Start transcription
	startTime := time.Now()
	log.Info().Msg("Starting transcription")

	// Create context with timeout
	transcribeCtx, cancel := context.WithTimeout(ctx, fp.config.ProcessingTimeout)
	defer cancel()

	result, err := fp.transcriber.Transcribe(transcribeCtx, req)
	if err != nil {
		// Record failure
		failedInfo := FailedInfo{
			FileHash: hash,
			FilePath: filePath,
			FailedAt: time.Now(),
			Error:    err.Error(),
		}
		if histErr := fp.history.RecordFailed(hash, &failedInfo); histErr != nil {
			log.Warn().Err(histErr).Msg("Failed to record failure in history")
		}

		fp.reportProgress(&ProgressEvent{
			Type:      "failed",
			FilePath:  filePath,
			Message:   "Transcription failed",
			Error:     err,
			Timestamp: time.Now(),
		})

		return fmt.Errorf("transcription failed: %w", err)
	}

	// Record success
	processedInfo := ProcessedInfo{
		FileHash:    hash,
		FilePath:    filePath,
		ProcessedAt: time.Now(),
		OutputPath:  outputPath,
		Duration:    time.Since(startTime),
		FileSize:    fileInfo.Size(),
	}
	if err := fp.history.RecordProcessed(hash, &processedInfo); err != nil {
		log.Warn().Err(err).Msg("Failed to record success in history")
	}

	// Move file if configured
	if fp.config.MoveToDir != "" {
		if err := fp.moveFile(filePath); err != nil {
			log.Warn().Err(err).Msg("Failed to move processed file")
		}
	}

	fp.reportProgress(&ProgressEvent{
		Type:      "completed",
		FilePath:  filePath,
		Message:   fmt.Sprintf("Transcription completed in %v", result.ProcessTime),
		Timestamp: time.Now(),
	})

	log.Info().
		Dur("duration", time.Since(startTime)).
		Str("output", outputPath).
		Msg("File processed successfully")

	return nil
}

// CanProcess checks if a file can be processed
func (fp *fileProcessor) CanProcess(filePath string) bool {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	// Check if file matches patterns
	matched := false
	filename := filepath.Base(filePath)
	for _, pattern := range fp.config.Patterns {
		if match, _ := filepath.Match(pattern, filename); match {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}

	// Check if file is stable
	if !fp.isFileStable(filePath) {
		return false
	}

	// Check if there's a processing marker
	if _, err := os.Stat(filePath + ".processing"); err == nil {
		return false
	}

	return true
}

// isFileStable checks if a file has been stable for the configured duration
func (fp *fileProcessor) isFileStable(filePath string) bool {
	info1, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	time.Sleep(fp.config.StabilityWait)

	info2, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	return info1.Size() == info2.Size() &&
		info1.ModTime().Equal(info2.ModTime())
}

// getFileHash calculates SHA256 hash of the file (first 1MB for performance)
func (fp *fileProcessor) getFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	// Read first 1MB for hash calculation (performance optimization for large files)
	if _, err := io.CopyN(hash, file, 1024*1024); err != nil && err != io.EOF {
		return "", err
	}

	// Also include file size in hash for better uniqueness
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	_, _ = fmt.Fprintf(hash, ":%d", info.Size())

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// getOutputPath determines the output path for the transcription
func (fp *fileProcessor) getOutputPath(inputPath string) string {
	basename := filepath.Base(inputPath)
	nameWithoutExt := strings.TrimSuffix(basename, filepath.Ext(basename))
	outputName := nameWithoutExt + ".txt"

	if fp.config.OutputDir != "" {
		return filepath.Join(fp.config.OutputDir, outputName)
	}

	return filepath.Join(filepath.Dir(inputPath), outputName)
}

// moveFile moves the processed file to the configured directory
func (fp *fileProcessor) moveFile(filePath string) error {
	if err := os.MkdirAll(fp.config.MoveToDir, 0o755); err != nil {
		return fmt.Errorf("failed to create move-to directory: %w", err)
	}

	destPath := filepath.Join(fp.config.MoveToDir, filepath.Base(filePath))

	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		// Add timestamp to avoid overwriting
		ext := filepath.Ext(filePath)
		name := strings.TrimSuffix(filepath.Base(filePath), ext)
		timestamp := time.Now().Format("20060102_150405")
		destPath = filepath.Join(fp.config.MoveToDir, fmt.Sprintf("%s_%s%s", name, timestamp, ext))
	}

	// Try rename first (faster for same filesystem)
	err := os.Rename(filePath, destPath)
	if err == nil {
		return nil
	}

	// Check if it's a cross-device link error
	if linkErr, ok := err.(*os.LinkError); ok {
		if errno, ok := linkErr.Err.(syscall.Errno); ok && errno == syscall.EXDEV {
			// Cross-device link error, fallback to copy-then-delete
			return fp.copyThenDelete(filePath, destPath)
		}
	}

	// Other error, return as-is
	return fmt.Errorf("failed to move file: %w", err)
}

// copyThenDelete copies a file then deletes the original (for cross-filesystem moves)
func (fp *fileProcessor) copyThenDelete(srcPath, destPath string) error {
	// Open source file
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = src.Close() }()

	// Create destination file
	dest, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() { _ = dest.Close() }()

	// Copy file contents
	_, err = io.Copy(dest, src)
	if err != nil {
		// Clean up partial destination file
		_ = os.Remove(destPath)
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Sync to ensure data is written
	if err := dest.Sync(); err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	// Copy file permissions
	srcInfo, err := src.Stat()
	if err != nil {
		// Non-fatal, continue
	} else {
		_ = dest.Chmod(srcInfo.Mode())
	}

	// Close files before deleting
	_ = dest.Close()
	_ = src.Close()

	// Delete original file
	if err := os.Remove(srcPath); err != nil {
		return fmt.Errorf("failed to delete original file after copy: %w", err)
	}

	return nil
}

// reportProgress reports progress if callback is set
func (fp *fileProcessor) reportProgress(event *ProgressEvent) {
	if fp.progress != nil {
		fp.progress(event)
	}
}
