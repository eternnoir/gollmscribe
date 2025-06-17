package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/eternnoir/gollmscribe/pkg/config"
	"github.com/eternnoir/gollmscribe/pkg/logger"
	"github.com/eternnoir/gollmscribe/pkg/providers/gemini"
	"github.com/eternnoir/gollmscribe/pkg/transcriber"
)

// transcribeCmd represents the transcribe command
var transcribeCmd = &cobra.Command{
	Use:   "transcribe [files...]",
	Short: "Transcribe audio/video files to text",
	Long: `Transcribe audio or video files to text using AI models.

Supported formats:
- Audio: WAV, MP3, M4A, FLAC
- Video: MP4 (automatically converted to audio)

Examples:
  # Transcribe a single file
  gollmscribe transcribe audio.mp3

  # Transcribe with custom output
  gollmscribe transcribe video.mp4 -o transcript.json

  # Transcribe with custom prompt
  gollmscribe transcribe meeting.wav -p "Transcribe the meeting and list action items"

  # Batch transcribe with custom settings
  gollmscribe transcribe *.wav --chunk-minutes 20 --overlap-seconds 45

  # Transcribe with speaker identification
  gollmscribe transcribe interview.mp3 --with-speaker-id --with-timestamp`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTranscribe,
}

func init() {
	rootCmd.AddCommand(transcribeCmd)

	// Output options
	transcribeCmd.Flags().StringP("output", "o", "", "output file path (default: input_file.txt)")
	transcribeCmd.Flags().String("format", "text", "output format (json, text, srt)")

	// Transcription options
	transcribeCmd.Flags().StringP("prompt", "p", "", "custom transcription prompt")
	transcribeCmd.Flags().String("prompt-file", "", "file containing custom prompt")
	transcribeCmd.Flags().String("language", "auto", "language code (auto, zh-TW, en, etc.)")
	transcribeCmd.Flags().Bool("with-timestamp", true, "include timestamps in output")
	transcribeCmd.Flags().Bool("with-speaker-id", true, "include speaker identification")

	// Processing options
	transcribeCmd.Flags().Int("chunk-minutes", 30, "chunk duration in minutes")
	transcribeCmd.Flags().Int("overlap-seconds", 60, "overlap duration in seconds")
	transcribeCmd.Flags().Int("workers", 3, "number of concurrent workers")
	transcribeCmd.Flags().Float32("temperature", 0.1, "LLM temperature (0.0-1.0)")

	// Advanced options
	transcribeCmd.Flags().Bool("preserve-audio", false, "keep temporary audio files")
	transcribeCmd.Flags().Bool("progress", true, "show progress during transcription")

	// Bind flags to viper
	_ = viper.BindPFlag("transcribe.format", transcribeCmd.Flags().Lookup("format"))
	_ = viper.BindPFlag("transcribe.language", transcribeCmd.Flags().Lookup("language"))
	_ = viper.BindPFlag("transcribe.with_timestamp", transcribeCmd.Flags().Lookup("with-timestamp"))
	_ = viper.BindPFlag("transcribe.with_speaker_id", transcribeCmd.Flags().Lookup("with-speaker-id"))
	_ = viper.BindPFlag("transcribe.chunk_minutes", transcribeCmd.Flags().Lookup("chunk-minutes"))
	_ = viper.BindPFlag("transcribe.overlap_seconds", transcribeCmd.Flags().Lookup("overlap-seconds"))
	_ = viper.BindPFlag("transcribe.workers", transcribeCmd.Flags().Lookup("workers"))
	_ = viper.BindPFlag("transcribe.temperature", transcribeCmd.Flags().Lookup("temperature"))
	_ = viper.BindPFlag("transcribe.preserve_audio", transcribeCmd.Flags().Lookup("preserve-audio"))
}

func runTranscribe(cmd *cobra.Command, args []string) error {
	log := logger.WithComponent("transcribe")

	log.Info().Int("file_count", len(args)).Strs("files", args).Msg("Starting transcription")

	// Validate API key
	apiKey := viper.GetString("api_key")
	if apiKey == "" {
		log.Error().Msg("API key is required")
		return fmt.Errorf("API key is required. Set GOLLMSCRIBE_API_KEY environment variable or use --api-key flag")
	}

	// Get configuration
	cfg := loadConfig()
	log.Debug().Interface("config", cfg).Msg("Loaded configuration")

	// Initialize provider
	provider, err := initializeProvider(cfg)
	if err != nil {
		log.Error().Err(err).Str("provider", cfg.Provider.Name).Msg("Failed to initialize provider")
		return fmt.Errorf("failed to initialize provider: %w", err)
	}
	log.Info().Str("provider", cfg.Provider.Name).Msg("Initialized LLM provider")

	// Initialize transcriber
	tempDir := viper.GetString("temp_dir")
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	log.Debug().Str("temp_dir", tempDir).Msg("Using temporary directory")
	tr := transcriber.NewTranscriber(provider, tempDir)

	// Get transcription options
	options := getTranscribeOptions(cmd)
	log.Debug().Interface("options", options).Msg("Transcription options configured")

	// Get custom prompt
	customPrompt, err := getCustomPrompt(cmd)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get custom prompt")
		return fmt.Errorf("failed to get custom prompt: %w", err)
	}
	if customPrompt != "" {
		log.Info().Str("prompt", customPrompt).Msg("Using custom transcription prompt")
	}

	// Process files
	successCount := 0
	failureCount := 0

	for _, filePath := range args {
		fileLog := log.WithField("file", filepath.Base(filePath))
		fileLog.Info().Msg("Processing file")

		if err := processFile(tr, filePath, options, customPrompt, cmd); err != nil {
			fileLog.Error().Err(err).Msg("Failed to process file")
			failureCount++
			continue
		}
		fileLog.Info().Msg("Successfully processed file")
		successCount++
	}

	log.Info().
		Int("successful", successCount).
		Int("failed", failureCount).
		Int("total", len(args)).
		Msg("Transcription batch completed")

	return nil
}

func loadConfig() *config.Config {
	cfg := config.DefaultConfig()

	// Override with viper values
	cfg.Provider.APIKey = viper.GetString("api_key")
	cfg.Provider.Name = viper.GetString("provider")
	cfg.Audio.TempDir = viper.GetString("temp_dir")
	cfg.Transcribe.Language = viper.GetString("transcribe.language")
	cfg.Output.Format = viper.GetString("transcribe.format")

	return cfg
}

func initializeProvider(cfg *config.Config) (*gemini.Provider, error) {
	log := logger.WithComponent("provider")

	switch cfg.Provider.Name {
	case "gemini":
		// Use longer timeout for audio transcription
		timeout := cfg.Provider.Timeout
		if timeout < 5*time.Minute {
			timeout = 5 * time.Minute // Minimum 5 minutes for audio processing
			log.Debug().
				Dur("original_timeout", cfg.Provider.Timeout).
				Dur("adjusted_timeout", timeout).
				Msg("Adjusted timeout for audio processing")
		}

		log.Debug().
			Dur("timeout", timeout).
			Int("retries", cfg.Provider.Retries).
			Msg("Creating Gemini provider")

		provider := gemini.NewProvider(
			cfg.Provider.APIKey,
			gemini.WithTimeout(timeout),
			gemini.WithRetries(cfg.Provider.Retries),
		)

		log.Debug().Msg("Validating provider configuration")
		if err := provider.ValidateConfig(); err != nil {
			log.Error().Err(err).Msg("Provider validation failed")
			return nil, fmt.Errorf("provider validation failed: %w", err)
		}

		log.Info().Msg("Gemini provider initialized successfully")
		return provider, nil
	default:
		log.Error().Str("provider", cfg.Provider.Name).Msg("Unsupported provider")
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider.Name)
	}
}

func getTranscribeOptions(cmd *cobra.Command) transcriber.TranscribeOptions {
	chunkMinutes, _ := cmd.Flags().GetInt("chunk-minutes")
	overlapSeconds, _ := cmd.Flags().GetInt("overlap-seconds")
	workers, _ := cmd.Flags().GetInt("workers")
	temperature, _ := cmd.Flags().GetFloat32("temperature")
	withTimestamp, _ := cmd.Flags().GetBool("with-timestamp")
	withSpeakerID, _ := cmd.Flags().GetBool("with-speaker-id")
	preserveAudio, _ := cmd.Flags().GetBool("preserve-audio")
	format, _ := cmd.Flags().GetString("format")

	return transcriber.TranscribeOptions{
		ChunkMinutes:   chunkMinutes,
		OverlapSeconds: overlapSeconds,
		WithTimestamp:  withTimestamp,
		WithSpeakerID:  withSpeakerID,
		Workers:        workers,
		Temperature:    temperature,
		OutputFormat:   format,
		PreserveAudio:  preserveAudio,
	}
}

func getCustomPrompt(cmd *cobra.Command) (string, error) {
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

func processFile(tr transcriber.Transcriber, filePath string, options transcriber.TranscribeOptions, customPrompt string, cmd *cobra.Command) error {
	log := logger.WithComponent("processor").WithField("file", filepath.Base(filePath))

	log.Debug().Str("full_path", filePath).Msg("Starting file processing")

	// Validate file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Error().Str("path", filePath).Msg("File does not exist")
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Get output path
	outputPath, _ := cmd.Flags().GetString("output")
	if outputPath == "" {
		ext := ".txt"
		if options.OutputFormat == "json" {
			ext = ".json"
		} else if options.OutputFormat == "srt" {
			ext = ".srt"
		}
		outputPath = strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ext
	}
	log.Debug().Str("output_path", outputPath).Str("format", options.OutputFormat).Msg("Output configuration")

	// Get language
	language, _ := cmd.Flags().GetString("language")
	log.Debug().Str("language", language).Msg("Language configuration")

	// Create transcription request
	req := &transcriber.TranscribeRequest{
		FilePath:     filePath,
		OutputPath:   outputPath,
		Language:     language,
		CustomPrompt: customPrompt,
		Options:      options,
	}
	log.Debug().Interface("request", req).Msg("Created transcription request")

	// Show progress
	showProgress, _ := cmd.Flags().GetBool("progress")
	var progressCallback transcriber.ProgressCallback
	if showProgress {
		progressCallback = func(completed, total int, currentChunk string) {
			fmt.Printf("\r[%s] Processing %s: %d/%d chunks completed",
				filepath.Base(filePath), currentChunk, completed, total)
			if completed == total {
				fmt.Println() // New line when complete
			}
		}
	}

	// Start transcription
	ctx := context.Background()
	startTime := time.Now()
	log.Info().Msg("Starting transcription")

	var result *transcriber.TranscribeResult
	var err error

	if progressCallback != nil {
		log.Debug().Msg("Using progress callback")
		result, err = tr.TranscribeWithProgress(ctx, req, progressCallback)
	} else {
		log.Debug().Msg("Running without progress callback")
		result, err = tr.Transcribe(ctx, req)
	}

	if err != nil {
		log.Error().Err(err).Dur("elapsed", time.Since(startTime)).Msg("Transcription failed")
		return fmt.Errorf("transcription failed: %w", err)
	}

	// Show results
	duration := time.Since(startTime)

	log.Info().
		Dur("duration", duration).
		Dur("audio_duration", result.Duration).
		Int("chunks", result.ChunkCount).
		Int("text_length", len(result.Text)).
		Int("segments", len(result.Segments)).
		Str("provider", result.Provider).
		Dur("processing_time", result.ProcessTime).
		Msg("Transcription completed successfully")

	fmt.Printf("✓ Transcribed %s in %v\n", filepath.Base(filePath), duration.Round(time.Second))
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Printf("  Duration: %v\n", result.Duration.Round(time.Second))
	fmt.Printf("  Chunks: %d\n", result.ChunkCount)
	fmt.Printf("  Text length: %d characters\n", len(result.Text))

	if len(result.Segments) > 0 {
		fmt.Printf("  Segments: %d\n", len(result.Segments))
	}

	if viper.GetBool("verbose") {
		fmt.Printf("  Provider: %s\n", result.Provider)
		fmt.Printf("  Processing time: %v\n", result.ProcessTime.Round(time.Millisecond))
	}

	return nil
}
