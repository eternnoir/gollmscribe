# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gollmscribe is a Go application that transforms audio files into precise text transcripts using advanced Large Language Models and multimodal AI processing capabilities.

## Development Documents

Important development documents are located in the `.development/` directory:
- `PROJECT_SPEC.md` - Original project specifications and requirements
- `IMPLEMENTATION_PLAN.md` - Detailed implementation plan based on AI discussions

Please review and update these documents as the project evolves.

## Development Commands

Since the Makefile is empty, use standard Go commands:

```bash
# Build the application
go build -o gollmscribe ./cmd/gollmscribe

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestName ./...

# Install the binary
go install ./cmd/gollmscribe

# Format code
go fmt ./...

# Vet code
go vet ./...
```

## Architecture

The codebase follows a clean architecture pattern with the following key components:

- **cmd/gollmscribe/**: Command-line interface entry point
- **gollmscribe.go**: Core library API and orchestration logic
- **transcriber.go**: Audio/video transcription implementation
- **llm.go**: LLM provider integration (OpenAI, Anthropic, etc.)
- **config.go**: Configuration management for API keys and settings
- **errors.go**: Domain-specific error types

The typical data flow:
1. CLI receives audio/video file path
2. Config loads API credentials and settings
3. Transcriber processes the media file
4. LLM provider transcribes audio to text
5. Results are returned to the user

## Important Notes

- The go.mod file specifies Go 1.24.4, which needs to be corrected to a valid version (e.g., 1.21.0 or 1.22.0)
- Test files are located in testdata/ (audio.wav and video.mp4)
- No external dependencies are currently declared - you'll need to add LLM client libraries when implementing