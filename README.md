# gollmscribe

<div align="center">

[![MIT License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/doc/install)
[![Go Report Card](https://goreportcard.com/badge/github.com/eternnoir/gollmscribe)](https://goreportcard.com/report/github.com/eternnoir/gollmscribe)
[![Documentation](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/eternnoir/gollmscribe)

A Go application that transforms audio files into precise text transcripts using advanced Large Language Models and multimodal AI processing capabilities.

[Features](#features) • [Installation](#installation) • [Usage](#usage) • [API](#api) • [Contributing](#contributing)

</div>

## 🎯 Features

- **Multi-format Support**: Process audio (WAV, MP3, M4A, FLAC) and video (MP4, AVI, MOV, MKV) files
- **Smart Chunking**: Automatically splits large files into manageable chunks with intelligent overlap handling
- **LLM Integration**: Supports multiple LLM providers (currently Gemini, more coming soon)
- **Concurrent Processing**: Efficient parallel processing of audio chunks for faster transcription
- **Flexible Output**: Export transcripts in JSON, plain text, or SRT subtitle format
- **Speaker Identification**: Automatic speaker diarization and identification
- **Timestamp Support**: Precise timestamps for each transcribed segment
- **Custom Prompts**: Use specialized prompts for different content types (meetings, interviews, lectures)
- **Configurable**: Comprehensive configuration options via YAML or environment variables

## 🚀 Installation

### Prerequisites

- Go 1.21 or higher
- FFmpeg (for audio/video processing)
- API key for supported LLM provider (e.g., Google Gemini)

### Install FFmpeg

**macOS:**
```bash
brew install ffmpeg
```

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install ffmpeg
```

**Windows:**
Download from [FFmpeg official website](https://ffmpeg.org/download.html)

### Install gollmscribe

#### From Source

```bash
go install github.com/eternnoir/gollmscribe/cmd/gollmscribe@latest
```

#### Build from Repository

```bash
git clone https://github.com/eternnoir/gollmscribe.git
cd gollmscribe
make build
```

## 📖 Usage

### Command Line Interface

#### Basic Usage

```bash
# Set your API key
export GOLLMSCRIBE_API_KEY="your-gemini-api-key"

# Transcribe an audio file
gollmscribe transcribe audio.mp3

# Transcribe with specific output format
gollmscribe transcribe --format json --output transcript.json audio.mp3

# Use a custom prompt
gollmscribe transcribe --prompt "Transcribe this meeting recording" meeting.mp4
```

#### Advanced Options

```bash
# Configure chunking parameters
gollmscribe transcribe --chunk-minutes 20 --overlap-seconds 30 audio.wav

# Enable speaker identification and timestamps
gollmscribe transcribe --with-speaker --with-timestamp interview.mp3

# Use specific language
gollmscribe transcribe --language zh-TW podcast.mp3

# Process multiple files
gollmscribe transcribe *.mp3
```

### Configuration

Create a configuration file at `~/.gollmscribe.yaml`:

```yaml
provider:
  name: "gemini"
  api_key: "your-api-key-here"  # Better to use GOLLMSCRIBE_API_KEY env var
  temperature: 0.1

audio:
  chunk_minutes: 30
  overlap_seconds: 60
  workers: 3

transcribe:
  language: "auto"
  with_timestamp: true
  with_speaker_id: true
```

See [.gollmscribe.yaml.example](.gollmscribe.yaml.example) for all available options.

### As a Library

```go
package main

import (
    "context"
    "log"
    
    "github.com/eternnoir/gollmscribe/pkg/providers/gemini"
    "github.com/eternnoir/gollmscribe/pkg/transcriber"
)

func main() {
    // Initialize provider
    provider := gemini.NewProvider("your-api-key")
    
    // Create transcriber
    tr := transcriber.NewTranscriber(provider, "")
    
    // Transcribe file
    req := &transcriber.TranscribeRequest{
        FilePath: "audio.mp3",
        Options: transcriber.TranscribeOptions{
            WithTimestamp: true,
            WithSpeakerID: true,
        },
    }
    
    result, err := tr.Transcribe(context.Background(), req)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Transcription: %s", result.Text)
}
```

## 🛠️ API Documentation

### Core Interfaces

#### Transcriber
The main interface for transcription operations:

```go
type Transcriber interface {
    Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResult, error)
    TranscribeWithProgress(ctx context.Context, req *TranscribeRequest, callback ProgressCallback) (*TranscribeResult, error)
    TranscribeBatch(ctx context.Context, requests []*TranscribeRequest) ([]*TranscribeResult, error)
}
```

#### LLMProvider
Interface for LLM provider implementations:

```go
type LLMProvider interface {
    Name() string
    Transcribe(ctx context.Context, req *TranscriptionRequest) (*TranscriptionResult, error)
    TranscribeChunk(ctx context.Context, chunk *AudioChunk, prompt string, options TranscriptionOptions) (*TranscriptionResult, error)
}
```

See [examples](examples/) directory for more usage examples.

## 🏗️ Architecture

```
gollmscribe/
├── cmd/gollmscribe/        # CLI application
├── pkg/
│   ├── audio/              # Audio processing and chunking
│   ├── config/             # Configuration management
│   ├── providers/          # LLM provider implementations
│   │   └── gemini/         # Google Gemini provider
│   └── transcriber/        # Core transcription logic
├── examples/               # Usage examples
└── testdata/              # Test files
```

## 🧪 Development

### Running Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific test
go test -run TestChunking ./pkg/audio
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Vet code
make vet
```

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Create release archives
make release
```

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [FFmpeg](https://ffmpeg.org/) for audio/video processing
- [Google Gemini](https://deepmind.google/technologies/gemini/) for multimodal AI capabilities
- [Cobra](https://github.com/spf13/cobra) for CLI framework
- [Viper](https://github.com/spf13/viper) for configuration management

## 📮 Support

- 🐛 Issues: [GitHub Issues](https://github.com/eternnoir/gollmscribe/issues)

<a href="https://www.buymeacoffee.com/eternnoir"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" width="300"></a>

---

Made with ❤️ by [Frank Wang](https://github.com/eternnoir)