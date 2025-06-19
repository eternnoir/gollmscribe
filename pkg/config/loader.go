package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Loader handles configuration loading and management
type Loader struct {
	configPath string
	viper      *viper.Viper
}

// NewLoader creates a new configuration loader
func NewLoader(configPath string) *Loader {
	v := viper.New()

	// Set up environment variable handling
	v.SetEnvPrefix("GOLLMSCRIBE")
	v.AutomaticEnv()

	// Set up configuration file search paths
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Search in multiple locations
		home, _ := os.UserHomeDir()
		v.AddConfigPath(home)
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/gollmscribe")
		v.SetConfigName(".gollmscribe")
		v.SetConfigType("yaml")
	}

	return &Loader{
		configPath: configPath,
		viper:      v,
	}
}

// Load reads and returns the configuration
func (l *Loader) Load() (*Config, error) {
	// Set defaults
	l.setDefaults()

	// Try to read config file
	if err := l.viper.ReadInConfig(); err != nil {
		// Config file not found is not an error - we'll use defaults and env vars
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal configuration
	var cfg Config
	if err := l.viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := l.validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// LoadWithOverrides loads configuration with command-line overrides
func (l *Loader) LoadWithOverrides(overrides map[string]interface{}) (*Config, error) {
	// Load base configuration
	cfg, err := l.Load()
	if err != nil {
		return nil, err
	}

	// Apply overrides
	for key, value := range overrides {
		l.viper.Set(key, value)
	}

	// Re-unmarshal with overrides
	if err := l.viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config with overrides: %w", err)
	}

	return cfg, nil
}

// Save writes the current configuration to file
func (l *Loader) Save(cfg *Config) error {
	// Determine config file path
	configFile := l.configPath
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configFile = filepath.Join(home, ".gollmscribe.yaml")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configFile), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal configuration to viper
	l.viper.Set("provider", cfg.Provider)
	l.viper.Set("audio", cfg.Audio)
	l.viper.Set("transcribe", cfg.Transcribe)
	l.viper.Set("output", cfg.Output)

	// Write to file
	if err := l.viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfigFile returns the path to the config file being used
func (l *Loader) GetConfigFile() string {
	return l.viper.ConfigFileUsed()
}

// setDefaults sets default configuration values
func (l *Loader) setDefaults() {
	// Provider defaults
	l.viper.SetDefault("provider.name", "gemini")
	l.viper.SetDefault("provider.timeout", "30s")
	l.viper.SetDefault("provider.retries", 3)
	l.viper.SetDefault("provider.temperature", 0.1)
	l.viper.SetDefault("provider.max_tokens", 4096)

	// Audio processing defaults
	l.viper.SetDefault("audio.chunk_minutes", 30)
	l.viper.SetDefault("audio.overlap_seconds", 60)
	l.viper.SetDefault("audio.output_format", "mp3")
	l.viper.SetDefault("audio.sample_rate", 44100)
	l.viper.SetDefault("audio.quality", 5)
	l.viper.SetDefault("audio.workers", 3)
	l.viper.SetDefault("audio.temp_dir", os.TempDir())
	l.viper.SetDefault("audio.keep_temp_files", false)

	// Transcription defaults
	l.viper.SetDefault("transcribe.language", "auto")
	l.viper.SetDefault("transcribe.with_timestamp", true)
	l.viper.SetDefault("transcribe.with_speaker_id", true)
	l.viper.SetDefault("transcribe.auto_language_detect", true)
	l.viper.SetDefault("transcribe.confidence_threshold", 0.8)
	l.viper.SetDefault("transcribe.default_prompt", "Please transcribe the following audio into an accurate verbatim transcript with timestamps and speaker identification. Maintain natural language flow and punctuate properly.")

	// Output defaults
	l.viper.SetDefault("output.format", "json")
	l.viper.SetDefault("output.include_metadata", true)
	l.viper.SetDefault("output.pretty_print", true)
}

// validateConfig validates the loaded configuration
func (l *Loader) validateConfig(cfg *Config) error {
	// Validate provider
	if cfg.Provider.Name == "" {
		return fmt.Errorf("provider name is required")
	}

	// Validate API key is set (either in config or environment)
	if cfg.Provider.APIKey == "" && os.Getenv("GOLLMSCRIBE_API_KEY") == "" {
		return fmt.Errorf("API key is required (set in config file or GOLLMSCRIBE_API_KEY environment variable)")
	}

	// Validate audio settings
	if cfg.Audio.ChunkMinutes <= 0 {
		return fmt.Errorf("chunk_minutes must be positive")
	}

	if cfg.Audio.OverlapSeconds < 0 {
		return fmt.Errorf("overlap_seconds cannot be negative")
	}

	if cfg.Audio.Workers <= 0 {
		return fmt.Errorf("workers must be positive")
	}

	// Validate temperature
	if cfg.Provider.Temperature < 0 || cfg.Provider.Temperature > 1 {
		return fmt.Errorf("temperature must be between 0 and 1")
	}

	return nil
}

// CreateSampleConfig creates a sample configuration file
func CreateSampleConfig(path string) error {
	cfg := DefaultConfig()

	// Remove sensitive information for sample
	cfg.Provider.APIKey = "your-api-key-here"

	loader := NewLoader(path)
	return loader.Save(cfg)
}

// GetFromEnv gets configuration values from environment variables
func GetFromEnv() map[string]interface{} {
	overrides := make(map[string]interface{})

	// Check for environment variables
	if apiKey := os.Getenv("GOLLMSCRIBE_API_KEY"); apiKey != "" {
		overrides["provider.api_key"] = apiKey
	}

	if provider := os.Getenv("GOLLMSCRIBE_PROVIDER"); provider != "" {
		overrides["provider.name"] = provider
	}

	if tempDir := os.Getenv("GOLLMSCRIBE_TEMP_DIR"); tempDir != "" {
		overrides["audio.temp_dir"] = tempDir
	}

	return overrides
}
