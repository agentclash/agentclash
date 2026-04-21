package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/google/uuid"
)

type createEvalSessionRequest struct {
	WorkspaceID            string                         `json:"workspace_id"`
	ChallengePackVersionID string                         `json:"challenge_pack_version_id"`
	ChallengeInputSetID    *string                        `json:"challenge_input_set_id,omitempty"`
	Participants           []createEvalSessionParticipant `json:"participants"`
	ExecutionMode          string                         `json:"execution_mode"`
	Name                   string                         `json:"name,omitempty"`
	EvalSession            json.RawMessage                `json:"eval_session"`
}

type createEvalSessionParticipant struct {
	AgentDeploymentID   string `json:"agent_deployment_id"`
	AgentBuildVersionID string `json:"agent_build_version_id"`
	Label               string `json:"label"`
}

type createEvalSessionResponse struct {
	EvalSession evalSessionResponse `json:"eval_session"`
	RunIDs      []uuid.UUID         `json:"run_ids"`
}

type evalSessionResponse struct {
	ID                     uuid.UUID                `json:"id"`
	Status                 domain.EvalSessionStatus `json:"status"`
	Repetitions            int32                    `json:"repetitions"`
	AggregationConfig      json.RawMessage          `json:"aggregation_config"`
	SuccessThresholdConfig json.RawMessage          `json:"success_threshold_config"`
	RoutingTaskSnapshot    json.RawMessage          `json:"routing_task_snapshot"`
	SchemaVersion          int32                    `json:"schema_version"`
	CreatedAt              time.Time                `json:"created_at"`
	StartedAt              *time.Time               `json:"started_at,omitempty"`
	FinishedAt             *time.Time               `json:"finished_at,omitempty"`
	UpdatedAt              time.Time                `json:"updated_at"`
}

type evalSessionValidationDetail struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type evalSessionValidationError struct {
	Errors []evalSessionValidationDetail
}

func (e evalSessionValidationError) Error() string {
	return "eval session request has validation errors"
}

type evalSessionValidationEnvelope struct {
	Errors []evalSessionValidationDetail `json:"errors"`
}

func createEvalSessionHandler(logger *slog.Logger, service RunCreationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxCreateRunRequestBytes)
		input, err := decodeCreateEvalSessionRequest(r.Context(), r)
		if err != nil {
			writeCreateEvalSessionError(logger, w, r, err)
			return
		}

		result, err := service.CreateEvalSession(r.Context(), caller, input)
		if err != nil {
			if errors.Is(err, ErrForbidden) {
				writeAuthzError(w, err)
				return
			}
			writeCreateEvalSessionError(logger, w, r, err)
			return
		}

		writeJSON(w, http.StatusCreated, createEvalSessionResponse{
			EvalSession: buildEvalSessionResponse(result.Session),
			RunIDs:      append([]uuid.UUID(nil), result.RunIDs...),
		})
	}
}

func writeCreateEvalSessionError(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	var validationErr RunCreationValidationError
	if errors.As(err, &validationErr) {
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
		return
	}

	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body must be 1 MiB or smaller")
		return
	}

	var evalValidationErr evalSessionValidationError
	if errors.As(err, &evalValidationErr) {
		writeJSON(w, http.StatusUnprocessableEntity, evalSessionValidationEnvelope{
			Errors: append([]evalSessionValidationDetail(nil), evalValidationErr.Errors...),
		})
		return
	}

	logger.Error("create eval session request failed",
		"method", r.Method,
		"path", r.URL.Path,
		"error", err,
	)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func buildEvalSessionResponse(session domain.EvalSession) evalSessionResponse {
	return evalSessionResponse{
		ID:                     session.ID,
		Status:                 session.Status,
		Repetitions:            session.Repetitions,
		AggregationConfig:      cloneJSON(session.AggregationConfig.Document),
		SuccessThresholdConfig: cloneJSON(session.SuccessThresholdConfig.Document),
		RoutingTaskSnapshot:    cloneJSON(session.RoutingTaskSnapshot.Document),
		SchemaVersion:          session.SchemaVersion,
		CreatedAt:              session.CreatedAt,
		StartedAt:              cloneTimePtr(session.StartedAt),
		FinishedAt:             cloneTimePtr(session.FinishedAt),
		UpdatedAt:              session.UpdatedAt,
	}
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func decodeCreateEvalSessionRequest(_ context.Context, r *http.Request) (CreateEvalSessionInput, error) {
	var body createEvalSessionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return CreateEvalSessionInput{}, err
		}
		if errors.Is(err, io.EOF) {
			return CreateEvalSessionInput{}, RunCreationValidationError{
				Code:    "invalid_request",
				Message: "request body is required",
			}
		}
		return CreateEvalSessionInput{}, RunCreationValidationError{
			Code:    "invalid_request",
			Message: "request body must be valid JSON",
		}
	}
	if decoder.More() {
		return CreateEvalSessionInput{}, RunCreationValidationError{
			Code:    "invalid_request",
			Message: "request body must contain exactly one JSON object",
		}
	}

	workspaceID, err := parseRequiredUUID(body.WorkspaceID, "workspace_id", "invalid_workspace_id")
	if err != nil {
		return CreateEvalSessionInput{}, err
	}
	challengePackVersionID, err := parseRequiredUUID(body.ChallengePackVersionID, "challenge_pack_version_id", "invalid_challenge_pack_version_id")
	if err != nil {
		return CreateEvalSessionInput{}, err
	}

	var challengeInputSetID *uuid.UUID
	if body.ChallengeInputSetID != nil && strings.TrimSpace(*body.ChallengeInputSetID) != "" {
		parsedID, parseErr := parseRequiredUUID(*body.ChallengeInputSetID, "challenge_input_set_id", "invalid_challenge_input_set_id")
		if parseErr != nil {
			return CreateEvalSessionInput{}, parseErr
		}
		challengeInputSetID = &parsedID
	}

	if len(body.EvalSession) == 0 || bytes.Equal(bytes.TrimSpace(body.EvalSession), []byte("null")) {
		return CreateEvalSessionInput{}, RunCreationValidationError{
			Code:    "missing_eval_session",
			Message: "eval_session is required",
		}
	}

	participants := make([]EvalSessionParticipantInput, 0, len(body.Participants))
	for _, participant := range body.Participants {
		var deploymentID *uuid.UUID
		if strings.TrimSpace(participant.AgentDeploymentID) != "" {
			parsedDeploymentID, parseErr := parseRequiredUUID(participant.AgentDeploymentID, "participants.agent_deployment_id", "invalid_participants")
			if parseErr != nil {
				return CreateEvalSessionInput{}, RunCreationValidationError{
					Code:    "invalid_participants",
					Message: "participants must contain valid agent_deployment_id values",
				}
			}
			deploymentID = &parsedDeploymentID
		}

		var buildVersionID *uuid.UUID
		if deploymentID == nil && strings.TrimSpace(participant.AgentBuildVersionID) != "" {
			parsedBuildVersionID, parseErr := parseRequiredUUID(participant.AgentBuildVersionID, "participants.agent_build_version_id", "invalid_participants")
			if parseErr != nil {
				return CreateEvalSessionInput{}, RunCreationValidationError{
					Code:    "invalid_participants",
					Message: "participants must contain valid agent_deployment_id values",
				}
			}
			buildVersionID = &parsedBuildVersionID
		}
		if deploymentID == nil && buildVersionID == nil {
			return CreateEvalSessionInput{}, RunCreationValidationError{
				Code:    "invalid_participants",
				Message: "participants must contain valid agent_deployment_id values",
			}
		}

		label := strings.TrimSpace(participant.Label)
		if label == "" {
			return CreateEvalSessionInput{}, RunCreationValidationError{
				Code:    "invalid_participants",
				Message: "participants must contain non-empty labels",
			}
		}
		participants = append(participants, EvalSessionParticipantInput{
			AgentDeploymentID:   deploymentID,
			AgentBuildVersionID: buildVersionID,
			Label:               label,
		})
	}

	config, err := decodeEvalSessionConfig(body.EvalSession)
	if err != nil {
		return CreateEvalSessionInput{}, err
	}

	return CreateEvalSessionInput{
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		ChallengeInputSetID:    challengeInputSetID,
		Participants:           participants,
		ExecutionMode:          strings.TrimSpace(body.ExecutionMode),
		Name:                   strings.TrimSpace(body.Name),
		EvalSession:            config,
	}, nil
}

func decodeEvalSessionConfig(raw json.RawMessage) (CreateEvalSessionConfigInput, error) {
	type evalSessionConfigRequest struct {
		Repetitions         json.RawMessage `json:"repetitions"`
		Aggregation         json.RawMessage `json:"aggregation"`
		SuccessThreshold    json.RawMessage `json:"success_threshold"`
		RoutingTaskSnapshot json.RawMessage `json:"routing_task_snapshot"`
		ReliabilityWeights  json.RawMessage `json:"reliability_weights"`
		SchemaVersion       json.RawMessage `json:"schema_version"`
	}

	var body evalSessionConfigRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return CreateEvalSessionConfigInput{}, RunCreationValidationError{
			Code:    "invalid_request",
			Message: "eval_session must be a valid JSON object",
		}
	}
	if decoder.More() {
		return CreateEvalSessionConfigInput{}, RunCreationValidationError{
			Code:    "invalid_request",
			Message: "eval_session must contain exactly one JSON object",
		}
	}

	details := make([]evalSessionValidationDetail, 0)

	repetitions, ok := decodeRequiredInt32(body.Repetitions)
	if !ok || repetitions < 1 || repetitions > 100 {
		details = append(details, evalSessionValidationDetail{
			Field:   "eval_session.repetitions",
			Code:    "eval_session.repetitions.invalid",
			Message: "repetitions must be an integer between 1 and 100",
		})
	}

	schemaVersion, ok := decodeRequiredInt32(body.SchemaVersion)
	if !ok || schemaVersion < 1 {
		details = append(details, evalSessionValidationDetail{
			Field:   "eval_session.schema_version",
			Code:    "eval_session.schema_version.invalid",
			Message: "schema_version must be an integer greater than or equal to 1",
		})
	}

	aggregation, aggregationDetails := decodeEvalSessionAggregation(body.Aggregation)
	details = append(details, aggregationDetails...)

	successThreshold, successDetails := decodeEvalSessionSuccessThreshold(body.SuccessThreshold)
	details = append(details, successDetails...)

	routingTaskSnapshot, routingDetails := decodeEvalSessionRoutingTaskSnapshot(body.RoutingTaskSnapshot)
	details = append(details, routingDetails...)

	reliabilityWeights, reliabilityDetails := decodeEvalSessionReliabilityWeights(body.ReliabilityWeights)
	details = append(details, reliabilityDetails...)

	if aggregation.Method == "weighted_mean" && reliabilityWeights == nil {
		details = append(details, evalSessionValidationDetail{
			Field:   "eval_session.reliability_weights",
			Code:    "eval_session.reliability_weights.required",
			Message: "weighted_mean requires at least one reliability_weights section",
		})
	}

	if len(details) > 0 {
		return CreateEvalSessionConfigInput{}, evalSessionValidationError{Errors: details}
	}

	return CreateEvalSessionConfigInput{
		Repetitions:         repetitions,
		Aggregation:         aggregation,
		SuccessThreshold:    successThreshold,
		RoutingTaskSnapshot: routingTaskSnapshot,
		ReliabilityWeights:  reliabilityWeights,
		SchemaVersion:       schemaVersion,
	}, nil
}

func decodeEvalSessionAggregation(raw json.RawMessage) (EvalSessionAggregationInput, []evalSessionValidationDetail) {
	type aggregationRequest struct {
		Method             json.RawMessage `json:"method"`
		ReportVariance     json.RawMessage `json:"report_variance"`
		ConfidenceInterval json.RawMessage `json:"confidence_interval"`
		ReliabilityWeight  json.RawMessage `json:"reliability_weight"`
	}

	var body aggregationRequest
	if !decodeJSONObject(raw, &body) {
		return EvalSessionAggregationInput{}, []evalSessionValidationDetail{{
			Field:   "eval_session.aggregation.method",
			Code:    "eval_session.aggregation.method.unsupported",
			Message: "aggregation must be an object with supported method, report_variance, and confidence_interval fields",
		}}
	}

	details := make([]evalSessionValidationDetail, 0)
	method, ok := decodeRequiredString(body.Method)
	if !ok || (method != "median" && method != "mean" && method != "weighted_mean") {
		details = append(details, evalSessionValidationDetail{
			Field:   "eval_session.aggregation.method",
			Code:    "eval_session.aggregation.method.unsupported",
			Message: "aggregation.method must be one of median, mean, or weighted_mean",
		})
	}

	reportVariance, ok := decodeRequiredBool(body.ReportVariance)
	if !ok {
		details = append(details, evalSessionValidationDetail{
			Field:   "eval_session.aggregation.report_variance",
			Code:    "eval_session.aggregation.report_variance.invalid",
			Message: "aggregation.report_variance must be a boolean",
		})
	}

	confidenceInterval, ok := decodeRequiredFloat64(body.ConfidenceInterval)
	if !ok || confidenceInterval <= 0 || confidenceInterval >= 1 {
		details = append(details, evalSessionValidationDetail{
			Field:   "eval_session.aggregation.confidence_interval",
			Code:    "eval_session.aggregation.confidence_interval.invalid",
			Message: "aggregation.confidence_interval must be a float between 0 and 1, exclusive",
		})
	}

	var reliabilityWeight *float64
	if len(bytes.TrimSpace(body.ReliabilityWeight)) > 0 {
		value, ok := decodeRequiredFloat64(body.ReliabilityWeight)
		if !ok || value < 0 || value > 1 {
			details = append(details, evalSessionValidationDetail{
				Field:   "eval_session.aggregation.reliability_weight",
				Code:    "eval_session.aggregation.reliability_weight.invalid",
				Message: "aggregation.reliability_weight must be a float between 0 and 1",
			})
		} else {
			reliabilityWeight = &value
		}
	}

	return EvalSessionAggregationInput{
		Method:             method,
		ReportVariance:     reportVariance,
		ConfidenceInterval: confidenceInterval,
		ReliabilityWeight:  reliabilityWeight,
	}, details
}

func decodeEvalSessionSuccessThreshold(raw json.RawMessage) (*EvalSessionSuccessThresholdInput, []evalSessionValidationDetail) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	type successThresholdRequest struct {
		MinPassRate          json.RawMessage `json:"min_pass_rate"`
		RequireAllDimensions json.RawMessage `json:"require_all_dimensions"`
	}

	var body successThresholdRequest
	if !decodeJSONObject(raw, &body) {
		return nil, []evalSessionValidationDetail{{
			Field:   "eval_session.success_threshold.min_pass_rate",
			Code:    "eval_session.success_threshold.min_pass_rate.invalid",
			Message: "success_threshold must be an object",
		}}
	}

	details := make([]evalSessionValidationDetail, 0)
	minPassRate, ok := decodeRequiredFloat64(body.MinPassRate)
	if !ok || minPassRate < 0 || minPassRate > 1 {
		details = append(details, evalSessionValidationDetail{
			Field:   "eval_session.success_threshold.min_pass_rate",
			Code:    "eval_session.success_threshold.min_pass_rate.invalid",
			Message: "success_threshold.min_pass_rate must be a float between 0 and 1",
		})
	}

	requireAllDimensions := []string{}
	if len(bytes.TrimSpace(body.RequireAllDimensions)) > 0 {
		var rawDimensions []string
		if err := json.Unmarshal(body.RequireAllDimensions, &rawDimensions); err != nil {
			details = append(details, evalSessionValidationDetail{
				Field:   "eval_session.success_threshold.require_all_dimensions",
				Code:    "eval_session.success_threshold.require_all_dimensions.invalid",
				Message: "success_threshold.require_all_dimensions must be an array of non-empty strings",
			})
		} else {
			for _, dimension := range rawDimensions {
				trimmed := strings.TrimSpace(dimension)
				if trimmed == "" {
					details = append(details, evalSessionValidationDetail{
						Field:   "eval_session.success_threshold.require_all_dimensions",
						Code:    "eval_session.success_threshold.require_all_dimensions.invalid",
						Message: "success_threshold.require_all_dimensions must be an array of non-empty strings",
					})
					break
				}
				requireAllDimensions = append(requireAllDimensions, trimmed)
			}
		}
	}

	if len(details) > 0 {
		return nil, details
	}

	return &EvalSessionSuccessThresholdInput{
		MinPassRate:          minPassRate,
		RequireAllDimensions: requireAllDimensions,
	}, nil
}

func decodeEvalSessionRoutingTaskSnapshot(raw json.RawMessage) (EvalSessionRoutingTaskSnapshotInput, []evalSessionValidationDetail) {
	var body map[string]json.RawMessage
	if !decodeJSONObject(raw, &body) {
		return EvalSessionRoutingTaskSnapshotInput{}, []evalSessionValidationDetail{{
			Field:   "eval_session.routing_task_snapshot",
			Code:    "eval_session.routing_task_snapshot.invalid",
			Message: "routing_task_snapshot must be an object with routing and task keys",
		}}
	}

	routingRaw, routingOK := body["routing"]
	taskRaw, taskOK := body["task"]
	if !routingOK || !taskOK || !jsonIsObject(routingRaw) || !jsonIsObject(taskRaw) {
		return EvalSessionRoutingTaskSnapshotInput{}, []evalSessionValidationDetail{{
			Field:   "eval_session.routing_task_snapshot",
			Code:    "eval_session.routing_task_snapshot.invalid",
			Message: "routing_task_snapshot must contain routing and task objects",
		}}
	}

	return EvalSessionRoutingTaskSnapshotInput{
		Routing: cloneJSON(routingRaw),
		Task:    cloneJSON(taskRaw),
	}, nil
}

func decodeEvalSessionReliabilityWeights(raw json.RawMessage) (*EvalSessionReliabilityWeightsInput, []evalSessionValidationDetail) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	type perRunRequest struct {
		Policy json.RawMessage `json:"policy"`
		Params json.RawMessage `json:"params"`
	}

	type reliabilityWeightsRequest struct {
		PerDimension json.RawMessage `json:"per_dimension"`
		PerJudge     json.RawMessage `json:"per_judge"`
		PerRun       json.RawMessage `json:"per_run"`
	}

	var body reliabilityWeightsRequest
	if !decodeJSONObject(raw, &body) {
		return nil, []evalSessionValidationDetail{{
			Field:   "eval_session.reliability_weights.required",
			Code:    "eval_session.reliability_weights.required",
			Message: "reliability_weights must be an object",
		}}
	}

	details := make([]evalSessionValidationDetail, 0)
	result := &EvalSessionReliabilityWeightsInput{}
	hasAny := false

	if len(bytes.TrimSpace(body.PerDimension)) > 0 {
		hasAny = true
		var perDimension map[string]float64
		if err := json.Unmarshal(body.PerDimension, &perDimension); err != nil || len(perDimension) == 0 {
			details = append(details, evalSessionValidationDetail{
				Field:   "eval_session.reliability_weights.per_dimension",
				Code:    "eval_session.reliability_weights.per_dimension.invalid",
				Message: "reliability_weights.per_dimension must be a non-empty object with values between 0 and 1",
			})
		} else {
			sum := 0.0
			for key, value := range perDimension {
				if strings.TrimSpace(key) == "" || value < 0 || value > 1 {
					details = append(details, evalSessionValidationDetail{
						Field:   "eval_session.reliability_weights.per_dimension",
						Code:    "eval_session.reliability_weights.per_dimension.invalid",
						Message: "reliability_weights.per_dimension must be a non-empty object with values between 0 and 1",
					})
					break
				}
				sum += value
			}
			if sum <= 0 {
				details = append(details, evalSessionValidationDetail{
					Field:   "eval_session.reliability_weights.per_dimension",
					Code:    "eval_session.reliability_weights.per_dimension.invalid",
					Message: "reliability_weights.per_dimension must sum to more than 0",
				})
			}
			result.PerDimension = perDimension
		}
	}

	if len(bytes.TrimSpace(body.PerJudge)) > 0 {
		hasAny = true
		var perJudge map[string]float64
		if err := json.Unmarshal(body.PerJudge, &perJudge); err != nil {
			details = append(details, evalSessionValidationDetail{
				Field:   "eval_session.reliability_weights.per_judge",
				Code:    "eval_session.reliability_weights.per_judge.invalid",
				Message: "reliability_weights.per_judge must be an object with non-negative numeric values",
			})
		} else {
			for key, value := range perJudge {
				if strings.TrimSpace(key) == "" || value < 0 {
					details = append(details, evalSessionValidationDetail{
						Field:   "eval_session.reliability_weights.per_judge",
						Code:    "eval_session.reliability_weights.per_judge.invalid",
						Message: "reliability_weights.per_judge must be an object with non-negative numeric values",
					})
					break
				}
			}
			result.PerJudge = perJudge
		}
	}

	if len(bytes.TrimSpace(body.PerRun)) > 0 {
		hasAny = true
		var perRunBody perRunRequest
		if !decodeJSONObject(body.PerRun, &perRunBody) {
			details = append(details, evalSessionValidationDetail{
				Field:   "eval_session.reliability_weights.per_run.policy",
				Code:    "eval_session.reliability_weights.per_run.policy.unsupported",
				Message: "reliability_weights.per_run must be an object with a supported policy",
			})
		} else {
			policy, ok := decodeRequiredString(perRunBody.Policy)
			if !ok || (policy != "equal" && policy != "downweight_outliers") {
				details = append(details, evalSessionValidationDetail{
					Field:   "eval_session.reliability_weights.per_run.policy",
					Code:    "eval_session.reliability_weights.per_run.policy.unsupported",
					Message: "reliability_weights.per_run.policy must be equal or downweight_outliers",
				})
			}
			params := json.RawMessage(`{}`)
			if len(bytes.TrimSpace(perRunBody.Params)) > 0 {
				if !jsonIsObject(perRunBody.Params) {
					details = append(details, evalSessionValidationDetail{
						Field:   "eval_session.reliability_weights.per_run.policy",
						Code:    "eval_session.reliability_weights.per_run.policy.unsupported",
						Message: "reliability_weights.per_run.params must be an object when present",
					})
				} else {
					params = cloneJSON(perRunBody.Params)
				}
			}
			result.PerRun = &EvalSessionPerRunReliabilityInput{
				Policy: policy,
				Params: params,
			}
		}
	}

	if !hasAny {
		return nil, nil
	}
	if len(details) > 0 {
		return nil, details
	}
	return result, nil
}

func decodeJSONObject(raw json.RawMessage, dest any) bool {
	if !jsonIsObject(raw) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dest) == nil
}

func jsonIsObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 1 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}'
}

func decodeRequiredInt32(raw json.RawMessage) (int32, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return 0, false
	}
	var value int32
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, false
	}
	return value, true
}

func decodeRequiredFloat64(raw json.RawMessage) (float64, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return 0, false
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, false
	}
	return value, true
}

func decodeRequiredBool(raw json.RawMessage) (bool, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false, false
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, false
	}
	return value, true
}

func decodeRequiredString(raw json.RawMessage) (string, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}
