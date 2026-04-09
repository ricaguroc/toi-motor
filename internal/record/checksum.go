package record

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ComputeChecksum produces a deterministic SHA-256 hex digest for r.
//
// The pre-image is built from the following fields, joined with "|":
//
//	record_id | occurred_at (RFC3339Nano, UTC) | source | record_type | payload (sorted JSON)
//
// A nil or empty Payload is treated as "{}" so the checksum is stable
// regardless of whether the caller passes nil or an empty map.
func ComputeChecksum(r Record) (string, error) {
	payloadJSON, err := marshalSorted(r.Payload)
	if err != nil {
		return "", fmt.Errorf("record: marshal payload for checksum: %w", err)
	}

	pre := strings.Join([]string{
		r.RecordID.String(),
		r.OccurredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), // RFC3339Nano
		r.Source,
		r.RecordType,
		string(payloadJSON),
	}, "|")

	sum := sha256.Sum256([]byte(pre))
	return hex.EncodeToString(sum[:]), nil
}

// marshalSorted encodes v as JSON with map keys sorted at every level so the
// output is deterministic regardless of Go's map-iteration order.
func marshalSorted(v map[string]any) ([]byte, error) {
	if len(v) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(sortedMap(v))
}

// sortedMap is a type alias used to implement deterministic JSON marshalling.
type sortedMap map[string]any

func (m sortedMap) MarshalJSON() ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		keyBytes, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		sb.Write(keyBytes)
		sb.WriteByte(':')

		val := m[k]
		var valBytes []byte
		var err2 error
		if nested, ok := val.(map[string]any); ok {
			valBytes, err2 = sortedMap(nested).MarshalJSON()
		} else {
			valBytes, err2 = json.Marshal(val)
		}
		if err2 != nil {
			return nil, err2
		}
		sb.Write(valBytes)
	}
	sb.WriteByte('}')
	return []byte(sb.String()), nil
}
