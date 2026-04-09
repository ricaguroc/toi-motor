package indexing

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ricaguroc/toi-motor/internal/record"
)

// GenerateText converts a Record into a structured text for embedding.
// The format is deterministic: payload keys are sorted alphabetically,
// nested maps are flattened using dot notation, and arrays are joined with ", ".
func GenerateText(r record.Record) string {
	var sb strings.Builder

	// Header line
	sb.WriteString(fmt.Sprintf("[RECORD TYPE: %s] [SOURCE: %s] [OCCURRED: %s]\n",
		r.RecordType,
		r.Source,
		r.OccurredAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	))

	// Entity
	entityRef := "N/A"
	if r.EntityRef != nil {
		entityRef = *r.EntityRef
	}
	sb.WriteString(fmt.Sprintf("ENTITY: %s\n", entityRef))

	// Actor
	actorRef := "N/A"
	if r.ActorRef != nil {
		actorRef = *r.ActorRef
	}
	sb.WriteString(fmt.Sprintf("ACTOR: %s\n", actorRef))

	// Blank line before TITLE
	sb.WriteString("\n")

	// Title
	title := ""
	if r.Title != nil {
		title = *r.Title
	}
	sb.WriteString(fmt.Sprintf("TITLE: %s\n", title))

	// Payload
	if len(r.Payload) > 0 {
		sb.WriteString("\nDETAILS:\n")
		flat := flattenMap(r.Payload, "")
		keys := make([]string, 0, len(flat))
		for k := range flat {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", k, flat[k]))
		}
	}

	// Tags
	sb.WriteString(fmt.Sprintf("\nTAGS: %s", strings.Join(r.Tags, ", ")))

	return sb.String()
}

// flattenMap recursively flattens a nested map[string]any into a flat map
// using dot notation for nested keys. Arrays are joined with ", ".
func flattenMap(m map[string]any, prefix string) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			for fk, fv := range flattenMap(val, key) {
				result[fk] = fv
			}
		case []any:
			parts := make([]string, 0, len(val))
			for _, item := range val {
				parts = append(parts, fmt.Sprintf("%v", item))
			}
			result[key] = strings.Join(parts, ", ")
		default:
			result[key] = fmt.Sprintf("%v", v)
		}
	}
	return result
}
