package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/eternnoir/gollmscribe/pkg/logger"
)

// Config represents the application configuration
type Config struct {
	// LLM Provider Configuration
	Provider ProviderConfig `yaml:"provider" mapstructure:"provider"`

	// Audio Processing Configuration
	Audio AudioConfig `yaml:"audio" mapstructure:"audio"`

	// Transcription Configuration
	Transcribe TranscribeConfig `yaml:"transcribe" mapstructure:"transcribe"`

	// Output Configuration
	Output OutputConfig `yaml:"output" mapstructure:"output"`

	// Logging Configuration
	Logging logger.Config `yaml:"logging" mapstructure:"logging"`
}

// ProviderConfig contains LLM provider settings
type ProviderConfig struct {
	// Provider name (gemini, openai, etc.)
	Name string `yaml:"name" mapstructure:"name"`

	// API Configuration
	APIKey  string `yaml:"api_key" mapstructure:"api_key"`
	BaseURL string `yaml:"base_url" mapstructure:"base_url"`

	// Request Configuration
	Timeout time.Duration `yaml:"timeout" mapstructure:"timeout"`
	Retries int           `yaml:"retries" mapstructure:"retries"`

	// Model Configuration
	Model       string  `yaml:"model" mapstructure:"model"`
	Temperature float32 `yaml:"temperature" mapstructure:"temperature"`
	MaxTokens   int     `yaml:"max_tokens" mapstructure:"max_tokens"`
}

// AudioConfig contains audio processing settings
type AudioConfig struct {
	// Chunking Configuration
	ChunkMinutes   int `yaml:"chunk_minutes" mapstructure:"chunk_minutes"`
	OverlapSeconds int `yaml:"overlap_seconds" mapstructure:"overlap_seconds"`

	// Conversion Configuration
	OutputFormat string `yaml:"output_format" mapstructure:"output_format"`
	SampleRate   int    `yaml:"sample_rate" mapstructure:"sample_rate"`
	Quality      int    `yaml:"quality" mapstructure:"quality"`

	// Processing Configuration
	TempDir       string `yaml:"temp_dir" mapstructure:"temp_dir"`
	KeepTempFiles bool   `yaml:"keep_temp_files" mapstructure:"keep_temp_files"`
	Workers       int    `yaml:"workers" mapstructure:"workers"`
}

// TranscribeConfig contains transcription settings
type TranscribeConfig struct {
	// Custom Prompts
	DefaultPrompt   string            `yaml:"default_prompt" mapstructure:"default_prompt"`
	PromptTemplates map[string]string `yaml:"prompt_templates" mapstructure:"prompt_templates"`
}

// OutputConfig contains output formatting settings
type OutputConfig struct {
	// File Options
	Directory string `yaml:"directory" mapstructure:"directory"`
	Filename  string `yaml:"filename" mapstructure:"filename"`

	// Content Options
	IncludeMetadata bool `yaml:"include_metadata" mapstructure:"include_metadata"`
	PrettyPrint     bool `yaml:"pretty_print" mapstructure:"pretty_print"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Provider: ProviderConfig{
			Name:        "gemini",
			Timeout:     300 * time.Second,
			Retries:     3,
			Temperature: 0.1,
			MaxTokens:   65535,
		},
		Audio: AudioConfig{
			ChunkMinutes:   15,
			OverlapSeconds: 30,
			OutputFormat:   "mp3",
			SampleRate:     44100,
			Quality:        5,
			Workers:        3,
			TempDir:        filepath.Join(os.TempDir(), "gollmscribe"),
		},
		Transcribe: TranscribeConfig{
			DefaultPrompt: "Please provide a complete, accurate, word-for-word transcription of the following audio. Include every word spoken, including filler words (um, uh, etc.), false starts, and repetitions. Maintain the speaker's original phrasing and word choice. Add appropriate punctuation and capitalization while preserving the natural speech patterns.",
			PromptTemplates: map[string]string{
				"meeting":   "Please transcribe this meeting recording, identify each speaker, and provide a summary of key discussion points and action items at the end.",
				"interview": "Please transcribe this interview, clearly distinguishing between interviewer and interviewee, maintaining the complete question-answer format.",
				"lecture":   "Please transcribe this educational content, identify the instructor's speech, and appropriately mark key concepts and section breaks.",
			},
		},
		Output: OutputConfig{
			IncludeMetadata: true,
			PrettyPrint:     true,
		},
		Logging: *logger.DefaultConfig(),
	}
}
