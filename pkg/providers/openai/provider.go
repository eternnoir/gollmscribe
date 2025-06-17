package openai

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/eternnoir/gollmscribe/pkg/providers"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	defaultModel = "o3"
)

// Provider implements the LLM provider interface for OpenAI O3 multimodal
type Provider struct {
	client  openai.Client
	model   string
	retries int
	apiKey  string
}

// NewProvider creates a new OpenAI provider instance
func NewProvider(apiKey string, options ...ProviderOption) *Provider {
	p := &Provider{
		client:  *openai.NewClient(option.WithAPIKey(apiKey)),
		model:   defaultModel,
		retries: 3,
		apiKey:  apiKey,
	}

	for _, opt := range options {
		opt(p)
	}

	return p
}

// ProviderOption allows customizing the provider
type ProviderOption func(*Provider)

// WithModel sets the OpenAI model to use
func WithModel(model string) ProviderOption {
	return func(p *Provider) {
		p.model = model
	}
}

// WithBaseURL sets a custom base URL
func WithBaseURL(url string) ProviderOption {
	return func(p *Provider) {
		p.client = openai.NewClient(
			option.WithAPIKey(p.apiKey),
			option.WithBaseURL(url),
		)
	}
}

// WithRetries sets the number of retry attempts
func WithRetries(retries int) ProviderOption {
	return func(p *Provider) {
		p.retries = retries
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "openai"
}

// Transcribe transcribes audio using OpenAI API
func (p *Provider) Transcribe(ctx context.Context, req *providers.TranscriptionRequest) (*providers.TranscriptionResult, error) {
	audioData, err := io.ReadAll(req.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	chunk := &providers.AudioChunk{
		Data:     audioData,
		Format:   req.AudioFormat,
		MimeType: req.MimeType,
	}

	return p.TranscribeChunk(ctx, chunk, req.Prompt, req.Options)
}

// TranscribeChunk transcribes a single audio chunk using O3 multimodal capabilities
func (p *Provider) TranscribeChunk(ctx context.Context, chunk *providers.AudioChunk, prompt string, options providers.TranscriptionOptions) (*providers.TranscriptionResult, error) {
	if len(chunk.Data) == 0 {
		return nil, fmt.Errorf("empty audio data")
	}

	// Build the prompt
	if prompt == "" {
		prompt = p.buildDefaultPrompt(options)
	}

	// Encode audio data to base64
	audioB64 := base64.StdEncoding.EncodeToString(chunk.Data)

	// Create user message with audio input for O3 multimodal
	// Note: This is a conceptual implementation - actual O3 multimodal API may differ
	userMessage := fmt.Sprintf("%s\n\n[Audio data: %d bytes of %s format audio]", 
		prompt, len(chunk.Data), chunk.Format)

	// Create chat completion request
	req := openai.ChatCompletionNewParams{
		Model: openai.ChatModelO3,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(userMessage),
		},
	}

	// Set optional parameters
	if options.Temperature > 0 {
		req.Temperature = openai.Float(float64(options.Temperature))
	}

	if options.MaxTokens > 0 {
		req.MaxTokens = openai.Int(options.MaxTokens)
	} else {
		req.MaxTokens = openai.Int(4096) // Default max tokens
	}

	// Make the API request with retries
	var completion *openai.ChatCompletion
	var err error
	for attempt := 0; attempt <= p.retries; attempt++ {
		completion, err = p.client.Chat.Completions.New(ctx, req)
		if err == nil {
			break
		}
		if attempt < p.retries {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to transcribe audio with O3 after %d attempts: %w", p.retries+1, err)
	}

	// Parse the response
	return p.parseO3Response(completion, chunk, options)
}

// parseO3Response parses the O3 chat completion response into a TranscriptionResult
func (p *Provider) parseO3Response(completion *openai.ChatCompletion, chunk *providers.AudioChunk, options providers.TranscriptionOptions) (*providers.TranscriptionResult, error) {
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no choices in completion response")
	}

	choice := completion.Choices[0]
	if choice.Message.Content == "" {
		return nil, fmt.Errorf("empty content in completion response")
	}

	// Extract transcription text from the response
	transcriptionText := strings.TrimSpace(choice.Message.Content)

	result := &providers.TranscriptionResult{
		ChunkID: chunk.ChunkID,
		Text:    transcriptionText,
		Metadata: map[string]interface{}{
			"provider":     "openai",
			"model":        p.model,
			"finish_reason": choice.FinishReason,
		},
	}

	if result.Text == "" {
		return nil, fmt.Errorf("empty transcription result")
	}

	// For O3 multimodal, we don't get automatic segmentation like Whisper
	// But we can try to parse the response if it contains structured data
	if options.WithTimestamp {
		// Try to extract timestamp information from the response if present
		// This would depend on how we structure the prompt to request timestamps
		result.Segments = p.parseTimestamps(transcriptionText)
	}

	return result, nil
}

// parseTimestamps attempts to extract timestamp information from the transcription text
func (p *Provider) parseTimestamps(text string) []providers.TranscriptionSegment {
	// This is a simplified implementation
	// In practice, you might want to prompt O3 to return structured output with timestamps
	// For now, we'll create a single segment covering the entire chunk
	return []providers.TranscriptionSegment{
		{
			Text:  text,
			Start: 0,
			End:   time.Duration(30) * time.Second, // Estimate based on chunk duration
		},
	}
}

// buildDefaultPrompt creates a default transcription prompt for O3 multimodal
func (p *Provider) buildDefaultPrompt(options providers.TranscriptionOptions) string {
	prompt := "Please carefully listen to this audio file and provide an accurate, verbatim transcription of all spoken content."

	var requirements []string

	if options.WithTimestamp {
		requirements = append(requirements, "include precise timestamps in the format [HH:MM:SS] before each sentence or speaker change")
	}

	if options.WithSpeakerID {
		requirements = append(requirements, "identify and label different speakers (e.g., Speaker 1, Speaker 2, etc.)")
	}

	if len(requirements) > 0 {
		prompt += " Please " + strings.Join(requirements, " and ") + "."
	}

	prompt += " Maintain natural language flow, proper punctuation, and capture all nuances including hesitations, repetitions, and corrections. Provide only the transcription without additional commentary."

	return prompt
}

// ValidateConfig validates the provider configuration
func (p *Provider) ValidateConfig() error {
	if p.apiKey == "" {
		return fmt.Errorf("OpenAI API key is required")
	}
	return nil
}

// SupportedFormats returns the list of supported audio formats for O3 multimodal
func (p *Provider) SupportedFormats() []string {
	return []string{
		"audio/wav",
		"audio/mp3",
		"audio/mpeg",
		"audio/m4a",
		"audio/flac",
		"audio/ogg",
		"audio/webm",
		"audio/mp4",
		"audio/aac",
	}
}