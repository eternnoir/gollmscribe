package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/eternnoir/gollmscribe/pkg/providers/gemini"
	"github.com/eternnoir/gollmscribe/pkg/transcriber"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("GOLLMSCRIBE_API_KEY")
	if apiKey == "" {
		log.Fatal("GOLLMSCRIBE_API_KEY environment variable is required")
	}

	// Check for input file argument
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <audio_file>")
	}
	inputFile := os.Args[1]

	// Initialize provider
	provider := gemini.NewProvider(apiKey)
	if err := provider.ValidateConfig(); err != nil {
		log.Fatalf("Provider configuration error: %v", err)
	}

	// Initialize transcriber
	tr := transcriber.NewTranscriber(provider, "")

	// Create transcription request
	req := &transcriber.TranscribeRequest{
		FilePath: inputFile,
		Language: "auto",
		Options: transcriber.TranscribeOptions{
			ChunkMinutes:   30,
			OverlapSeconds: 60,
			WithTimestamp:  true,
			WithSpeakerID:  true,
			Workers:        3,
			Temperature:    0.1,
			OutputFormat:   "json",
		},
	}

	// Progress callback
	progressCallback := func(completed, total int, currentChunk string) {
		fmt.Printf("\rProgress: %d/%d chunks completed (%s)", completed, total, currentChunk)
		if completed == total {
			fmt.Println() // New line when complete
		}
	}

	// Transcribe the file
	fmt.Printf("Transcribing: %s\n", inputFile)
	ctx := context.Background()
	result, err := tr.TranscribeWithProgress(ctx, req, progressCallback)
	if err != nil {
		log.Fatalf("Transcription failed: %v", err)
	}

	// Display results
	fmt.Println("\n=== Transcription Results ===")
	fmt.Printf("File: %s\n", result.FilePath)
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Provider: %s\n", result.Provider)
	fmt.Printf("Processing time: %v\n", result.ProcessTime)
	fmt.Printf("Chunks processed: %d\n", result.ChunkCount)
	fmt.Printf("Language: %s\n", result.Language)
	fmt.Printf("Segments: %d\n", len(result.Segments))
	fmt.Printf("Text length: %d characters\n", len(result.Text))

	fmt.Println("\n=== Transcribed Text ===")
	fmt.Println(result.Text)

	// Show first few segments if available
	if len(result.Segments) > 0 {
		fmt.Println("\n=== First 3 Segments ===")
		for i, segment := range result.Segments {
			if i >= 3 {
				break
			}
			fmt.Printf("[%v - %v]", segment.Start, segment.End)
			if segment.SpeakerID != "" {
				fmt.Printf(" %s:", segment.SpeakerID)
			}
			fmt.Printf(" %s\n", segment.Text)
		}
	}
}
