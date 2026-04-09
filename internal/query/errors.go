package query

import "errors"

var (
	// ErrValidation is returned when the QueryRequest fails validation.
	ErrValidation = errors.New("query validation failed")

	// ErrLLMUnavailable is returned when the language model call fails.
	ErrLLMUnavailable = errors.New("language model unavailable")

	// ErrLLMParseError is returned when the language model response cannot be parsed.
	ErrLLMParseError = errors.New("language model returned invalid response")
)
