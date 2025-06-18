package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/eternnoir/gollmscribe/pkg/config"
	"github.com/eternnoir/gollmscribe/pkg/providers/gemini"
	"github.com/eternnoir/gollmscribe/pkg/transcriber"
)

func main() {
	// Load configuration
	cfg, err := loadConfiguration()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Check for input files
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <audio_files...>")
	}
	inputFiles := os.Args[1:]

	// Initialize provider
	provider, err := initializeProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize provider: %v", err)
	}

	// Initialize transcriber
	tr := transcriber.NewTranscriber(provider, cfg)

	// Process files
	if len(inputFiles) == 1 {
		// Single file processing
		if err := processSingleFile(tr, inputFiles[0], cfg); err != nil {
			log.Fatalf("Failed to process file: %v", err)
		}
	} else {
		// Batch processing
		if err := processBatchFiles(tr, inputFiles, cfg); err != nil {
			log.Fatalf("Failed to process batch: %v", err)
		}
	}
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
		return nil, fmt.Errorf("API key is required. Set GOLLMSCRIBE_API_KEY environment variable or configure in ~/.gollmscribe.yaml")
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

func processSingleFile(tr transcriber.Transcriber, inputFile string, cfg *config.Config) error {
	fmt.Printf("Processing file: %s\n", inputFile)

	// Determine output path
	outputPath := generateOutputPath(inputFile, "txt")

	// Choose prompt based on file name hints
	customPrompt := choosePromptByFilename(inputFile, cfg)

	// Create transcription request
	req := &transcriber.TranscribeRequest{
		FilePath:     inputFile,
		OutputPath:   outputPath,
		CustomPrompt: customPrompt,
		Options: transcriber.TranscribeOptions{
			ChunkMinutes:   cfg.Audio.ChunkMinutes,
			OverlapSeconds: cfg.Audio.OverlapSeconds,
			Workers:        cfg.Audio.Workers,
			Temperature:    cfg.Provider.Temperature,
			PreserveAudio:  cfg.Audio.KeepTempFiles,
		},
	}

	// Progress callback with detailed information
	progressCallback := func(completed, total int, currentChunk string) {
		percentage := float64(completed) / float64(total) * 100
		fmt.Printf("\r[%s] Processing %s: %d/%d chunks (%.1f%%) completed",
			filepath.Base(inputFile), currentChunk, completed, total, percentage)
		if completed == total {
			fmt.Println() // New line when complete
		}
	}

	// Transcribe with progress
	ctx := context.Background()
	result, err := tr.TranscribeWithProgress(ctx, req, progressCallback)
	if err != nil {
		return fmt.Errorf("transcription failed: %w", err)
	}

	// Display detailed results
	displayResults(result, cfg)

	return nil
}

func processBatchFiles(tr transcriber.Transcriber, inputFiles []string, cfg *config.Config) error {
	fmt.Printf("Processing %d files in batch mode...\n", len(inputFiles))

	// Create batch requests
	var requests []*transcriber.TranscribeRequest
	for _, inputFile := range inputFiles {
		outputPath := generateOutputPath(inputFile, "txt")
		customPrompt := choosePromptByFilename(inputFile, cfg)

		req := &transcriber.TranscribeRequest{
			FilePath:     inputFile,
			OutputPath:   outputPath,
			CustomPrompt: customPrompt,
			Options: transcriber.TranscribeOptions{
				ChunkMinutes:   cfg.Audio.ChunkMinutes,
				OverlapSeconds: cfg.Audio.OverlapSeconds,
				Workers:        cfg.Audio.Workers,
				Temperature:    cfg.Provider.Temperature,
				PreserveAudio:  cfg.Audio.KeepTempFiles,
			},
		}
		requests = append(requests, req)
	}

	// Process batch
	ctx := context.Background()
	results, err := tr.TranscribeBatch(ctx, requests)
	if err != nil {
		return fmt.Errorf("batch processing failed: %w", err)
	}

	// Display batch results summary
	fmt.Println("\n=== Batch Processing Summary ===")
	successful := 0
	totalDuration := int64(0)
	totalChunks := 0

	for i, result := range results {
		if result != nil {
			successful++
			totalDuration += int64(result.Duration.Seconds())
			totalChunks += result.ChunkCount
			fmt.Printf("✓ %s - Duration: %v, Chunks: %d\n",
				filepath.Base(result.FilePath), result.Duration, result.ChunkCount)
		} else {
			fmt.Printf("✗ %s - Failed\n", filepath.Base(inputFiles[i]))
		}
	}

	fmt.Printf("\nSummary: %d/%d files processed successfully\n", successful, len(inputFiles))
	fmt.Printf("Total audio duration: %d seconds\n", totalDuration)
	fmt.Printf("Total chunks processed: %d\n", totalChunks)

	return nil
}

func generateOutputPath(inputFile, format string) string {
	ext := ".json"
	switch format {
	case "text":
		ext = ".txt"
	case "srt":
		ext = ".srt"
	}
	return strings.TrimSuffix(inputFile, filepath.Ext(inputFile)) + ext
}

func choosePromptByFilename(filename string, cfg *config.Config) string {
	basename := strings.ToLower(filepath.Base(filename))

	// Check for keywords in filename
	if strings.Contains(basename, "meeting") || strings.Contains(basename, "會議") {
		if prompt, exists := cfg.Transcribe.PromptTemplates["meeting"]; exists {
			return prompt
		}
	}

	if strings.Contains(basename, "interview") || strings.Contains(basename, "訪談") {
		if prompt, exists := cfg.Transcribe.PromptTemplates["interview"]; exists {
			return prompt
		}
	}

	if strings.Contains(basename, "lecture") || strings.Contains(basename, "課程") || strings.Contains(basename, "教學") {
		if prompt, exists := cfg.Transcribe.PromptTemplates["lecture"]; exists {
			return prompt
		}
	}

	// Return default prompt
	return cfg.Transcribe.DefaultPrompt
}

func displayResults(result *transcriber.TranscribeResult, cfg *config.Config) {
	fmt.Println("\n=== Transcription Results ===")
	fmt.Printf("File: %s\n", result.FilePath)
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Provider: %s\n", result.Provider)
	fmt.Printf("Processing time: %v\n", result.ProcessTime)
	fmt.Printf("Chunks processed: %d\n", result.ChunkCount)

	if result.Language != "" {
		fmt.Printf("Detected language: %s\n", result.Language)
	}

	fmt.Printf("Segments: %d\n", len(result.Segments))
	fmt.Printf("Text length: %d characters\n", len(result.Text))

	// Show text preview
	textPreview := result.Text
	if len(textPreview) > 200 {
		textPreview = textPreview[:200] + "..."
	}
	fmt.Printf("Text preview: %s\n", textPreview)

	// Show segment statistics
	if len(result.Segments) > 0 {
		fmt.Printf("Average segment length: %.1f seconds\n",
			float64(result.Duration.Seconds())/float64(len(result.Segments)))

		// Count unique speakers
		speakers := make(map[string]bool)
		for _, segment := range result.Segments {
			if segment.SpeakerID != "" {
				speakers[segment.SpeakerID] = true
			}
		}
		if len(speakers) > 0 {
			fmt.Printf("Unique speakers detected: %d\n", len(speakers))
		}
	}

	// Show metadata if available
	if cfg.Output.IncludeMetadata && len(result.Metadata) > 0 {
		fmt.Println("\n=== Metadata ===")
		for key, value := range result.Metadata {
			fmt.Printf("%s: %v\n", key, value)
		}
	}
}
