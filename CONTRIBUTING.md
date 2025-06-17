# Contributing to gollmscribe

First off, thank you for considering contributing to gollmscribe! It's people like you that make gollmscribe such a great tool.

## Code of Conduct

By participating in this project, you are expected to uphold our Code of Conduct:

- Be respectful and inclusive
- Welcome newcomers and help them get started
- Focus on what is best for the community
- Show empathy towards other community members

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check existing issues to avoid duplicates. When you create a bug report, include as many details as possible:

- **Use a clear and descriptive title**
- **Describe the exact steps to reproduce the problem**
- **Provide specific examples**
- **Describe the behavior you observed and what you expected**
- **Include logs and error messages**
- **Include your environment details** (OS, Go version, etc.)

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion:

- **Use a clear and descriptive title**
- **Provide a detailed description of the proposed enhancement**
- **Explain why this enhancement would be useful**
- **List any alternatives you've considered**

### Your First Code Contribution

Unsure where to begin? You can start by looking through these issues:

- Issues labeled `good first issue`
- Issues labeled `help wanted`

### Pull Requests

1. Fork the repo and create your branch from `main`
2. If you've added code that should be tested, add tests
3. If you've changed APIs, update the documentation
4. Ensure the test suite passes
5. Make sure your code follows the existing style
6. Issue that pull request!

## Development Process

### Setting Up Your Development Environment

```bash
# Clone your fork
git clone https://github.com/your-username/gollmscribe.git
cd gollmscribe

# Add upstream remote
git remote add upstream https://github.com/eternnoir/gollmscribe.git

# Install dependencies
go mod download

# Run tests
make test
```

### Code Style

- Follow standard Go conventions
- Use `gofmt` to format your code
- Run `go vet` to catch common mistakes
- Use meaningful variable and function names
- Add comments for exported functions and types

### Testing

- Write tests for new functionality
- Ensure all tests pass before submitting PR
- Aim for good test coverage
- Include both unit tests and integration tests where appropriate

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific tests
go test -run TestName ./pkg/...
```

### Commit Messages

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line

Example:
```
Add support for OpenAI provider

- Implement OpenAI API client
- Add configuration options for OpenAI
- Update documentation with OpenAI examples

Fixes #123
```

### Documentation

- Update the README.md if needed
- Add/update code comments
- Update configuration examples if you add new options
- Add examples for new features

## Project Structure

```
gollmscribe/
├── cmd/gollmscribe/    # CLI application
├── pkg/
│   ├── audio/          # Audio processing
│   ├── config/         # Configuration
│   ├── providers/      # LLM providers
│   └── transcriber/    # Core transcription logic
├── examples/           # Usage examples
└── testdata/          # Test files
```

## Adding a New Provider

If you want to add support for a new LLM provider:

1. Create a new package under `pkg/providers/yourprovider`
2. Implement the `LLMProvider` interface
3. Add configuration options in `pkg/config`
4. Add tests for your provider
5. Add an example in the `examples` directory
6. Update the README with your provider information

## Release Process

Releases are managed by maintainers. The process is:

1. Update version in Makefile
2. Update CHANGELOG.md
3. Create a git tag
4. Push tag to trigger release build

## Questions?

Feel free to open an issue with your question or reach out to the maintainers.

Thank you for contributing! 🎉