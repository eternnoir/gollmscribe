# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2025-06-18

### Added
- **Watch Folder Feature**: Automatic directory monitoring for new audio/video files
  - Real-time file system monitoring with fsnotify
  - Shared prompts and configuration across all files in a watch session
  - Concurrent processing with configurable worker pools
  - Comprehensive deduplication using content hashing
  - Processing history with BoltDB for crash recovery
  - Cross-filesystem file movement support
  - Event debouncing to prevent duplicate processing
  - `--once` mode for batch processing existing files
  - Stale processing marker cleanup on startup
  - Recursive directory watching
  - File pattern filtering
  - Progress tracking and statistics
- **Enhanced CLI**: New `watch` command with extensive configuration options
- **Improved Error Handling**: Better recovery from interrupted processing
- **Performance Optimizations**: Event deduplication and efficient file monitoring

### Fixed
- Cross-device link errors when moving files across different filesystems
- Stale processing markers left by crashed processes
- CLI flag propagation in watch mode
- Event handling for large files and duplicate filesystem events

### Security
- API keys are never logged or exposed
- Secure handling of temporary files
- Input validation for all user-provided data

## [0.1.1] - 2025-06-17

### Fixed
- Code formatting and linting issues
- Bug fixes in core functionality

## [0.1.0] - 2025-06-17

### Added
- Initial release of gollmscribe
- Support for audio formats: WAV, MP3, M4A, FLAC
- Support for video formats: MP4, AVI, MOV, MKV
- Google Gemini LLM provider integration
- Smart audio chunking with configurable overlap
- Concurrent chunk processing for improved performance
- Multiple output formats: JSON, plain text, SRT
- Speaker identification and timestamp support
- Configuration via YAML file and environment variables
- Command-line interface with Cobra
- Comprehensive examples and documentation
- GitHub Actions CI/CD pipeline
- Cross-platform binary builds

[Unreleased]: https://github.com/eternnoir/gollmscribe/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/eternnoir/gollmscribe/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/eternnoir/gollmscribe/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/eternnoir/gollmscribe/releases/tag/v0.1.0