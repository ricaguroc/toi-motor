package query

import (
	"testing"
)

func TestExtractEntityHints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		query    string
		wantRefs []string // expected Ref values, in any order
	}{
		{
			name:     "explicit lote prefix",
			query:    "lote L-2024-001",
			wantRefs: []string{"lot:L-2024-001"},
		},
		{
			name:     "LP lot format",
			query:    "LP-4821",
			wantRefs: []string{"lot:LP-4821"},
		},
		{
			name:     "PUMP equipment",
			query:    "PUMP-01",
			wantRefs: []string{"equipment:PUMP-01"},
		},
		{
			name:     "P-NNN equipment",
			query:    "P-091",
			wantRefs: []string{"equipment:P-091"},
		},
		{
			name:     "TRK vehicle",
			query:    "TRK-123",
			wantRefs: []string{"vehicle:TRK-123"},
		},
		{
			name:     "ORD order",
			query:    "ORD-98765",
			wantRefs: []string{"order:ORD-98765"},
		},
		{
			name:     "email user",
			query:    "maria@empresa.com",
			wantRefs: []string{"user:maria@empresa.com"},
		},
		{
			name:     "no entities",
			query:    "no entities here",
			wantRefs: []string{},
		},
		{
			name:     "multiple entities in one query",
			query:    "lote L-2024-001 PUMP-03 TRK-007 maria@empresa.com",
			wantRefs: []string{"lot:L-2024-001", "equipment:PUMP-03", "vehicle:TRK-007", "user:maria@empresa.com"},
		},
		{
			name:     "EQ equipment",
			query:    "equipo EQ-999 tiene falla",
			wantRefs: []string{"equipment:EQ-999"},
		},
		{
			name:     "OC order",
			query:    "OC-123",
			wantRefs: []string{"order:OC-123"},
		},
		{
			name:     "explicit lot: colon prefix",
			query:    "lot:ABC-001",
			wantRefs: []string{"lot:ABC-001"},
		},
		{
			name:     "case insensitive LP",
			query:    "lp-4821",
			wantRefs: []string{"lot:lp-4821"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractEntityHints(tc.query)

			if len(tc.wantRefs) == 0 {
				if len(got) != 0 {
					t.Errorf("expected empty, got %v", got)
				}
				return
			}

			refSet := make(map[string]struct{}, len(got))
			for _, h := range got {
				refSet[h.Ref] = struct{}{}
			}

			for _, want := range tc.wantRefs {
				if _, ok := refSet[want]; !ok {
					t.Errorf("expected ref %q not found in results %v", want, got)
				}
			}

			if len(got) < len(tc.wantRefs) {
				t.Errorf("got %d hints, want at least %d", len(got), len(tc.wantRefs))
			}
		})
	}
}

func TestExtractEntityHints_NoDuplicates(t *testing.T) {
	t.Parallel()
	// If the same entity appears multiple times, deduplicate by Ref
	got := ExtractEntityHints("TRK-001 and also TRK-001")
	count := 0
	for _, h := range got {
		if h.Ref == "vehicle:TRK-001" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 occurrence of vehicle:TRK-001, got %d", count)
	}
}
