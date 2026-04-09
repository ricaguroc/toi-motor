package indexing_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ricaguroc/toi-motor/internal/indexing"
	"github.com/ricaguroc/toi-motor/internal/record"
)

func benchRecord(payloadKeys int) record.Record {
	payload := make(map[string]any, payloadKeys)
	for i := range payloadKeys {
		payload[fmt.Sprintf("field_%03d", i)] = fmt.Sprintf("value-%d", i)
	}
	entity := "entity-bench"
	actor := "actor-bench"
	title := "Benchmark operational record"
	return record.Record{
		ID:         uuid.New(),
		RecordID:   uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		OccurredAt: time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
		Source:     "benchmark-service",
		RecordType: "document",
		EntityRef:  &entity,
		ActorRef:   &actor,
		Title:      &title,
		Payload:    payload,
		Tags:       []string{"bench", "perf", "test"},
	}
}

func benchRecordNested(depth int) record.Record {
	var nest func(d int) map[string]any
	nest = func(d int) map[string]any {
		if d == 0 {
			return map[string]any{"leaf": "value", "count": 42}
		}
		return map[string]any{
			"level": fmt.Sprintf("depth-%d", d),
			"data":  strings.Repeat("x", 50),
			"child": nest(d - 1),
		}
	}
	r := benchRecord(0)
	r.Payload = nest(depth)
	return r
}

// --- GenerateText ---

func BenchmarkGenerateText_Small(b *testing.B) {
	r := benchRecord(5)
	b.ResetTimer()
	for b.Loop() {
		indexing.GenerateText(r)
	}
}

func BenchmarkGenerateText_Medium(b *testing.B) {
	r := benchRecord(50)
	b.ResetTimer()
	for b.Loop() {
		indexing.GenerateText(r)
	}
}

func BenchmarkGenerateText_Large(b *testing.B) {
	r := benchRecord(500)
	b.ResetTimer()
	for b.Loop() {
		indexing.GenerateText(r)
	}
}

func BenchmarkGenerateText_NestedPayload(b *testing.B) {
	r := benchRecordNested(10)
	b.ResetTimer()
	for b.Loop() {
		indexing.GenerateText(r)
	}
}

// --- Chunk ---

func BenchmarkChunk_SingleChunk(b *testing.B) {
	r := benchRecord(5)
	r.RecordType = "scan" // strategyNoSplit
	text := indexing.GenerateText(r)
	chunker := indexing.DefaultChunker{}
	b.ResetTimer()
	for b.Loop() {
		chunker.Chunk(r, text)
	}
}

func BenchmarkChunk_SlidingWindow_Medium(b *testing.B) {
	r := benchRecord(100)
	r.RecordType = "document" // strategySlidingWindow
	text := indexing.GenerateText(r)
	chunker := indexing.DefaultChunker{}
	b.ResetTimer()
	for b.Loop() {
		chunker.Chunk(r, text)
	}
}

func BenchmarkChunk_SlidingWindow_Large(b *testing.B) {
	r := benchRecord(500)
	r.RecordType = "document"
	text := indexing.GenerateText(r)
	chunker := indexing.DefaultChunker{}
	b.ResetTimer()
	for b.Loop() {
		chunker.Chunk(r, text)
	}
}

func BenchmarkChunk_Paragraph(b *testing.B) {
	r := benchRecord(5)
	r.RecordType = "note" // strategyParagraph
	// Generate a long text with paragraph breaks
	var sb strings.Builder
	sb.WriteString(indexing.GenerateText(r))
	for range 20 {
		sb.WriteString("\n\n")
		sb.WriteString(strings.Repeat("This is a paragraph of operational notes. ", 10))
	}
	text := sb.String()
	chunker := indexing.DefaultChunker{}
	b.ResetTimer()
	for b.Loop() {
		chunker.Chunk(r, text)
	}
}

// --- Full pipeline (GenerateText + Chunk) ---

func BenchmarkTextAndChunk_Document100Keys(b *testing.B) {
	r := benchRecord(100)
	r.RecordType = "document"
	chunker := indexing.DefaultChunker{}
	b.ResetTimer()
	for b.Loop() {
		text := indexing.GenerateText(r)
		chunker.Chunk(r, text)
	}
}
