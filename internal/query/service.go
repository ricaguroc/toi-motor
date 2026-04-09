package query

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// QueryService orchestrates entity extraction, retrieval, context assembly, and LLM completion.
type QueryService struct {
	retriever Retriever
	llm       LanguageModel
}

// NewQueryService constructs a QueryService wired to the given retriever and LLM.
func NewQueryService(retriever Retriever, llm LanguageModel) *QueryService {
	return &QueryService{retriever: retriever, llm: llm}
}

// Query executes a natural language query over ingested operational records.
func (s *QueryService) Query(ctx context.Context, req QueryRequest) (QueryResponse, error) {
	start := time.Now()

	// 1. Validate: q required, 1–2000 chars.
	if len(req.Q) == 0 || len(req.Q) > 2000 {
		return QueryResponse{}, fmt.Errorf("%w: q must be between 1 and 2000 characters", ErrValidation)
	}

	// 2. Default format.
	if req.Format == "" {
		req.Format = "conversational"
	}

	// 3. Default limit; cap at 20.
	if req.LimitRecords <= 0 {
		req.LimitRecords = 10
	}
	if req.LimitRecords > 20 {
		req.LimitRecords = 20
	}

	// 4. Determine entity filter.
	filter := RetrievalFilter{
		From:  req.DateFrom,
		To:    req.DateTo,
		Limit: req.LimitRecords,
	}

	if req.EntityScope != nil {
		filter.EntityRef = req.EntityScope
	} else {
		hints := ExtractEntityHints(req.Q)
		if len(hints) > 0 {
			ref := hints[0].Ref
			filter.EntityRef = &ref
		}
	}

	// 5. Retrieve chunks.
	chunks, err := s.retriever.Retrieve(ctx, req.Q, filter)
	if err != nil {
		return QueryResponse{}, fmt.Errorf("query/service: retrieve: %w", err)
	}

	// 6. If no chunks retrieved → return low confidence directly without calling LLM.
	if len(chunks) == 0 {
		gaps := "No hay registros que coincidan con la consulta."
		return QueryResponse{
			Answer:         "No se encontraron registros relevantes para esta consulta.",
			Confidence:     "low",
			RecordsCited:   []string{},
			Gaps:           &gaps,
			RetrievedCount: 0,
			QueryMs:        time.Since(start).Milliseconds(),
		}, nil
	}

	// 7. Assemble context.
	assembledCtx := AssembleContext(chunks)

	// 8. Build system prompt.
	systemPrompt := BuildSystemPrompt(assembledCtx)

	// 9. Call LLM.
	completionResp, err := s.llm.Complete(ctx, CompletionRequest{
		SystemPrompt: systemPrompt,
		UserMessage:  req.Q,
	})
	if err != nil {
		return QueryResponse{}, fmt.Errorf("%w: %v", ErrLLMUnavailable, err)
	}

	// 10. Parse LLM response JSON.
	parsed, err := parseLLMResponse(completionResp.RawContent)
	if err != nil {
		return QueryResponse{}, fmt.Errorf("%w: %v", ErrLLMParseError, err)
	}

	// 11. Build final response with timing.
	return QueryResponse{
		Answer:            parsed.Answer,
		Confidence:        parsed.Confidence,
		RecordsCited:      parsed.RecordsCited,
		Gaps:              parsed.Gaps,
		SuggestedFollowup: parsed.SuggestedFollowup,
		RetrievedCount:    len(chunks),
		QueryMs:           time.Since(start).Milliseconds(),
	}, nil
}

// llmResponse mirrors the JSON structure the LLM is prompted to return.
type llmResponse struct {
	Answer            string   `json:"answer"`
	Confidence        string   `json:"confidence"`
	RecordsCited      []string `json:"records_cited"`
	Gaps              *string  `json:"gaps"`
	SuggestedFollowup []string `json:"suggested_followup"`
}

// parseLLMResponse unmarshals the raw LLM output and validates the confidence field.
func parseLLMResponse(raw string) (llmResponse, error) {
	var resp llmResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return llmResponse{}, fmt.Errorf("unmarshal: %w", err)
	}

	switch resp.Confidence {
	case "high", "medium", "low":
		// valid
	default:
		return llmResponse{}, fmt.Errorf("unexpected confidence value %q", resp.Confidence)
	}

	return resp, nil
}
