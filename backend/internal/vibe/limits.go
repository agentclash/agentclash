// Package vibe implements the private, bounded conversational evaluation flow.
// It deliberately has no shell, generic HTTP, deployment or payment tools.
package vibe

import "time"

type Limits struct {
	RequestBytes     int `json:"max_request_bytes"`
	MessageBytes     int `json:"max_message_bytes"`
	FileBytes        int `json:"max_file_bytes"`
	Files            int `json:"max_attachments"`
	StoredBytes      int `json:"max_stored_attachment_bytes"`
	Depth            int `json:"max_nesting"`
	Nodes            int `json:"max_nodes"`
	Keys             int `json:"max_object_keys"`
	Array            int `json:"max_array_items"`
	StringBytes      int `json:"max_string_bytes"`
	ImportCases      int `json:"max_import_cases"`
	Cases            int `json:"max_cases"`
	Versions         int `json:"max_versions"`
	Evaluators       int `json:"max_evaluators"`
	Checks           int `json:"max_checks_per_case"`
	ContextTokens    int `json:"max_context_tokens"`
	OutputTokens     int `json:"max_output_tokens"`
	Helpers          int `json:"max_helper_calls"`
	Tools            int `json:"max_tool_calls"`
	ModelCalls       int `json:"max_model_calls"`
	Retries          int `json:"max_pre_dispatch_retries"`
	Running          int `json:"max_concurrent_operations"`
	Queued           int `json:"max_queued_operations"`
	Rate             int `json:"requests_per_minute"`
	OperationSeconds int `json:"max_operation_seconds"`
	QueueSeconds     int `json:"max_queue_seconds"`
	ProviderSeconds  int `json:"max_provider_seconds"`
}

const (
	NanoUSD                   int64 = 1_000_000_000
	TrialBudget                     = NanoUSD
	TrialExploreBudget              = NanoUSD / 4
	TrialMessages                   = 20
	TrialCalls                      = 40
	TrialExploreCalls               = 28 // Reserve six calls each for the initial check and retest.
	MaxFreeDailyCalls               = 40
	MaxOperationCost                = 10 * NanoUSD
	AutomaticApprovalCost           = NanoUSD / 4
	MaxKeyBytes                     = 128
	MaxNumberBytes                  = 64
	MaxConversationMessages         = 200
	MaxDocumentBytes                = 8 << 20
	MaxConversationOperations       = 100
	MaxOutputTextBytes              = 64 << 10
	MaxRevisions                    = 100
	MaxRequirements                 = 100
	MaxSSEConnections               = 2
	MaxRepetitions                  = 1
	MaxSamples                      = 1
	MaxAuthoringRepairs             = 1
	MaxEvaluatorRepairs             = 0
	MaxProviderResponseBytes        = 1 << 20
	MaxWorkspaceRunning             = 4
	MaxWorkspaceQueued              = 10
)

func LimitsFor(anonymous bool) Limits {
	if anonymous {
		return Limits{64 << 10, 16 << 10, 256 << 10, 3, 2 << 20, 16, 10000, 128, 256, 16 << 10, 50, 3, 1, 1, 8, 16384, 2048, 4, 6, 12, 1, 1, 1, 10, 180, 60, 45}
	}
	return Limits{256 << 10, 64 << 10, 1 << 20, 5, 20 << 20, 24, 50000, 256, 1024, 64 << 10, 200, 20, 2, 2, 20, 32768, 4096, 6, 12, 128, 2, 2, 3, 30, 900, 300, 60}
}

func (l Limits) OperationTimeout() time.Duration {
	return time.Duration(l.OperationSeconds) * time.Second
}

type Fault struct {
	Code    string             `json:"code"`
	Message string             `json:"message"`
	Context *ContextDiagnostic `json:"context,omitempty"`
}

func (e *Fault) Error() string         { return e.Message }
func fault(code, message string) error { return &Fault{Code: code, Message: message} }

func GraphCalls(cases, versions, repetitions, evaluators, samples, helpers, retries int, l Limits) (int, error) {
	if cases < 1 || cases > l.Cases || versions < 1 || versions > l.Versions || repetitions != 1 || evaluators < 0 || evaluators > l.Evaluators || samples != 1 || helpers < 0 || helpers > l.Helpers || retries < 0 || retries > l.Retries {
		return 0, fault("graph_limit", "The requested check exceeds the execution limits.")
	}
	n := cases*versions*(1+evaluators) + helpers + retries
	if n > l.ModelCalls {
		return 0, fault("graph_limit", "The complete check requires too many model calls.")
	}
	return n, nil
}
