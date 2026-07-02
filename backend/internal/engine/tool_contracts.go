package engine

import "github.com/agentclash/agentclash/runtime/runner"

type ToolCategory = runner.ToolCategory
type ToolFailureOrigin = runner.ToolFailureOrigin

const (
	ToolCategoryPrimitive = runner.ToolCategoryPrimitive
	ToolCategoryComposed  = runner.ToolCategoryComposed
	ToolCategoryMock      = runner.ToolCategoryMock

	ToolFailureOriginPolicy     = runner.ToolFailureOriginPolicy
	ToolFailureOriginResolution = runner.ToolFailureOriginResolution
	ToolFailureOriginPrimitive  = runner.ToolFailureOriginPrimitive
	ToolFailureOriginDelegation = runner.ToolFailureOriginDelegation
	ToolFailureOriginCycle      = runner.ToolFailureOriginCycle
	ToolFailureOriginDepth      = runner.ToolFailureOriginDepth
)

type ToolExecutionResult = runner.ToolExecutionResult
type ToolExecutionRecord = runner.ToolExecutionRecord
