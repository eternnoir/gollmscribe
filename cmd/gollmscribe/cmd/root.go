package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/eternnoir/gollmscribe/pkg/config"
	"github.com/eternnoir/gollmscribe/pkg/logger"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gollmscribe",
	Short: "AI-powered audio transcription tool",
	Long: `gollmscribe is a Go application that transforms audio files into precise text 
transcripts using advanced Large Language Models and multimodal AI processing capabilities.

Features:
- Support for multiple audio/video formats (WAV, MP3, M4A, FLAC, MP4)
- Automatic video to audio conversion
- Intelligent chunking with overlap for long audio files
- Speaker identification and timestamps
- Multiple output formats (JSON, text, SRT)
- Custom transcription prompts
- Batch processing support`,
	Version: "1.0.0",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gollmscribe.yaml)")
	rootCmd.PersistentFlags().String("api-key", "", "LLM provider API key")
	rootCmd.PersistentFlags().String("provider", "gemini", "LLM provider (gemini, openai)")
	rootCmd.PersistentFlags().String("model", "", "model name to use (e.g., gemini-1.5-pro, gemini-2.5-flash)")
	rootCmd.PersistentFlags().String("temp-dir", "", "temporary directory for processing")
	rootCmd.PersistentFlags().Bool("verbose", false, "verbose output (deprecated, use --log-level debug)")

	// Logging flags
	rootCmd.PersistentFlags().String("log-level", "info", "log level (trace, debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-format", "console", "log format (console, json)")
	rootCmd.PersistentFlags().String("log-output", "stdout", "log output (stdout, stderr, file path)")
	rootCmd.PersistentFlags().Bool("log-no-color", false, "disable colored log output")
	rootCmd.PersistentFlags().Bool("log-caller", false, "include caller information in logs")

	// Bind flags to viper
	_ = viper.BindPFlag("api_key", rootCmd.PersistentFlags().Lookup("api-key"))
	_ = viper.BindPFlag("provider", rootCmd.PersistentFlags().Lookup("provider"))
	_ = viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	_ = viper.BindPFlag("temp_dir", rootCmd.PersistentFlags().Lookup("temp-dir"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Bind logging flags to viper
	_ = viper.BindPFlag("logging.level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("logging.format", rootCmd.PersistentFlags().Lookup("log-format"))
	_ = viper.BindPFlag("logging.output", rootCmd.PersistentFlags().Lookup("log-output"))
	_ = viper.BindPFlag("logging.caller", rootCmd.PersistentFlags().Lookup("log-caller"))
	_ = viper.BindPFlag("logging.no_color", rootCmd.PersistentFlags().Lookup("log-no-color"))

	// Environment variable bindings
	viper.SetEnvPrefix("GOLLMSCRIBE")
	viper.AutomaticEnv()
}

// initConfig reads in config file and ENV variables.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".gollmscribe" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".gollmscribe")
	}

	// If a config file is found, read it in.
	configFileUsed := ""
	if err := viper.ReadInConfig(); err == nil {
		configFileUsed = viper.ConfigFileUsed()
	}

	// Initialize logger
	initLogger()

	// Log config file usage after logger is initialized
	if configFileUsed != "" {
		logger.Info().Str("config_file", configFileUsed).Msg("Loaded configuration file")
	}
}

// initLogger initializes the logger based on configuration
func initLogger() {
	cfg := config.DefaultConfig()

	// Update logging config from viper
	cfg.Logging.Level = viper.GetString("logging.level")
	cfg.Logging.Format = viper.GetString("logging.format")
	cfg.Logging.Output = viper.GetString("logging.output")
	cfg.Logging.Caller = viper.GetBool("logging.caller")

	// Handle legacy verbose flag
	if viper.GetBool("verbose") && cfg.Logging.Level == "info" {
		cfg.Logging.Level = "debug"
	}

	// Handle no-color flag
	if viper.GetBool("logging.no_color") {
		cfg.Logging.PrettyMode = false
	}

	// Initialize the logger
	if err := logger.Initialize(&cfg.Logging); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
}
