package vibe

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

type Execution string

const (
	Created          Execution = "CREATED"
	Validating       Execution = "VALIDATING"
	AwaitingInput    Execution = "AWAITING_INPUT"
	AwaitingApproval Execution = "AWAITING_APPROVAL"
	Reserved         Execution = "RESERVED"
	Queued           Execution = "QUEUED"
	Running          Execution = "RUNNING"
	Finalizing       Execution = "FINALIZING"
	Cancelling       Execution = "CANCELLING"
	Completed        Execution = "COMPLETED"
	Partial          Execution = "PARTIAL"
	Failed           Execution = "FAILED"
	Cancelled        Execution = "CANCELLED"
	Expired          Execution = "EXPIRED"
)

func (s Execution) Terminal() bool {
	return s == Completed || s == Partial || s == Failed || s == Cancelled || s == Expired
}
func CanTransition(from, to Execution) bool {
	if from.Terminal() {
		return false
	}
	if to == Cancelling {
		return from != Cancelling
	}
	switch from {
	case Created:
		return to == Validating
	case Validating:
		return to == AwaitingInput || to == AwaitingApproval || to == Reserved || to == Failed
	case AwaitingInput, AwaitingApproval:
		return to == Validating || to == Expired || to == Cancelled
	case Reserved:
		return to == Queued || to == Failed || to == Expired
	case Queued:
		return to == Running || to == Failed || to == Expired
	case Running:
		return to == Finalizing
	case Finalizing:
		return to == Completed || to == Partial || to == Failed
	case Cancelling:
		return to == Cancelled
	}
	return false
}

type BillingState string

const (
	Unreserved      BillingState = "UNRESERVED"
	BillingReserved BillingState = "RESERVED"
	Reconciling     BillingState = "RECONCILING"
	Settled         BillingState = "SETTLED"
	Released        BillingState = "RELEASED"
)

type Role string

const (
	Assistant Role = "assistant"
	Target    Role = "target"
	Evaluator Role = "evaluator"
)

type Models struct {
	Assistant string `json:"assistant"`
	Target    string `json:"target"`
	Evaluator string `json:"evaluator"`
}

func DefaultModels() Models {
	return Models{"openai/gpt-4.1-mini", "openai/gpt-4.1-mini", "openai/gpt-4.1-mini"}
}

type Message struct {
	ID          uuid.UUID  `json:"id"`
	Role        string     `json:"role"`
	Content     string     `json:"content"`
	CreatedAt   time.Time  `json:"created_at"`
	Origin      string     `json:"origin,omitempty"`
	OperationID *uuid.UUID `json:"operation_id,omitempty"`
	ArtifactID  *uuid.UUID `json:"artifact_id,omitempty"`
}
type Requirement struct {
	ProposedBy        string     `json:"proposed_by,omitempty"`
	ProposalMessageID *uuid.UUID `json:"proposal_message_id,omitempty"`
	ID                uuid.UUID  `json:"id"`
	Statement         string     `json:"statement"`
	Status            string     `json:"status"`
	SourceMessageID   uuid.UUID  `json:"source_message_id"`
	AcceptedBy        string     `json:"accepted_by,omitempty"`
	AcceptedAt        *time.Time `json:"accepted_at,omitempty"`
	SupersedesID      *uuid.UUID `json:"supersedes_id,omitempty"`
	Change            string     `json:"change,omitempty"`
}
type Artifact struct {
	CriteriaRequirementIDs []string        `json:"criteria_requirement_ids,omitempty"`
	Kind                   string          `json:"kind,omitempty"`
	TestPlan               *TestPlan       `json:"test_plan,omitempty"`
	Proposal               *DraftProposal  `json:"proposal,omitempty"`
	ID                     uuid.UUID       `json:"id"`
	Title                  string          `json:"title"`
	AgentPrompt            string          `json:"agent_prompt"`
	Blueprint              json.RawMessage `json:"blueprint"`
	SourceMessageID        uuid.UUID       `json:"source_message_id"`
	ParentID               *uuid.UUID      `json:"parent_id,omitempty"`
	Accepted               bool            `json:"accepted"`
	CreatedAt              time.Time       `json:"created_at"`
}
type Document struct {
	Journey          Journey       `json:"journey,omitempty"`
	AttachmentCount  int           `json:"attachment_count"`
	Messages         []Message     `json:"messages"`
	Requirements     []Requirement `json:"requirements"`
	Artifacts        []Artifact    `json:"artifacts"`
	Models           Models        `json:"models"`
	ActiveArtifactID *uuid.UUID    `json:"active_artifact_id,omitempty"`
}
type Session struct {
	EventCursor     int64       `json:"event_cursor"`
	ID              uuid.UUID   `json:"id"`
	Actor           string      `json:"-"`
	WorkspaceID     *uuid.UUID  `json:"workspace_id,omitempty"`
	Anonymous       bool        `json:"anonymous"`
	Revision        int64       `json:"revision"`
	Title           string      `json:"title"`
	Document        Document    `json:"document"`
	Operations      []Operation `json:"operations"`
	UpdatedAt       time.Time   `json:"updated_at"`
	SavedDraftID    *uuid.UUID  `json:"saved_draft_id,omitempty"`
	SavedArtifactID *uuid.UUID  `json:"saved_artifact_id,omitempty"`
	SavedModels     *Models     `json:"saved_models,omitempty"`
}
type Operation struct {
	BaselineID *uuid.UUID      `json:"baseline_id,omitempty"`
	ID         uuid.UUID       `json:"id"`
	SessionID  uuid.UUID       `json:"session_id"`
	Actor      string          `json:"-"`
	Kind       string          `json:"kind"`
	State      Execution       `json:"state"`
	Billing    BillingState    `json:"billing"`
	Models     Models          `json:"models"`
	Input      json.RawMessage `json:"-"`
	MaxCost    int64           `json:"max_cost_nano_usd"`
	ActualCost *int64          `json:"actual_cost_nano_usd"`
	ModelCalls int             `json:"model_calls"`
	Error      *Fault          `json:"error,omitempty"`
	Results    []CaseResult    `json:"results"`
	Scorecard  *Scorecard      `json:"scorecard,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	Deadline   time.Time       `json:"deadline"`
}
type Verdict string

const (
	Pass    Verdict = "PASS"
	Fail    Verdict = "FAIL"
	Unknown Verdict = "UNKNOWN"
)

type CheckResult struct {
	Key      string  `json:"key"`
	Verdict  Verdict `json:"verdict"`
	Evidence string  `json:"evidence"`
	Error    *Fault  `json:"error,omitempty"`
}
type CaseResult struct {
	ExpectedChecks int             `json:"expected_checks"`
	CaseKey        string          `json:"case_key"`
	Version        string          `json:"version"`
	Input          json.RawMessage `json:"input"`
	Output         string          `json:"output"`
	Verdict        Verdict         `json:"verdict"`
	Checks         []CheckResult   `json:"checks"`
	Error          *Fault          `json:"error,omitempty"`
}
type Scorecard struct {
	ChecksExpected  int      `json:"checks_expected"`
	ChecksEvaluated int      `json:"checks_evaluated"`
	IncompleteCases int      `json:"incomplete_cases"`
	Passed          int      `json:"passed"`
	Failed          int      `json:"failed"`
	Unknown         int      `json:"unknown"`
	Total           int      `json:"total"`
	Evaluated       int      `json:"evaluated"`
	PassRate        *float64 `json:"pass_rate"`
	Coverage        float64  `json:"coverage"`
}

func Aggregate(results []CaseResult) Scorecard {
	s := Scorecard{Total: len(results)}
	for _, r := range results {
		expected, evaluated := r.ExpectedChecks, 0
		if len(r.Checks) > expected {
			expected = len(r.Checks)
		}
		for _, check := range r.Checks {
			if check.Verdict == Pass || check.Verdict == Fail {
				evaluated++
			}
		}
		// Legacy/test fixtures without check metadata still have one case verdict.
		if expected == 0 {
			expected = 1
			if r.Verdict == Pass || r.Verdict == Fail {
				evaluated = 1
			}
		}
		s.ChecksExpected += expected
		s.ChecksEvaluated += evaluated
		if evaluated < expected {
			s.IncompleteCases++
		}
		switch r.Verdict {
		case Pass:
			s.Passed++
		case Fail:
			s.Failed++
		default:
			s.Unknown++
		}
	}
	s.Evaluated = s.Passed + s.Failed
	if s.ChecksExpected > 0 {
		s.Coverage = float64(s.ChecksEvaluated) / float64(s.ChecksExpected)
	}
	if s.Evaluated > 0 {
		v := float64(s.Passed) / float64(s.Evaluated)
		s.PassRate = &v
	}
	return s
}
func CaseVerdict(checks []CheckResult) Verdict {
	v := Pass
	if len(checks) == 0 {
		return Unknown
	}
	for _, c := range checks {
		if c.Verdict == Fail {
			return Fail
		}
		if c.Verdict != Pass {
			v = Unknown
		}
	}
	return v
}
