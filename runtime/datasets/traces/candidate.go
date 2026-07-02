package traces

import (
	"encoding/json"

	"github.com/google/uuid"
)

type SourcePlatform string

const (
	SourceOTel       SourcePlatform = "otel"
	SourceBraintrust SourcePlatform = "braintrust"
	SourceLangSmith  SourcePlatform = "langsmith"
	SourcePhoenix    SourcePlatform = "phoenix"
	SourceAgentClash SourcePlatform = "agentclash"
)

type Candidate struct {
	SourceTraceID    string
	SourceRunID      *uuid.UUID
	SourceRunAgentID *uuid.UUID
	ExternalID       *string
	Input            json.RawMessage
	Output           json.RawMessage
	Expected         json.RawMessage
	Metadata         json.RawMessage
	Tags             []string
}

type ImportResult struct {
	Platform   SourcePlatform
	Candidates []Candidate
	Errors     []ParseError
}

type ParseError struct {
	Row     int
	Field   string
	Message string
}

func NormalizePlatform(raw string) (SourcePlatform, error) {
	switch SourcePlatform(raw) {
	case SourceOTel, SourceBraintrust, SourceLangSmith, SourcePhoenix, SourceAgentClash:
		return SourcePlatform(raw), nil
	default:
		return "", errUnsupportedPlatform(raw)
	}
}
