# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

### Security
- API keys are never logged or exposed
- Secure handling of temporary files
- Input validation for all user-provided data

## [0.1.0] - 2025-06-17

### Added
- First public release
- Core transcription functionality
- Basic CLI commands
- Gemini provider support

[Unreleased]: https://github.com/eternnoir/gollmscribe/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/eternnoir/gollmscribe/releases/tag/v0.1.0