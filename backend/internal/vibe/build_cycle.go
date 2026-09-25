package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

const buildAuthoringVersion = 16

type BuildProgress struct {
	CycleID            uuid.UUID  `json:"cycle_id"`
	Phase              string     `json:"phase"`
	ClarificationsUsed int        `json:"clarifications_used"`
	ArtifactID         *uuid.UUID `json:"artifact_id,omitempty"`
	CheckID            *uuid.UUID `json:"check_id,omitempty"`
	Sample             string     `json:"sample,omitempty"`
	Error              *Fault     `json:"error,omitempty"`
}
type BuildCyclePlan struct {
	ID                 uuid.UUID `json:"id"`
	Step               string    `json:"step"`
	ClarificationsUsed int       `json:"clarifications_used"`
	Sample             string    `json:"sample,omitempty"`
}
type BuildQuoteRequest struct {
	Content string `json:"content"`
	Models  Models `json:"models"`
}
type BuildQuote struct {
	ID            uuid.UUID         `json:"id"`
	Request       BuildQuoteRequest `json:"request"`
	MaxCost       int64             `json:"max_cost_nano_usd"`
	EstimatedCost int64             `json:"estimated_cost_nano_usd"`
	Cases         int               `json:"cases"`
	MaxCalls      int               `json:"max_calls"`
	ExpiresAt     time.Time         `json:"expires_at"`
	Profiles      map[string]string `json:"profiles"`
}

func (s *Service) QuoteBuild(ctx context.Context, actor string, id uuid.UUID, request BuildQuoteRequest) (BuildQuote, error) {
	v, err := s.Store.GetSession(ctx, actor, id)
	if err != nil {
		return BuildQuote{}, err
	}
	if !s.Config.TwoDoor {
		return BuildQuote{}, fault("hosted_disabled", "The new Build flow is not enabled.")
	}
	if v.Document.Evaluation == nil || v.Document.Evaluation.Door != "build" || len(v.Document.Artifacts) > 0 || v.Document.Build != nil {
		return BuildQuote{}, fault("invalid_request", "Start a new Build evaluation for this prototype.")
	}
	if len(request.Content) == 0 || len(request.Content) > s.Config.Limits(v.Anonymous).MessageBytes {
		return BuildQuote{}, fault("invalid_message", "Describe a task within the message limit.")
	}
	if err = s.Config.ValidateModels(request.Models, v.Anonymous); err != nil {
		return BuildQuote{}, err
	}
	l := s.Config.Limits(v.Anonymous)
	quote := BuildQuote{ID: uuid.New(), Request: request, Cases: 3, MaxCalls: 26, ExpiresAt: timestamp().Add(30 * time.Minute), Profiles: map[string]string{}}
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
	quote.MaxCost = 20*costs[request.Models.Assistant] + 3*(costs[request.Models.Target]+costs[request.Models.Evaluator])
	primary, _ := s.Config.Profile(request.Models.Assistant)
	temp := Plan{AuthoringVersion: guidedAuthoringVersion, Conversation: &ConversationContext{Profile: &primary}, Anonymous: v.Anonymous, LocalTesting: s.Config.TestingLocally()}
	if err = prepareInterpretedPlan(&temp, s.Config, primary); err != nil {
		return quote, err
	}
	if temp.AssistantRecovery != nil {
		quote.MaxCost += 2 * temp.AssistantRecovery.MaxCost
		quote.MaxCalls += 2
		quote.Profiles[temp.AssistantRecovery.Profile.ID] = Hash(raw(temp.AssistantRecovery.Profile))
	}
	quote.EstimatedCost = quote.MaxCost // only the auditable ceiling is advertised

	_, err = s.Store.DB.Exec(ctx, `INSERT INTO vibe_cycle_quotes(id,session_id,request_hash,specification,max_cost,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, quote.ID, id, Hash(raw(request)), raw(quote), quote.MaxCost, quote.ExpiresAt)
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
	if v.Document.Build != nil && v.Document.Build.CycleID != quote.ID {
		return fault("invalid_state", "Continue this evaluation’s existing cycle; its question budget cannot be reset.")
	}
	cycle := &BuildCyclePlan{ID: quote.ID, Step: "prepare"}
	if progress := v.Document.Build; progress != nil && progress.CycleID == quote.ID {
		cycle.ClarificationsUsed = progress.ClarificationsUsed
		cycle.Sample = progress.Sample
		switch progress.Phase {
		case "clarifying":
			cycle.Step = "answer"
		case "ready":
			if sub.Kind != "check" || progress.ArtifactID == nil || sub.ArtifactID == nil || *progress.ArtifactID != *sub.ArtifactID {
				return fault("invalid_request", "Use the prepared prototype for this check.")
			}
			cycle.Step = "check"
		default:
			return fault("invalid_state", "This Build cycle already has work in progress or results.")
		}
	}
	if cycle.Step == "prepare" && (quote.Request.Content != sub.Content || timestamp().After(quote.ExpiresAt)) {
		return fault("quote_expired", "Your request changed or its estimate expired. Get a new estimate.")
	}
	if cycle.Step != "check" && sub.Kind != "message" {
		return fault("invalid_request", "This estimate authorizes prototype preparation and its first three examples.")
	}
	p.Cycle = cycle
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
	v, err := s.Store.GetSession(ctx, o.Actor, o.SessionID)
	if err != nil {
		return err
	}
	progress := v.Document.Build
	if progress == nil || progress.CycleID != p.Cycle.ID || progress.Phase != "ready" || progress.ArtifactID == nil {
		return nil
	}
	var quote BuildQuote
	var specification []byte
	if err = s.Store.DB.QueryRow(ctx, "SELECT specification FROM vibe_cycle_quotes WHERE id=$1", p.Cycle.ID).Scan(&specification); err != nil {
		return err
	}
	if err = json.Unmarshal(specification, &quote); err != nil {
		return err
	}
	sub := Submission{ClientID: deterministicID(p.Cycle.ID, "check"), Revision: v.Revision, Kind: "check", Models: quote.Request.Models, ArtifactID: progress.ArtifactID, ApproveArtifact: true, CycleID: &p.Cycle.ID}
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
				current.Document.Build.Phase = "error"
				current.Document.Build.Error = f
			}
			return nil
		})
	}
	return nil
}

func (s *Store) syncBuildResult(ctx context.Context, id uuid.UUID) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1", id))
		if err != nil {
			return err
		}
		var p Plan
		if json.Unmarshal(o.Input, &p) != nil || p.Cycle == nil {
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
	rows, err := s.Store.DB.Query(ctx, `SELECT o.id FROM vibe_cycle_steps c JOIN vibe_operations o ON o.id=c.operation_id JOIN vibe_sessions s ON s.id=o.session_id JOIN vibe_cycle_quotes q ON q.id=c.cycle_id WHERE (c.step IN ('prepare','answer') OR c.step LIKE 'retry:%') AND o.state='COMPLETED' AND s.document#>>'{build,phase}'='ready' AND s.document#>>'{build,cycle_id}'=c.cycle_id::text AND q.stopped_at IS NULL ORDER BY o.created_at LIMIT 20`)
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
