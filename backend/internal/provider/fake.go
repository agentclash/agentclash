package provider

import (
	"context"
	"encoding/json"
)

type FakeClient struct {
	Response Response
	Err      error
	Requests []Request
}

func (f *FakeClient) InvokeModel(_ context.Context, request Request) (Response, error) {
	f.Requests = append(f.Requests, cloneRequest(request))
	if f.Err != nil {
		return Response{}, f.Err
	}
	return cloneResponse(f.Response), nil
}

func cloneRequest(request Request) Request {
	cloned := request
	cloned.Messages = make([]Message, 0, len(request.Messages))
	for _, message := range request.Messages {
		cloned.Messages = append(cloned.Messages, cloneMessage(message))
	}
	cloned.Tools = make([]ToolDefinition, 0, len(request.Tools))
	for _, tool := range request.Tools {
		cloned.Tools = append(cloned.Tools, ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  cloneJSON(tool.Parameters),
		})
	}
	cloned.Metadata = cloneJSON(request.Metadata)
	return cloned
}

func cloneResponse(response Response) Response {
	cloned := response
	cloned.ToolCalls = make([]ToolCall, 0, len(response.ToolCalls))
	for _, toolCall := range response.ToolCalls {
		cloned.ToolCalls = append(cloned.ToolCalls, ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Name,
			Arguments: cloneJSON(toolCall.Arguments),
		})
	}
	cloned.RawResponse = cloneJSON(response.RawResponse)
	return cloned
}

type FakeStreamingClient struct {
	Response Response
	Deltas   []StreamDelta
	Err      error
	Requests []Request
}

func (f *FakeStreamingClient) InvokeModel(_ context.Context, request Request) (Response, error) {
	f.Requests = append(f.Requests, cloneRequest(request))
	if f.Err != nil {
		return Response{}, f.Err
	}
	return cloneResponse(f.Response), nil
}

func (f *FakeStreamingClient) StreamModel(_ context.Context, request Request, onDelta func(StreamDelta) error) (Response, error) {
	f.Requests = append(f.Requests, cloneRequest(request))
	if f.Err != nil {
		return Response{}, f.Err
	}
	for _, delta := range f.Deltas {
		if onDelta != nil {
			if err := onDelta(cloneStreamDelta(delta)); err != nil {
				return Response{}, err
			}
		}
	}
	return cloneResponse(f.Response), nil
}

func cloneMessage(message Message) Message {
	cloned := message
	cloned.ToolCalls = make([]ToolCall, 0, len(message.ToolCalls))
	for _, toolCall := range message.ToolCalls {
		cloned.ToolCalls = append(cloned.ToolCalls, ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Name,
			Arguments: cloneJSON(toolCall.Arguments),
		})
	}
	return cloned
}

func cloneJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

func cloneStreamDelta(delta StreamDelta) StreamDelta {
	cloned := delta
	cloned.Terminal.RawResponse = cloneJSON(delta.Terminal.RawResponse)
	return cloned
}
