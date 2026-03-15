package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type StreamingClient interface {
	StreamModel(ctx context.Context, request Request, onDelta func(StreamDelta) error) (Response, error)
}

type Timing struct {
	StartedAt    time.Time
	FirstTokenAt time.Time
	CompletedAt  time.Time
	TTFT         time.Duration
	TotalLatency time.Duration
}

type StreamDeltaKind string

const (
	StreamDeltaKindText     StreamDeltaKind = "text"
	StreamDeltaKindToolCall StreamDeltaKind = "tool_call"
	StreamDeltaKindTerminal StreamDeltaKind = "terminal"

	maxToolCallIndex = 128
)

type StreamDelta struct {
	Kind      StreamDeltaKind
	Timestamp time.Time
	Text      string
	ToolCall  ToolCallFragment
	Terminal  StreamTerminal
}

type ToolCallFragment struct {
	Index             int
	IDFragment        string
	NameFragment      string
	ArgumentsFragment string
}

type StreamTerminal struct {
	ProviderModelID string
	FinishReason    string
	Usage           *Usage
	RawResponse     json.RawMessage
}

type StreamAccumulator struct {
	providerKey string
	startedAt   time.Time
	streamed    bool

	outputText      string
	toolCalls       []toolCallAccumulator
	providerModelID string
	finishReason    string
	usage           Usage
	usageKnown      bool
	rawResponse     json.RawMessage
	firstTokenAt    time.Time
}

type toolCallAccumulator struct {
	id        string
	name      string
	arguments string
}

func NewStreamAccumulator(providerKey string, startedAt time.Time) *StreamAccumulator {
	return &StreamAccumulator{
		providerKey: providerKey,
		startedAt:   startedAt.UTC(),
	}
}

func (a *StreamAccumulator) Consume(delta StreamDelta) error {
	if delta.Timestamp.IsZero() {
		return errors.New("stream delta timestamp is required")
	}

	switch delta.Kind {
	case StreamDeltaKindText:
		a.streamed = true
		a.markFirstToken(delta.Timestamp, delta.Text != "")
		a.outputText += delta.Text
	case StreamDeltaKindToolCall:
		a.streamed = true
		fragment := delta.ToolCall
		if fragment.Index < 0 {
			return errors.New("tool call fragment index must be greater than or equal to zero")
		}
		if fragment.Index >= maxToolCallIndex {
			return fmt.Errorf("tool call fragment index %d exceeds maximum %d", fragment.Index, maxToolCallIndex)
		}
		a.markFirstToken(delta.Timestamp, fragment.IDFragment != "" || fragment.NameFragment != "" || fragment.ArgumentsFragment != "")
		for len(a.toolCalls) <= fragment.Index {
			a.toolCalls = append(a.toolCalls, toolCallAccumulator{})
		}
		target := &a.toolCalls[fragment.Index]
		target.id += fragment.IDFragment
		target.name += fragment.NameFragment
		target.arguments += fragment.ArgumentsFragment
	case StreamDeltaKindTerminal:
		a.streamed = true
		if delta.Terminal.ProviderModelID != "" {
			a.providerModelID = delta.Terminal.ProviderModelID
		}
		if delta.Terminal.FinishReason != "" {
			a.finishReason = delta.Terminal.FinishReason
		}
		if delta.Terminal.Usage != nil {
			a.usage = *delta.Terminal.Usage
			a.usageKnown = true
		}
		if len(delta.Terminal.RawResponse) > 0 {
			a.rawResponse = cloneJSON(delta.Terminal.RawResponse)
		}
	default:
		return fmt.Errorf("unsupported stream delta kind %q", delta.Kind)
	}

	return nil
}

func (a *StreamAccumulator) Finalize(completedAt time.Time) (Response, error) {
	if completedAt.IsZero() {
		return Response{}, errors.New("completed_at is required")
	}
	if completedAt.Before(a.startedAt) {
		return Response{}, errors.New("completed_at must not be before started_at")
	}

	toolCalls := make([]ToolCall, 0, len(a.toolCalls))
	for idx, accumulated := range a.toolCalls {
		if accumulated.name == "" {
			return Response{}, NewFailure(
				a.providerKey,
				FailureCodeMalformedResponse,
				fmt.Sprintf("streamed tool call %d is missing a name", idx),
				false,
				nil,
			)
		}

		arguments := accumulated.arguments
		if arguments == "" {
			arguments = `{}`
		}
		rawArguments := json.RawMessage(arguments)
		if !json.Valid(rawArguments) {
			return Response{}, NewFailure(
				a.providerKey,
				FailureCodeMalformedResponse,
				fmt.Sprintf("streamed tool call %q returned invalid JSON arguments", accumulated.name),
				false,
				nil,
			)
		}

		toolCalls = append(toolCalls, ToolCall{
			ID:        accumulated.id,
			Name:      accumulated.name,
			Arguments: cloneJSON(rawArguments),
		})
	}

	timing := Timing{
		StartedAt:    a.startedAt,
		CompletedAt:  completedAt.UTC(),
		TotalLatency: completedAt.UTC().Sub(a.startedAt),
	}
	if !a.firstTokenAt.IsZero() {
		timing.FirstTokenAt = a.firstTokenAt.UTC()
		timing.TTFT = a.firstTokenAt.UTC().Sub(a.startedAt)
	}

	response := Response{
		ProviderKey:     a.providerKey,
		ProviderModelID: a.providerModelID,
		FinishReason:    a.finishReason,
		OutputText:      a.outputText,
		ToolCalls:       toolCalls,
		Streamed:        a.streamed,
		Timing:          timing,
		RawResponse:     cloneJSON(a.rawResponse),
	}
	if a.usageKnown {
		response.Usage = a.usage
	}

	return response, nil
}

func (a *StreamAccumulator) markFirstToken(timestamp time.Time, meaningful bool) {
	if !meaningful || !a.firstTokenAt.IsZero() {
		return
	}
	a.firstTokenAt = timestamp.UTC()
}
