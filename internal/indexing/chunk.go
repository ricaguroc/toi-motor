package indexing

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ricaguroc/toi-motor/internal/record"
)

// Chunk is a single indexable unit derived from a Record.
type Chunk struct {
	RecordID   uuid.UUID
	ChunkIndex int
	Text       string
	EntityRef  *string
	ActorRef   *string
	RecordType string
	OccurredAt time.Time
}

// Chunker splits a record's text representation into indexable chunks.
type Chunker interface {
	Chunk(r record.Record, text string) ([]Chunk, error)
}

// chunkStrategy defines how a record type should be split.
type chunkStrategy int

const (
	strategyNoSplit       chunkStrategy = iota // always single chunk
	strategyParagraph                          // split at paragraph boundaries if > 512 tokens
	strategySlidingWindow                      // sliding window with configurable size + overlap
)

const tokensPerChar = 4 // 1 token ≈ 4 characters
const maxTokens = 512

type windowConfig struct {
	size    int // in tokens
	overlap int // in tokens
}

// DefaultChunker implements the strategy table from the spec.
type DefaultChunker struct{}

// Chunk splits text into chunks according to the strategy for r.RecordType.
// Every chunk's Text contains the metadata header prepended to the chunk content.
func (c DefaultChunker) Chunk(r record.Record, text string) ([]Chunk, error) {
	strategy, wcfg := strategyFor(r.RecordType)

	header := extractHeader(text)

	var segments []string
	switch strategy {
	case strategyNoSplit:
		segments = []string{text}
	case strategyParagraph:
		segments = splitParagraph(text, header, maxTokens)
	case strategySlidingWindow:
		segments = splitSlidingWindow(text, header, wcfg)
	}

	chunks := make([]Chunk, 0, len(segments))
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		chunks = append(chunks, Chunk{
			RecordID:   r.RecordID,
			ChunkIndex: i,
			Text:       seg,
			EntityRef:  r.EntityRef,
			ActorRef:   r.ActorRef,
			RecordType: r.RecordType,
			OccurredAt: r.OccurredAt,
		})
	}
	return chunks, nil
}

// strategyFor returns the chunking strategy and window config for a given record type.
func strategyFor(recordType string) (chunkStrategy, windowConfig) {
	switch recordType {
	case "scan", "movement", "photo":
		return strategyNoSplit, windowConfig{}
	case "note":
		return strategyParagraph, windowConfig{}
	case "ticket", "log":
		return strategySlidingWindow, windowConfig{size: maxTokens, overlap: 50}
	case "document", "email":
		return strategySlidingWindow, windowConfig{size: maxTokens, overlap: 100}
	default:
		return strategySlidingWindow, windowConfig{size: maxTokens, overlap: 50}
	}
}

// extractHeader returns everything in text up to (but not including) "DETAILS:" or "TAGS:".
// If neither marker is found, the entire text is used as header.
func extractHeader(text string) string {
	for _, marker := range []string{"DETAILS:", "TAGS:"} {
		if idx := strings.Index(text, marker); idx != -1 {
			return strings.TrimRight(text[:idx], "\n")
		}
	}
	return text
}

// charsFor converts a token count to an approximate character count.
func charsFor(tokens int) int { return tokens * tokensPerChar }

// tokensFor converts a character count to an approximate token count.
func tokensFor(chars int) int { return (chars + tokensPerChar - 1) / tokensPerChar }

// splitParagraph splits text at double-newline boundaries when the full text
// exceeds maxTokens. Header is prepended to every continuation chunk.
func splitParagraph(text, header string, maxTok int) []string {
	if tokensFor(len(text)) <= maxTok {
		return []string{text}
	}

	paragraphs := strings.Split(text, "\n\n")
	maxChars := charsFor(maxTok)

	var result []string
	current := ""
	for _, para := range paragraphs {
		candidate := current
		if candidate != "" {
			candidate += "\n\n"
		}
		candidate += para

		if len(candidate) > maxChars && current != "" {
			result = append(result, current)
			current = header + "\n\n" + para
		} else {
			current = candidate
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// splitSlidingWindow splits text using a sliding window of wcfg.size tokens
// with wcfg.overlap tokens of overlap between consecutive windows.
// Header is prepended to every chunk beyond the first.
func splitSlidingWindow(text, header string, wcfg windowConfig) []string {
	maxChars := charsFor(wcfg.size)
	overlapChars := charsFor(wcfg.overlap)

	if len(text) <= maxChars {
		return []string{text}
	}

	headerChars := len(header)
	// The payload space available per continuation chunk (header eats into the window).
	// For the first chunk we use the full window.
	firstWindow := maxChars
	contWindow := maxChars - headerChars - 2 // 2 for "\n\n"
	if contWindow <= 0 {
		// Header alone fills the window — fall back to full window per chunk.
		contWindow = maxChars
	}

	var result []string
	pos := 0

	for pos < len(text) {
		var end int
		if len(result) == 0 {
			end = pos + firstWindow
		} else {
			end = pos + contWindow
		}
		if end > len(text) {
			end = len(text)
		}

		var seg string
		if len(result) == 0 {
			seg = text[pos:end]
		} else {
			seg = header + "\n\n" + text[pos:end]
		}
		result = append(result, seg)

		if end >= len(text) {
			break
		}

		// Advance by (window - overlap); advance from the raw text position.
		var stepChars int
		if len(result) == 1 {
			stepChars = firstWindow - overlapChars
		} else {
			stepChars = contWindow - overlapChars
		}
		if stepChars <= 0 {
			stepChars = 1 // prevent infinite loop
		}
		pos += stepChars
	}

	return result
}
