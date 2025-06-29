package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/eternnoir/gollmscribe/pkg/logger"
	"github.com/eternnoir/gollmscribe/pkg/providers"
)

const (
	defaultBaseURL = "https://generativelanguage.googleapis.com"
	apiVersion     = "v1beta"
	modelName      = "gemini-2.5-flash"
)

// Provider implements the LLM provider interface for Google Gemini
type Provider struct {
	apiKey     string
	baseURL    string
	model      string
	timeout    time.Duration
	retries    int
	httpClient *http.Client
}

// GeminiRequest represents the request structure for Gemini API
type GeminiRequest struct {
	Contents         []Content         `json:"contents"`
	GenerationConfig *GenerationConfig `json:"generationConfig,omitempty"`
}

// Content represents a content part in the request
type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role,omitempty"`
}

// Part represents a part of the content (text or inline data)
type Part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *InlineData `json:"inlineData,omitempty"`
}

// InlineData represents inline binary data
type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64 encoded
}

// ThinkingConfig contains thinking configuration
type ThinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget,omitempty"`
}

// GenerationConfig contains generation parameters
type GenerationConfig struct {
	Temperature      float32         `json:"temperature,omitempty"`
	MaxOutputTokens  int             `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string          `json:"responseMimeType,omitempty"`
	ThinkingConfig   *ThinkingConfig `json:"thinkingConfig,omitempty"`
}

// GeminiResponse represents the response from Gemini API
type GeminiResponse struct {
	Candidates []Candidate `json:"candidates"`
	Error      *APIError   `json:"error,omitempty"`
}

// Candidate represents a response candidate
type Candidate struct {
	Content          Content       `json:"content"`
	FinishReason     string        `json:"finishReason"`
	CitationMetadata interface{}   `json:"citationMetadata,omitempty"`
	SafetyRatings    []interface{} `json:"safetyRatings,omitempty"`
}

// APIError represents an API error response
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// NewProvider creates a new Gemini provider instance
func NewProvider(apiKey string, options ...ProviderOption) *Provider {
	p := &Provider{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		model:   modelName, // Use default model if not specified
		timeout: 30 * time.Second,
		retries: 3,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute, // 10 minutes for long audio files
		},
	}

	for _, opt := range options {
		opt(p)
	}

	return p
}

// ProviderOption allows customizing the provider
type ProviderOption func(*Provider)

// WithBaseURL sets a custom base URL
func WithBaseURL(baseURL string) ProviderOption {
	return func(p *Provider) {
		p.baseURL = baseURL
	}
}

// WithTimeout sets the request timeout
func WithTimeout(timeout time.Duration) ProviderOption {
	return func(p *Provider) {
		p.timeout = timeout
		// Set HTTP client timeout to be longer than the request timeout
		if timeout > 5*time.Minute {
			p.httpClient.Timeout = timeout + 2*time.Minute
		} else {
			p.httpClient.Timeout = timeout * 2
		}
	}
}

// WithRetries sets the number of retry attempts
func WithRetries(retries int) ProviderOption {
	return func(p *Provider) {
		p.retries = retries
	}
}

// WithModel sets the model name
func WithModel(model string) ProviderOption {
	return func(p *Provider) {
		if model != "" {
			p.model = model
		}
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "gemini"
}

// Transcribe transcribes audio using Gemini API
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

// TranscribeChunk transcribes a single audio chunk
func (p *Provider) TranscribeChunk(ctx context.Context, chunk *providers.AudioChunk, prompt string, options providers.TranscriptionOptions) (*providers.TranscriptionResult, error) {
	if len(chunk.Data) == 0 {
		return nil, fmt.Errorf("empty audio data")
	}

	// Build the prompt
	if prompt == "" {
		prompt = p.buildDefaultPrompt(options)
	}

	// Prepare the request
	geminiReq := &GeminiRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{
						Text: prompt,
					},
					{
						InlineData: &InlineData{
							MimeType: chunk.MimeType,
							Data:     base64.StdEncoding.EncodeToString(chunk.Data),
						},
					},
				},
				Role: "user",
			},
		},
		GenerationConfig: &GenerationConfig{
			Temperature:      options.Temperature,
			MaxOutputTokens:  options.MaxTokens,
			ResponseMimeType: "text/plain",
			ThinkingConfig: &ThinkingConfig{
				ThinkingBudget: -1,
			},
		},
	}

	// Make the API request with retries
	var resp *GeminiResponse
	var err error
	for attempt := 0; attempt <= p.retries; attempt++ {
		resp, err = p.makeRequest(ctx, geminiReq)
		if err == nil {
			break
		}
		if attempt < p.retries {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to make API request after %d attempts: %w", p.retries+1, err)
	}

	// Parse the response
	return p.parseResponse(resp, chunk)
}

// makeRequest makes an HTTP request to the Gemini API
func (p *Provider) makeRequest(ctx context.Context, req *GeminiRequest) (*GeminiResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log request details (without API key)
	url := fmt.Sprintf("%s/%s/models/%s:generateContent", p.baseURL, apiVersion, p.model)
	logger.Debug().
		Str("component", "gemini-provider").
		Str("url", url).
		Str("model", p.model).
		Int("request_size", len(jsonData)).
		Msg("Sending request to Gemini API")

	url = fmt.Sprintf("%s/%s/models/%s:generateContent?key=%s", p.baseURL, apiVersion, p.model, p.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()

	respData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", httpResp.StatusCode, string(respData))
	}

	// Log raw response for debugging
	logger.Debug().
		Str("component", "gemini-provider").
		Str("raw_response", string(respData)).
		Msg("Received raw response from Gemini API")

	var geminiResp GeminiResponse
	if err := json.Unmarshal(respData, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Log parsed response structure
	logger.Debug().
		Str("component", "gemini-provider").
		Int("candidates_count", len(geminiResp.Candidates)).
		Msg("Parsed Gemini response")

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("API error %d: %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	return &geminiResp, nil
}

// parseResponse parses the Gemini API response into a TranscriptionResult
func (p *Provider) parseResponse(resp *GeminiResponse, chunk *providers.AudioChunk) (*providers.TranscriptionResult, error) {
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := resp.Candidates[0]

	// Log candidate details for debugging
	logger.Debug().
		Str("component", "gemini-provider").
		Str("finish_reason", candidate.FinishReason).
		Int("content_parts", len(candidate.Content.Parts)).
		Msg("Processing candidate")

	if len(candidate.Content.Parts) == 0 {
		// Log the entire candidate structure for debugging
		candidateJSON, _ := json.Marshal(candidate)
		logger.Error().
			Str("component", "gemini-provider").
			Str("candidate_json", string(candidateJSON)).
			Msg("No content parts in candidate")
		return nil, fmt.Errorf("no content parts in response")
	}

	responseText := candidate.Content.Parts[0].Text

	result := &providers.TranscriptionResult{
		ChunkID: chunk.ChunkID,
		Text:    strings.TrimSpace(responseText),
		Metadata: map[string]interface{}{
			"provider": "gemini",
			"model":    p.model, // Use the actual model being used
		},
	}

	if result.Text == "" {
		return nil, fmt.Errorf("empty transcription result")
	}

	return result, nil
}

// buildDefaultPrompt creates a default transcription prompt
func (p *Provider) buildDefaultPrompt(_ providers.TranscriptionOptions) string {
	prompt := "Please provide a complete, accurate, word-for-word transcription of the following audio. Include every word spoken, including filler words (um, uh, etc.), false starts, and repetitions. Maintain the speaker's original phrasing and word choice."

	// Add appropriate punctuation and capitalization while preserving the natural speech patterns
	prompt += " Add appropriate punctuation and capitalization while preserving the natural speech patterns."

	return prompt
}

// ValidateConfig validates the provider configuration
func (p *Provider) ValidateConfig() error {
	if p.apiKey == "" {
		return fmt.Errorf("API key is required")
	}
	return nil
}

// SupportedFormats returns the list of supported audio formats
func (p *Provider) SupportedFormats() []string {
	return []string{
		"audio/wav",
		"audio/mp3",
		"audio/mpeg",
		"audio/m4a",
		"audio/flac",
		"audio/ogg",
	}
}
