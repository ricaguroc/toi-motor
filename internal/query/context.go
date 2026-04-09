package query

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const maxContextBytes = 32_000

// AssembleContext deduplicates chunks by RecordID (keeping the highest-scoring
// chunk per record), sorts by score descending, formats each chunk with a
// structured header, joins them with double newlines, and truncates the result
// to ~32 000 characters (≈8 000 tokens) by dropping the lowest-scoring chunks.
func AssembleContext(chunks []RetrievedChunk) string {
	if len(chunks) == 0 {
		return ""
	}

	// 1. Deduplicate — keep highest-scoring chunk per RecordID.
	best := make(map[uuid.UUID]RetrievedChunk, len(chunks))
	for _, c := range chunks {
		if existing, ok := best[c.RecordID]; !ok || c.Score > existing.Score {
			best[c.RecordID] = c
		}
	}

	// 2. Sort by score descending.
	unique := make([]RetrievedChunk, 0, len(best))
	for _, c := range best {
		unique = append(unique, c)
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].Score > unique[j].Score
	})

	// 3 & 4. Format and join, truncating by dropping tail chunks when needed.
	const sep = "\n\n"
	var sb strings.Builder

	for i, c := range unique {
		block := formatChunk(i+1, c)
		candidate := block
		if sb.Len() > 0 {
			candidate = sep + block
		}
		// 5. Truncate: if adding this block would exceed the limit, stop.
		if sb.Len()+len(candidate) > maxContextBytes {
			break
		}
		sb.WriteString(candidate)
	}

	return sb.String()
}

func formatChunk(i int, c RetrievedChunk) string {
	return fmt.Sprintf(
		"--- RECORD %d (record_id: %s, type: %s, occurred: %s) ---\n%s\n---",
		i,
		c.RecordID.String(),
		c.RecordType,
		c.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"),
		c.ChunkText,
	)
}
