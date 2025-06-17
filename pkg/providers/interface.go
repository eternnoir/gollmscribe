package providers

import (
	"context"
	"io"
	"time"
)

// AudioChunk represents a chunk of audio data with metadata
type AudioChunk struct {
	Data     []byte
	Start    time.Duration
	End      time.Duration
	ChunkID  int
	Format   string
	MimeType string
}

// TranscriptionRequest represents a request to transcribe audio
type TranscriptionRequest struct {
	Audio       io.Reader
	AudioFormat string
	MimeType    string
	Filename    string
	Prompt      string
	Options     TranscriptionOptions
}

// TranscriptionOptions provides additional configuration for transcription
type TranscriptionOptions struct {
	Temperature    float32
	MaxTokens      int
	TimeoutSeconds int
}

// TranscriptionSegment represents a segment of transcribed text
type TranscriptionSegment struct {
	Text       string        `json:"text"`
	Start      time.Duration `json:"start,omitempty"`
	End        time.Duration `json:"end,omitempty"`
	SpeakerID  string        `json:"speaker_id,omitempty"`
	Confidence float32       `json:"confidence,omitempty"`
}

// TranscriptionResult represents the result of a transcription request
type TranscriptionResult struct {
	Text     string                 `json:"text"`
	Segments []TranscriptionSegment `json:"segments,omitempty"`
	Language string                 `json:"language,omitempty"`
	Duration time.Duration          `json:"duration,omitempty"`
	ChunkID  int                    `json:"chunk_id,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// LLMProvider defines the interface for LLM transcription providers
type LLMProvider interface {
	// Name returns the provider name (e.g., "gemini", "openai")
	Name() string

	// Transcribe transcribes audio using the LLM provider
	Transcribe(ctx context.Context, req *TranscriptionRequest) (*TranscriptionResult, error)

	// TranscribeChunk transcribes a single audio chunk
	TranscribeChunk(ctx context.Context, chunk *AudioChunk, prompt string, options TranscriptionOptions) (*TranscriptionResult, error)

	// ValidateConfig validates the provider configuration
	ValidateConfig() error

	// SupportedFormats returns the list of supported audio formats
	SupportedFormats() []string
}

// ProviderConfig represents common configuration for providers
type ProviderConfig struct {
	APIKey        string
	BaseURL       string
	Timeout       time.Duration
	RetryAttempts int
}
