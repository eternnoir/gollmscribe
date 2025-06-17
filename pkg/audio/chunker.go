package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

// ChunkerImpl implements the Chunker interface
type ChunkerImpl struct {
	tempDir string
}

// NewChunker creates a new audio chunker
func NewChunker(tempDir string) *ChunkerImpl {
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	return &ChunkerImpl{
		tempDir: tempDir,
	}
}

// ChunkAudio splits an audio file into overlapping chunks
func (c *ChunkerImpl) ChunkAudio(inputPath string, options ProcessorOptions) ([]*ChunkInfo, error) {
	// Get audio duration first
	processor := NewProcessor(c.tempDir)
	audioInfo, err := processor.GetAudioInfo(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio info: %w", err)
	}

	// Calculate chunk boundaries
	chunks := c.CalculateChunks(audioInfo.Duration, options.ChunkDuration, options.OverlapDuration)

	// Create temporary directory for chunks
	chunkDir := filepath.Join(c.tempDir, fmt.Sprintf("gollmscribe_chunks_%d", time.Now().Unix()))
	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create chunk directory: %w", err)
	}

	// Create each chunk
	for i, chunk := range chunks {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%03d.mp3", i))
		chunk.TempFilePath = chunkPath
		chunk.FilePath = inputPath

		if err := c.CreateChunk(inputPath, chunk.Start, chunk.Duration, chunkPath); err != nil {
			// Clean up on error
			_ = c.CleanupChunks(chunks[:i])
			return nil, fmt.Errorf("failed to create chunk %d: %w", i, err)
		}
	}

	return chunks, nil
}

// CreateChunk creates a single chunk from the audio file
func (c *ChunkerImpl) CreateChunk(inputPath string, start, duration time.Duration, outputPath string) error {
	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create ffmpeg command to extract the chunk
	stream := ffmpeg.Input(inputPath, ffmpeg.KwArgs{
		"ss": formatDuration(start),
		"t":  formatDuration(duration),
	}).Output(outputPath, ffmpeg.KwArgs{
		"acodec": "libmp3lame",
		"ab":     "192k",
		"ar":     "44100",
		"ac":     "2",
	})

	// Execute the command
	err := stream.OverWriteOutput().ErrorToStdOut().Run()
	if err != nil {
		return fmt.Errorf("ffmpeg chunk extraction failed: %w", err)
	}

	return nil
}

// CleanupChunks removes temporary chunk files
func (c *ChunkerImpl) CleanupChunks(chunks []*ChunkInfo) error {
	var lastErr error

	for _, chunk := range chunks {
		if chunk.TempFilePath != "" {
			if err := os.Remove(chunk.TempFilePath); err != nil && !os.IsNotExist(err) {
				lastErr = err
			}
		}
	}

	// Try to remove the chunk directory if it's empty
	if len(chunks) > 0 && chunks[0].TempFilePath != "" {
		chunkDir := filepath.Dir(chunks[0].TempFilePath)
		_ = os.Remove(chunkDir) // Ignore error if directory is not empty
	}

	return lastErr
}

// CalculateChunks determines chunk boundaries with overlap
func (c *ChunkerImpl) CalculateChunks(duration, chunkDuration, overlapDuration time.Duration) []*ChunkInfo {
	var chunks []*ChunkInfo

	if duration <= chunkDuration {
		// File is shorter than chunk duration, return single chunk
		chunks = append(chunks, &ChunkInfo{
			Index:    0,
			Start:    0,
			End:      duration,
			Duration: duration,
		})
		return chunks
	}

	// Calculate step size (chunk duration minus overlap)
	stepSize := chunkDuration - overlapDuration
	if stepSize <= 0 {
		// If overlap is too large, use half chunk duration as step
		stepSize = chunkDuration / 2
	}

	chunkIndex := 0
	for start := time.Duration(0); start < duration; start += stepSize {
		end := start + chunkDuration
		if end > duration {
			end = duration
		}

		chunk := &ChunkInfo{
			Index:    chunkIndex,
			Start:    start,
			End:      end,
			Duration: end - start,
		}

		chunks = append(chunks, chunk)
		chunkIndex++

		// Break if we've covered the entire file
		if end >= duration {
			break
		}
	}

	return chunks
}

// formatDuration formats a time.Duration for ffmpeg
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	milliseconds := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
}

// GetChunkDuration calculates the actual duration of a chunk file
func (c *ChunkerImpl) GetChunkDuration(chunkPath string) (time.Duration, error) {
	processor := NewProcessor(c.tempDir)
	info, err := processor.GetAudioInfo(chunkPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get chunk duration: %w", err)
	}
	return info.Duration, nil
}

// ValidateChunks validates that all chunks were created successfully
func (c *ChunkerImpl) ValidateChunks(chunks []*ChunkInfo) error {
	for i, chunk := range chunks {
		if chunk.TempFilePath == "" {
			return fmt.Errorf("chunk %d has no temp file path", i)
		}

		if _, err := os.Stat(chunk.TempFilePath); os.IsNotExist(err) {
			return fmt.Errorf("chunk %d temp file does not exist: %s", i, chunk.TempFilePath)
		}

		// Validate chunk file is not empty
		if stat, err := os.Stat(chunk.TempFilePath); err == nil {
			if stat.Size() == 0 {
				return fmt.Errorf("chunk %d temp file is empty: %s", i, chunk.TempFilePath)
			}
		}
	}

	return nil
}
