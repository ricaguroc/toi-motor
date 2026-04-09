package indexing_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ricaguroc/toi-motor/internal/indexing"
	"github.com/ricaguroc/toi-motor/internal/record"
)

func ptr(s string) *string { return &s }

var fixedTime = time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)

func baseRecord() record.Record {
	return record.Record{
		ID:         uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		RecordID:   uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		OccurredAt: fixedTime,
		Source:     "warehouse-system",
		RecordType: "movement",
		EntityRef:  ptr("pallet-42"),
		ActorRef:   ptr("operator-7"),
		Title:      ptr("Pallet moved to zone B"),
		Payload:    map[string]any{"from": "zone-A", "to": "zone-B"},
		Tags:       []string{"logistics", "pallet"},
	}
}

// TestGenerateText_MovementGolden verifies the full output for a movement record.
func TestGenerateText_MovementGolden(t *testing.T) {
	r := baseRecord()
	got := indexing.GenerateText(r)

	expected := "[RECORD TYPE: movement] [SOURCE: warehouse-system] [OCCURRED: 2024-03-15T10:30:00Z]\n" +
		"ENTITY: pallet-42\n" +
		"ACTOR: operator-7\n" +
		"\n" +
		"TITLE: Pallet moved to zone B\n" +
		"\n" +
		"DETAILS:\n" +
		"- from: zone-A\n" +
		"- to: zone-B\n" +
		"\n" +
		"TAGS: logistics, pallet"

	if got != expected {
		t.Errorf("golden mismatch.\nwant:\n%s\n\ngot:\n%s", expected, got)
	}
}

// TestGenerateText_NilEntityRef verifies that nil EntityRef becomes "N/A".
func TestGenerateText_NilEntityRef(t *testing.T) {
	r := baseRecord()
	r.EntityRef = nil

	got := indexing.GenerateText(r)
	if !contains(got, "ENTITY: N/A") {
		t.Errorf("expected 'ENTITY: N/A' in output, got:\n%s", got)
	}
}

// TestGenerateText_NilActorRef verifies that nil ActorRef becomes "N/A".
func TestGenerateText_NilActorRef(t *testing.T) {
	r := baseRecord()
	r.ActorRef = nil

	got := indexing.GenerateText(r)
	if !contains(got, "ACTOR: N/A") {
		t.Errorf("expected 'ACTOR: N/A' in output, got:\n%s", got)
	}
}

// TestGenerateText_NestedPayload verifies dot notation for nested maps.
func TestGenerateText_NestedPayload(t *testing.T) {
	r := baseRecord()
	r.Payload = map[string]any{
		"location": map[string]any{
			"zone": "B",
			"rack": "R3",
		},
	}

	got := indexing.GenerateText(r)
	if !contains(got, "- location.rack: R3") {
		t.Errorf("expected 'location.rack: R3' in output, got:\n%s", got)
	}
	if !contains(got, "- location.zone: B") {
		t.Errorf("expected 'location.zone: B' in output, got:\n%s", got)
	}
}

// TestGenerateText_EmptyPayload verifies that an empty payload produces no DETAILS section.
func TestGenerateText_EmptyPayload(t *testing.T) {
	r := baseRecord()
	r.Payload = nil

	got := indexing.GenerateText(r)
	if contains(got, "DETAILS:") {
		t.Errorf("expected no DETAILS section for empty payload, got:\n%s", got)
	}
}

// TestGenerateText_ArrayPayload verifies that array values are comma-joined.
func TestGenerateText_ArrayPayload(t *testing.T) {
	r := baseRecord()
	r.Payload = map[string]any{
		"items": []any{"box-1", "box-2", "box-3"},
	}

	got := indexing.GenerateText(r)
	if !contains(got, "- items: box-1, box-2, box-3") {
		t.Errorf("expected '- items: box-1, box-2, box-3' in output, got:\n%s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
