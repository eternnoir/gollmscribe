package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/eternnoir/gollmscribe/pkg/config"
	"github.com/eternnoir/gollmscribe/pkg/providers/gemini"
	"github.com/eternnoir/gollmscribe/pkg/transcriber"
	"github.com/eternnoir/gollmscribe/pkg/watcher"
)

func main() {
	// Check command line arguments
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <watch_directory>")
	}
	watchDir := os.Args[1]

	// Validate directory
	if info, err := os.Stat(watchDir); err != nil || !info.IsDir() {
		log.Fatalf("Invalid watch directory: %v", err)
	}

	// Load configuration
	cfg, err := loadConfiguration()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize provider
	provider, err := initializeProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize provider: %v", err)
	}

	// Create transcriber
	tr := transcriber.NewTranscriber(provider, cfg)

	// Create watch configuration
	watchConfig := createWatchConfig(watchDir, cfg)

	// Create file watcher
	fileWatcher, err := watcher.NewFileWatcher(watchConfig, tr)
	if err != nil {
		log.Fatalf("Failed to create file watcher: %v", err)
	}

	// Set up progress callback for detailed monitoring
	fileWatcher.SetProgressCallback(func(event *watcher.ProgressEvent) {
		timestamp := event.Timestamp.Format("15:04:05")
		filename := filepath.Base(event.FilePath)

		switch event.Type {
		case "found":
			fmt.Printf("[%s] 📁 Found: %s\n", timestamp, filename)
		case "processing":
			fmt.Printf("[%s] ⏳ Processing: %s - %s\n", timestamp, filename, event.Message)
		case "completed":
			fmt.Printf("[%s] ✅ Completed: %s - %s\n", timestamp, filename, event.Message)
		case "failed":
			fmt.Printf("[%s] ❌ Failed: %s - %v\n", timestamp, filename, event.Error)
		case "skipped":
			fmt.Printf("[%s] ⏭️  Skipped: %s - %s\n", timestamp, filename, event.Message)
		}
	})

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start the file watcher
	fmt.Printf("🚀 Starting file watcher for directory: %s\n", watchDir)
	fmt.Printf("📋 Configuration:\n")
	fmt.Printf("   Patterns: %v\n", watchConfig.Patterns)
	fmt.Printf("   Recursive: %v\n", watchConfig.Recursive)
	fmt.Printf("   Workers: %d\n", watchConfig.MaxWorkers)
	if watchConfig.OutputDir != "" {
		fmt.Printf("   Output: %s\n", watchConfig.OutputDir)
	}
	if watchConfig.MoveToDir != "" {
		fmt.Printf("   Move to: %s\n", watchConfig.MoveToDir)
	}
	if watchConfig.SharedPrompt != "" {
		fmt.Printf("   Shared prompt: %.50s...\n", watchConfig.SharedPrompt)
	}
	fmt.Println()

	if err := fileWatcher.Start(ctx); err != nil {
		log.Fatalf("Failed to start file watcher: %v", err)
	}

	// Start statistics display routine
	go displayStatistics(fileWatcher)

	// Show watching message
	fmt.Printf("👀 Watching for new files... Press Ctrl+C to stop\n\n")

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n\n🛑 Shutting down...")

	// Stop the watcher gracefully
	if err := fileWatcher.Stop(); err != nil {
		log.Printf("Error stopping file watcher: %v", err)
	}

	// Display final statistics
	stats := fileWatcher.GetStats()
	fmt.Printf("\n📊 Final Statistics:\n")
	fmt.Printf("   Processed: %d files\n", stats.ProcessedCount)
	fmt.Printf("   Failed: %d files\n", stats.FailedCount)
	fmt.Printf("   Skipped: %d files\n", stats.SkippedCount)
	fmt.Printf("   Total size: %.2f MB\n", float64(stats.TotalSize)/1024/1024)
	fmt.Printf("   Runtime: %v\n", time.Since(stats.StartTime).Round(time.Second))

	fmt.Println("\n✅ File watcher stopped successfully")
}

func loadConfiguration() (*config.Config, error) {
	// Try to load from config file
	loader := config.NewLoader("")
	cfg, err := loader.Load()
	if err != nil {
		// If config loading fails, use defaults
		fmt.Printf("Warning: Failed to load config file, using defaults: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// Override with environment variables
	if apiKey := os.Getenv("GOLLMSCRIBE_API_KEY"); apiKey != "" {
		cfg.Provider.APIKey = apiKey
	}

	// Validate API key
	if cfg.Provider.APIKey == "" {
		return nil, fmt.Errorf("API key is required. Set GOLLMSCRIBE_API_KEY environment variable")
	}

	return cfg, nil
}

func initializeProvider(cfg *config.Config) (*gemini.Provider, error) {
	switch cfg.Provider.Name {
	case "gemini":
		provider := gemini.NewProvider(
			cfg.Provider.APIKey,
			gemini.WithTimeout(cfg.Provider.Timeout),
			gemini.WithRetries(cfg.Provider.Retries),
		)
		if err := provider.ValidateConfig(); err != nil {
			return nil, fmt.Errorf("provider validation failed: %w", err)
		}
		return provider, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider.Name)
	}
}

func createWatchConfig(watchDir string, cfg *config.Config) *watcher.WatchConfig {
	// Create default watch config
	watchConfig := watcher.DefaultWatchConfig()
	watchConfig.WatchDir = watchDir

	// Apply configuration from file if available
	if len(cfg.Watch.Patterns) > 0 {
		watchConfig.Patterns = cfg.Watch.Patterns
	}
	watchConfig.Recursive = cfg.Watch.Recursive
	watchConfig.Interval = cfg.Watch.Interval
	watchConfig.StabilityWait = cfg.Watch.StabilityWait
	watchConfig.ProcessingTimeout = cfg.Watch.ProcessingTimeout
	watchConfig.MaxWorkers = cfg.Watch.MaxWorkers
	watchConfig.OutputDir = cfg.Watch.OutputDir
	watchConfig.MoveToDir = cfg.Watch.MoveToDir
	watchConfig.HistoryDB = cfg.Watch.HistoryDB
	watchConfig.ProcessExisting = cfg.Watch.ProcessExisting
	watchConfig.RetryFailed = cfg.Watch.RetryFailed

	// Set transcribe options from config
	watchConfig.TranscribeOptions = transcriber.TranscribeOptions{
		ChunkMinutes:   cfg.Audio.ChunkMinutes,
		OverlapSeconds: cfg.Audio.OverlapSeconds,
		Workers:        cfg.Audio.Workers,
		Temperature:    cfg.Provider.Temperature,
		PreserveAudio:  cfg.Audio.KeepTempFiles,
	}

	// Use default prompt if no custom prompt specified
	if watchConfig.SharedPrompt == "" && cfg.Transcribe.DefaultPrompt != "" {
		watchConfig.SharedPrompt = cfg.Transcribe.DefaultPrompt
	}

	return watchConfig
}

func displayStatistics(fw watcher.FileWatcher) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := fw.GetStats()
		if stats.ProcessedCount > 0 || stats.FailedCount > 0 || stats.InProgress > 0 {
			runtime := time.Since(stats.StartTime).Round(time.Second)
			fmt.Printf("\r📊 [%v] Processed: %d | Failed: %d | In Progress: %d | Size: %.2f MB",
				runtime, stats.ProcessedCount, stats.FailedCount, stats.InProgress,
				float64(stats.TotalSize)/1024/1024)
		}
	}
}
