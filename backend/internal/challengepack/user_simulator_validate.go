package challengepack

import (
	"fmt"
	"strings"
)

func validateBundleUserSimulators(bundle Bundle) ValidationErrors {
	var errs ValidationErrors
	isMultiTurn := bundle.Version.ExecutionMode == ExecutionModeMultiTurn

	versionAssetKeys := map[string]struct{}{}
	for _, asset := range bundle.Version.Assets {
		if asset.Key != "" {
			versionAssetKeys[asset.Key] = struct{}{}
		}
	}

	for inputSetIndex, inputSet := range bundle.InputSets {
		for caseIndex, caseDef := range inputSet.Cases {
			path := fmt.Sprintf("input_sets[%d].cases[%d].user_simulator", inputSetIndex, caseIndex)
			if caseDef.UserSimulator == nil {
				if isMultiTurn {
					errs = append(errs, ValidationError{Field: path, Message: "is required when version.execution_mode is multi_turn"})
				}
				continue
			}
			if !isMultiTurn {
				errs = append(errs, ValidationError{Field: path, Message: "is only allowed when version.execution_mode is multi_turn"})
				continue
			}
			errs = append(errs, validateUserSimulatorSpec(path, *caseDef.UserSimulator, caseDef, versionAssetKeys)...)
		}
	}
	return errs
}

func validateUserSimulatorSpec(path string, spec UserSimulatorSpec, caseDef CaseDefinition, versionAssetKeys map[string]struct{}) ValidationErrors {
	var errs ValidationErrors

	if spec.SchemaVersion != UserSimulatorSchemaVersionV1 {
		errs = append(errs, ValidationError{
			Field:   path + ".schema_version",
			Message: fmt.Sprintf("must be %d", UserSimulatorSchemaVersionV1),
		})
	}
	if strings.TrimSpace(spec.Kind) != UserSimulatorKindHybrid {
		errs = append(errs, ValidationError{
			Field:   path + ".kind",
			Message: fmt.Sprintf("must be %q", UserSimulatorKindHybrid),
		})
	}
	if spec.MaxTurns < 0 {
		errs = append(errs, ValidationError{Field: path + ".max_turns", Message: "must be greater than or equal to 0"})
	}
	if len(spec.Phases) == 0 {
		errs = append(errs, ValidationError{Field: path + ".phases", Message: "must contain at least one phase"})
	}

	phaseIDs := map[string]struct{}{}
	for i, phase := range spec.Phases {
		phasePath := fmt.Sprintf("%s.phases[%d]", path, i)
		errs = append(errs, validateUserSimulatorPhase(phasePath, phase, i == 0, caseDef.Inputs, versionAssetKeys)...)
		if id := strings.TrimSpace(phase.ID); id != "" {
			if _, exists := phaseIDs[id]; exists {
				errs = append(errs, ValidationError{Field: phasePath + ".id", Message: "must be unique"})
			}
			phaseIDs[id] = struct{}{}
		}
	}

	if spec.Calibration != nil {
		errs = append(errs, validateUserSimulatorCalibration(path+".calibration", *spec.Calibration)...)
	}
	if spec.PostRun != nil {
		errs = append(errs, validateUserSimulatorPostRun(path+".post_run", *spec.PostRun)...)
	}

	errs = append(errs, validateUserSimulatorTemplates(path, spec, caseDef)...)
	return errs
}

func validateUserSimulatorPhase(path string, phase UserSimulatorPhase, isFirst bool, caseInputs []CaseInput, versionAssetKeys map[string]struct{}) ValidationErrors {
	var errs ValidationErrors

	if strings.TrimSpace(phase.ID) == "" {
		errs = append(errs, ValidationError{Field: path + ".id", Message: "is required"})
	}
	actor := strings.TrimSpace(phase.Actor)
	if actor == "" {
		errs = append(errs, ValidationError{Field: path + ".actor", Message: "is required"})
	} else if _, ok := supportedUserSimulatorActors[actor]; !ok {
		errs = append(errs, ValidationError{
			Field:   path + ".actor",
			Message: "must be one of scripted, llm, human",
		})
	}

	trigger := normalizeUserSimulatorTrigger(phase.Trigger)
	if _, ok := supportedUserSimulatorTriggers[trigger]; !ok {
		errs = append(errs, ValidationError{Field: path + ".trigger", Message: "must be a supported trigger expression"})
	}
	if isFirst && trigger != UserSimulatorTriggerAlways {
		errs = append(errs, ValidationError{
			Field:   path + ".trigger",
			Message: `first phase trigger must be empty or "always"`,
		})
	}
	if !isFirst && trigger == UserSimulatorTriggerAlways && strings.TrimSpace(phase.Trigger) == "" {
		// empty trigger normalizes to always — only first phase may use always implicitly.
		errs = append(errs, ValidationError{
			Field:   path + ".trigger",
			Message: "is required for non-first phases",
		})
	}

	switch actor {
	case UserSimulatorActorScripted:
		if len(phase.Turns) == 0 {
			errs = append(errs, ValidationError{Field: path + ".turns", Message: "must contain at least one turn for scripted actor"})
		}
		for turnIndex, turn := range phase.Turns {
			turnPath := fmt.Sprintf("%s.turns[%d]", path, turnIndex)
			if strings.TrimSpace(turn.Message) == "" {
				errs = append(errs, ValidationError{Field: turnPath + ".message", Message: "is required for scripted turns"})
			}
			errs = append(errs, validateCaseExpectations(turnPath+".expects", turn.Expects, caseInputs, versionAssetKeys)...)
		}
	case UserSimulatorActorLLM:
		if strings.TrimSpace(phase.Persona) == "" {
			errs = append(errs, ValidationError{Field: path + ".persona", Message: "is required for llm actor"})
		}
		if phase.MaxTurns < 0 {
			errs = append(errs, ValidationError{Field: path + ".max_turns", Message: "must be greater than or equal to 0"})
		}
		for untilIndex, condition := range phase.Until {
			if strings.TrimSpace(condition) == "" {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("%s.until[%d]", path, untilIndex),
					Message: "must be a non-empty condition expression",
				})
			}
		}
	case UserSimulatorActorHuman:
		if phase.TimeoutMS < 0 {
			errs = append(errs, ValidationError{Field: path + ".timeout_ms", Message: "must be greater than or equal to 0"})
		}
	}

	return errs
}

func validateUserSimulatorCalibration(path string, calibration UserSimulatorCalibration) ValidationErrors {
	var errs ValidationErrors
	if !calibration.Enabled {
		return errs
	}
	if calibration.SampleRate < 0 || calibration.SampleRate > 1 {
		errs = append(errs, ValidationError{Field: path + ".sample_rate", Message: "must be between 0 and 1 when calibration is enabled"})
	}
	return errs
}

func validateUserSimulatorPostRun(path string, postRun UserSimulatorPostRun) ValidationErrors {
	var errs ValidationErrors
	if postRun.Arena == nil {
		return errs
	}
	arenaPath := path + ".arena"
	if !postRun.Arena.Enabled {
		return errs
	}
	comparison := strings.TrimSpace(postRun.Arena.Comparison)
	if comparison == "" {
		comparison = UserSimulatorArenaComparisonPairwise
	}
	if _, ok := supportedUserSimulatorArenaComparisons[comparison]; !ok {
		errs = append(errs, ValidationError{
			Field:   arenaPath + ".comparison",
			Message: fmt.Sprintf("must be one of %q", UserSimulatorArenaComparisonPairwise),
		})
	}
	return errs
}

func validateUserSimulatorTemplates(path string, spec UserSimulatorSpec, caseDef CaseDefinition) ValidationErrors {
	var errs ValidationErrors
	ctx := BuildCaseTemplateContext(cloneObject(caseDef.Payload), caseDef.Inputs)
	for phaseIndex, phase := range spec.Phases {
		if strings.TrimSpace(phase.Actor) != UserSimulatorActorScripted {
			continue
		}
		for turnIndex, turn := range phase.Turns {
			fieldPath := fmt.Sprintf("%s.phases[%d].turns[%d].message", path, phaseIndex, turnIndex)
			if err := ValidateCaseTemplate(turn.Message, ctx, fieldPath); err != nil {
				switch typed := err.(type) {
				case ValidationError:
					errs = append(errs, typed)
				case ValidationErrors:
					errs = append(errs, typed...)
				}
			}
		}
	}
	return errs
}
