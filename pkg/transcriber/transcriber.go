package transcriber

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/eternnoir/gollmscribe/pkg/audio"
	"github.com/eternnoir/gollmscribe/pkg/logger"
	"github.com/eternnoir/gollmscribe/pkg/providers"
)

// TranscriberImpl implements the Transcriber interface
type TranscriberImpl struct {
	provider  providers.LLMProvider
	processor audio.Processor
	chunker   audio.Chunker
	reader    audio.Reader
	merger    ChunkMerger
	tempDir   string
}

// NewTranscriber creates a new transcriber instance
func NewTranscriber(provider providers.LLMProvider, tempDir string) *TranscriberImpl {
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	return &TranscriberImpl{
		provider:  provider,
		processor: audio.NewProcessor(tempDir),
		chunker:   audio.NewChunker(tempDir),
		reader:    audio.NewReader(tempDir),
		merger:    NewChunkMerger(),
		tempDir:   tempDir,
	}
}

// Transcribe processes a single audio/video file
func (t *TranscriberImpl) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResult, error) {
	return t.TranscribeWithProgress(ctx, req, nil)
}

// TranscribeWithProgress processes a file with progress reporting
func (t *TranscriberImpl) TranscribeWithProgress(ctx context.Context, req *TranscribeRequest, callback ProgressCallback) (*TranscribeResult, error) {
	log := logger.WithComponent("transcriber").WithField("file", filepath.Base(req.FilePath))
	startTime := time.Now()

	log.Info().
		Str("output_path", req.OutputPath).
		Str("language", req.Language).
		Interface("options", req.Options).
		Msg("Starting transcription with progress")

	// Validate input file
	log.Debug().Msg("Validating input file")
	if err := t.processor.ValidateFile(req.FilePath); err != nil {
		log.Error().Err(err).Msg("File validation failed")
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	// Get audio info
	log.Debug().Msg("Getting audio information")
	audioInfo, err := t.processor.GetAudioInfo(req.FilePath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get audio info")
		return nil, fmt.Errorf("failed to get audio info: %w", err)
	}

	log.Info().
		Dur("duration", audioInfo.Duration).
		Bool("is_video", audioInfo.IsVideo).
		Str("format", string(audioInfo.Format)).
		Msg("Audio information retrieved")

	// Convert video to audio if needed
	audioPath := req.FilePath
	if audioInfo.IsVideo {
		log.Info().Msg("Converting video to audio")
		audioPath, err = t.convertVideoToAudio(req.FilePath)
		if err != nil {
			log.Error().Err(err).Msg("Video conversion failed")
			return nil, fmt.Errorf("video conversion failed: %w", err)
		}
		log.Info().Str("audio_path", audioPath).Msg("Video converted to audio")
		defer func() {
			if !req.Options.PreserveAudio {
				log.Debug().Str("audio_path", audioPath).Msg("Cleaning up converted audio file")
				_ = os.Remove(audioPath)
			}
		}()
	}

	// Create audio chunks
	log.Info().
		Int("chunk_minutes", req.Options.ChunkMinutes).
		Int("overlap_seconds", req.Options.OverlapSeconds).
		Msg("Creating audio chunks")
	chunks, err := t.createChunks(audioPath, req.Options)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create chunks")
		return nil, fmt.Errorf("failed to create chunks: %w", err)
	}
	log.Info().Int("chunk_count", len(chunks)).Msg("Audio chunks created")
	defer func() {
		if !req.Options.PreserveAudio {
			log.Debug().Int("chunk_count", len(chunks)).Msg("Cleaning up chunk files")
			_ = t.chunker.CleanupChunks(chunks)
		}
	}()

	// Transcribe chunks in parallel
	log.Info().
		Int("workers", req.Options.Workers).
		Int("chunks", len(chunks)).
		Msg("Starting parallel chunk transcription")
	results, err := t.transcribeChunks(ctx, chunks, req, callback)
	if err != nil {
		log.Error().Err(err).Msg("Chunk transcription failed")
		return nil, fmt.Errorf("chunk transcription failed: %w", err)
	}

	// Merge results
	log.Info().Msg("Merging transcription results")
	finalResult, err := t.merger.MergeChunks(results)
	if err != nil {
		log.Error().Err(err).Msg("Failed to merge chunks")
		return nil, fmt.Errorf("failed to merge chunks: %w", err)
	}

	// Fill in additional metadata
	finalResult.FilePath = req.FilePath
	finalResult.Duration = audioInfo.Duration
	finalResult.ChunkCount = len(chunks)
	finalResult.ProcessTime = time.Since(startTime)
	finalResult.Provider = t.provider.Name()

	log.Info().
		Int("final_text_length", len(finalResult.Text)).
		Int("segments", len(finalResult.Segments)).
		Dur("processing_time", finalResult.ProcessTime).
		Msg("Transcription results merged")

	// Save output if specified
	if req.OutputPath != "" {
		log.Info().Str("output_path", req.OutputPath).Str("format", req.Options.OutputFormat).Msg("Saving transcription result")
		if err := t.saveResult(finalResult, req.OutputPath, req.Options.OutputFormat); err != nil {
			log.Error().Err(err).Str("output_path", req.OutputPath).Msg("Failed to save result")
			return nil, fmt.Errorf("failed to save result: %w", err)
		}
		log.Info().Str("output_path", req.OutputPath).Msg("Transcription result saved")
	}

	return finalResult, nil
}

// TranscribeBatch processes multiple files
func (t *TranscriberImpl) TranscribeBatch(ctx context.Context, requests []*TranscribeRequest) ([]*TranscribeResult, error) {
	results := make([]*TranscribeResult, len(requests))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	// Use semaphore to limit concurrent transcriptions
	workers := 3
	if len(requests) > 0 && requests[0].Options.Workers > 0 {
		workers = requests[0].Options.Workers
	}
	semaphore := make(chan struct{}, workers)

	for i, req := range requests {
		wg.Add(1)
		go func(index int, request *TranscribeRequest) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, err := t.Transcribe(ctx, request)
			mu.Lock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			results[index] = result
			mu.Unlock()
		}(i, req)
	}

	wg.Wait()
	return results, firstErr
}

// SupportedFormats returns supported file formats
func (t *TranscriberImpl) SupportedFormats() []string {
	return []string{
		"audio/wav", "audio/mp3", "audio/mpeg", "audio/m4a", "audio/flac",
		"video/mp4", "video/avi", "video/mov", "video/mkv",
	}
}

// SetProvider changes the LLM provider
func (t *TranscriberImpl) SetProvider(provider providers.LLMProvider) {
	t.provider = provider
}

// convertVideoToAudio converts video file to audio
func (t *TranscriberImpl) convertVideoToAudio(videoPath string) (string, error) {
	audioPath := filepath.Join(t.tempDir, fmt.Sprintf("audio_%d.mp3", time.Now().Unix()))

	if err := t.processor.ConvertToAudio(videoPath, audioPath, audio.FormatMP3); err != nil {
		return "", err
	}

	return audioPath, nil
}

// createChunks creates audio chunks based on options
func (t *TranscriberImpl) createChunks(audioPath string, options TranscribeOptions) ([]*audio.ChunkInfo, error) {
	processorOptions := audio.ProcessorOptions{
		ChunkDuration:   time.Duration(options.ChunkMinutes) * time.Minute,
		OverlapDuration: time.Duration(options.OverlapSeconds) * time.Second,
		OutputFormat:    audio.FormatMP3,
		TempDir:         t.tempDir,
		KeepTemp:        options.PreserveAudio,
	}

	// Set defaults if not specified
	if processorOptions.ChunkDuration == 0 {
		processorOptions.ChunkDuration = 30 * time.Minute
	}
	if processorOptions.OverlapDuration == 0 {
		processorOptions.OverlapDuration = 60 * time.Second
	}

	return t.chunker.ChunkAudio(audioPath, processorOptions)
}

// transcribeChunks transcribes all chunks in parallel
func (t *TranscriberImpl) transcribeChunks(ctx context.Context, chunks []*audio.ChunkInfo, req *TranscribeRequest, callback ProgressCallback) ([]*providers.TranscriptionResult, error) {
	log := logger.WithComponent("chunk-processor").WithField("file", filepath.Base(req.FilePath))

	results := make([]*providers.TranscriptionResult, len(chunks))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	// Use semaphore to limit concurrent requests
	workers := req.Options.Workers
	if workers <= 0 {
		workers = 3
	}
	log.Debug().Int("workers", workers).Int("total_chunks", len(chunks)).Msg("Initializing chunk transcription workers")
	semaphore := make(chan struct{}, workers)

	completed := 0

	for i, chunk := range chunks {
		wg.Add(1)
		go func(index int, chunkInfo *audio.ChunkInfo) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			chunkLog := log.WithField("chunk_index", index)
			chunkLog.Debug().
				Dur("start", chunkInfo.Start).
				Dur("end", chunkInfo.End).
				Str("temp_file", chunkInfo.TempFilePath).
				Msg("Starting chunk transcription")

			// Transcribe chunk
			result, err := t.transcribeChunk(ctx, chunkInfo, req)

			mu.Lock()
			if err != nil {
				chunkLog.Error().Err(err).Msg("Chunk transcription failed")
				if firstErr == nil {
					firstErr = err
				}
			} else if result != nil {
				result.ChunkID = index
				results[index] = result
				chunkLog.Debug().
					Int("text_length", len(result.Text)).
					Int("segments", len(result.Segments)).
					Msg("Chunk transcription completed")
			}
			completed++
			if callback != nil {
				callback(completed, len(chunks), fmt.Sprintf("Chunk %d", index+1))
			}
			mu.Unlock()
		}(i, chunk)
	}

	wg.Wait()

	if firstErr != nil {
		log.Error().Err(firstErr).Int("completed", completed).Int("total", len(chunks)).Msg("Chunk transcription failed")
		return nil, firstErr
	}

	log.Info().Int("completed", completed).Int("total", len(chunks)).Msg("All chunks transcribed successfully")
	return results, nil
}

// transcribeChunk transcribes a single chunk
func (t *TranscriberImpl) transcribeChunk(ctx context.Context, chunk *audio.ChunkInfo, req *TranscribeRequest) (*providers.TranscriptionResult, error) {
	log := logger.WithComponent("chunk").WithField("temp_file", filepath.Base(chunk.TempFilePath))

	// Read chunk data
	log.Debug().Msg("Opening chunk file")
	chunkReader, err := t.reader.OpenAudio(chunk.TempFilePath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to open chunk file")
		return nil, fmt.Errorf("failed to open chunk: %w", err)
	}
	defer func() {
		_ = chunkReader.Close()
	}()

	// Create transcription request
	transcReq := &providers.TranscriptionRequest{
		Audio:       chunkReader,
		AudioFormat: "mp3",
		MimeType:    "audio/mpeg",
		Filename:    filepath.Base(chunk.TempFilePath),
		Language:    req.Language,
		Prompt:      req.CustomPrompt,
		Options: providers.TranscriptionOptions{
			WithTimestamp:  req.Options.WithTimestamp,
			WithSpeakerID:  req.Options.WithSpeakerID,
			Temperature:    req.Options.Temperature,
			MaxTokens:      65535,
			TimeoutSeconds: 300,
		},
	}

	log.Debug().
		Str("language", req.Language).
		Float32("temperature", req.Options.Temperature).
		Bool("with_timestamp", req.Options.WithTimestamp).
		Bool("with_speaker_id", req.Options.WithSpeakerID).
		Msg("Sending chunk to provider for transcription")

	// Transcribe using provider
	result, err := t.provider.Transcribe(ctx, transcReq)
	if err != nil {
		log.Error().Err(err).Msg("Provider transcription failed")
		return nil, fmt.Errorf("provider transcription failed: %w", err)
	}

	log.Debug().
		Int("text_length", len(result.Text)).
		Int("segments", len(result.Segments)).
		Msg("Received transcription result from provider")

	// Adjust timestamps based on chunk start time
	if len(result.Segments) > 0 {
		log.Debug().
			Dur("chunk_start", chunk.Start).
			Int("segments_count", len(result.Segments)).
			Msg("Adjusting timestamps for chunk offset")
		for i := range result.Segments {
			result.Segments[i].Start += chunk.Start
			result.Segments[i].End += chunk.Start
		}
	}

	return result, nil
}

// saveResult saves the transcription result to file
func (t *TranscriberImpl) saveResult(result *TranscribeResult, outputPath, format string) error {
	log := logger.WithComponent("file-writer").WithField("output_path", outputPath)

	log.Debug().Str("format", format).Msg("Formatting transcription result")

	var content []byte
	var err error

	switch format {
	case "json":
		if result.Metadata == nil {
			result.Metadata = make(map[string]interface{})
		}
		result.Metadata["saved_at"] = time.Now().Format(time.RFC3339)
		content, err = result.ToJSON(true)
	case "text":
		content = []byte(result.Text)
	case "srt":
		content, err = result.ToSRT()
	default:
		log.Warn().Str("format", format).Msg("Unknown format, defaulting to JSON")
		content, err = result.ToJSON(true)
	}

	if err != nil {
		log.Error().Err(err).Str("format", format).Msg("Failed to format result")
		return fmt.Errorf("failed to format result: %w", err)
	}

	log.Debug().Int("content_size", len(content)).Msg("Content formatted successfully")

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	log.Debug().Str("output_dir", outputDir).Msg("Creating output directory")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Error().Err(err).Str("output_dir", outputDir).Msg("Failed to create output directory")
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write to file
	log.Debug().Int("content_size", len(content)).Msg("Writing result to file")
	if err := os.WriteFile(outputPath, content, 0o644); err != nil {
		log.Error().Err(err).Str("output_path", outputPath).Msg("Failed to write result file")
		return fmt.Errorf("failed to write result file: %w", err)
	}

	log.Info().
		Str("output_path", outputPath).
		Str("format", format).
		Int("size_bytes", len(content)).
		Msg("Transcription result saved successfully")

	return nil
}
