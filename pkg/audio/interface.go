package audio

import (
	"io"
	"time"
)

// AudioFormat represents supported audio formats
type AudioFormat string

const (
	FormatWAV  AudioFormat = "wav"
	FormatMP3  AudioFormat = "mp3"
	FormatM4A  AudioFormat = "m4a"
	FormatFLAC AudioFormat = "flac"
	FormatMP4  AudioFormat = "mp4"
)

// AudioInfo contains metadata about an audio file
type AudioInfo struct {
	FilePath   string
	Format     AudioFormat
	MimeType   string
	Duration   time.Duration
	SampleRate int
	Channels   int
	BitRate    int
	Size       int64
	IsVideo    bool
}

// ChunkInfo represents information about an audio chunk
type ChunkInfo struct {
	Index        int
	Start        time.Duration
	End          time.Duration
	Duration     time.Duration
	FilePath     string
	TempFilePath string
}

// ProcessorOptions provides configuration for audio processing
type ProcessorOptions struct {
	ChunkDuration   time.Duration // Default: 30 minutes
	OverlapDuration time.Duration // Default: 1 minute
	OutputFormat    AudioFormat   // Target format for conversion
	SampleRate      int           // Target sample rate
	Quality         int           // Compression quality (1-9)
	TempDir         string        // Temporary directory for processing
	KeepTemp        bool          // Keep temporary files after processing
}

// Processor handles audio file processing and conversion
type Processor interface {
	// GetAudioInfo extracts metadata from an audio/video file
	GetAudioInfo(filePath string) (*AudioInfo, error)

	// ConvertToAudio converts video files (MP4) to audio format
	ConvertToAudio(inputPath, outputPath string, format AudioFormat) error

	// IsSupported checks if the file format is supported
	IsSupported(filePath string) bool

	// ValidateFile validates the audio file
	ValidateFile(filePath string) error
}

// Chunker handles splitting audio files into overlapping chunks
type Chunker interface {
	// ChunkAudio splits an audio file into overlapping chunks
	ChunkAudio(inputPath string, options ProcessorOptions) ([]*ChunkInfo, error)

	// CreateChunk creates a single chunk from the audio file
	CreateChunk(inputPath string, start, duration time.Duration, outputPath string) error

	// CleanupChunks removes temporary chunk files
	CleanupChunks(chunks []*ChunkInfo) error

	// CalculateChunks determines chunk boundaries with overlap
	CalculateChunks(duration time.Duration, chunkDuration, overlapDuration time.Duration) []*ChunkInfo
}

// Reader provides streaming access to audio data
type Reader interface {
	// OpenAudio opens an audio file for reading
	OpenAudio(filePath string) (io.ReadCloser, error)

	// ReadChunk reads a specific chunk of audio data
	ReadChunk(filePath string, start, duration time.Duration) (io.ReadCloser, error)

	// GetMimeType returns the MIME type for the audio format
	GetMimeType(format AudioFormat) string
}
