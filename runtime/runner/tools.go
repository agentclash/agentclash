package runner

import (
	"encoding/json"

	"github.com/agentclash/agentclash/runtime/provider"
)

type ToolCategory string
type ToolFailureOrigin string

const (
	ToolCategoryPrimitive ToolCategory = "primitive"
	ToolCategoryComposed  ToolCategory = "composed"
	ToolCategoryMock      ToolCategory = "mock"

	ToolFailureOriginPolicy     ToolFailureOrigin = "policy"
	ToolFailureOriginResolution ToolFailureOrigin = "resolution"
	ToolFailureOriginPrimitive  ToolFailureOrigin = "primitive"
	ToolFailureOriginDelegation ToolFailureOrigin = "delegation"
	ToolFailureOriginCycle      ToolFailureOrigin = "cycle"
	ToolFailureOriginDepth      ToolFailureOrigin = "depth"
)

type ToolExecutionResult struct {
	Content              string
	IsError              bool
	Completed            bool
	FinalOutput          string
	ResolvedToolName     string
	ResolvedToolCategory ToolCategory
	FailureOrigin        ToolFailureOrigin
	ResolutionChain      []string
	FailureDepth         int
}

type ToolExecutionRecord struct {
	ToolCall             provider.ToolCall
	Result               provider.ToolResult
	ToolCategory         ToolCategory
	ResolvedToolName     string
	ResolvedToolCategory ToolCategory
	FailureOrigin        ToolFailureOrigin
	ResolutionChain      []string
	FailureDepth         int
}

type ToolDescriptor struct {
	Name        string
	Description string
	Parameters  json.RawMessage
	Category    ToolCategory
}
