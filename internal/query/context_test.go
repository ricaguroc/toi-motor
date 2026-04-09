package query

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAssembleContext_Empty(t *testing.T) {
	t.Parallel()
	if got := AssembleContext(nil); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
	if got := AssembleContext([]RetrievedChunk{}); got != "" {
		t.Errorf("expected empty string for empty slice, got %q", got)
	}
}

func TestAssembleContext_DeduplicatesByRecordID(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	ts := time.Now().UTC()

	chunks := []RetrievedChunk{
		{RecordID: id, ChunkIndex: 0, ChunkText: "low score chunk", Score: 0.5, OccurredAt: ts, RecordType: "event"},
		{RecordID: id, ChunkIndex: 1, ChunkText: "high score chunk", Score: 0.9, OccurredAt: ts, RecordType: "event"},
	}

	result := AssembleContext(chunks)

	// Only one RECORD header should appear.
	count := strings.Count(result, "--- RECORD")
	if count != 1 {
		t.Errorf("expected 1 RECORD block, got %d", count)
	}

	// Should contain the high-score chunk text.
	if !strings.Contains(result, "high score chunk") {
		t.Errorf("expected high-score chunk text in output, got: %s", result)
	}
	if strings.Contains(result, "low score chunk") {
		t.Errorf("low-score chunk should have been deduplicated, but it appears in output")
	}
}

func TestAssembleContext_HeaderFormat(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ts, _ := time.Parse(time.RFC3339, "2024-03-15T10:00:00Z")

	chunks := []RetrievedChunk{
		{RecordID: id, ChunkIndex: 0, ChunkText: "some text", Score: 1.0, OccurredAt: ts, RecordType: "inspection"},
	}

	result := AssembleContext(chunks)

	wantHeader := "--- RECORD 1 (record_id: 00000000-0000-0000-0000-000000000001, type: inspection, occurred: 2024-03-15T10:00:00Z) ---"
	if !strings.Contains(result, wantHeader) {
		t.Errorf("header format mismatch.\nwant substring: %q\ngot: %q", wantHeader, result)
	}
	if !strings.Contains(result, "some text") {
		t.Errorf("chunk text missing from output")
	}
}

func TestAssembleContext_TruncatesAt32000Chars(t *testing.T) {
	t.Parallel()

	// Generate 100 chunks with distinct RecordIDs and ~400 byte text each.
	// 100 * 400 = 40 000 bytes — well above the 32 000 limit.
	chunks := make([]RetrievedChunk, 100)
	ts := time.Now().UTC()
	for i := range chunks {
		chunks[i] = RetrievedChunk{
			RecordID:   uuid.New(),
			ChunkIndex: 0,
			ChunkText:  strings.Repeat("x", 400),
			Score:      float32(100 - i), // descending so order is deterministic
			OccurredAt: ts,
			RecordType: "event",
		}
	}

	result := AssembleContext(chunks)
	if len(result) > maxContextBytes {
		t.Errorf("output length %d exceeds max %d", len(result), maxContextBytes)
	}
}

func TestAssembleContext_SortsByScoreDescending(t *testing.T) {
	t.Parallel()

	ts := time.Now().UTC()
	idA := uuid.New()
	idB := uuid.New()

	chunks := []RetrievedChunk{
		{RecordID: idA, ChunkText: "second", Score: 0.5, OccurredAt: ts, RecordType: "t"},
		{RecordID: idB, ChunkText: "first", Score: 0.9, OccurredAt: ts, RecordType: "t"},
	}

	result := AssembleContext(chunks)

	posFirst := strings.Index(result, "first")
	posSecond := strings.Index(result, "second")

	if posFirst == -1 || posSecond == -1 {
		t.Fatalf("expected both texts in output, got: %s", result)
	}
	if posFirst >= posSecond {
		t.Errorf("expected 'first' (score 0.9) to appear before 'second' (score 0.5)")
	}
}
