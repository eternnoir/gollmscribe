package transcriber

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/eternnoir/gollmscribe/pkg/providers"
)

// ChunkMergerImpl implements the ChunkMerger interface
type ChunkMergerImpl struct {
	overlapThreshold time.Duration
}

// NewChunkMerger creates a new chunk merger
func NewChunkMerger() *ChunkMergerImpl {
	return &ChunkMergerImpl{
		overlapThreshold: 30 * time.Second, // Minimum overlap to consider for merging
	}
}

// MergeChunks combines multiple transcription results with overlap handling
func (m *ChunkMergerImpl) MergeChunks(chunks []*providers.TranscriptionResult) (*TranscribeResult, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks to merge")
	}

	// Sort chunks by chunk ID to ensure proper order
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].ChunkID < chunks[j].ChunkID
	})

	// Filter out nil chunks
	validChunks := make([]*providers.TranscriptionResult, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk != nil && chunk.Text != "" {
			validChunks = append(validChunks, chunk)
		}
	}

	if len(validChunks) == 0 {
		return nil, fmt.Errorf("no valid chunks to merge")
	}

	// If only one chunk, return it directly
	if len(validChunks) == 1 {
		return m.convertToTranscribeResult(validChunks[0]), nil
	}

	// Merge chunks with overlap handling
	merged := m.mergeWithOverlap(validChunks)

	return merged, nil
}

// DetectOverlap identifies overlapping content between chunks
func (m *ChunkMergerImpl) DetectOverlap(chunk1, chunk2 *providers.TranscriptionResult) (overlap1, overlap2 time.Duration, err error) {
	if len(chunk1.Segments) == 0 || len(chunk2.Segments) == 0 {
		return 0, 0, nil
	}

	// Get the last segments from chunk1 and first segments from chunk2
	lastSegments1 := chunk1.Segments
	firstSegments2 := chunk2.Segments

	// Find potential overlap by comparing text content
	overlapStart := time.Duration(0)
	overlapEnd := time.Duration(0)

	// Look for text similarity in the last part of chunk1 and first part of chunk2
	chunk1LastPart := m.getLastTextPart(chunk1.Text, 100)   // Last 100 characters
	chunk2FirstPart := m.getFirstTextPart(chunk2.Text, 100) // First 100 characters

	similarity := m.calculateTextSimilarity(chunk1LastPart, chunk2FirstPart)

	if similarity > 0.3 { // 30% similarity threshold
		// Find the actual time boundaries
		if len(lastSegments1) > 0 && len(firstSegments2) > 0 {
			// Look for segment overlap
			for i := len(lastSegments1) - 1; i >= 0; i-- {
				for j := 0; j < len(firstSegments2); j++ {
					seg1 := lastSegments1[i]
					seg2 := firstSegments2[j]

					if m.segmentsOverlap(seg1, seg2) {
						overlapStart = seg1.Start
						overlapEnd = seg2.End
						break
					}
				}
				if overlapEnd > 0 {
					break
				}
			}
		}
	}

	return overlapStart, overlapEnd, nil
}

// mergeWithOverlap merges chunks while handling overlapping content
func (m *ChunkMergerImpl) mergeWithOverlap(chunks []*providers.TranscriptionResult) *TranscribeResult {
	var allSegments []providers.TranscriptionSegment
	var fullText strings.Builder
	var totalDuration time.Duration

	// Process first chunk completely
	firstChunk := chunks[0]
	allSegments = append(allSegments, firstChunk.Segments...)
	fullText.WriteString(firstChunk.Text)

	if len(firstChunk.Segments) > 0 {
		totalDuration = firstChunk.Segments[len(firstChunk.Segments)-1].End
	}

	// Process subsequent chunks with overlap detection
	for i := 1; i < len(chunks); i++ {
		currentChunk := chunks[i]
		previousChunk := chunks[i-1]

		// Detect overlap
		overlapStart, overlapEnd, err := m.DetectOverlap(previousChunk, currentChunk)
		switch {
		case err != nil:
			// If overlap detection fails, just append
			allSegments = append(allSegments, currentChunk.Segments...)
			fullText.WriteString(" ")
			fullText.WriteString(currentChunk.Text)
		case overlapEnd > overlapStart && overlapEnd-overlapStart > m.overlapThreshold:
			// Handle overlap by removing duplicated content
			mergedSegments := m.mergeOverlappingSegments(allSegments, currentChunk.Segments, overlapStart, overlapEnd)
			allSegments = mergedSegments

			// For text, remove overlap and merge
			cleanText := m.removeOverlapFromText(currentChunk.Text, overlapStart, overlapEnd)
			if cleanText != "" {
				fullText.WriteString(" ")
				fullText.WriteString(cleanText)
			}
		default:
			// No significant overlap, just append
			allSegments = append(allSegments, currentChunk.Segments...)
			fullText.WriteString(" ")
			fullText.WriteString(currentChunk.Text)
		}

		// Update total duration
		if len(currentChunk.Segments) > 0 {
			lastEnd := currentChunk.Segments[len(currentChunk.Segments)-1].End
			if lastEnd > totalDuration {
				totalDuration = lastEnd
			}
		}
	}

	// Create final result
	result := &TranscribeResult{
		Text:     strings.TrimSpace(fullText.String()),
		Segments: allSegments,
		Duration: totalDuration,
		Metadata: make(map[string]interface{}),
	}

	// Merge metadata from all chunks
	for _, chunk := range chunks {
		if chunk.Language != "" {
			result.Language = chunk.Language
		}
		// Merge metadata
		for k, v := range chunk.Metadata {
			result.Metadata[k] = v
		}
	}

	result.Metadata["merged_chunks"] = len(chunks)
	result.Metadata["merge_method"] = "overlap_detection"

	return result
}

// mergeOverlappingSegments merges segments while handling overlap
func (m *ChunkMergerImpl) mergeOverlappingSegments(existingSegments, newSegments []providers.TranscriptionSegment, overlapStart, overlapEnd time.Duration) []providers.TranscriptionSegment {
	var result []providers.TranscriptionSegment

	// Add all existing segments that end before the overlap
	for _, seg := range existingSegments {
		if seg.End <= overlapStart {
			result = append(result, seg)
		}
	}

	// Add new segments that start after the overlap
	for _, seg := range newSegments {
		if seg.Start >= overlapEnd {
			result = append(result, seg)
		}
	}

	// Sort by start time
	sort.Slice(result, func(i, j int) bool {
		return result[i].Start < result[j].Start
	})

	return result
}

// Helper functions

func (m *ChunkMergerImpl) getLastTextPart(text string, length int) string {
	if len(text) <= length {
		return text
	}
	return text[len(text)-length:]
}

func (m *ChunkMergerImpl) getFirstTextPart(text string, length int) string {
	if len(text) <= length {
		return text
	}
	return text[:length]
}

func (m *ChunkMergerImpl) calculateTextSimilarity(text1, text2 string) float64 {
	if text1 == "" || text2 == "" {
		return 0
	}

	// Simple similarity based on common words
	words1 := strings.Fields(strings.ToLower(text1))
	words2 := strings.Fields(strings.ToLower(text2))

	if len(words1) == 0 || len(words2) == 0 {
		return 0
	}

	wordSet1 := make(map[string]bool)
	for _, word := range words1 {
		wordSet1[word] = true
	}

	commonWords := 0
	for _, word := range words2 {
		if wordSet1[word] {
			commonWords++
		}
	}

	return float64(commonWords) / float64(len(words2))
}

func (m *ChunkMergerImpl) segmentsOverlap(seg1, seg2 providers.TranscriptionSegment) bool {
	return seg1.End > seg2.Start && seg2.End > seg1.Start
}

func (m *ChunkMergerImpl) removeOverlapFromText(text string, overlapStart, overlapEnd time.Duration) string {
	// This is a simplified implementation
	// In practice, you might want to use the segment timing to precisely remove overlap
	return text
}

func (m *ChunkMergerImpl) convertToTranscribeResult(chunk *providers.TranscriptionResult) *TranscribeResult {
	return &TranscribeResult{
		Text:     chunk.Text,
		Segments: chunk.Segments,
		Language: chunk.Language,
		Duration: chunk.Duration,
		Metadata: chunk.Metadata,
	}
}

// Extension methods for TranscribeResult

// ToJSON converts the result to JSON format
func (r *TranscribeResult) ToJSON(pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(r, "", "  ")
	}
	return json.Marshal(r)
}

// ToSRT converts the result to SRT subtitle format
func (r *TranscribeResult) ToSRT() ([]byte, error) {
	if len(r.Segments) == 0 {
		return []byte(r.Text), nil
	}

	var srt strings.Builder
	for i, segment := range r.Segments {
		srt.WriteString(fmt.Sprintf("%d\n", i+1))
		srt.WriteString(fmt.Sprintf("%s --> %s\n",
			formatSRTTime(segment.Start),
			formatSRTTime(segment.End)))

		text := segment.Text
		if segment.SpeakerID != "" {
			text = fmt.Sprintf("%s: %s", segment.SpeakerID, text)
		}
		srt.WriteString(text)
		srt.WriteString("\n\n")
	}

	return []byte(srt.String()), nil
}

// formatSRTTime formats duration for SRT format
func formatSRTTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	milliseconds := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, milliseconds)
}
