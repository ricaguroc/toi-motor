package record_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ricaguroc/toi-motor/internal/record"
)

// baseRecord returns a fully populated Record so individual tests can mutate
// one field at a time and verify the checksum changes.
func baseRecord(t *testing.T) record.Record {
	t.Helper()
	return record.Record{
		ID:         uuid.New(),
		RecordID:   uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		OccurredAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Source:     "test-service",
		RecordType: "user.created",
		Payload:    map[string]any{"name": "Alice", "role": "admin"},
	}
}

func TestComputeChecksum_Deterministic(t *testing.T) {
	r := baseRecord(t)

	c1, err := record.ComputeChecksum(r)
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}

	c2, err := record.ComputeChecksum(r)
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}

	if c1 != c2 {
		t.Errorf("checksum is not deterministic: %q != %q", c1, c2)
	}
}

func TestComputeChecksum_SamePayloadDifferentID(t *testing.T) {
	r1 := baseRecord(t)
	r2 := baseRecord(t)
	r2.RecordID = uuid.MustParse("22222222-2222-2222-2222-222222222222")

	c1, err := record.ComputeChecksum(r1)
	if err != nil {
		t.Fatalf("r1: %v", err)
	}
	c2, err := record.ComputeChecksum(r2)
	if err != nil {
		t.Fatalf("r2: %v", err)
	}

	if c1 == c2 {
		t.Errorf("different record_id must produce different checksum, both got %q", c1)
	}
}

func TestComputeChecksum_ChangingFieldChangesChecksum(t *testing.T) {
	base := baseRecord(t)

	baseCS, err := record.ComputeChecksum(base)
	if err != nil {
		t.Fatalf("base: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(r record.Record) record.Record
	}{
		{
			name: "changed_source",
			mutate: func(r record.Record) record.Record {
				r.Source = "other-service"
				return r
			},
		},
		{
			name: "changed_record_type",
			mutate: func(r record.Record) record.Record {
				r.RecordType = "user.deleted"
				return r
			},
		},
		{
			name: "changed_occurred_at",
			mutate: func(r record.Record) record.Record {
				r.OccurredAt = r.OccurredAt.Add(time.Second)
				return r
			},
		},
		{
			name: "changed_payload_value",
			mutate: func(r record.Record) record.Record {
				r.Payload = map[string]any{"name": "Bob", "role": "admin"}
				return r
			},
		},
		{
			name: "changed_payload_key",
			mutate: func(r record.Record) record.Record {
				r.Payload = map[string]any{"username": "Alice", "role": "admin"}
				return r
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutated := tt.mutate(base)
			cs, err := record.ComputeChecksum(mutated)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cs == baseCS {
				t.Errorf("mutating %q did not change the checksum (still %q)", tt.name, cs)
			}
		})
	}
}

func TestComputeChecksum_PayloadKeyOrderIsIrrelevant(t *testing.T) {
	// Two records with the same logical payload but different insertion order
	// must produce the same checksum (sorted JSON).
	r1 := baseRecord(t)
	r1.Payload = map[string]any{"a": 1, "b": 2, "c": 3}

	r2 := baseRecord(t)
	r2.Payload = map[string]any{"c": 3, "a": 1, "b": 2}

	c1, err := record.ComputeChecksum(r1)
	if err != nil {
		t.Fatalf("r1: %v", err)
	}
	c2, err := record.ComputeChecksum(r2)
	if err != nil {
		t.Fatalf("r2: %v", err)
	}

	if c1 != c2 {
		t.Errorf("payload key order must not affect checksum: %q != %q", c1, c2)
	}
}

func TestComputeChecksum_NilAndEmptyPayloadAreEquivalent(t *testing.T) {
	r1 := baseRecord(t)
	r1.Payload = nil

	r2 := baseRecord(t)
	r2.Payload = map[string]any{}

	c1, err := record.ComputeChecksum(r1)
	if err != nil {
		t.Fatalf("nil payload: %v", err)
	}
	c2, err := record.ComputeChecksum(r2)
	if err != nil {
		t.Fatalf("empty payload: %v", err)
	}

	if c1 != c2 {
		t.Errorf("nil and empty payload must produce the same checksum: %q != %q", c1, c2)
	}
}

func TestComputeChecksum_TimezoneNormalisedToUTC(t *testing.T) {
	r1 := baseRecord(t)
	r1.OccurredAt = time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	r2 := baseRecord(t)
	// Same instant expressed in UTC+3.
	loc := time.FixedZone("UTC+3", 3*60*60)
	r2.OccurredAt = time.Date(2024, 1, 15, 13, 30, 0, 0, loc)

	c1, err := record.ComputeChecksum(r1)
	if err != nil {
		t.Fatalf("r1: %v", err)
	}
	c2, err := record.ComputeChecksum(r2)
	if err != nil {
		t.Fatalf("r2: %v", err)
	}

	if c1 != c2 {
		t.Errorf("same instant in different timezone must produce same checksum: %q != %q", c1, c2)
	}
}

func TestComputeChecksum_ResultIsHex64Chars(t *testing.T) {
	r := baseRecord(t)
	cs, err := record.ComputeChecksum(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs) != 64 {
		t.Errorf("expected 64-char hex string (SHA-256), got len=%d: %q", len(cs), cs)
	}
}
