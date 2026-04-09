package query

import "time"

// QueryRequest is the input for a natural language query over operational records.
type QueryRequest struct {
	Q            string     `json:"q"`
	Format       string     `json:"format"`        // "conversational" or "structured"
	EntityScope  *string    `json:"entity_scope"`
	DateFrom     *time.Time `json:"date_from"`
	DateTo       *time.Time `json:"date_to"`
	LimitRecords int        `json:"limit_records"`
}

// QueryResponse is the structured answer returned by QueryService.
type QueryResponse struct {
	Answer            string   `json:"answer"`
	Confidence        string   `json:"confidence"`
	RecordsCited      []string `json:"records_cited"`
	Gaps              *string  `json:"gaps"`
	SuggestedFollowup []string `json:"suggested_followup"`
	RetrievedCount    int      `json:"retrieved_count"`
	QueryMs           int64    `json:"query_ms"`
}
