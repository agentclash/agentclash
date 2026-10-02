package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/backend/internal/vibe/inputs"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"slices"
	"time"
)

const assertionBuildAuthoringVersion = 17
const buildAuthoringVersion = 18
const materialBuildAuthoringVersion = 19
const groundedBuildAuthoringVersion = 20
const contextualBuildAuthoringVersion = 21

// Provisional ceilings; live model/quality benchmarks are a release gate.
const FirstBuildSpendCeiling = NanoUSD / 2
const FirstBuildTokenTarget = 500_000

func (p Plan) taskBuild() bool {
	return p.AuthoringVersion >= assertionBuildAuthoringVersion && p.AuthoringVersion <= contextualBuildAuthoringVersion
}

func (p Plan) continuingBuild() bool {
	return p.AuthoringVersion >= buildAuthoringVersion && p.AuthoringVersion <= contextualBuildAuthoringVersion
}

type BuildProgress struct {
	InlineInput          *InlineInput `json:"inline_input,omitempty"`
	TrialID              *uuid.UUID   `json:"trial_id,omitempty"`
	RespondingToQuestion string       `json:"responding_to_question,omitempty"`
	CycleID              uuid.UUID    `json:"cycle_id"`
	Phase                string       `json:"phase"`
	ClarificationsUsed   int          `json:"clarifications_used"`
	ArtifactID           *uuid.UUID   `json:"artifact_id,omitempty"`
	CheckID              *uuid.UUID   `json:"check_id,omitempty"`
	Sample               string       `json:"sample,omitempty"`
	AdmissionRetries     int          `json:"admission_retries,omitempty"`
	Error                *Fault       `json:"error,omitempty"`
}
type BuildCyclePlan struct {
	AdoptedRules       []DocumentSource `json:"adopted_rules,omitempty"`
	Materials          []inputs.Binding `json:"materials,omitempty"`
	ID                 uuid.UUID        `json:"id"`
	Step               string           `json:"step"`
	ClarificationsUsed int              `json:"clarifications_used"`
	Sample             string           `json:"sample,omitempty"`
}
type BuildQuoteRequest struct {
	AdoptRules         []DocumentSource `json:"adopt_rules,omitempty"`
	Inputs             []inputs.Binding `json:"inputs,omitempty"`
	Content            string           `json:"content"`
	ArtifactID         *uuid.UUID       `json:"artifact_id,omitempty"`
	AdditionalExamples int              `json:"additional_examples,omitempty"`
	Models             Models           `json:"models"`
}
type BuildQuote struct {
	TokenBound    int               `json:"token_bound,omitempty"`
	Version       int               `json:"version,omitempty"`
	ID            uuid.UUID         `json:"id"`
	Request       BuildQuoteRequest `json:"request"`
	MaxCost       int64             `json:"max_cost_nano_usd"`
	EstimatedCost int64             `json:"estimated_cost_nano_usd"`
	Cases         int               `json:"cases"`
	MaxCalls      int               `json:"max_calls"`
	ExpiresAt     time.Time         `json:"expires_at"`
	Profiles      map[string]string `json:"profiles"`
	Revision      int64             `json:"revision,omitempty"`
	BaselineHash  string            `json:"baseline_hash,omitempty"`
}

func (q BuildQuote) executionLimits(cfg Config, anonymous bool) Limits {
	l := cfg.Limits(anonymous)
	if q.Version >= 2 && q.Request.AdditionalExamples == 0 {
		// The bounded first journey uses the same context envelope locally and
		// hosted. Local quota exemptions must not reserve entire model windows.
		l.ContextTokens = LimitsFor(anonymous).ContextTokens
	}
	return l
}

func (s *Service) QuoteBuild(ctx context.Context, actor string, id uuid.UUID, request BuildQuoteRequest) (BuildQuote, error) {
	v, err := s.Store.GetSession(ctx, actor, id)
	if err != nil {
		return BuildQuote{}, err
	}
	if !s.Config.Enabled || s.Config.Credential == "" || !s.Config.TwoDoor {
		return BuildQuote{}, fault("hosted_disabled", "The new Build flow is not enabled.")
	}
	if request.ArtifactID != nil {
		for _, a := range v.Document.Artifacts {
			if a.ID == *request.ArtifactID {
				if err = artifactReady(a, s.Config.MaterialBuild); err != nil {
					return BuildQuote{}, err
				}
				break
			}
		}
	}
	if request.AdditionalExamples == 0 && (v.Document.Evaluation == nil || v.Document.Evaluation.Door != "build" || len(v.Document.Artifacts) > 0 || v.Document.Build != nil) {
		return BuildQuote{}, fault("invalid_request", "Start a new Build evaluation for this prototype.")
	}
	if len(request.Content) == 0 || len(request.Content) > s.Config.Limits(v.Anonymous).MessageBytes {
		return BuildQuote{}, fault("invalid_message", "Describe a task within the message limit.")
	}
	if err = s.Config.ValidateModels(request.Models, v.Anonymous); err != nil {
		return BuildQuote{}, err
	}
	if _, err = resolveMaterials(ctx, s.Store.DB, id, request.Inputs); err != nil {
		return BuildQuote{}, err
	}
	if len(request.AdoptRules) > 8 {
		return BuildQuote{}, fault("invalid_input", "Use at most eight policy excerpts.")
	}
	for _, source := range request.AdoptRules {
		if err = validateDocumentSource(ctx, s.Store.DB, id, source); err != nil {
			return BuildQuote{}, err
		}
	}
	buildPolicy, _ := authoringPolicyFor(buildAuthoringVersion)
	quote := BuildQuote{Version: 1, ID: uuid.New(), Request: request, Cases: 3, MaxCalls: 2*buildPolicy.Calls + 6, ExpiresAt: timestamp().Add(30 * time.Minute), Profiles: map[string]string{}}
	if s.Config.MaterialBuild {
		quote.Version = 2
	}
	l := quote.executionLimits(s.Config, v.Anonymous)
	if len(request.Inputs) > 0 {
		// Reject material that cannot fit even the initial input before spending
		// on authoring. Each final invocation still checks its complete prompt.
		messages, e := materialMessages(ctx, s.Store.DB, id, []provider.Message{{Role: "system", Content: PreviewPrompt(request.Content)}}, request.Inputs)
		if e != nil {
			return BuildQuote{}, e
		}
		profile, e := s.Config.Profile(request.Models.Target)
		if e != nil {
			return BuildQuote{}, e
		}
		if _, e = CountContext(provider.Request{Messages: messages, MaxOutputTokens: l.OutputTokens}, profile, l); e != nil {
			return BuildQuote{}, e
		}
	}
	if len(request.Inputs) > 0 && !s.Config.MaterialBuild {
		return BuildQuote{}, fault("hosted_disabled", "Material-based Build is not enabled.")
	}
	judges := 1
	if request.AdditionalExamples != 0 {
		cases, count, hash, e := s.expansionQuote(ctx, v, request)
		if e != nil {
			return BuildQuote{}, e
		}
		quote.Cases, judges, quote.BaselineHash, quote.Revision = cases, count, hash, v.Revision
		quote.MaxCalls = buildPolicy.Calls + cases*(1+judges)
	}
	costs := map[string]int64{}
	for _, model := range []string{request.Models.Assistant, request.Models.Target, request.Models.Evaluator} {
		profile, e := s.Config.Profile(model)
		if e != nil {
			return quote, e
		}
		costs[model], e = profile.BoundCost(profile.inputLimit(l), l.OutputTokens)
		if e != nil {
			return quote, e
		}
		quote.Profiles[model] = Hash(raw(profile))
	}
	primary, _ := s.Config.Profile(request.Models.Assistant)
	temp := Plan{AuthoringVersion: guidedAuthoringVersion, Conversation: &ConversationContext{Profile: &primary}, Anonymous: v.Anonymous, LocalTesting: s.Config.TestingLocally()}
	temp.ExecutionLimits = &l
	if s.Config.MaterialBuild {
		temp.Document.Evaluation = v.Document.Evaluation
	}
	if err = s.freezeReviewVersion(&temp); err != nil {
		return quote, err
	}
	if err = prepareInterpretedPlan(&temp, s.Config, primary); err != nil {
		return quote, err
	}
	// Authoring includes the complete semantic review. Quote the same frozen
	// output allowance used at admission, independently of target/judge limits.
	authorCost, err := primary.BoundCost(primary.inputLimit(temp.limits()), temp.limits().OutputTokens)
	if err != nil {
		return quote, err
	}
	turns := int64(2)
	if request.AdditionalExamples > 0 {
		turns = 1
	}
	authorCalls := turns * int64(buildPolicy.Calls)
	quote.MaxCost = authorCalls*authorCost + int64(quote.Cases)*(costs[request.Models.Target]+int64(judges)*costs[request.Models.Evaluator])
	if temp.AssistantRecovery != nil {
		quote.MaxCost += 2 * temp.AssistantRecovery.MaxCost
		quote.MaxCalls += 2
		quote.Profiles[temp.AssistantRecovery.Profile.ID] = Hash(raw(temp.AssistantRecovery.Profile))
	}
	if hasTaskInput(request.Inputs) || quote.Version >= 2 && request.AdditionalExamples == 0 {
		quote.MaxCost += costs[request.Models.Target]
		quote.MaxCalls++
	}
	quote.EstimatedCost = quote.MaxCost // only the auditable ceiling is advertised
	if quote.Version >= 2 && request.AdditionalExamples == 0 {
		quote.TokenBound = int(authorCalls)*(primary.inputLimit(temp.limits())+temp.limits().OutputTokens) + (quote.MaxCalls-int(authorCalls))*(l.ContextTokens+l.OutputTokens)
		if quote.MaxCost > FirstBuildSpendCeiling {
			return BuildQuote{}, fault("budget_limit", "The complete first run cannot fit the current $0.50 model allowance with these models. Choose a lower-cost model in Settings. Nothing was run or shortened.")
		}
		if v.Anonymous && !s.Config.TestingLocally() {
			if err = checkInitialBuildQuota(ctx, s.Store.DB, v.ID, uuid.Nil, quote); err != nil {
				return BuildQuote{}, err
			}
		}
	}

	err = s.Store.persistQuote(ctx, actor, id, v.Revision, quoteRecord{quote.ID, Hash(raw(request)), raw(quote), quote.MaxCost, quote.ExpiresAt})
	return quote, err
}

func (s *Service) prepareBuildCycle(ctx context.Context, v Session, sub Submission, p *Plan) error {
	if sub.CycleID == nil {
		return nil
	}
	var quote BuildQuote
	var bytes []byte
	var stopped *time.Time
	if err := s.Store.DB.QueryRow(ctx, "SELECT specification,stopped_at FROM vibe_cycle_quotes WHERE id=$1 AND session_id=$2", *sub.CycleID, v.ID).Scan(&bytes, &stopped); err != nil {
		return fault("quote_expired", "Get a current Build estimate before running.")
	}
	if json.Unmarshal(bytes, &quote) != nil || stopped != nil {
		return fault("invalid_state", "This Build cycle was stopped. Your work is preserved.")
	}
	if quote.Request.Models != sub.Models {
		return fault("quote_expired", "The models changed. Get a new estimate before running.")
	}
	for model, hash := range quote.Profiles {
		profile, err := s.Config.Profile(model)
		if err != nil {
			return err
		}
		if Hash(raw(profile)) != hash {
			return fault("quote_expired", "Model pricing changed. Get a new estimate before running.")
		}
	}
	if quote.Request.AdditionalExamples == 0 && v.Document.Build != nil && v.Document.Build.CycleID != quote.ID {
		return fault("invalid_state", "Continue this evaluation’s existing cycle; its question budget cannot be reset.")
	}
	cycle := &BuildCyclePlan{ID: quote.ID, Step: "prepare", Materials: quote.Request.Inputs, AdoptedRules: adoptedSources(sub)}
	if len(quote.Request.AdoptRules) > 0 && v.Document.Build != nil {
		var saved []byte
		if err := s.Store.DB.QueryRow(ctx, `SELECT o.input FROM vibe_cycle_steps c JOIN vibe_operations o ON o.id=c.operation_id WHERE c.cycle_id=$1 AND c.step='prepare'`, quote.ID).Scan(&saved); err != nil {
			return err
		}
		var original Plan
		if err := json.Unmarshal(saved, &original); err != nil {
			return err
		}
		cycle.AdoptedRules = adoptedSources(original.Submission)
	}
	if progress := v.Document.Build; progress != nil && progress.CycleID == quote.ID {
		cycle.ClarificationsUsed = progress.ClarificationsUsed
		cycle.Sample = progress.Sample
		switch progress.Phase {
		case "clarifying", "waiting":
			cycle.Step = "message:" + sub.ClientID.String()
		case "ready":
			if (sub.Kind != "check" && sub.Kind != "playground") || progress.ArtifactID == nil || sub.ArtifactID == nil || *progress.ArtifactID != *sub.ArtifactID {
				return fault("invalid_request", "Use the prepared prototype for this check.")
			}
			cycle.Step = "check"
			trialInputs := initialTrialInputs(quote, progress)
			if quote.Version >= 2 && hasTaskInput(trialInputs) && progress.TrialID == nil {
				if sub.Kind != "playground" || !slices.Equal(sub.Inputs, trialInputs) {
					return fault("invalid_request", "Try the supplied material before checking this prototype.")
				}
				cycle.Step = "initial_trial"
			} else if sub.Kind != "check" {
				return fault("invalid_request", "The initial trial is already complete.")
			}
		default:
			return fault("invalid_state", "This Build cycle already has work in progress or results.")
		}
	}
	if cycle.Step == "prepare" && (!slices.Equal(quote.Request.AdoptRules, sub.AdoptRules) || quote.Request.Content != sub.Content || !slices.Equal(quote.Request.Inputs, sub.Inputs) || timestamp().After(quote.ExpiresAt)) {
		return fault("quote_expired", "Your request changed or its estimate expired. Get a new estimate.")
	}
	if cycle.Step != "check" && cycle.Step != "initial_trial" && sub.Kind != "message" {
		return fault("invalid_request", "This estimate authorizes prototype preparation and its first three examples.")
	}
	if quote.Request.AdditionalExamples > 0 {
		if cycle.Step != "prepare" && cycle.Step != "check" {
			return fault("invalid_state", "This batch cannot start an onboarding conversation.")
		}
		if cycle.Step == "prepare" {
			_, _, hash, e := s.expansionQuote(ctx, v, quote.Request)
			if e != nil {
				return e
			}
			if hash != quote.BaselineHash || v.Revision != quote.Revision || sub.ArtifactID == nil || quote.Request.ArtifactID == nil || *sub.ArtifactID != *quote.Request.ArtifactID || sub.AdditionalExamples != quote.Request.AdditionalExamples {
				return fault("quote_expired", "This batch changed. Review a new estimate.")
			}
		}
	}
	p.Cycle = cycle
	if quote.Version >= 2 && quote.Request.AdditionalExamples == 0 {
		l := quote.executionLimits(s.Config, v.Anonymous)
		p.ExecutionLimits = &l
	}
	return nil
}

// Called inside admission's transaction, before reservations or provider work.
func admitBuildCycle(ctx context.Context, tx pgx.Tx, v Session, sub Submission, p Plan, o Operation) error {
	if p.Cycle == nil {
		return nil
	}
	var bytes []byte
	var authorized, stopped *time.Time
	var maxCost int64
	if err := tx.QueryRow(ctx, "SELECT specification,authorized_at,stopped_at,max_cost FROM vibe_cycle_quotes WHERE id=$1 AND session_id=$2 FOR UPDATE", p.Cycle.ID, v.ID).Scan(&bytes, &authorized, &stopped, &maxCost); err != nil {
		return err
	}
	var q BuildQuote
	if json.Unmarshal(bytes, &q) != nil || stopped != nil {
		return fault("invalid_state", "This Build cycle is no longer active.")
	}
	if authorized == nil && (p.Cycle.Step != "prepare" || timestamp().After(q.ExpiresAt) || q.Request.Content != sub.Content) {
		return fault("quote_expired", "Get a current estimate before running.")
	}
	if authorized == nil && q.Version >= 2 && q.Request.AdditionalExamples == 0 && v.Anonymous && !p.LocalTesting {
		if err := checkInitialBuildQuota(ctx, tx, v.ID, o.ID, q); err != nil {
			return err
		}
	}
	var used int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(CASE WHEN o.billing IN ('SETTLED','RELEASED') THEN COALESCE(o.actual_cost,0) ELSE o.max_cost END),0) FROM vibe_cycle_steps s JOIN vibe_operations o ON o.id=s.operation_id WHERE s.cycle_id=$1`, p.Cycle.ID).Scan(&used); err != nil {
		return err
	}
	var calls int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(CASE WHEN o.state IN ('COMPLETED','FAILED','CANCELLED','EXPIRED') THEN o.model_calls ELSE (o.input->>'calls')::int END),0) FROM vibe_cycle_steps s JOIN vibe_operations o ON o.id=s.operation_id WHERE s.cycle_id=$1`, p.Cycle.ID).Scan(&calls); err != nil {
		return err
	}
	if calls+p.Calls > q.MaxCalls {
		return fault("budget_limit", "This initial cycle reached its call allowance. Your progress is preserved.")
	}
	if used+p.MaxCost > maxCost {
		return fault("budget_limit", "This cycle needs a new cost authorization. Previous results are preserved.")
	}
	if _, err := tx.Exec(ctx, "INSERT INTO vibe_cycle_steps(cycle_id,step,operation_id) VALUES($1,$2,$3)", p.Cycle.ID, p.Cycle.Step, o.ID); err != nil {
		return fault("idempotency_conflict", "This Build step was already submitted. Reload its saved progress.")
	}
	_, err := tx.Exec(ctx, "UPDATE vibe_cycle_quotes SET authorized_at=COALESCE(authorized_at,now()) WHERE id=$1", p.Cycle.ID)
	return err
}

// Continuation is server owned. Stable submission identity makes a lost response
// or a second worker harmless. There is no browser effect that starts this run.
func (s *Service) AdvanceBuild(ctx context.Context, id uuid.UUID) error {
	o, err := s.Store.Operation(ctx, id)
	if err != nil {
		return err
	}
	var p Plan
	if json.Unmarshal(o.Input, &p) != nil || p.Cycle == nil || !o.State.Terminal() {
		return nil
	}
	actor, deleted, err := s.Store.currentOwner(ctx, o.SessionID)
	if err != nil {
		return err
	}
	if deleted {
		return nil
	}
	v, err := s.Store.GetSession(ctx, actor, o.SessionID)
	if err != nil {
		return err
	}
	progress := v.Document.Build
	if progress == nil || progress.CycleID != p.Cycle.ID || progress.Phase != "ready" || progress.ArtifactID == nil {
		return nil
	}
	if progress.Error != nil && progress.Error.RetryAvailableAt != nil && timestamp().Before(*progress.Error.RetryAvailableAt) {
		return nil
	}
	var quote BuildQuote
	var specification []byte
	var stopped *time.Time
	if err = s.Store.DB.QueryRow(ctx, "SELECT specification,stopped_at FROM vibe_cycle_quotes WHERE id=$1", p.Cycle.ID).Scan(&specification, &stopped); err != nil {
		return err
	}
	if stopped != nil {
		return nil
	}
	if err = json.Unmarshal(specification, &quote); err != nil {
		return err
	}
	sub := Submission{ClientID: deterministicID(p.Cycle.ID, "check"), Revision: v.Revision, Kind: "check", Models: quote.Request.Models, ArtifactID: progress.ArtifactID, ApproveArtifact: true, CycleID: &p.Cycle.ID}
	trialInputs := initialTrialInputs(quote, progress)
	if quote.Version >= 2 && hasTaskInput(trialInputs) && progress.TrialID == nil {
		sub.Kind = "playground"
		sub.ClientID = deterministicID(p.Cycle.ID, "initial_trial")
		sub.ApproveArtifact = false
		sub.Inputs = trialInputs
		sub.Content = "Process the supplied material according to your instructions."
		thread := deterministicID(p.Cycle.ID, "initial_trial_thread")
		sub.PreviewThreadID = &thread
	}
	if receipt, e := s.Store.submissionReceipt(ctx, v.ID, sub); receipt != nil || e != nil {
		return e
	}
	_, err = s.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil {
		var f *Fault
		if !errors.As(err, &f) {
			return err
		}
		return s.Store.Edit(ctx, v.Actor, v.ID, v.Revision, func(current *Session) error {
			if current.Document.Build != nil && current.Document.Build.CycleID == p.Cycle.ID && current.Document.Build.Phase == "ready" {
				b := current.Document.Build
				b.Phase = "error"
				if f.AdmissionRetryable {
					b.Phase = "ready"
					delay := 5 * time.Second * time.Duration(1<<min(b.AdmissionRetries, 4))
					available := timestamp().Add(min(delay, time.Minute))
					if f.RetryAvailableAt != nil && available.Before(*f.RetryAvailableAt) {
						available = *f.RetryAvailableAt
					}
					copy := *f
					copy.RetryAvailableAt = &available
					f = &copy
					b.AdmissionRetries++
				}
				b.Error = f
			}
			return nil
		})
	}
	return nil
}

func (s *Store) syncBuildResult(ctx context.Context, id uuid.UUID) error {
	return s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		o, err := lockOperation(ctx, tx, id, projectWrite)
		if err != nil {
			return err
		}
		var p Plan
		if json.Unmarshal(o.Input, &p) != nil || p.Cycle == nil {
			return nil
		}
		var deleted bool
		if err = tx.QueryRow(ctx, "SELECT deleted_at IS NOT NULL FROM vibe_sessions WHERE id=$1", o.SessionID).Scan(&deleted); err != nil {
			return err
		}
		if deleted {
			return nil
		}
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", o.SessionID))
		if err != nil {
			return err
		}
		b := v.Document.Build
		if b == nil || b.CycleID != p.Cycle.ID {
			return nil
		}
		if p.Cycle.Step == "check" {
			if b.Phase == "results" && b.CheckID != nil && *b.CheckID == id {
				return nil
			}
			b.Phase = "results"
			b.CheckID = &id
			b.Error = o.Error
		} else if o.Completion == nil && o.State.Terminal() {
			b.Phase = "error"
			b.Error = o.Error
		} else {
			return nil
		}
		if err = s.updateDocument(ctx, tx, v); err != nil {
			return err
		}
		return event(ctx, tx, v.ID, &id, "build.updated")
	})
}

func ResumeBuilds(ctx context.Context, s *Service) {
	rows, err := s.Store.DB.Query(ctx, `SELECT o.id FROM vibe_cycle_steps c JOIN vibe_operations o ON o.id=c.operation_id JOIN vibe_sessions s ON s.id=o.session_id JOIN vibe_cycle_quotes q ON q.id=c.cycle_id WHERE (c.step IN ('prepare','answer','initial_trial') OR c.step LIKE 'retry:%' OR c.step LIKE 'message:%') AND o.state='COMPLETED' AND s.document->>'format_version'='1' AND s.document#>>'{build,phase}'='ready' AND s.document#>>'{build,cycle_id}'=c.cycle_id::text AND q.stopped_at IS NULL AND s.deleted_at IS NULL AND (s.document#>>'{build,error,retry_available_at}' IS NULL OR (s.document#>>'{build,error,retry_available_at}')::timestamptz<=now()) ORDER BY o.created_at LIMIT 20`)
	if err != nil {
		return
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		_ = s.AdvanceBuild(ctx, id)
	}
}
