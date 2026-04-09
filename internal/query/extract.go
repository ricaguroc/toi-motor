package query

import "regexp"

// EntityHint represents a detected entity reference within a query string.
type EntityHint struct {
	Raw string // as found in query (e.g. "lote L-2024-001")
	Ref string // normalized canonical reference (e.g. "lot:L-2024-001")
}

// patterns is the ordered list of extraction rules applied by ExtractEntityHints.
// Priority is enforced by applying them in order and by the regex specificity.
var patterns = []struct {
	re     *regexp.Regexp
	mapFn  func(match string) EntityHint
}{
	{
		// 1. Explicit lot: or lote: prefix
		re: regexp.MustCompile(`(?i)(?:lote?):(\S+)`),
		mapFn: func(match string) EntityHint {
			// match is the full submatch group 0; we need group 1 inside the fn
			// handled via FindAllStringSubmatch below
			return EntityHint{}
		},
	},
	{
		// 2. L-YYYY-NNN format
		re: regexp.MustCompile(`(?i)\b(L-\d{4}-\d{3,})\b`),
		mapFn: func(match string) EntityHint {
			return EntityHint{Raw: match, Ref: "lot:" + match}
		},
	},
	{
		// 3. LP-NNNN format
		re: regexp.MustCompile(`(?i)\b(LP-\d{4,})\b`),
		mapFn: func(match string) EntityHint {
			return EntityHint{Raw: match, Ref: "lot:" + match}
		},
	},
	{
		// 4. PUMP-NN or EQ-NNN
		re: regexp.MustCompile(`(?i)\b((?:PUMP|EQ)-\d+)\b`),
		mapFn: func(match string) EntityHint {
			return EntityHint{Raw: match, Ref: "equipment:" + match}
		},
	},
	{
		// 5. TRK-NNN
		re: regexp.MustCompile(`(?i)\b(TRK-\d+)\b`),
		mapFn: func(match string) EntityHint {
			return EntityHint{Raw: match, Ref: "vehicle:" + match}
		},
	},
	{
		// 6. ORD-NNNNN or OC-NNN
		re: regexp.MustCompile(`(?i)\b((?:ORD|OC)-\d+)\b`),
		mapFn: func(match string) EntityHint {
			return EntityHint{Raw: match, Ref: "order:" + match}
		},
	},
	{
		// 7. P-NNN (e.g. P-091)
		re: regexp.MustCompile(`(?i)\b(P-\d{3,})\b`),
		mapFn: func(match string) EntityHint {
			return EntityHint{Raw: match, Ref: "equipment:" + match}
		},
	},
	{
		// 8. Email-like pattern
		re: regexp.MustCompile(`(?i)\b([a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,})\b`),
		mapFn: func(match string) EntityHint {
			return EntityHint{Raw: match, Ref: "user:" + match}
		},
	},
}

// explicitLotRe handles the explicit "lot:" / "lote:" prefix separately because
// the submatch group requires two captures (prefix + value).
var explicitLotRe = regexp.MustCompile(`(?i)(?:lote?):(\S+)`)

// ExtractEntityHints scans q for structured entity references and returns all
// matches. Patterns are applied in priority order; all distinct matches from all
// patterns are collected. Case-insensitive.
func ExtractEntityHints(q string) []EntityHint {
	var hints []EntityHint
	seen := make(map[string]struct{})

	add := func(h EntityHint) {
		if _, ok := seen[h.Ref]; !ok {
			seen[h.Ref] = struct{}{}
			hints = append(hints, h)
		}
	}

	// 1. Explicit lot: / lote: prefix (needs group 1 extraction)
	for _, m := range explicitLotRe.FindAllStringSubmatch(q, -1) {
		// m[0] = full match (e.g. "lote:L-2024-001"), m[1] = value
		add(EntityHint{Raw: m[0], Ref: "lot:" + m[1]})
	}

	// Remaining patterns: indices 1..7 in the patterns slice (0 was the placeholder)
	for _, p := range patterns[1:] {
		for _, m := range p.re.FindAllString(q, -1) {
			add(p.mapFn(m))
		}
	}

	return hints
}
