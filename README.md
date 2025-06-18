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
- **Custom Prompts**: Use specialized prompts for different content types (meetings, interviews, lectures)
- **Prompt-driven Features**: Control output format, speaker identification, timestamps, and more through intelligent prompts
- **Watch Folder**: Automatically monitor directories for new audio/video files and transcribe them in real-time
- **Batch Processing**: Process existing files or watch for new ones with shared configuration and prompts
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

# Transcribe with custom output file
gollmscribe transcribe --output transcript.txt audio.mp3

# Use a custom prompt
gollmscribe transcribe --prompt "Transcribe this meeting recording" meeting.mp4

# Use prompt from file
gollmscribe transcribe --prompt-file my-prompt.txt interview.mp3
```

#### Advanced Options

```bash
# Configure chunking parameters
gollmscribe transcribe --chunk-minutes 20 --overlap-seconds 30 audio.wav

# Process multiple files
gollmscribe transcribe *.mp3

# Adjust processing settings
gollmscribe transcribe --workers 5 --temperature 0.2 conference.mp4
```

#### Watch Folder Mode

Monitor a directory for new audio/video files and automatically transcribe them:

```bash
# Watch current directory
gollmscribe watch .

# Watch with custom prompt for all files
gollmscribe watch ./recordings -p "Transcribe and identify speakers"

# Watch recursively with file movement
gollmscribe watch ./inbox -r --move-to ./processed

# Watch with custom output directory
gollmscribe watch ./meetings --output-dir ./transcripts

# Process existing files once and exit
gollmscribe watch ./batch --once

# Watch specific file types
gollmscribe watch ./audio --pattern "*.mp3,*.m4a"

# Advanced watch options
gollmscribe watch ./monitor \
  --recursive \
  --max-workers 5 \
  --chunk-minutes 20 \
  --processing-timeout 45m \
  --stability-wait 5s \
  --move-to ./completed \
  --output-dir ./transcripts
```

**Watch Mode Features:**
- **Real-time monitoring**: Detects new files as they're added
- **Shared configuration**: All files in a watch session use the same prompt and settings
- **Deduplication**: Prevents processing the same file multiple times using content hashing
- **Crash recovery**: Cleans up stale processing markers from interrupted sessions
- **Cross-filesystem moves**: Handles moving files across different disk partitions
- **Concurrent processing**: Multiple files processed simultaneously with configurable worker limits
- **Progress tracking**: Real-time status updates and statistics

### Prompt Examples for Different Use Cases

The power of gollmscribe lies in using custom prompts for different transcription scenarios. Here are some practical examples:

#### Meeting Transcription with Speaker Identification

```bash
# Create a prompt file for meeting transcription
cat > meeting-prompt.txt << 'EOF'
Please create a verbatim transcript of the provided meeting recording. No timestamps are needed, but speaker identification is required. Combine consecutive speeches from the same speaker. Use the format specified in <output_format>. You must generate the transcript completely and accurately without missing any words.

<output_format>
English Name (中文名稱)：
Speech content................
</output_format>
EOF

# Use the prompt file
gollmscribe transcribe --prompt-file meeting-prompt.txt meeting.wav
```

#### Interview Transcription

```bash
gollmscribe transcribe --prompt "Please transcribe this interview with clear speaker identification. Format as Q: [Question] A: [Answer]. Include natural speech patterns and pauses marked with [pause]." interview.mp3
```

#### Educational Content

```bash
gollmscribe transcribe --prompt "Transcribe this lecture content. Structure the output with clear headings for different topics discussed. Include important terminology and concepts exactly as spoken." lecture.mp4
```

#### Medical/Legal Documentation

```bash
gollmscribe transcribe --prompt "Provide precise medical transcription with exact terminology. Include speaker identification for doctor and patient. Mark any unclear sections with [unclear]." medical-consultation.wav
```

#### Podcast Transcription

```bash
gollmscribe transcribe --prompt "Transcribe this podcast episode. Include speaker names, natural conversation flow, and mark significant pauses or laughter as [laughter] or [pause]." podcast.mp3
```

#### Multilingual Content

```bash
gollmscribe transcribe --prompt "Transcribe this Chinese content while maintaining the original tone and expressions. Keep English words in their original form and appropriately mark mixed Chinese-English sections." chinese-content.wav
```

#### Summary with Key Points

```bash
gollmscribe transcribe --prompt "Transcribe the full content and then provide a summary with key action items at the end. Format: [FULL TRANSCRIPT] followed by [SUMMARY] and [ACTION ITEMS]." business-meeting.mp4
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
  default_prompt: "Please provide a complete, accurate transcription..."
  prompt_templates:
    meeting: "Please transcribe this meeting recording, identify each speaker..."
    interview: "Please transcribe this interview, clearly distinguishing..."
    lecture: "Please transcribe this educational content..."

watch:
  patterns: ["*.mp3", "*.wav", "*.mp4", "*.m4a"]
  recursive: false
  interval: 5s
  stability_wait: 2s
  processing_timeout: 30m
  max_workers: 3
  output_dir: ""
  move_to: ""
  history_db: ".gollmscribe-watch.db"
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
        CustomPrompt: "Please transcribe with speaker identification",
        Options: transcriber.TranscribeOptions{
            ChunkMinutes:   30,
            OverlapSeconds: 60,
            Workers:        3,
            Temperature:    0.1,
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
│   ├── transcriber/        # Core transcription logic
│   └── watcher/            # File watching and batch processing
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