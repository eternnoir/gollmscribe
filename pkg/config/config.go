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

	// Watch Configuration
	Watch WatchConfig `yaml:"watch" mapstructure:"watch"`

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

// WatchConfig contains watch mode settings
type WatchConfig struct {
	// File patterns to watch (e.g., "*.mp3", "*.wav")
	Patterns []string `yaml:"patterns" mapstructure:"patterns"`

	// Whether to watch subdirectories recursively
	Recursive bool `yaml:"recursive" mapstructure:"recursive"`

	// Polling interval for checking new files
	Interval time.Duration `yaml:"interval" mapstructure:"interval"`

	// Time to wait for file stability before processing
	StabilityWait time.Duration `yaml:"stability_wait" mapstructure:"stability_wait"`

	// Maximum time allowed for processing a single file
	ProcessingTimeout time.Duration `yaml:"processing_timeout" mapstructure:"processing_timeout"`

	// Directory to move processed files to (optional)
	MoveToDir string `yaml:"move_to_dir" mapstructure:"move_to_dir"`

	// Directory to output transcriptions to
	OutputDir string `yaml:"output_dir" mapstructure:"output_dir"`

	// Shared prompt for all transcriptions
	SharedPrompt string `yaml:"shared_prompt" mapstructure:"shared_prompt"`

	// Path to the BoltDB history database
	HistoryDB string `yaml:"history_db" mapstructure:"history_db"`

	// Whether to process existing files on startup
	ProcessExisting bool `yaml:"process_existing" mapstructure:"process_existing"`

	// Whether to retry failed files
	RetryFailed bool `yaml:"retry_failed" mapstructure:"retry_failed"`

	// Maximum number of concurrent processing workers
	MaxWorkers int `yaml:"max_workers" mapstructure:"max_workers"`
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
		Watch: WatchConfig{
			Patterns:          []string{"*.mp3", "*.wav", "*.mp4", "*.m4a"},
			Recursive:         false,
			Interval:          5 * time.Second,
			StabilityWait:     2 * time.Second,
			ProcessingTimeout: 30 * time.Minute,
			HistoryDB:         ".gollmscribe-watch.db",
			ProcessExisting:   true,
			RetryFailed:       false,
			MaxWorkers:        3,
		},
		Logging: *logger.DefaultConfig(),
	}
}
