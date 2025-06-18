# Watch Folder Example

This example demonstrates how to use gollmscribe's watch folder functionality programmatically in Go. The watch folder feature automatically monitors a directory for new audio/video files and transcribes them in real-time.

## Features Demonstrated

- **Automatic File Monitoring**: Watches a directory for new audio/video files
- **Real-time Processing**: Processes files as they appear
- **Progress Tracking**: Shows detailed progress for each file
- **Statistics Display**: Live statistics updates every 30 seconds
- **Graceful Shutdown**: Handles Ctrl+C to stop watching cleanly
- **Configuration Integration**: Uses gollmscribe configuration files

## Usage

### Prerequisites

1. Set your API key:
   ```bash
   export GOLLMSCRIBE_API_KEY="your-gemini-api-key"
   ```

2. Create a configuration file (optional) at `~/.gollmscribe.yaml`:
   ```yaml
   provider:
     name: "gemini"
     temperature: 0.1

   audio:
     chunk_minutes: 15
     overlap_seconds: 30
     workers: 3

   watch:
     patterns: ["*.mp3", "*.wav", "*.mp4", "*.m4a"]
     recursive: true
     max_workers: 3
     output_dir: "./transcripts"
     move_to: "./processed"
   ```

### Running the Example

```bash
# Navigate to the example directory
cd examples/watchfolder

# Run with a directory to watch
go run main.go /path/to/watch/directory

# Example: Watch current directory
go run main.go .

# Example: Watch a specific folder
go run main.go ~/Downloads/audio
```

### What Happens

1. **Startup**: The watcher initializes and shows configuration
2. **File Detection**: New files matching patterns are detected
3. **Processing**: Files are transcribed automatically with progress updates
4. **Statistics**: Live statistics are displayed every 30 seconds
5. **Shutdown**: Press Ctrl+C to stop watching and see final statistics

### Sample Output

```
🚀 Starting file watcher for directory: ./test-audio
📋 Configuration:
   Patterns: [*.mp3 *.wav *.mp4 *.m4a]
   Recursive: true
   Workers: 3
   Output: ./transcripts
   Move to: ./processed

👀 Watching for new files... Press Ctrl+C to stop

[14:30:15] 📁 Found: meeting.mp3
[14:30:15] ⏳ Processing: meeting.mp3 - Starting processing
[14:30:45] ✅ Completed: meeting.mp3 - Transcription completed in 30s
📊 [2m30s] Processed: 1 | Failed: 0 | In Progress: 0 | Size: 15.2 MB

^C

🛑 Shutting down...

📊 Final Statistics:
   Processed: 1 files
   Failed: 0 files
   Skipped: 0 files
   Total size: 15.20 MB
   Runtime: 2m45s

✅ File watcher stopped successfully
```

## Key Components

### 1. Configuration Loading
The example shows how to load gollmscribe configuration and apply it to the watch functionality.

### 2. Progress Monitoring
Detailed progress callbacks show:
- File discovery
- Processing start/progress
- Completion status
- Error handling

### 3. Statistics Display
Live statistics include:
- Files processed, failed, and in progress
- Total data size processed
- Runtime duration

### 4. Graceful Shutdown
Proper signal handling ensures:
- Current processing completes
- Resources are cleaned up
- Final statistics are displayed

## Customization

You can modify the example to:

- **Custom Prompts**: Set `watchConfig.SharedPrompt` for specific transcription needs
- **File Patterns**: Adjust `watchConfig.Patterns` for different file types
- **Output Configuration**: Change `watchConfig.OutputDir` and `watchConfig.MoveToDir`
- **Processing Options**: Modify `TranscribeOptions` for different chunking strategies
- **Advanced Callbacks**: Enhance progress callbacks for custom notifications

## Error Handling

The example includes robust error handling for:
- Invalid directories
- Configuration loading failures
- Provider initialization issues
- File processing errors

## Integration with CLI

This example shows the same functionality available in the CLI command:
```bash
gollmscribe watch /path/to/directory
```

But with programmatic control and custom callbacks for integration into larger applications.