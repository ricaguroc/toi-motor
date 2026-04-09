package indexing_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ricaguroc/toi-motor/internal/indexing"
	"github.com/ricaguroc/toi-motor/internal/record"
)

func makeRecord(recordType string) record.Record {
	return record.Record{
		ID:         uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001"),
		RecordID:   uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000002"),
		OccurredAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Source:     "test-source",
		RecordType: recordType,
		EntityRef:  ptr("entity-1"),
		ActorRef:   ptr("actor-1"),
		Title:      ptr("Test Title"),
		Tags:       []string{"test"},
	}
}

// TestChunk_ScanSingleChunk verifies scan records always produce exactly 1 chunk.
func TestChunk_ScanSingleChunk(t *testing.T) {
	r := makeRecord("scan")
	text := indexing.GenerateText(r)

	c := indexing.DefaultChunker{}
	chunks, err := c.Chunk(r, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for scan, got %d", len(chunks))
	}
}

// TestChunk_MovementSingleChunk verifies movement records always produce exactly 1 chunk.
func TestChunk_MovementSingleChunk(t *testing.T) {
	r := makeRecord("movement")
	r.Payload = map[string]any{"from": "A", "to": "B"}
	text := indexing.GenerateText(r)

	c := indexing.DefaultChunker{}
	chunks, err := c.Chunk(r, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for movement, got %d", len(chunks))
	}
}

// TestChunk_DocumentLargePayloadMultipleChunks verifies that a document with
// a ~2000-char payload produces at least 2 chunks.
func TestChunk_DocumentLargePayloadMultipleChunks(t *testing.T) {
	r := makeRecord("document")
	// Build a payload that will produce ~2000 chars in the rendered text.
	// Each entry is ~20 chars: "- keyXX: val...\n"
	bigPayload := make(map[string]any, 100)
	for i := 0; i < 100; i++ {
		key := "field" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		// ~16-char value to push total over 2000
		bigPayload[key] = "value-long-content"
	}
	r.Payload = bigPayload
	text := indexing.GenerateText(r)

	if len(text) < 2000 {
		t.Logf("text length: %d — may not be large enough for multi-chunk test", len(text))
	}

	c := indexing.DefaultChunker{}
	chunks, err := c.Chunk(r, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks for large document, got %d (text len: %d)", len(chunks), len(text))
	}
}

// TestChunk_OverlapBetweenConsecutiveChunks verifies that consecutive chunks share content.
func TestChunk_OverlapBetweenConsecutiveChunks(t *testing.T) {
	r := makeRecord("document")
	bigPayload := make(map[string]any, 100)
	for i := 0; i < 100; i++ {
		key := "field" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		bigPayload[key] = "value-long-content"
	}
	r.Payload = bigPayload
	text := indexing.GenerateText(r)

	c := indexing.DefaultChunker{}
	chunks, err := c.Chunk(r, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Skip("not enough chunks to test overlap")
	}

	// Strip the header prefix from chunk[1] to get the raw content portion.
	// Then verify that the end of chunk[0]'s content overlaps with chunk[1]'s content.
	found := false
	for i := 1; i < len(chunks); i++ {
		prev := chunks[i-1].Text
		curr := chunks[i].Text

		// Find overlap: look for a suffix of prev appearing in curr.
		// We check a 50-char window at the end of the previous chunk's content.
		checkLen := 50
		if len(prev) < checkLen {
			checkLen = len(prev) / 2
		}
		suffix := prev[len(prev)-checkLen:]
		if strings.Contains(curr, suffix) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no overlap found between consecutive chunks")
	}
}

// TestChunk_EveryChunkContainsHeader verifies the metadata header is in every chunk.
func TestChunk_EveryChunkContainsHeader(t *testing.T) {
	r := makeRecord("document")
	bigPayload := make(map[string]any, 100)
	for i := 0; i < 100; i++ {
		key := "field" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		bigPayload[key] = "value-long-content"
	}
	r.Payload = bigPayload
	text := indexing.GenerateText(r)

	c := indexing.DefaultChunker{}
	chunks, err := c.Chunk(r, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Skip("not enough chunks to test header presence in continuation chunks")
	}

	// The header contains the RECORD TYPE line — that's the fingerprint.
	headerLine := "[RECORD TYPE: document]"
	for i, ch := range chunks {
		if !strings.Contains(ch.Text, headerLine) {
			t.Errorf("chunk[%d] is missing metadata header", i)
		}
	}
}

// TestChunk_NoEmptyChunks verifies no chunk has empty Text.
func TestChunk_NoEmptyChunks(t *testing.T) {
	types := []string{"scan", "movement", "note", "ticket", "log", "document", "email"}

	c := indexing.DefaultChunker{}
	for _, rt := range types {
		r := makeRecord(rt)
		r.Payload = map[string]any{"key": "value"}
		text := indexing.GenerateText(r)
		chunks, err := c.Chunk(r, text)
		if err != nil {
			t.Fatalf("[%s] unexpected error: %v", rt, err)
		}
		for i, ch := range chunks {
			if strings.TrimSpace(ch.Text) == "" {
				t.Errorf("[%s] chunk[%d] has empty text", rt, i)
			}
		}
	}
}
