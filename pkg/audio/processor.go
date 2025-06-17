package audio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"
	"github.com/eternnoir/gollmscribe/pkg/logger"
)

// ProcessorImpl implements the Processor interface
type ProcessorImpl struct {
	tempDir string
}

// NewProcessor creates a new audio processor
func NewProcessor(tempDir string) *ProcessorImpl {
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	return &ProcessorImpl{
		tempDir: tempDir,
	}
}

// GetAudioInfo extracts metadata from an audio/video file
func (p *ProcessorImpl) GetAudioInfo(filePath string) (*AudioInfo, error) {
	log := logger.WithComponent("audio-processor").WithField("file", filepath.Base(filePath))
	
	log.Debug().Str("full_path", filePath).Msg("Getting audio information")
	
	if !p.fileExists(filePath) {
		log.Error().Str("path", filePath).Msg("File does not exist")
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	// Use ffprobe to get file information
	log.Debug().Msg("Probing file with ffprobe")
	info, err := ffmpeg.Probe(filePath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to probe file")
		return nil, fmt.Errorf("failed to probe file: %w", err)
	}

	audioInfo := &AudioInfo{
		FilePath: filePath,
	}

	// Parse ffprobe output
	log.Debug().Msg("Parsing probe information")
	if err := p.parseProbeInfo(info, audioInfo); err != nil {
		log.Error().Err(err).Msg("Failed to parse probe info")
		return nil, fmt.Errorf("failed to parse probe info: %w", err)
	}

	log.Info().
		Dur("duration", audioInfo.Duration).
		Bool("is_video", audioInfo.IsVideo).
		Str("format", string(audioInfo.Format)).
		Int("sample_rate", audioInfo.SampleRate).
		Int("channels", audioInfo.Channels).
		Msg("Audio information extracted successfully")

	return audioInfo, nil
}

// ConvertToAudio converts video files (MP4) to audio format
func (p *ProcessorImpl) ConvertToAudio(inputPath, outputPath string, format AudioFormat) error {
	log := logger.WithComponent("audio-converter").
		WithField("input", filepath.Base(inputPath)).
		WithField("output", filepath.Base(outputPath))
	
	log.Info().
		Str("input_path", inputPath).
		Str("output_path", outputPath).
		Str("format", string(format)).
		Msg("Starting audio conversion")
	
	if !p.fileExists(inputPath) {
		log.Error().Str("path", inputPath).Msg("Input file does not exist")
		return fmt.Errorf("input file does not exist: %s", inputPath)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	log.Debug().Str("output_dir", outputDir).Msg("Creating output directory")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Error().Err(err).Str("output_dir", outputDir).Msg("Failed to create output directory")
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build ffmpeg command based on output format
	log.Debug().Str("format", string(format)).Msg("Building ffmpeg command")
	stream := ffmpeg.Input(inputPath)

	switch format {
	case FormatMP3:
		log.Debug().Msg("Configuring MP3 output parameters")
		stream = stream.Output(outputPath, ffmpeg.KwArgs{
			"acodec": "libmp3lame",
			"ab":     "192k",
			"ar":     "44100",
			"ac":     "2",
		})
	case FormatWAV:
		log.Debug().Msg("Configuring WAV output parameters")
		stream = stream.Output(outputPath, ffmpeg.KwArgs{
			"acodec": "pcm_s16le",
			"ar":     "44100",
			"ac":     "2",
		})
	case FormatFLAC:
		log.Debug().Msg("Configuring FLAC output parameters")
		stream = stream.Output(outputPath, ffmpeg.KwArgs{
			"acodec": "flac",
			"ar":     "44100",
			"ac":     "2",
		})
	default:
		log.Error().Str("format", string(format)).Msg("Unsupported output format")
		return fmt.Errorf("unsupported output format: %s", format)
	}

	// Execute the conversion
	log.Info().Msg("Executing ffmpeg conversion")
	startTime := time.Now()
	err := stream.OverWriteOutput().ErrorToStdOut().Run()
	duration := time.Since(startTime)
	
	if err != nil {
		log.Error().Err(err).Dur("duration", duration).Msg("FFmpeg conversion failed")
		return fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	// Check if output file was created successfully
	if !p.fileExists(outputPath) {
		log.Error().Str("output_path", outputPath).Msg("Output file was not created")
		return fmt.Errorf("output file was not created: %s", outputPath)
	}

	// Get output file size for logging
	if stat, err := os.Stat(outputPath); err == nil {
		log.Info().
			Dur("duration", duration).
			Int64("output_size_bytes", stat.Size()).
			Msg("Audio conversion completed successfully")
	} else {
		log.Info().Dur("duration", duration).Msg("Audio conversion completed successfully")
	}

	return nil
}

// IsSupported checks if the file format is supported
func (p *ProcessorImpl) IsSupported(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	supportedExts := []string{".wav", ".mp3", ".m4a", ".flac", ".mp4", ".avi", ".mov", ".mkv"}

	for _, supportedExt := range supportedExts {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// ValidateFile validates the audio file
func (p *ProcessorImpl) ValidateFile(filePath string) error {
	if !p.fileExists(filePath) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	if !p.IsSupported(filePath) {
		return fmt.Errorf("unsupported file format: %s", filepath.Ext(filePath))
	}

	// Try to probe the file to ensure it's valid
	_, err := ffmpeg.Probe(filePath)
	if err != nil {
		return fmt.Errorf("invalid or corrupted file: %w", err)
	}

	return nil
}

// fileExists checks if a file exists
func (p *ProcessorImpl) fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// parseProbeInfo parses ffprobe output and fills AudioInfo
func (p *ProcessorImpl) parseProbeInfo(probeData string, info *AudioInfo) error {
	// Parse JSON output from ffprobe
	var probe struct {
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
			Size     string `json:"size"`
		} `json:"format"`
		Streams []struct {
			CodecType  string `json:"codec_type"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
		} `json:"streams"`
	}

	if err := json.Unmarshal([]byte(probeData), &probe); err != nil {
		return fmt.Errorf("failed to parse probe JSON: %w", err)
	}

	// Parse duration
	if probe.Format.Duration != "" {
		durationFloat, err := strconv.ParseFloat(probe.Format.Duration, 64)
		if err == nil {
			info.Duration = time.Duration(durationFloat * float64(time.Second))
		}
	}

	// Parse bit rate
	if probe.Format.BitRate != "" {
		bitRate, err := strconv.ParseInt(probe.Format.BitRate, 10, 64)
		if err == nil {
			info.BitRate = int(bitRate)
		}
	}

	// Parse size
	if probe.Format.Size != "" {
		size, err := strconv.ParseInt(probe.Format.Size, 10, 64)
		if err == nil {
			info.Size = size
		}
	}

	// Parse audio stream info
	for _, stream := range probe.Streams {
		if stream.CodecType == "audio" {
			if stream.SampleRate != "" {
				sampleRate, err := strconv.Atoi(stream.SampleRate)
				if err == nil {
					info.SampleRate = sampleRate
				}
			}
			info.Channels = stream.Channels
			break
		}
	}

	// Determine if it's a video file
	ext := strings.ToLower(filepath.Ext(info.FilePath))
	videoExts := []string{".mp4", ".avi", ".mov", ".mkv", ".webm"}
	for _, videoExt := range videoExts {
		if ext == videoExt {
			info.IsVideo = true
			break
		}
	}

	// Set format based on extension
	switch ext {
	case ".wav":
		info.Format = FormatWAV
		info.MimeType = "audio/wav"
	case ".mp3":
		info.Format = FormatMP3
		info.MimeType = "audio/mpeg"
	case ".m4a":
		info.Format = FormatM4A
		info.MimeType = "audio/m4a"
	case ".flac":
		info.Format = FormatFLAC
		info.MimeType = "audio/flac"
	case ".mp4":
		info.Format = FormatMP4
		info.MimeType = "video/mp4"
	default:
		info.MimeType = "application/octet-stream"
	}

	return nil
}

// GetMimeType returns the MIME type for the audio format
func GetMimeType(format AudioFormat) string {
	switch format {
	case FormatWAV:
		return "audio/wav"
	case FormatMP3:
		return "audio/mpeg"
	case FormatM4A:
		return "audio/m4a"
	case FormatFLAC:
		return "audio/flac"
	case FormatMP4:
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

// DetectFormat detects audio format from file extension
func DetectFormat(filePath string) AudioFormat {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".wav":
		return FormatWAV
	case ".mp3":
		return FormatMP3
	case ".m4a":
		return FormatM4A
	case ".flac":
		return FormatFLAC
	case ".mp4":
		return FormatMP4
	default:
		return ""
	}
}
