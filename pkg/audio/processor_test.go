package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewProcessor(t *testing.T) {
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
			processor := NewProcessor(tt.tempDir)
			if processor.tempDir != tt.want {
				t.Errorf("NewProcessor() tempDir = %v, want %v", processor.tempDir, tt.want)
			}
		})
	}
}

func TestIsSupported(t *testing.T) {
	processor := NewProcessor("")

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "wav file",
			filePath: "test.wav",
			want:     true,
		},
		{
			name:     "mp3 file",
			filePath: "test.mp3",
			want:     true,
		},
		{
			name:     "m4a file",
			filePath: "test.m4a",
			want:     true,
		},
		{
			name:     "flac file",
			filePath: "test.flac",
			want:     true,
		},
		{
			name:     "mp4 file",
			filePath: "test.mp4",
			want:     true,
		},
		{
			name:     "avi file",
			filePath: "test.avi",
			want:     true,
		},
		{
			name:     "mov file",
			filePath: "test.mov",
			want:     true,
		},
		{
			name:     "mkv file",
			filePath: "test.mkv",
			want:     true,
		},
		{
			name:     "uppercase extension",
			filePath: "test.MP3",
			want:     true,
		},
		{
			name:     "unsupported extension",
			filePath: "test.txt",
			want:     false,
		},
		{
			name:     "no extension",
			filePath: "test",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.IsSupported(tt.filePath)
			if result != tt.want {
				t.Errorf("IsSupported() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     AudioFormat
	}{
		{
			name:     "wav file",
			filePath: "test.wav",
			want:     FormatWAV,
		},
		{
			name:     "mp3 file",
			filePath: "test.mp3",
			want:     FormatMP3,
		},
		{
			name:     "m4a file",
			filePath: "test.m4a",
			want:     FormatM4A,
		},
		{
			name:     "flac file",
			filePath: "test.flac",
			want:     FormatFLAC,
		},
		{
			name:     "mp4 file",
			filePath: "test.mp4",
			want:     FormatMP4,
		},
		{
			name:     "uppercase extension",
			filePath: "test.WAV",
			want:     FormatWAV,
		},
		{
			name:     "unsupported extension",
			filePath: "test.txt",
			want:     "",
		},
		{
			name:     "no extension",
			filePath: "test",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFormat(tt.filePath)
			if result != tt.want {
				t.Errorf("DetectFormat() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestGetMimeType(t *testing.T) {
	tests := []struct {
		name   string
		format AudioFormat
		want   string
	}{
		{
			name:   "wav format",
			format: FormatWAV,
			want:   "audio/wav",
		},
		{
			name:   "mp3 format",
			format: FormatMP3,
			want:   "audio/mpeg",
		},
		{
			name:   "m4a format",
			format: FormatM4A,
			want:   "audio/m4a",
		},
		{
			name:   "flac format",
			format: FormatFLAC,
			want:   "audio/flac",
		},
		{
			name:   "mp4 format",
			format: FormatMP4,
			want:   "video/mp4",
		},
		{
			name:   "unknown format",
			format: "unknown",
			want:   "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMimeType(tt.format)
			if result != tt.want {
				t.Errorf("GetMimeType() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestValidateFile(t *testing.T) {
	processor := NewProcessor("")

	// Create temporary directory for test
	testDir, err := os.MkdirTemp("", "processor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	tests := []struct {
		name      string
		filePath  string
		setupFunc func() error
		wantError bool
	}{
		{
			name:     "file does not exist",
			filePath: filepath.Join(testDir, "nonexistent.wav"),
			setupFunc: func() error {
				return nil
			},
			wantError: true,
		},
		{
			name:     "unsupported file format",
			filePath: filepath.Join(testDir, "test.txt"),
			setupFunc: func() error {
				file, err := os.Create(filepath.Join(testDir, "test.txt"))
				if err != nil {
					return err
				}
				return file.Close()
			},
			wantError: true,
		},
		{
			name:     "supported file format exists",
			filePath: filepath.Join(testDir, "test.wav"),
			setupFunc: func() error {
				file, err := os.Create(filepath.Join(testDir, "test.wav"))
				if err != nil {
					return err
				}
				return file.Close()
			},
			wantError: true, // Will fail at ffprobe step since it's not a real audio file
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setupFunc(); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			err := processor.ValidateFile(tt.filePath)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateFile() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	processor := NewProcessor("")

	// Create temporary directory for test
	testDir, err := os.MkdirTemp("", "processor_fileexists_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Create a test file
	testFile := filepath.Join(testDir, "test.wav")
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "file exists",
			filePath: testFile,
			want:     true,
		},
		{
			name:     "file does not exist",
			filePath: filepath.Join(testDir, "nonexistent.wav"),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.fileExists(tt.filePath)
			if result != tt.want {
				t.Errorf("fileExists() = %v, want %v", result, tt.want)
			}
		})
	}
}

// Integration test with real audio file (requires testdata)
func TestGetAudioInfoIntegration(t *testing.T) {
	// Skip if no testdata available
	testFile := "../../testdata/audio.wav"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Skipping integration test: testdata/audio.wav not found")
	}

	processor := NewProcessor("")

	audioInfo, err := processor.GetAudioInfo(testFile)
	if err != nil {
		t.Fatalf("GetAudioInfo() failed: %v", err)
	}

	if audioInfo == nil {
		t.Fatal("GetAudioInfo() returned nil")
	}

	if audioInfo.FilePath != testFile {
		t.Errorf("FilePath = %v, want %v", audioInfo.FilePath, testFile)
	}

	if audioInfo.Duration <= 0 {
		t.Errorf("Duration should be positive, got %v", audioInfo.Duration)
	}

	if audioInfo.Format != FormatWAV {
		t.Errorf("Format = %v, want %v", audioInfo.Format, FormatWAV)
	}

	if audioInfo.MimeType != "audio/wav" {
		t.Errorf("MimeType = %v, want %v", audioInfo.MimeType, "audio/wav")
	}
}

func TestConvertToAudioValidation(t *testing.T) {
	processor := NewProcessor("")

	// Create temporary directory for test
	testDir, err := os.MkdirTemp("", "processor_convert_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	tests := []struct {
		name       string
		inputPath  string
		outputPath string
		format     AudioFormat
		setupFunc  func() error
		wantError  bool
	}{
		{
			name:       "input file does not exist",
			inputPath:  filepath.Join(testDir, "nonexistent.mp4"),
			outputPath: filepath.Join(testDir, "output.mp3"),
			format:     FormatMP3,
			setupFunc:  func() error { return nil },
			wantError:  true,
		},
		{
			name:       "unsupported output format",
			inputPath:  filepath.Join(testDir, "input.mp4"),
			outputPath: filepath.Join(testDir, "output.unknown"),
			format:     "unknown",
			setupFunc: func() error {
				file, err := os.Create(filepath.Join(testDir, "input.mp4"))
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

			err := processor.ConvertToAudio(tt.inputPath, tt.outputPath, tt.format)
			if (err != nil) != tt.wantError {
				t.Errorf("ConvertToAudio() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// Benchmark tests
func BenchmarkIsSupported(b *testing.B) {
	processor := NewProcessor("")
	filePath := "test.mp3"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.IsSupported(filePath)
	}
}

func BenchmarkDetectFormat(b *testing.B) {
	filePath := "test.mp3"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectFormat(filePath)
	}
}

func BenchmarkGetMimeType(b *testing.B) {
	format := FormatMP3

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetMimeType(format)
	}
}
