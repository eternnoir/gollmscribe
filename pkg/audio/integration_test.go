package audio

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestChunkingWorkflow tests the complete chunking workflow
func TestChunkingWorkflow(t *testing.T) {
	// Skip if no testdata available
	testFile := "../../testdata/audio.wav"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Skipping integration test: testdata/audio.wav not found")
	}

	// Create temporary directory for test
	testDir, err := os.MkdirTemp("", "chunking_workflow_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Initialize components
	processor := NewProcessor(testDir)
	chunker := NewChunker(testDir)

	// Test 1: Get audio info
	t.Run("get_audio_info", func(t *testing.T) {
		audioInfo, err := processor.GetAudioInfo(testFile)
		if err != nil {
			t.Fatalf("GetAudioInfo() failed: %v", err)
		}

		if audioInfo.Duration <= 0 {
			t.Errorf("Duration should be positive, got %v", audioInfo.Duration)
		}

		t.Logf("Audio info: Duration=%v, Format=%v, SampleRate=%d, Channels=%d",
			audioInfo.Duration, audioInfo.Format, audioInfo.SampleRate, audioInfo.Channels)
	})

	// Test 2: Create chunks with different settings
	testCases := []struct {
		name            string
		chunkDuration   time.Duration
		overlapDuration time.Duration
		expectedChunks  int
	}{
		{
			name:            "30_second_chunks_5_second_overlap",
			chunkDuration:   30 * time.Second,
			overlapDuration: 5 * time.Second,
			expectedChunks:  1, // Will depend on actual audio duration
		},
		{
			name:            "10_second_chunks_2_second_overlap",
			chunkDuration:   10 * time.Second,
			overlapDuration: 2 * time.Second,
			expectedChunks:  1, // Will depend on actual audio duration
		},
	}

	for _, tc := range testCases {
		t.Run("chunk_"+tc.name, func(t *testing.T) {
			options := ProcessorOptions{
				ChunkDuration:   tc.chunkDuration,
				OverlapDuration: tc.overlapDuration,
				OutputFormat:    FormatMP3,
				TempDir:         testDir,
				KeepTemp:        false,
			}

			chunks, err := chunker.ChunkAudio(testFile, options)
			if err != nil {
				t.Fatalf("ChunkAudio() failed: %v", err)
			}

			if len(chunks) == 0 {
				t.Fatal("No chunks created")
			}

			t.Logf("Created %d chunks with %v duration and %v overlap",
				len(chunks), tc.chunkDuration, tc.overlapDuration)

			// Validate chunks
			if err := chunker.ValidateChunks(chunks); err != nil {
				t.Errorf("ValidateChunks() failed: %v", err)
			}

			// Test chunk properties
			for i, chunk := range chunks {
				t.Logf("Chunk %d: Start=%v, End=%v, Duration=%v, File=%s",
					i, chunk.Start, chunk.End, chunk.Duration, chunk.TempFilePath)

				// Verify file exists and has content
				stat, err := os.Stat(chunk.TempFilePath)
				if err != nil {
					t.Errorf("Chunk file doesn't exist: %v", err)
					continue
				}

				if stat.Size() == 0 {
					t.Errorf("Chunk file is empty: %s", chunk.TempFilePath)
				}

				// Verify duration
				if chunk.Duration <= 0 {
					t.Errorf("Chunk duration should be positive: %v", chunk.Duration)
				}

				// Verify start/end consistency
				if chunk.End <= chunk.Start {
					t.Errorf("Chunk end should be after start: Start=%v, End=%v", chunk.Start, chunk.End)
				}

				// Verify calculated duration matches
				expectedDuration := chunk.End - chunk.Start
				if chunk.Duration != expectedDuration {
					t.Errorf("Chunk duration mismatch: got %v, expected %v", chunk.Duration, expectedDuration)
				}
			}

			// Test overlap (if multiple chunks)
			if len(chunks) > 1 {
				for i := 1; i < len(chunks); i++ {
					prev := chunks[i-1]
					curr := chunks[i]

					// Check that chunks have proper overlap
					if curr.Start >= prev.End {
						t.Errorf("Chunks %d and %d should overlap: prev.End=%v, curr.Start=%v",
							i-1, i, prev.End, curr.Start)
					}

					// Verify overlap duration is approximately correct
					actualOverlap := prev.End - curr.Start
					if actualOverlap < 0 {
						actualOverlap = 0
					}

					expectedOverlap := tc.overlapDuration
					tolerance := 1 * time.Second // Allow 1 second tolerance

					if actualOverlap < expectedOverlap-tolerance || actualOverlap > expectedOverlap+tolerance {
						t.Logf("Warning: Overlap duration variance: expected=%v, actual=%v, tolerance=±%v",
							expectedOverlap, actualOverlap, tolerance)
					}
				}
			}

			// Clean up chunks
			if err := chunker.CleanupChunks(chunks); err != nil {
				t.Errorf("CleanupChunks() failed: %v", err)
			}

			// Verify chunks are cleaned up
			for _, chunk := range chunks {
				if _, err := os.Stat(chunk.TempFilePath); !os.IsNotExist(err) {
					t.Errorf("Chunk file should be removed: %s", chunk.TempFilePath)
				}
			}
		})
	}
}

// TestVideoToAudioConversion tests video file processing
func TestVideoToAudioConversion(t *testing.T) {
	// Skip if no testdata available
	testFile := "../../testdata/video.mp4"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Skipping integration test: testdata/video.mp4 not found")
	}

	// Create temporary directory for test
	testDir, err := os.MkdirTemp("", "video_conversion_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	processor := NewProcessor(testDir)

	// Test video info extraction
	t.Run("get_video_info", func(t *testing.T) {
		videoInfo, err := processor.GetAudioInfo(testFile)
		if err != nil {
			t.Fatalf("GetAudioInfo() on video failed: %v", err)
		}

		if !videoInfo.IsVideo {
			t.Error("IsVideo should be true for MP4 file")
		}

		if videoInfo.Format != FormatMP4 {
			t.Errorf("Format should be MP4, got %v", videoInfo.Format)
		}

		t.Logf("Video info: Duration=%v, Format=%v, IsVideo=%v",
			videoInfo.Duration, videoInfo.Format, videoInfo.IsVideo)
	})

	// Test video to audio conversion
	t.Run("convert_video_to_audio", func(t *testing.T) {
		outputPath := filepath.Join(testDir, "converted_audio.mp3")

		err := processor.ConvertToAudio(testFile, outputPath, FormatMP3)
		if err != nil {
			t.Fatalf("ConvertToAudio() failed: %v", err)
		}

		// Verify output file exists
		stat, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("Converted audio file doesn't exist: %v", err)
		}

		if stat.Size() == 0 {
			t.Error("Converted audio file is empty")
		}

		// Verify converted file is valid audio
		audioInfo, err := processor.GetAudioInfo(outputPath)
		if err != nil {
			t.Fatalf("GetAudioInfo() on converted audio failed: %v", err)
		}

		if audioInfo.Format != FormatMP3 {
			t.Errorf("Converted format should be MP3, got %v", audioInfo.Format)
		}

		if audioInfo.IsVideo {
			t.Error("Converted file should not be marked as video")
		}

		t.Logf("Converted audio info: Duration=%v, Format=%v, Size=%d bytes",
			audioInfo.Duration, audioInfo.Format, stat.Size())
	})
}

// TestChunkingPerformance tests chunking performance with different settings
func TestChunkingPerformance(t *testing.T) {
	// Skip if no testdata available
	testFile := "../../testdata/audio.wav"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Skipping performance test: testdata/audio.wav not found")
	}

	// Create temporary directory for test
	testDir, err := os.MkdirTemp("", "chunking_performance_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	chunker := NewChunker(testDir)

	testCases := []struct {
		name            string
		chunkDuration   time.Duration
		overlapDuration time.Duration
	}{
		{
			name:            "large_chunks",
			chunkDuration:   5 * time.Minute,
			overlapDuration: 30 * time.Second,
		},
		{
			name:            "small_chunks",
			chunkDuration:   30 * time.Second,
			overlapDuration: 5 * time.Second,
		},
		{
			name:            "tiny_chunks",
			chunkDuration:   10 * time.Second,
			overlapDuration: 2 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()

			options := ProcessorOptions{
				ChunkDuration:   tc.chunkDuration,
				OverlapDuration: tc.overlapDuration,
				OutputFormat:    FormatMP3,
				TempDir:         testDir,
				KeepTemp:        false,
			}

			chunks, err := chunker.ChunkAudio(testFile, options)
			if err != nil {
				t.Fatalf("ChunkAudio() failed: %v", err)
			}

			elapsed := time.Since(start)
			t.Logf("Created %d chunks in %v (chunk_duration=%v, overlap=%v)",
				len(chunks), elapsed, tc.chunkDuration, tc.overlapDuration)

			// Clean up
			chunker.CleanupChunks(chunks)
		})
	}
}

// TestErrorHandling tests various error conditions
func TestErrorHandling(t *testing.T) {
	// Create temporary directory for test
	testDir, err := os.MkdirTemp("", "error_handling_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	processor := NewProcessor(testDir)
	chunker := NewChunker(testDir)

	tests := []struct {
		name        string
		testFunc    func() error
		expectError bool
	}{
		{
			name: "nonexistent_file",
			testFunc: func() error {
				_, err := processor.GetAudioInfo("/nonexistent/file.wav")
				return err
			},
			expectError: true,
		},
		{
			name: "chunk_nonexistent_file",
			testFunc: func() error {
				options := ProcessorOptions{
					ChunkDuration:   30 * time.Second,
					OverlapDuration: 5 * time.Second,
				}
				_, err := chunker.ChunkAudio("/nonexistent/file.wav", options)
				return err
			},
			expectError: true,
		},
		{
			name: "invalid_chunk_parameters",
			testFunc: func() error {
				// Create a dummy file
				dummyFile := filepath.Join(testDir, "dummy.wav")
				file, err := os.Create(dummyFile)
				if err != nil {
					return err
				}
				file.Close()

				options := ProcessorOptions{
					ChunkDuration:   0, // Invalid duration
					OverlapDuration: 5 * time.Second,
				}
				_, err = chunker.ChunkAudio(dummyFile, options)
				return err
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.testFunc()
			if (err != nil) != tt.expectError {
				t.Errorf("Test %s: error = %v, expectError %v", tt.name, err, tt.expectError)
			}
		})
	}
}
