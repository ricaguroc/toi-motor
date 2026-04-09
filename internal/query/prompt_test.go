package query

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_ContextInjected(t *testing.T) {
	result := BuildSystemPrompt("ABC")

	const marker = "REGISTROS RECUPERADOS:\nABC"
	if !strings.Contains(result, marker) {
		t.Errorf("expected output to contain %q after REGISTROS RECUPERADOS:, got:\n%s", marker, result)
	}
}

func TestBuildSystemPrompt_AllSixRules(t *testing.T) {
	result := BuildSystemPrompt("")

	rules := []string{
		"1. Responde ÚNICAMENTE con información de los registros proporcionados. No inventes datos.",
		"2. Si los registros no contienen suficiente información para responder, dilo explícitamente.",
		"3. Cita los record_id relevantes en el campo records_cited.",
		"4. Responde en el mismo idioma de la pregunta del usuario.",
		`5. Evalúa tu confianza: "high" si los registros son claros y completos, "medium" si son parciales, "low" si son escasos o ambiguos.`,
		`6. Identifica "gaps": qué información faltaría para dar una respuesta completa.`,
	}

	for _, rule := range rules {
		if !strings.Contains(result, rule) {
			t.Errorf("expected output to contain rule:\n%q\nbut it was not found", rule)
		}
	}
}

func TestBuildSystemPrompt_JSONFormatInstruction(t *testing.T) {
	result := BuildSystemPrompt("")

	const jsonInstruction = "RESPONDE SIEMPRE EN ESTE FORMATO JSON EXACTO (sin markdown, sin texto extra):"
	if !strings.Contains(result, jsonInstruction) {
		t.Errorf("expected output to contain JSON format instruction %q", jsonInstruction)
	}

	// Verify the JSON structure fields are present.
	jsonFields := []string{
		`"answer"`,
		`"confidence"`,
		`"records_cited"`,
		`"gaps"`,
		`"suggested_followup"`,
	}
	for _, field := range jsonFields {
		if !strings.Contains(result, field) {
			t.Errorf("expected JSON format to include field %q", field)
		}
	}
}

func TestBuildSystemPrompt_PlaceholderFullyReplaced(t *testing.T) {
	result := BuildSystemPrompt("some context data")

	if strings.Contains(result, "{context}") {
		t.Error("expected {context} placeholder to be fully replaced, but it still appears in output")
	}
}
