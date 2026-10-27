package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/evalset"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"strings"
)

type controlledEvalSetStore interface {
	AdmitControlledEvalSet(context.Context, repository.CreateEvalSetParams, uuid.UUID, string) (repository.EvalSet, error)
}

func (m *EvalSetManager) CreateControlled(ctx context.Context, caller Caller, input CreateEvalSessionInput) (repository.EvalSet, []uuid.UUID, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionCreateRun); err != nil {
		return repository.EvalSet{}, nil, err
	}
	store, ok := m.store.(controlledEvalSetStore)
	if !ok || m.sessions == nil {
		return repository.EvalSet{}, nil, fmt.Errorf("controlled Fleet admission unavailable")
	}
	if input.AdmissionRequestID == uuid.Nil || len(input.ControlledSettings) == 0 || input.ChallengeInputSetID == nil || len(input.EvalSession.RunMatrix) > 0 || len(input.EvalSession.SeedFanout) > 0 || input.EvalSession.Repetitions < 1 || input.EvalSession.Repetitions > 100 || len(input.Participants) == 0 {
		return repository.EvalSet{}, nil, repository.ErrControlledModeUnsupported
	}
	body, _ := json.Marshal(input)
	digest := sha256.Sum256(body)
	// Manifest describes only real deployment/repetition axes. The original fixed
	// scope is retained in expansion; no unused model or seed axis is advertised.
	manifest := evalset.Manifest{Schema: evalset.SchemaV1, Name: input.Name, Packs: []string{input.ChallengePackVersionID.String()}, Repeats: int(input.EvalSession.Repetitions), Limits: evalset.Limits{MaxConcurrentRuns: 1}}
	for _, p := range input.Participants {
		if p.AgentDeploymentID == nil || p.AgentBuildVersionID != nil {
			return repository.EvalSet{}, nil, repository.ErrControlledModeUnsupported
		}
		manifest.Agents = append(manifest.Agents, evalset.AgentEntry{Deployment: p.AgentDeploymentID.String(), Label: p.Label})
	}
	org, err := m.store.GetOrganizationIDByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return repository.EvalSet{}, nil, err
	}
	raw, _ := json.Marshal(manifest)
	set, err := store.AdmitControlledEvalSet(ctx, repository.CreateEvalSetParams{WorkspaceID: input.WorkspaceID, OrganizationID: org, Name: input.Name, Manifest: raw, Expansion: body, MaxConcurrentRuns: 1, CombinationCount: input.EvalSession.Repetitions * int32(len(input.Participants)), CreatedByUserID: &caller.UserID}, input.AdmissionRequestID, hex.EncodeToString(digest[:]))
	if err != nil {
		return set, nil, err
	}
	ids, _, err := m.store.ListEvalSessionsByEvalSetID(ctx, set.ID)
	if err != nil {
		return set, nil, err
	}
	if domain.IsEvalSetTerminal(set.Status) {
		return set, ids, nil
	}
	input.AdmissionRequestID = uuid.NewSHA1(set.ID, []byte(input.ChallengePackVersionID.String()))
	input.SkipWorkflowStart = true
	input.ControlledParentEvalSetID = set.ID
	input.PreviewOnly = false
	// Recover the canonical child before any mutable re-admission. A lost child
	// response, missing link, or lost Temporal acknowledgment reuses this identity.
	child, err := m.sessions.CreateEvalSession(ctx, caller, input)
	if err != nil {
		if errors.Is(err, repository.ErrControlledParentTerminal) {
			latest, getErr := m.store.GetEvalSetByID(ctx, set.ID)
			return latest, ids, getErr
		}
		var validation RunCreationValidationError
		var gate billing.GateError
		// These owners reject before committing a child. Preserve legacy
		// compensation for confirmed rejection, but never compensate a lost
		// response or unknown database/start outcome.
		if errors.As(err, &validation) || errors.As(err, &gate) || errors.Is(err, repository.ErrControlledSettingsChanged) || errors.Is(err, repository.ErrControlledModeUnsupported) || errors.Is(err, repository.ErrConnectorUnapproved) {
			m.compensatePartialCreate(ctx, set.ID, ids, err)
		}
		return set, ids, err
	}
	ids = []uuid.UUID{child.Session.ID}
	set, err = m.store.GetEvalSetByID(ctx, set.ID)
	if err != nil || domain.IsEvalSetTerminal(set.Status) {
		return set, ids, err
	}
	if m.starter != nil {
		err = m.starter.StartEvalSetWorkflow(ctx, set.ID)
	}
	return set, ids, err
}
func controlledEvalSetHandler(logger *slog.Logger, m *EvalSetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		if err = requireJSONContentType(r); err != nil {
			writeError(w, 415, "unsupported_media_type", err.Error())
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxCreateRunRequestBytes)
		input, err := decodeCreateEvalSessionRequest(r.Context(), r)
		if err != nil {
			writeCreateEvalSessionError(logger, w, r, err)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/preview") {
			input.PreviewOnly = true
			if m.sessions == nil {
				writeError(w, 503, "unavailable", "Fleet preview unavailable")
				return
			}
			result, e := m.sessions.CreateEvalSession(r.Context(), caller, input)
			if e != nil {
				if e == ErrForbidden {
					writeAuthzError(w, e)
				} else {
					writeCreateEvalSessionError(logger, w, r, e)
				}
				return
			}
			writeJSON(w, 200, result.Preview)
			return
		}
		set, ids, err := m.CreateControlled(r.Context(), caller, input)
		if err != nil {
			if err == ErrForbidden {
				writeAuthzError(w, err)
			} else {
				writeCreateEvalSessionError(logger, w, r, err)
			}
			return
		}
		writeJSON(w, 201, map[string]any{"eval_set": set, "eval_session_ids": ids})
	}
}
