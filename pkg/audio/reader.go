package audio

import (
	"fmt"
	"io"
	"os"
	"time"
)

// ReaderImpl implements the Reader interface
type ReaderImpl struct {
	chunker *ChunkerImpl
}

// NewReader creates a new audio reader
func NewReader(tempDir string) *ReaderImpl {
	return &ReaderImpl{
		chunker: NewChunker(tempDir),
	}
}

// OpenAudio opens an audio file for reading
func (r *ReaderImpl) OpenAudio(filePath string) (io.ReadCloser, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	return file, nil
}

// ReadChunk reads a specific chunk of audio data
func (r *ReaderImpl) ReadChunk(filePath string, start, duration time.Duration) (io.ReadCloser, error) {
	// Create a temporary chunk file
	tempFile, err := os.CreateTemp(r.chunker.tempDir, "chunk_*.mp3")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Extract the chunk
	if err := r.chunker.CreateChunk(filePath, start, duration, tempPath); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("failed to create chunk: %w", err)
	}

	// Open the chunk file for reading
	chunkFile, err := os.Open(tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("failed to open chunk file: %w", err)
	}

	// Return a wrapper that cleans up the temp file when closed
	return &tempFileReader{
		ReadCloser: chunkFile,
		tempPath:   tempPath,
	}, nil
}

// GetMimeType returns the MIME type for the audio format
func (r *ReaderImpl) GetMimeType(format AudioFormat) string {
	return GetMimeType(format)
}

// tempFileReader wraps a ReadCloser and cleans up a temporary file when closed
type tempFileReader struct {
	io.ReadCloser
	tempPath string
}

// Close implements io.Closer, cleaning up the temporary file
func (t *tempFileReader) Close() error {
	// Close the underlying reader first
	if err := t.ReadCloser.Close(); err != nil {
		return err
	}

	// Remove the temporary file
	if err := os.Remove(t.tempPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove temp file: %w", err)
	}

	return nil
}
