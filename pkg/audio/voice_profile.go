package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"

	"github.com/eternnoir/gollmscribe/pkg/logger"
)

// VoiceProfileProcessor handles processing voice profile audio files
type VoiceProfileProcessor struct {
	processor Processor
	tempDir   string
}

// VoiceProfileOptions provides configuration for voice profile processing
type VoiceProfileOptions struct {
	TempDir        string        // Temporary directory for processing
	OutputFormat   AudioFormat   // Target format for merged audio
	SilencePadding time.Duration // Padding between audio files (default: 1 second)
	KeepTemp       bool          // Keep temporary files after processing
}

// VoiceProfileResult contains the result of voice profile processing
type VoiceProfileResult struct {
	MergedFilePath string
	SourceFiles    []string
	TotalDuration  time.Duration
	ProfileCount   int
}

// NewVoiceProfileProcessor creates a new voice profile processor
func NewVoiceProfileProcessor(processor Processor, tempDir string) *VoiceProfileProcessor {
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	return &VoiceProfileProcessor{
		processor: processor,
		tempDir:   tempDir,
	}
}

// ProcessVoiceProfiles scans a directory for voice profile audio files and merges them
func (vp *VoiceProfileProcessor) ProcessVoiceProfiles(profileDir string, options VoiceProfileOptions) (*VoiceProfileResult, error) {
	log := logger.WithComponent("voice-profile").WithField("profile_dir", profileDir)

	log.Info().Str("profile_dir", profileDir).Msg("Starting voice profile processing")

	// Set default options
	if options.TempDir == "" {
		options.TempDir = vp.tempDir
	}
	if options.OutputFormat == "" {
		options.OutputFormat = FormatMP3
	}
	if options.SilencePadding == 0 {
		options.SilencePadding = time.Second
	}

	// Scan directory for audio files
	audioFiles, err := vp.scanAudioFiles(profileDir)
	if err != nil {
		log.Error().Err(err).Msg("Failed to scan audio files")
		return nil, fmt.Errorf("failed to scan audio files: %w", err)
	}

	if len(audioFiles) == 0 {
		log.Warn().Msg("No audio files found in profile directory")
		return &VoiceProfileResult{
			SourceFiles:  []string{},
			ProfileCount: 0,
		}, nil
	}

	log.Info().Int("file_count", len(audioFiles)).Msg("Found audio files for voice profiles")

	// Validate all files first
	validFiles := []string{}
	for _, file := range audioFiles {
		if err := vp.processor.ValidateFile(file); err != nil {
			log.Warn().Str("file", filepath.Base(file)).Err(err).Msg("Skipping invalid audio file")
			continue
		}
		validFiles = append(validFiles, file)
	}

	if len(validFiles) == 0 {
		log.Error().Msg("No valid audio files found")
		return nil, fmt.Errorf("no valid audio files found in directory: %s", profileDir)
	}

	log.Info().Int("valid_file_count", len(validFiles)).Msg("Validated audio files")

	// Convert all files to the same format if needed
	normalizedFiles, totalDuration, err := vp.normalizeAudioFiles(validFiles, options)
	if err != nil {
		log.Error().Err(err).Msg("Failed to normalize audio files")
		return nil, fmt.Errorf("failed to normalize audio files: %w", err)
	}

	// Merge all normalized files
	mergedFile, err := vp.mergeAudioFiles(normalizedFiles, options)
	if err != nil {
		log.Error().Err(err).Msg("Failed to merge audio files")
		return nil, fmt.Errorf("failed to merge audio files: %w", err)
	}

	// Cleanup normalized files if they are temporary
	if !options.KeepTemp {
		for _, file := range normalizedFiles {
			if file != mergedFile {
				_ = os.Remove(file)
			}
		}
	}

	result := &VoiceProfileResult{
		MergedFilePath: mergedFile,
		SourceFiles:    validFiles,
		TotalDuration:  totalDuration,
		ProfileCount:   len(validFiles),
	}

	log.Info().
		Str("merged_file", filepath.Base(mergedFile)).
		Int("profile_count", result.ProfileCount).
		Dur("total_duration", result.TotalDuration).
		Msg("Voice profile processing completed successfully")

	return result, nil
}

// scanAudioFiles scans a directory for supported audio files
func (vp *VoiceProfileProcessor) scanAudioFiles(dir string) ([]string, error) {
	log := logger.WithComponent("voice-profile-scanner").WithField("dir", dir)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Error().Str("directory", dir).Msg("Directory does not exist")
		return nil, fmt.Errorf("directory does not exist: %s", dir)
	}

	var audioFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warn().Str("path", path).Err(err).Msg("Error accessing path")
			return nil // Continue walking, don't fail the entire operation
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is supported
		if vp.processor.IsSupported(path) {
			log.Debug().Str("file", filepath.Base(path)).Msg("Found supported audio file")
			audioFiles = append(audioFiles, path)
		} else {
			log.Debug().Str("file", filepath.Base(path)).Msg("Skipping unsupported file")
		}

		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Error walking directory")
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	// Sort files alphabetically for consistent processing order
	sort.Strings(audioFiles)

	log.Info().Int("file_count", len(audioFiles)).Msg("Audio file scan completed")
	return audioFiles, nil
}

// normalizeAudioFiles converts all files to the same format and sample rate
func (vp *VoiceProfileProcessor) normalizeAudioFiles(files []string, options VoiceProfileOptions) ([]string, time.Duration, error) {
	log := logger.WithComponent("voice-profile-normalizer")

	normalizedFiles := make([]string, 0, len(files))
	var totalDuration time.Duration

	for i, file := range files {
		log.Debug().Str("file", filepath.Base(file)).Int("index", i+1).Int("total", len(files)).Msg("Processing file")

		// Get audio info
		audioInfo, err := vp.processor.GetAudioInfo(file)
		if err != nil {
			log.Error().Str("file", filepath.Base(file)).Err(err).Msg("Failed to get audio info")
			return nil, 0, fmt.Errorf("failed to get audio info for %s: %w", file, err)
		}

		totalDuration += audioInfo.Duration

		// Check if conversion is needed
		if audioInfo.Format == options.OutputFormat && !audioInfo.IsVideo {
			// File is already in the target format, use as-is
			log.Debug().Str("file", filepath.Base(file)).Msg("File already in target format, using as-is")
			normalizedFiles = append(normalizedFiles, file)
			continue
		}

		// Convert file to target format
		ext := vp.getFileExtension(options.OutputFormat)
		normalizedPath := filepath.Join(options.TempDir, fmt.Sprintf("normalized_%d%s", i, ext))

		log.Debug().
			Str("input", filepath.Base(file)).
			Str("output", filepath.Base(normalizedPath)).
			Str("format", string(options.OutputFormat)).
			Msg("Converting file to target format")

		if err := vp.processor.ConvertToAudio(file, normalizedPath, options.OutputFormat); err != nil {
			log.Error().Str("file", filepath.Base(file)).Err(err).Msg("Failed to convert audio file")
			return nil, 0, fmt.Errorf("failed to convert %s to %s: %w", file, options.OutputFormat, err)
		}

		normalizedFiles = append(normalizedFiles, normalizedPath)
	}

	log.Info().
		Int("file_count", len(normalizedFiles)).
		Dur("total_duration", totalDuration).
		Msg("Audio file normalization completed")

	return normalizedFiles, totalDuration, nil
}

// mergeAudioFiles merges multiple audio files into a single file
func (vp *VoiceProfileProcessor) mergeAudioFiles(files []string, options VoiceProfileOptions) (string, error) {
	log := logger.WithComponent("voice-profile-merger")

	if len(files) == 0 {
		return "", fmt.Errorf("no files to merge")
	}

	if len(files) == 1 {
		// Only one file, no merging needed
		log.Info().Str("file", filepath.Base(files[0])).Msg("Only one file, no merging needed")
		return files[0], nil
	}

	// Create output file path
	ext := vp.getFileExtension(options.OutputFormat)
	timestamp := time.Now().Unix()
	outputPath := filepath.Join(options.TempDir, fmt.Sprintf("voice_profiles_%d%s", timestamp, ext))

	log.Info().
		Int("file_count", len(files)).
		Str("output", filepath.Base(outputPath)).
		Dur("padding", options.SilencePadding).
		Msg("Starting audio file merge")

	// Use ffmpeg to concatenate files with padding
	if err := vp.concatenateWithPadding(files, outputPath, options); err != nil {
		log.Error().Err(err).Msg("Failed to concatenate audio files")
		return "", fmt.Errorf("failed to concatenate audio files: %w", err)
	}

	// Verify output file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		log.Error().Str("output_path", outputPath).Msg("Output file was not created")
		return "", fmt.Errorf("merged file was not created: %s", outputPath)
	}

	log.Info().Str("merged_file", filepath.Base(outputPath)).Msg("Audio file merge completed successfully")
	return outputPath, nil
}

// concatenateWithPadding concatenates audio files with silence padding between them
func (vp *VoiceProfileProcessor) concatenateWithPadding(files []string, outputPath string, options VoiceProfileOptions) error {
	log := logger.WithComponent("audio-concatenator")

	// For simple concatenation without complex padding, use a simpler approach
	if options.SilencePadding == 0 {
		return vp.simpleConcatenate(files, outputPath, options)
	}

	// Create a temporary concat list file
	concatListPath := filepath.Join(options.TempDir, fmt.Sprintf("concat_list_%d.txt", time.Now().Unix()))
	defer func() { _ = os.Remove(concatListPath) }()

	// Create concat list with silence padding
	if err := vp.createConcatList(files, concatListPath, options); err != nil {
		return fmt.Errorf("failed to create concat list: %w", err)
	}

	log.Debug().
		Str("concat_list", concatListPath).
		Int("input_count", len(files)).
		Msg("Building ffmpeg concat command with list")

	// Use ffmpeg concat demuxer for reliable concatenation
	stream := ffmpeg.Input(concatListPath, ffmpeg.KwArgs{
		"f":    "concat",
		"safe": "0",
	}).Output(outputPath, ffmpeg.KwArgs{
		"acodec": vp.getCodec(options.OutputFormat),
		"ar":     "44100", // Standard sample rate
		"ac":     "2",     // Stereo
	})

	log.Info().Msg("Executing ffmpeg concatenation with list")
	err := stream.OverWriteOutput().ErrorToStdOut().Run()
	if err != nil {
		log.Error().Err(err).Msg("FFmpeg concatenation failed")
		return fmt.Errorf("ffmpeg concatenation failed: %w", err)
	}

	return nil
}

// simpleConcatenate performs simple concatenation without padding
func (vp *VoiceProfileProcessor) simpleConcatenate(files []string, outputPath string, options VoiceProfileOptions) error {
	log := logger.WithComponent("audio-concatenator")

	// Create a temporary concat list file
	concatListPath := filepath.Join(options.TempDir, fmt.Sprintf("simple_concat_%d.txt", time.Now().Unix()))
	defer func() { _ = os.Remove(concatListPath) }()

	// Write file list
	var listContent strings.Builder
	for _, file := range files {
		// Escape the file path for ffmpeg
		escapedPath := strings.ReplaceAll(file, "'", "'\\''")
		listContent.WriteString(fmt.Sprintf("file '%s'\n", escapedPath))
	}

	if err := os.WriteFile(concatListPath, []byte(listContent.String()), 0644); err != nil {
		return fmt.Errorf("failed to write concat list: %w", err)
	}

	log.Debug().
		Str("concat_list", concatListPath).
		Int("input_count", len(files)).
		Msg("Building simple ffmpeg concat command")

	// Use ffmpeg concat demuxer
	stream := ffmpeg.Input(concatListPath, ffmpeg.KwArgs{
		"f":    "concat",
		"safe": "0",
	}).Output(outputPath, ffmpeg.KwArgs{
		"c": "copy", // Copy streams without re-encoding for speed
	})

	log.Info().Msg("Executing simple ffmpeg concatenation")
	err := stream.OverWriteOutput().ErrorToStdOut().Run()
	if err != nil {
		log.Error().Err(err).Msg("Simple ffmpeg concatenation failed")
		return fmt.Errorf("simple ffmpeg concatenation failed: %w", err)
	}

	return nil
}

// createConcatList creates a concat list file with silence padding
func (vp *VoiceProfileProcessor) createConcatList(files []string, listPath string, options VoiceProfileOptions) error {
	var listContent strings.Builder

	for i, file := range files {
		// Escape the file path for ffmpeg
		escapedPath := strings.ReplaceAll(file, "'", "'\\''")
		listContent.WriteString(fmt.Sprintf("file '%s'\n", escapedPath))

		// Add silence padding between files (except after the last file)
		if i < len(files)-1 && options.SilencePadding > 0 {
			// Create a temporary silence file
			silenceFile, err := vp.createSilenceFile(options.SilencePadding, options)
			if err != nil {
				return fmt.Errorf("failed to create silence file: %w", err)
			}

			escapedSilencePath := strings.ReplaceAll(silenceFile, "'", "'\\''")
			listContent.WriteString(fmt.Sprintf("file '%s'\n", escapedSilencePath))
		}
	}

	return os.WriteFile(listPath, []byte(listContent.String()), 0644)
}

// createSilenceFile creates a temporary silence audio file
func (vp *VoiceProfileProcessor) createSilenceFile(duration time.Duration, options VoiceProfileOptions) (string, error) {
	ext := vp.getFileExtension(options.OutputFormat)
	silenceFile := filepath.Join(options.TempDir, fmt.Sprintf("silence_%d%s", time.Now().UnixNano(), ext))

	// Generate silence using ffmpeg
	stream := ffmpeg.Input("anullsrc", ffmpeg.KwArgs{
		"f": "lavfi",
		"t": fmt.Sprintf("%.2f", duration.Seconds()),
		"r": "44100",
	}).Output(silenceFile, ffmpeg.KwArgs{
		"acodec": vp.getCodec(options.OutputFormat),
		"ar":     "44100",
		"ac":     "2",
	})

	err := stream.OverWriteOutput().ErrorToStdOut().Run()
	if err != nil {
		return "", fmt.Errorf("failed to generate silence: %w", err)
	}

	return silenceFile, nil
}

// getFileExtension returns the file extension for the given audio format
func (vp *VoiceProfileProcessor) getFileExtension(format AudioFormat) string {
	switch format {
	case FormatWAV:
		return ".wav"
	case FormatMP3:
		return ".mp3"
	case FormatM4A:
		return ".m4a"
	case FormatFLAC:
		return ".flac"
	default:
		return ".mp3"
	}
}

// getCodec returns the codec name for the given audio format
func (vp *VoiceProfileProcessor) getCodec(format AudioFormat) string {
	switch format {
	case FormatWAV:
		return "pcm_s16le"
	case FormatMP3:
		return "libmp3lame"
	case FormatM4A:
		return "aac"
	case FormatFLAC:
		return "flac"
	default:
		return "libmp3lame"
	}
}

// Cleanup removes temporary files created during voice profile processing
func (vp *VoiceProfileProcessor) Cleanup(result *VoiceProfileResult) error {
	if result == nil || result.MergedFilePath == "" {
		return nil
	}

	log := logger.WithComponent("voice-profile-cleanup")

	log.Debug().Str("file", filepath.Base(result.MergedFilePath)).Msg("Cleaning up voice profile file")

	if err := os.Remove(result.MergedFilePath); err != nil && !os.IsNotExist(err) {
		log.Warn().Str("file", result.MergedFilePath).Err(err).Msg("Failed to cleanup voice profile file")
		return fmt.Errorf("failed to cleanup voice profile file: %w", err)
	}

	log.Debug().Msg("Voice profile cleanup completed")
	return nil
}
