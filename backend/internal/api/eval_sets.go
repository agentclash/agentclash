package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/agentclash/agentclash/runtime/evalset"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type expandEvalSetRequest struct {
	WorkspaceID string          `json:"workspace_id"`
	Manifest    json.RawMessage `json:"manifest"`
	MaxCombos   int             `json:"max_combinations,omitempty"`
}

// EvalSetManager handles eval-set expansion and persistence.
type EvalSetManager struct {
	authorizer  WorkspaceAuthorizer
	store       EvalSetStore
	sessions    EvalSessionCreator
	starter     EvalSetWorkflowStarter
	cases       CaseResultsStore
	findings    ScanFindingStore
	scanStarter ScanWorkflowStarter
}

func NewEvalSetManager(authorizer WorkspaceAuthorizer) *EvalSetManager {
	return &EvalSetManager{authorizer: authorizer}
}

// configuredEvalSetManager is set from api-server main before NewServer builds routes.
var configuredEvalSetManager *EvalSetManager

// ConfigureEvalSetManager installs the process-wide eval-set manager used by routes.
func ConfigureEvalSetManager(manager *EvalSetManager) {
	configuredEvalSetManager = manager
}

func evalSetManagerOrDefault(authorizer WorkspaceAuthorizer) *EvalSetManager {
	if configuredEvalSetManager != nil {
		return configuredEvalSetManager
	}
	return NewEvalSetManager(authorizer)
}

type ExpandEvalSetResult struct {
	Report   evalset.ExpansionReport `json:"report"`
	Estimate evalset.CostEstimate    `json:"estimate"`
}

func (m *EvalSetManager) Expand(ctx context.Context, caller Caller, workspaceID uuid.UUID, manifestJSON json.RawMessage, maxCombos int) (evalset.ExpansionReport, error) {
	result, err := m.ExpandWithEstimate(ctx, caller, workspaceID, manifestJSON, maxCombos)
	if err != nil {
		return evalset.ExpansionReport{}, err
	}
	return result.Report, nil
}

func (m *EvalSetManager) ExpandWithEstimate(ctx context.Context, caller Caller, workspaceID uuid.UUID, manifestJSON json.RawMessage, maxCombos int) (ExpandEvalSetResult, error) {
	if m == nil || m.authorizer == nil {
		return ExpandEvalSetResult{}, errors.New("eval set manager is not configured")
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return ExpandEvalSetResult{}, err
	}
	manifest, err := evalset.ParseManifest(manifestJSON)
	if err != nil {
		return ExpandEvalSetResult{}, err
	}
	if maxCombos <= 0 {
		maxCombos = evalset.DefaultMaxCombos
	}
	if maxCombos > evalset.MaxAllowedCombos {
		return ExpandEvalSetResult{}, fmt.Errorf("max_combinations %d exceeds server limit %d", maxCombos, evalset.MaxAllowedCombos)
	}
	report, err := manifest.Expand(maxCombos)
	if err != nil {
		return ExpandEvalSetResult{}, err
	}
	budget := budgetPtr(report.BudgetUSD)
	estimate := evalset.EstimateCost(report, budget, manifest.Models)
	return ExpandEvalSetResult{Report: report, Estimate: estimate}, nil
}

func registerEvalSetRoutes(router chi.Router, logger *slog.Logger, manager *EvalSetManager) {
	if manager == nil {
		return
	}
	router.Post("/eval-sets/expand", expandEvalSetHandler(logger, manager))
	router.Post("/eval-sets", createEvalSetHandler(logger, manager))
	router.Get("/eval-sets", listEvalSetsHandler(logger, manager))
	router.Get("/eval-sets/{evalSetID}", getEvalSetHandler(logger, manager))
	router.Post("/eval-sets/{evalSetID}/cancel", cancelEvalSetHandler(logger, manager))
	registerEvalSetWarehouseRoutes(router, logger, manager)
	registerEmergencyStopRoute(router, logger, manager)
	registerScanFindingRoutes(router, logger, manager)
}

func expandEvalSetHandler(_ *slog.Logger, manager *EvalSetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "failed to read request body")
			return
		}
		var req expandEvalSetRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		workspaceID, err := uuid.Parse(req.WorkspaceID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace_id must be a valid UUID")
			return
		}
		if len(req.Manifest) == 0 {
			writeError(w, http.StatusBadRequest, "manifest_required", "manifest is required")
			return
		}
		result, err := manager.ExpandWithEstimate(r.Context(), caller, workspaceID, req.Manifest, req.MaxCombos)
		if err != nil {
			switch {
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				// Validation / parse errors are actionable 400s.
				writeError(w, http.StatusBadRequest, "evalset_expand_invalid", err.Error())
			}
			return
		}
		// Keep prior ExpansionReport fields at the top level for CLI compat;
		// attach estimate + warning when budget would be breached.
		payload := map[string]any{
			"name":                result.Report.Name,
			"combinations":        result.Report.Combinations,
			"count":               result.Report.Count,
			"pack_count":          result.Report.PackCount,
			"agent_count":         result.Report.AgentCount,
			"model_count":         result.Report.ModelCount,
			"repeats":             result.Report.Repeats,
			"max_concurrent_runs": result.Report.MaxConcurrent,
			"budget_usd":          result.Report.BudgetUSD,
			"case_fanout":         result.Report.CaseFanout,
			"estimate":            result.Estimate,
		}
		if result.Estimate.ExceedsBudget {
			payload["warning"] = "estimated_cost_exceeds_budget"
		}
		writeJSON(w, http.StatusOK, payload)
	}
}
