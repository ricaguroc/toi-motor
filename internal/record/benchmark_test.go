package record_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ricaguroc/toi-motor/internal/record"
)

func benchRecord(payloadKeys int) record.Record {
	payload := make(map[string]any, payloadKeys)
	for i := range payloadKeys {
		payload[fmt.Sprintf("field_%03d", i)] = fmt.Sprintf("value-%d", i)
	}
	return record.Record{
		ID:         uuid.New(),
		RecordID:   uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		OccurredAt: time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
		Source:     "benchmark-service",
		RecordType: "benchmark.event",
		Payload:    payload,
	}
}

func benchRecordNested(depth int) record.Record {
	var nest func(d int) map[string]any
	nest = func(d int) map[string]any {
		if d == 0 {
			return map[string]any{"leaf": "value"}
		}
		return map[string]any{
			"level": fmt.Sprintf("depth-%d", d),
			"child": nest(d - 1),
		}
	}
	r := benchRecord(2)
	r.Payload = nest(depth)
	return r
}

// --- ComputeChecksum ---

func BenchmarkComputeChecksum_Small(b *testing.B) {
	r := benchRecord(5)
	b.ResetTimer()
	for b.Loop() {
		record.ComputeChecksum(r)
	}
}

func BenchmarkComputeChecksum_Medium(b *testing.B) {
	r := benchRecord(50)
	b.ResetTimer()
	for b.Loop() {
		record.ComputeChecksum(r)
	}
}

func BenchmarkComputeChecksum_Large(b *testing.B) {
	r := benchRecord(500)
	b.ResetTimer()
	for b.Loop() {
		record.ComputeChecksum(r)
	}
}

func BenchmarkComputeChecksum_Nested(b *testing.B) {
	r := benchRecordNested(10)
	b.ResetTimer()
	for b.Loop() {
		record.ComputeChecksum(r)
	}
}

// --- ValidateIngestRequest ---

func benchIngestRequest(payloadKeys int) record.IngestRequest {
	now := time.Now().UTC()
	entity := "entity-bench"
	actor := "actor-bench"
	title := "Benchmark record title"
	payload := make(map[string]any, payloadKeys)
	for i := range payloadKeys {
		payload[fmt.Sprintf("field_%03d", i)] = fmt.Sprintf("value-%d", i)
	}
	tags := []string{"bench", "test", "performance"}
	refs := []string{"ref-001", "ref-002"}
	return record.IngestRequest{
		OccurredAt: &now,
		Source:     "benchmark-service",
		RecordType: "benchmark.event",
		EntityRef:  &entity,
		ActorRef:   &actor,
		Title:      &title,
		Payload:    payload,
		Tags:       tags,
		ObjectRefs: refs,
	}
}

func BenchmarkValidateIngestRequest_Minimal(b *testing.B) {
	req := record.IngestRequest{
		Source:     "bench",
		RecordType: "event",
	}
	b.ResetTimer()
	for b.Loop() {
		record.ValidateIngestRequest(req)
	}
}

func BenchmarkValidateIngestRequest_Full(b *testing.B) {
	req := benchIngestRequest(20)
	b.ResetTimer()
	for b.Loop() {
		record.ValidateIngestRequest(req)
	}
}

func BenchmarkValidateIngestRequest_LargePayload(b *testing.B) {
	req := benchIngestRequest(0)
	req.Payload = map[string]any{"data": strings.Repeat("x", 500_000)}
	b.ResetTimer()
	for b.Loop() {
		record.ValidateIngestRequest(req)
	}
}
