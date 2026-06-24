package evalpack

import (
	"fmt"

	"github.com/agentclash/agentclash/backend/internal/scoring"
)

func validateBundleCaseTemplates(bundle Bundle) ValidationErrors {
	var errs ValidationErrors
	if len(bundle.Version.EvaluationSpec.Validators) == 0 {
		return errs
	}

	type templatedCommand struct {
		fieldPath   string
		testCommand string
	}
	commands := make([]templatedCommand, 0)
	for i, validator := range bundle.Version.EvaluationSpec.Validators {
		if validator.Type != scoring.ValidatorTypeCodeExecution {
			continue
		}
		cfg, err := scoring.ParseCodeExecutionConfig(validator.Config)
		if err != nil {
			continue
		}
		if len(ExtractCaseTemplatePlaceholders(cfg.TestCommand)) == 0 {
			continue
		}
		commands = append(commands, templatedCommand{
			fieldPath:   fmt.Sprintf("version.evaluation_spec.validators[%d].config.test_command", i),
			testCommand: cfg.TestCommand,
		})
	}
	if len(commands) == 0 {
		return errs
	}

	for inputSetIndex, inputSet := range bundle.InputSets {
		for caseIndex, caseDef := range inputSet.Cases {
			ctx := BuildCaseTemplateContext(cloneObject(caseDef.Payload), caseDef.Inputs)
			casePath := fmt.Sprintf("input_sets[%d].cases[%d]", inputSetIndex, caseIndex)
			for _, command := range commands {
				fieldPath := command.fieldPath + " (" + casePath + ")"
				if err := ValidateCaseTemplate(command.testCommand, ctx, fieldPath); err != nil {
					switch typed := err.(type) {
					case ValidationError:
						errs = append(errs, typed)
					case ValidationErrors:
						errs = append(errs, typed...)
					}
				}
			}
		}
	}

	return errs
}
