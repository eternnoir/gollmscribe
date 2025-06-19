package transcriber

import (
	"context"
	"time"

	"github.com/eternnoir/gollmscribe/pkg/providers"
)

// TranscribeRequest represents a complete transcription request
type TranscribeRequest struct {
	FilePath     string
	OutputPath   string
	CustomPrompt string
	Options      TranscribeOptions
}

// TranscribeOptions provides configuration for the transcription process
type TranscribeOptions struct {
	ChunkMinutes     int // Default: 30
	OverlapSeconds   int // Default: 60
	Workers          int // Default: 3
	Temperature      float32
	PreserveAudio    bool   // Keep temporary audio files
	VoiceProfilesDir string // Directory containing voice profile audio files for speaker identification
}

// TranscribeResult represents the complete transcription result
type TranscribeResult struct {
	FilePath    string                           `json:"file_path"`
	Text        string                           `json:"text"`
	Segments    []providers.TranscriptionSegment `json:"segments,omitempty"`
	Language    string                           `json:"language,omitempty"`
	Duration    time.Duration                    `json:"duration,omitempty"`
	ChunkCount  int                              `json:"chunk_count,omitempty"`
	ProcessTime time.Duration                    `json:"process_time,omitempty"`
	Provider    string                           `json:"provider"`
	Metadata    map[string]interface{}           `json:"metadata,omitempty"`
}

// ProgressCallback is called during transcription to report progress
type ProgressCallback func(completed, total int, currentChunk string)

// Transcriber defines the interface for the main transcription orchestrator
type Transcriber interface {
	// Transcribe processes a single audio/video file
	Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResult, error)

	// TranscribeWithProgress processes a file with progress reporting
	TranscribeWithProgress(ctx context.Context, req *TranscribeRequest, callback ProgressCallback) (*TranscribeResult, error)

	// TranscribeBatch processes multiple files
	TranscribeBatch(ctx context.Context, requests []*TranscribeRequest) ([]*TranscribeResult, error)

	// SupportedFormats returns supported file formats
	SupportedFormats() []string

	// SetProvider changes the LLM provider
	SetProvider(provider providers.LLMProvider)
}

// ChunkMerger handles merging overlapping transcript chunks
type ChunkMerger interface {
	// MergeChunks combines multiple transcription results with overlap handling
	MergeChunks(chunks []*providers.TranscriptionResult) (*TranscribeResult, error)

	// DetectOverlap identifies overlapping content between chunks
	DetectOverlap(chunk1, chunk2 *providers.TranscriptionResult) (time.Duration, time.Duration, error)
}
