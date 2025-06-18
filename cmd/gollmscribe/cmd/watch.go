package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/eternnoir/gollmscribe/pkg/config"
	"github.com/eternnoir/gollmscribe/pkg/logger"
	"github.com/eternnoir/gollmscribe/pkg/transcriber"
	"github.com/eternnoir/gollmscribe/pkg/watcher"
)

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:   "watch [directory]",
	Short: "Watch a directory for new audio/video files and transcribe them",
	Long: `Watch a directory for new audio/video files and automatically transcribe them.

The watch command monitors a directory for new or modified audio/video files and
automatically transcribes them using the configured AI model. All files in the
watch session share the same prompt and configuration.

Examples:
  # Watch current directory
  gollmscribe watch .

  # Watch with custom prompt
  gollmscribe watch ./recordings -p "Transcribe and identify speakers"

  # Watch recursively with file movement
  gollmscribe watch ./inbox -r --move-to ./processed

  # Watch with custom output directory
  gollmscribe watch ./meetings --output-dir ./transcripts

  # Process existing files once and exit
  gollmscribe watch ./batch --once

  # Watch specific file types
  gollmscribe watch ./audio --pattern "*.mp3,*.m4a"`,
	Args: cobra.ExactArgs(1),
	RunE: runWatch,
}

func init() {
	rootCmd.AddCommand(watchCmd)

	// Watch options
	watchCmd.Flags().StringSliceP("pattern", "", []string{"*.mp3", "*.wav", "*.mp4", "*.m4a"},
		"file patterns to watch (comma-separated)")
	watchCmd.Flags().BoolP("recursive", "r", false, "watch subdirectories recursively")
	watchCmd.Flags().Duration("interval", 5*time.Second, "polling interval for new files")
	watchCmd.Flags().Bool("once", false, "process existing files and exit")
	watchCmd.Flags().Bool("no-existing", false, "skip processing existing files on startup")

	// Processing options
	watchCmd.Flags().StringP("prompt", "p", "", "shared prompt for all transcriptions")
	watchCmd.Flags().String("prompt-file", "", "file containing shared prompt")
	watchCmd.Flags().Duration("stability-wait", 2*time.Second, "time to wait for file stability")
	watchCmd.Flags().Duration("processing-timeout", 30*time.Minute, "maximum time to process a single file")
	watchCmd.Flags().Int("max-workers", 3, "maximum concurrent processing workers")

	// Output options
	watchCmd.Flags().String("output-dir", "", "directory for transcription outputs")
	watchCmd.Flags().String("move-to", "", "move processed files to this directory")

	// History options
	watchCmd.Flags().String("history-db", ".gollmscribe-watch.db", "path to history database")
	watchCmd.Flags().Bool("retry-failed", false, "retry previously failed files")

	// Transcription options (inherited from transcribe command)
	watchCmd.Flags().Int("chunk-minutes", 15, "chunk duration in minutes")
	watchCmd.Flags().Int("overlap-seconds", 30, "overlap duration in seconds")
	watchCmd.Flags().Float32("temperature", 0.1, "LLM temperature (0.0-1.0)")
	watchCmd.Flags().Bool("preserve-audio", false, "keep temporary audio files")

	// Bind flags to viper
	_ = viper.BindPFlag("watch.pattern", watchCmd.Flags().Lookup("pattern"))
	_ = viper.BindPFlag("watch.recursive", watchCmd.Flags().Lookup("recursive"))
	_ = viper.BindPFlag("watch.interval", watchCmd.Flags().Lookup("interval"))
	_ = viper.BindPFlag("watch.stability_wait", watchCmd.Flags().Lookup("stability-wait"))
	_ = viper.BindPFlag("watch.processing_timeout", watchCmd.Flags().Lookup("processing-timeout"))
	_ = viper.BindPFlag("watch.max_workers", watchCmd.Flags().Lookup("max-workers"))
	_ = viper.BindPFlag("watch.output_dir", watchCmd.Flags().Lookup("output-dir"))
	_ = viper.BindPFlag("watch.move_to", watchCmd.Flags().Lookup("move-to"))
	_ = viper.BindPFlag("watch.history_db", watchCmd.Flags().Lookup("history-db"))
}

func runWatch(cmd *cobra.Command, args []string) error {
	log := logger.WithComponent("watch")

	watchDir := args[0]
	log.Info().Str("directory", watchDir).Msg("Starting watch mode")

	// Validate directory
	info, err := os.Stat(watchDir)
	if err != nil {
		return fmt.Errorf("invalid watch directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("watch path must be a directory")
	}

	// Validate API key
	apiKey := viper.GetString("api_key")
	if apiKey == "" {
		log.Error().Msg("API key is required")
		return fmt.Errorf("API key is required. Set GOLLMSCRIBE_API_KEY environment variable or use --api-key flag")
	}

	// Initialize provider first
	appCfg := loadConfig()
	provider, err := initializeProvider(appCfg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize provider")
		return fmt.Errorf("failed to initialize provider: %w", err)
	}

	// Get configuration with transcribe options
	cfg := loadWatchConfig(cmd, watchDir)

	// Get transcribe options from CLI and apply to config
	transcribeOpts := getWatchTranscribeOptions(cmd, appCfg)
	cfg.TranscribeOptions = transcribeOpts

	log.Debug().Interface("config", cfg).Msg("Loaded watch configuration")

	// Get custom prompt
	customPrompt, err := getWatchPrompt(cmd)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get custom prompt")
		return fmt.Errorf("failed to get custom prompt: %w", err)
	}
	if customPrompt != "" {
		cfg.SharedPrompt = customPrompt
		log.Info().Str("prompt_preview", truncateString(customPrompt, 100)).Msg("Using shared custom prompt")
	}

	// Create transcriber
	tr := transcriber.NewTranscriber(provider, appCfg)

	// Create file watcher
	fileWatcher, err := watcher.NewFileWatcher(cfg, tr)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create file watcher")
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Set progress callback
	fileWatcher.SetProgressCallback(func(event *watcher.ProgressEvent) {
		switch event.Type {
		case "found":
			fmt.Printf("📁 Found: %s\n", event.FilePath)
		case "processing":
			fmt.Printf("⏳ Processing: %s\n", event.FilePath)
		case "completed":
			fmt.Printf("✅ Completed: %s - %s\n", event.FilePath, event.Message)
		case "failed":
			fmt.Printf("❌ Failed: %s - %v\n", event.FilePath, event.Error)
		case "skipped":
			fmt.Printf("⏭️  Skipped: %s - %s\n", event.FilePath, event.Message)
		}
	})

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start watcher
	if err := fileWatcher.Start(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to start file watcher")
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Check if running in once mode
	once, _ := cmd.Flags().GetBool("once")
	if once {
		log.Info().Msg("Running in once mode, will exit after processing existing files")

		// Wait for initial processing to complete using WaitGroup
		wg := fileWatcher.WaitForInitialProcessing()
		wg.Wait()

		log.Info().Msg("Initial processing completed, exiting")
	} else {
		// Show watching message
		fmt.Printf("\n👀 Watching directory: %s\n", watchDir)
		if cfg.Recursive {
			fmt.Println("   Recursive: Yes")
		}
		fmt.Printf("   Patterns: %s\n", strings.Join(cfg.Patterns, ", "))
		fmt.Printf("   Workers: %d\n", cfg.MaxWorkers)
		if cfg.OutputDir != "" {
			fmt.Printf("   Output: %s\n", cfg.OutputDir)
		}
		if cfg.MoveToDir != "" {
			fmt.Printf("   Move to: %s\n", cfg.MoveToDir)
		}
		fmt.Println("\nPress Ctrl+C to stop watching...")

		// Start stats display routine
		go displayStats(fileWatcher)

		// Wait for shutdown signal
		<-sigChan
		fmt.Println("\n\n🛑 Shutting down...")
	}

	// Stop watcher
	if err := fileWatcher.Stop(); err != nil {
		log.Error().Err(err).Msg("Error stopping file watcher")
		return fmt.Errorf("error stopping file watcher: %w", err)
	}

	// Display final stats
	stats := fileWatcher.GetStats()
	fmt.Printf("\n📊 Final Statistics:\n")
	fmt.Printf("   Processed: %d files\n", stats.ProcessedCount)
	fmt.Printf("   Failed: %d files\n", stats.FailedCount)
	fmt.Printf("   Skipped: %d files\n", stats.SkippedCount)
	fmt.Printf("   Duration: %v\n", time.Since(stats.StartTime).Round(time.Second))

	return nil
}

func loadWatchConfig(cmd *cobra.Command, watchDir string) *watcher.WatchConfig {
	cfg := watcher.DefaultWatchConfig()
	cfg.WatchDir = watchDir

	// Get values from flags/viper
	patterns, _ := cmd.Flags().GetStringSlice("pattern")
	if len(patterns) > 0 {
		cfg.Patterns = patterns
	}

	cfg.Recursive, _ = cmd.Flags().GetBool("recursive")
	cfg.Interval, _ = cmd.Flags().GetDuration("interval")
	cfg.StabilityWait, _ = cmd.Flags().GetDuration("stability-wait")
	cfg.ProcessingTimeout, _ = cmd.Flags().GetDuration("processing-timeout")
	cfg.MaxWorkers, _ = cmd.Flags().GetInt("max-workers")

	cfg.OutputDir, _ = cmd.Flags().GetString("output-dir")
	cfg.MoveToDir, _ = cmd.Flags().GetString("move-to")
	cfg.HistoryDB, _ = cmd.Flags().GetString("history-db")

	noExisting, _ := cmd.Flags().GetBool("no-existing")
	cfg.ProcessExisting = !noExisting

	cfg.RetryFailed, _ = cmd.Flags().GetBool("retry-failed")

	return cfg
}

func getWatchPrompt(cmd *cobra.Command) (string, error) {
	// Check direct prompt flag
	if prompt, _ := cmd.Flags().GetString("prompt"); prompt != "" {
		return prompt, nil
	}

	// Check prompt file flag
	if promptFile, _ := cmd.Flags().GetString("prompt-file"); promptFile != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			return "", fmt.Errorf("failed to read prompt file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	return "", nil
}

func getWatchTranscribeOptions(cmd *cobra.Command, cfg *config.Config) transcriber.TranscribeOptions {
	chunkMinutes, _ := cmd.Flags().GetInt("chunk-minutes")
	if !cmd.Flags().Changed("chunk-minutes") {
		chunkMinutes = cfg.Audio.ChunkMinutes
	}

	overlapSeconds, _ := cmd.Flags().GetInt("overlap-seconds")
	if !cmd.Flags().Changed("overlap-seconds") {
		overlapSeconds = cfg.Audio.OverlapSeconds
	}

	temperature, _ := cmd.Flags().GetFloat32("temperature")
	if !cmd.Flags().Changed("temperature") {
		temperature = cfg.Provider.Temperature
	}

	preserveAudio, _ := cmd.Flags().GetBool("preserve-audio")

	// Use max workers from watch config
	workers, _ := cmd.Flags().GetInt("max-workers")

	return transcriber.TranscribeOptions{
		ChunkMinutes:   chunkMinutes,
		OverlapSeconds: overlapSeconds,
		Workers:        workers,
		Temperature:    temperature,
		PreserveAudio:  preserveAudio,
	}
}

func displayStats(fw watcher.FileWatcher) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := fw.GetStats()
		if stats.ProcessedCount > 0 || stats.FailedCount > 0 {
			fmt.Printf("\r📊 Stats - Processed: %d | Failed: %d | In Progress: %d",
				stats.ProcessedCount, stats.FailedCount, stats.InProgress)
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
