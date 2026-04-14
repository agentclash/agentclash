package scoring

import (
	"fmt"
	"strings"
)

// validateLLMJudges runs the LLMJudgeDeclaration-specific validation rules
// for issue #148 phase 1. It returns the collected errors; the caller
// merges them into the top-level ValidationErrors slice.
//
// The validator takes the already-collected validatorKeys and metricKeys
// maps so it can reject judge keys that collide with other scope namespaces.
// A single key like "correctness" must unambiguously identify one validator,
// one metric, OR one judge — never more than one — so dimension dispatch
// can look up evidence by key without ambiguity.
func validateLLMJudges(
	spec EvaluationSpec,
	validatorKeys map[string]struct{},
	metricKeys map[string]struct{},
) (map[string]JudgeMethodMode, ValidationErrors) {
	judgeKeys := make(map[string]JudgeMethodMode, len(spec.LLMJudges))
	var errs ValidationErrors

	for i, judge := range spec.LLMJudges {
		path := fmt.Sprintf("evaluation_spec.llm_judges[%d]", i)

		// Rule 1: key uniqueness + no cross-namespace collision.
		key := strings.TrimSpace(judge.Key)
		if key == "" {
			errs = append(errs, ValidationError{Field: path + ".key", Message: "is required"})
			continue
		}
		if _, exists := judgeKeys[key]; exists {
			errs = append(errs, ValidationError{Field: path + ".key", Message: "must be unique"})
			continue
		}
		if _, exists := validatorKeys[key]; exists {
			errs = append(errs, ValidationError{
				Field:   path + ".key",
				Message: fmt.Sprintf("collides with validator key %q", key),
			})
			continue
		}
		if _, exists := metricKeys[key]; exists {
			errs = append(errs, ValidationError{
				Field:   path + ".key",
				Message: fmt.Sprintf("collides with metric key %q", key),
			})
			continue
		}
		judgeKeys[key] = judge.Mode

		// Rule 2: mode required and valid.
		if !judge.Mode.IsValid() {
			errs = append(errs, ValidationError{
				Field:   path + ".mode",
				Message: "must be one of rubric, assertion, n_wise, reference",
			})
			continue
		}

		// Rule 3: mode-specific required fields.
		switch judge.Mode {
		case JudgeMethodRubric:
			if strings.TrimSpace(judge.Rubric) == "" {
				errs = append(errs, ValidationError{
					Field:   path + ".rubric",
					Message: "is required for rubric mode",
				})
			}
		case JudgeMethodReference:
			if strings.TrimSpace(judge.Rubric) == "" {
				errs = append(errs, ValidationError{
					Field:   path + ".rubric",
					Message: "is required for reference mode",
				})
			}
			refFrom := strings.TrimSpace(judge.ReferenceFrom)
			if refFrom == "" {
				errs = append(errs, ValidationError{
					Field:   path + ".reference_from",
					Message: "is required for reference mode",
				})
			} else if !isSupportedEvidenceReference(refFrom) {
				errs = append(errs, ValidationError{
					Field:   path + ".reference_from",
					Message: "must be a supported evidence reference",
				})
			}
		case JudgeMethodAssertion:
			if strings.TrimSpace(judge.Assertion) == "" {
				errs = append(errs, ValidationError{
					Field:   path + ".assertion",
					Message: "is required for assertion mode",
				})
			}
		case JudgeMethodNWise:
			if strings.TrimSpace(judge.Prompt) == "" {
				errs = append(errs, ValidationError{
					Field:   path + ".prompt",
					Message: "is required for n_wise mode",
				})
			}
		}

		// Rule 4: fan-out — exactly one of Model or Models (non-empty).
		hasSingleModel := strings.TrimSpace(judge.Model) != ""
		hasMultiModel := len(judge.Models) > 0
		switch {
		case !hasSingleModel && !hasMultiModel:
			errs = append(errs, ValidationError{
				Field:   path + ".model",
				Message: "must set exactly one of model or models",
			})
		case hasSingleModel && hasMultiModel:
			errs = append(errs, ValidationError{
				Field:   path + ".models",
				Message: "must set exactly one of model or models, not both",
			})
		case hasMultiModel:
			for j, m := range judge.Models {
				if strings.TrimSpace(m) == "" {
					errs = append(errs, ValidationError{
						Field:   fmt.Sprintf("%s.models[%d]", path, j),
						Message: "must be a non-empty model identifier",
					})
				}
			}
		}

		// Rule 5: samples range [0, JudgeMaxSamplesCeiling]. 0 is
		// normalised to JudgeDefaultSamples in normalizeEvaluationSpec;
		// we reject the ceiling breach unconditionally so a malicious
		// spec cannot trivially escalate the LLM call count.
		if judge.Samples < 0 {
			errs = append(errs, ValidationError{
				Field:   path + ".samples",
				Message: "must be greater than or equal to 0",
			})
		} else if judge.Samples > JudgeMaxSamplesCeiling {
			errs = append(errs, ValidationError{
				Field:   path + ".samples",
				Message: fmt.Sprintf("must be at most %d", JudgeMaxSamplesCeiling),
			})
		}

		// Rule 6: consensus rules — required when len(Models) > 1,
		// aggregation must match the mode's output shape.
		if hasMultiModel && len(judge.Models) > 1 {
			if judge.Consensus == nil {
				errs = append(errs, ValidationError{
					Field:   path + ".consensus",
					Message: "is required when multiple models are declared",
				})
			} else if err := validateConsensusForMode(judge.Mode, *judge.Consensus, path+".consensus"); len(err) > 0 {
				errs = append(errs, err...)
			}
		} else if judge.Consensus != nil {
			errs = append(errs, ValidationError{
				Field:   path + ".consensus",
				Message: "is only valid when multiple models are declared",
			})
		}

		// Rule 7: ContextFrom entries must be supported evidence refs.
		for j, ctx := range judge.ContextFrom {
			if !isSupportedEvidenceReference(ctx) {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("%s.context_from[%d]", path, j),
					Message: "must be a supported evidence reference",
				})
			}
		}

		// Rule 8: OutputSchema, when non-empty, must parse as JSON
		// Schema draft-07 or 2020-12 via the existing parseJSONSchema
		// helper. Reuse of the helper keeps the judge validator
		// consistent with the json_schema validator type.
		if len(judge.OutputSchema) > 0 {
			if _, _, err := parseJSONSchema(string(judge.OutputSchema)); err != nil {
				errs = append(errs, ValidationError{
					Field:   path + ".output_schema",
					Message: fmt.Sprintf("must be valid JSON Schema: %v", err),
				})
			}
		}

		// Rule 9: secret leakage guard. Rubric/Assertion/Prompt text
		// goes into every LLM call for this judge — if a pack
		// interpolates ${secrets.*} references into them, the secrets
		// leak into provider logs and judge payloads. Reject at load
		// time so the evaluator never has to handle this.
		for _, check := range []struct {
			field string
			value string
		}{
			{path + ".rubric", judge.Rubric},
			{path + ".assertion", judge.Assertion},
			{path + ".prompt", judge.Prompt},
		} {
			if containsSecretReference(check.value) {
				errs = append(errs, ValidationError{
					Field:   check.field,
					Message: "must not contain ${secrets.*} references; secrets are not allowed in judge prompt text",
				})
			}
		}

		// Rule 10: ScoreScale bounds.
		if judge.ScoreScale != nil {
			if judge.ScoreScale.Min >= judge.ScoreScale.Max {
				errs = append(errs, ValidationError{
					Field:   path + ".score_scale",
					Message: "min must be strictly less than max",
				})
			}
		}

		// Rule 11: TimeoutMS sanity.
		if judge.TimeoutMS != nil && *judge.TimeoutMS <= 0 {
			errs = append(errs, ValidationError{
				Field:   path + ".timeout_ms",
				Message: "must be greater than 0",
			})
		}
	}

	// JudgeLimits range checks (scorecard-level).
	if limits := spec.Scorecard.JudgeLimits; limits != nil {
		if limits.MaxSamplesPerJudge < 0 || limits.MaxSamplesPerJudge > JudgeMaxSamplesCeiling {
			errs = append(errs, ValidationError{
				Field:   "evaluation_spec.scorecard.judge_limits.max_samples_per_judge",
				Message: fmt.Sprintf("must be between 0 and %d", JudgeMaxSamplesCeiling),
			})
		}
		if limits.MaxCallsUSD < 0 {
			errs = append(errs, ValidationError{
				Field:   "evaluation_spec.scorecard.judge_limits.max_calls_usd",
				Message: "must be greater than or equal to 0",
			})
		}
		if limits.MaxTokens < 0 {
			errs = append(errs, ValidationError{
				Field:   "evaluation_spec.scorecard.judge_limits.max_tokens",
				Message: "must be greater than or equal to 0",
			})
		}
	}

	return judgeKeys, errs
}

// validateConsensusForMode enforces that the aggregation strategy matches the
// mode's output shape: numeric modes accept median/mean; assertion accepts
// majority_vote/unanimous. Cross-shape combinations are nonsense and we
// reject them at load time instead of picking a silent fallback at runtime.
func validateConsensusForMode(mode JudgeMethodMode, cfg ConsensusConfig, path string) ValidationErrors {
	var errs ValidationErrors
	if !cfg.Aggregation.IsValid() {
		errs = append(errs, ValidationError{
			Field:   path + ".aggregation",
			Message: "must be one of median, mean, majority_vote, unanimous",
		})
		return errs
	}
	switch cfg.Aggregation {
	case ConsensusAggMedian, ConsensusAggMean:
		if mode.IsBooleanScope() {
			errs = append(errs, ValidationError{
				Field:   path + ".aggregation",
				Message: fmt.Sprintf("%q aggregation is only valid for numeric modes (rubric, reference, n_wise)", cfg.Aggregation),
			})
		}
	case ConsensusAggMajorityVote:
		if !mode.IsBooleanScope() {
			errs = append(errs, ValidationError{
				Field:   path + ".aggregation",
				Message: "majority_vote aggregation is only valid for assertion mode",
			})
		}
	case ConsensusAggUnanimous:
		// Unanimous is meaningful for BOTH shapes: numeric unanimous
		// means "all per-model scores agree within MinAgreementThreshold",
		// boolean unanimous means "all per-model verdicts identical".
		// No extra shape check needed.
	}
	if cfg.MinAgreementThreshold < 0 || cfg.MinAgreementThreshold > 1 {
		errs = append(errs, ValidationError{
			Field:   path + ".min_agreement_threshold",
			Message: "must be between 0 and 1",
		})
	}
	return errs
}

// containsSecretReference reports whether value contains any ${secrets.*}
// template reference. Secrets are allowed in validators (via config) because
// those run in the sandbox context; judge prompt text goes straight to a
// third-party LLM provider, so we hard-reject the same pattern here.
func containsSecretReference(value string) bool {
	// Cheap fast-path: if there's no "${" substring at all, skip the
	// slower scan.
	if !strings.Contains(value, "${") {
		return false
	}
	// Match ${secrets. with optional whitespace. This is a simplified
	// pattern — it doesn't need to match every legal template expression,
	// only enough to reject plausibly intentional uses. Pack authors who
	// get a false positive can rename their variable.
	for _, needle := range []string{"${secrets.", "${ secrets.", "${secrets ."} {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
