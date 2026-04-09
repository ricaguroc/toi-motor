package query

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- ExtractEntityHints ---

func BenchmarkExtractEntityHints_NoMatch(b *testing.B) {
	q := "show me all records from last week"
	b.ResetTimer()
	for b.Loop() {
		ExtractEntityHints(q)
	}
}

func BenchmarkExtractEntityHints_SingleMatch(b *testing.B) {
	q := "what happened with lot LP-4821"
	b.ResetTimer()
	for b.Loop() {
		ExtractEntityHints(q)
	}
}

func BenchmarkExtractEntityHints_MultiMatch(b *testing.B) {
	q := "lote L-2024-001 PUMP-03 TRK-007 maria@empresa.com ORD-55123 EQ-42 P-091 LP-9999"
	b.ResetTimer()
	for b.Loop() {
		ExtractEntityHints(q)
	}
}

func BenchmarkExtractEntityHints_LongQuery(b *testing.B) {
	// Simulate a verbose natural language query with entities scattered in it
	q := strings.Repeat("check the status of lot LP-4821 and equipment PUMP-03 in zone A. ", 20)
	b.ResetTimer()
	for b.Loop() {
		ExtractEntityHints(q)
	}
}

// --- AssembleContext ---

func benchChunks(n int, textLen int) []RetrievedChunk {
	chunks := make([]RetrievedChunk, n)
	entity := "entity-bench"
	for i := range n {
		chunks[i] = RetrievedChunk{
			RecordID:   uuid.New(),
			ChunkIndex: 0,
			ChunkText:  strings.Repeat("x", textLen),
			Score:      float32(n - i),
			OccurredAt: time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
			RecordType: "document",
			EntityRef:  &entity,
		}
	}
	return chunks
}

func BenchmarkAssembleContext_10Chunks(b *testing.B) {
	chunks := benchChunks(10, 500)
	b.ResetTimer()
	for b.Loop() {
		AssembleContext(chunks)
	}
}

func BenchmarkAssembleContext_50Chunks(b *testing.B) {
	chunks := benchChunks(50, 500)
	b.ResetTimer()
	for b.Loop() {
		AssembleContext(chunks)
	}
}

func BenchmarkAssembleContext_100Chunks_Truncation(b *testing.B) {
	// 100 chunks x 500 chars = 50KB, will trigger truncation at 32K
	chunks := benchChunks(100, 500)
	b.ResetTimer()
	for b.Loop() {
		AssembleContext(chunks)
	}
}

func BenchmarkAssembleContext_DuplicateRecords(b *testing.B) {
	// 50 chunks but only 10 unique RecordIDs — tests dedup path
	chunks := make([]RetrievedChunk, 50)
	ids := make([]uuid.UUID, 10)
	for i := range 10 {
		ids[i] = uuid.New()
	}
	entity := "entity-bench"
	for i := range 50 {
		chunks[i] = RetrievedChunk{
			RecordID:   ids[i%10],
			ChunkIndex: i / 10,
			ChunkText:  fmt.Sprintf("chunk text content %d with some operational data", i),
			Score:      float32(50 - i),
			OccurredAt: time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
			RecordType: "document",
			EntityRef:  &entity,
		}
	}
	b.ResetTimer()
	for b.Loop() {
		AssembleContext(chunks)
	}
}

// --- BuildSystemPrompt ---

func BenchmarkBuildSystemPrompt_SmallContext(b *testing.B) {
	ctx := strings.Repeat("record data line\n", 10)
	b.ResetTimer()
	for b.Loop() {
		BuildSystemPrompt(ctx)
	}
}

func BenchmarkBuildSystemPrompt_MaxContext(b *testing.B) {
	ctx := strings.Repeat("x", 32_000)
	b.ResetTimer()
	for b.Loop() {
		BuildSystemPrompt(ctx)
	}
}
