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

Use the Makefile targets for development tasks:

```bash
# Build the application
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Code quality checks (MUST run after code changes)
make check  # Runs fmt, vet, and lint

# Individual quality checks
make fmt    # Format code
make vet    # Vet code
make lint   # Run linter

# Build for all platforms
make build-all

# Clean build artifacts
make clean

# Show all available targets
make help
```

**IMPORTANT**: Always run `make check` after making code changes to ensure code quality and prevent CI failures. This runs all the same linting checks as the GitHub Actions workflow.

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