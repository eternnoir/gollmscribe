package audio

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewChunker(t *testing.T) {
	tests := []struct {
		name    string
		tempDir string
		want    string
	}{
		{
			name:    "default temp dir",
			tempDir: "",
			want:    os.TempDir(),
		},
		{
			name:    "custom temp dir",
			tempDir: "/custom/temp",
			want:    "/custom/temp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewChunker(tt.tempDir)
			if chunker.tempDir != tt.want {
				t.Errorf("NewChunker() tempDir = %v, want %v", chunker.tempDir, tt.want)
			}
		})
	}
}

func TestCalculateChunks(t *testing.T) {
	chunker := NewChunker("")

	tests := []struct {
		name            string
		duration        time.Duration
		chunkDuration   time.Duration
		overlapDuration time.Duration
		wantChunks      int
		wantFirstStart  time.Duration
		wantFirstEnd    time.Duration
	}{
		{
			name:            "single chunk - file shorter than chunk duration",
			duration:        10 * time.Minute,
			chunkDuration:   30 * time.Minute,
			overlapDuration: 1 * time.Minute,
			wantChunks:      1,
			wantFirstStart:  0,
			wantFirstEnd:    10 * time.Minute,
		},
		{
			name:            "multiple chunks - normal case",
			duration:        120 * time.Minute,
			chunkDuration:   30 * time.Minute,
			overlapDuration: 5 * time.Minute,
			wantChunks:      5,
			wantFirstStart:  0,
			wantFirstEnd:    30 * time.Minute,
		},
		{
			name:            "no overlap",
			duration:        60 * time.Minute,
			chunkDuration:   30 * time.Minute,
			overlapDuration: 0,
			wantChunks:      2,
			wantFirstStart:  0,
			wantFirstEnd:    30 * time.Minute,
		},
		{
			name:            "large overlap - should use half chunk as step",
			duration:        60 * time.Minute,
			chunkDuration:   30 * time.Minute,
			overlapDuration: 40 * time.Minute, // larger than chunk
			wantChunks:      3,
			wantFirstStart:  0,
			wantFirstEnd:    30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := chunker.CalculateChunks(tt.duration, tt.chunkDuration, tt.overlapDuration)

			if len(chunks) != tt.wantChunks {
				t.Errorf("CalculateChunks() chunks count = %v, want %v", len(chunks), tt.wantChunks)
			}

			if len(chunks) > 0 {
				if chunks[0].Start != tt.wantFirstStart {
					t.Errorf("First chunk start = %v, want %v", chunks[0].Start, tt.wantFirstStart)
				}
				if chunks[0].End != tt.wantFirstEnd {
					t.Errorf("First chunk end = %v, want %v", chunks[0].End, tt.wantFirstEnd)
				}

				// Verify chunk indices are sequential
				for i, chunk := range chunks {
					if chunk.Index != i {
						t.Errorf("Chunk %d has index %d, want %d", i, chunk.Index, i)
					}
				}

				// Verify last chunk ends at file duration
				lastChunk := chunks[len(chunks)-1]
				if lastChunk.End != tt.duration {
					t.Errorf("Last chunk end = %v, want %v", lastChunk.End, tt.duration)
				}
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "zero duration",
			duration: 0,
			want:     "00:00:00.000",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			want:     "00:02:30.000",
		},
		{
			name:     "hours, minutes, seconds",
			duration: 1*time.Hour + 23*time.Minute + 45*time.Second,
			want:     "01:23:45.000",
		},
		{
			name:     "with milliseconds",
			duration: 1*time.Minute + 30*time.Second + 500*time.Millisecond,
			want:     "00:01:30.500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.want {
				t.Errorf("formatDuration() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestChunkerCleanupChunks(t *testing.T) {
	chunker := NewChunker("")

	// Create temporary directory for test
	testDir, err := os.MkdirTemp("", "chunker_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Create test chunk files
	chunks := []*ChunkInfo{
		{
			Index:        0,
			TempFilePath: filepath.Join(testDir, "chunk_001.mp3"),
		},
		{
			Index:        1,
			TempFilePath: filepath.Join(testDir, "chunk_002.mp3"),
		},
	}

	// Create the files
	for _, chunk := range chunks {
		file, err := os.Create(chunk.TempFilePath)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		file.Close()
	}

	// Verify files exist
	for _, chunk := range chunks {
		if _, err := os.Stat(chunk.TempFilePath); os.IsNotExist(err) {
			t.Fatalf("Test file should exist: %s", chunk.TempFilePath)
		}
	}

	// Clean up chunks
	err = chunker.CleanupChunks(chunks)
	if err != nil {
		t.Errorf("CleanupChunks() returned error: %v", err)
	}

	// Verify files are removed
	for _, chunk := range chunks {
		if _, err := os.Stat(chunk.TempFilePath); !os.IsNotExist(err) {
			t.Errorf("File should be removed: %s", chunk.TempFilePath)
		}
	}
}

func TestChunkerValidateChunks(t *testing.T) {
	chunker := NewChunker("")

	// Create temporary directory for test
	testDir, err := os.MkdirTemp("", "chunker_validate_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	tests := []struct {
		name      string
		chunks    []*ChunkInfo
		setupFunc func() error
		wantError bool
	}{
		{
			name: "valid chunks",
			chunks: []*ChunkInfo{
				{
					Index:        0,
					TempFilePath: filepath.Join(testDir, "valid_chunk.mp3"),
				},
			},
			setupFunc: func() error {
				file, err := os.Create(filepath.Join(testDir, "valid_chunk.mp3"))
				if err != nil {
					return err
				}
				defer file.Close()
				_, err = file.WriteString("fake audio data")
				return err
			},
			wantError: false,
		},
		{
			name: "chunk with no temp file path",
			chunks: []*ChunkInfo{
				{
					Index:        0,
					TempFilePath: "",
				},
			},
			setupFunc: func() error { return nil },
			wantError: true,
		},
		{
			name: "chunk file does not exist",
			chunks: []*ChunkInfo{
				{
					Index:        0,
					TempFilePath: filepath.Join(testDir, "nonexistent.mp3"),
				},
			},
			setupFunc: func() error { return nil },
			wantError: true,
		},
		{
			name: "empty chunk file",
			chunks: []*ChunkInfo{
				{
					Index:        0,
					TempFilePath: filepath.Join(testDir, "empty_chunk.mp3"),
				},
			},
			setupFunc: func() error {
				file, err := os.Create(filepath.Join(testDir, "empty_chunk.mp3"))
				if err != nil {
					return err
				}
				return file.Close()
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setupFunc(); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			err := chunker.ValidateChunks(tt.chunks)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateChunks() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// Benchmark tests for performance
func BenchmarkCalculateChunks(b *testing.B) {
	chunker := NewChunker("")
	duration := 2 * time.Hour
	chunkDuration := 30 * time.Minute
	overlapDuration := 1 * time.Minute

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunker.CalculateChunks(duration, chunkDuration, overlapDuration)
	}
}

func BenchmarkFormatDuration(b *testing.B) {
	duration := 1*time.Hour + 23*time.Minute + 45*time.Second + 678*time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatDuration(duration)
	}
}
