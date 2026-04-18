package failurereview

import "testing"

func TestClassifyFailureClassMappings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input ClassificationInput
		want  FailureClass
	}{
		{
			name: "policy violation",
			input: ClassificationInput{
				FailedChecks: []string{"policy.filesystem"},
			},
			want: FailureClassPolicyViolation,
		},
		{
			name: "tool argument",
			input: ClassificationInput{
				FailedChecks: []string{"tool_argument.schema"},
			},
			want: FailureClassToolArgumentError,
		},
		{
			name: "tool selection",
			input: ClassificationInput{
				FailedChecks: []string{"tool_selection.allowed"},
			},
			want: FailureClassToolSelectionError,
		},
		{
			name: "timeout and budget",
			input: ClassificationInput{
				HasTimeoutOrBudgetSignal: true,
			},
			want: FailureClassTimeoutOrBudget,
		},
		{
			name: "sandbox failure",
			input: ClassificationInput{
				HasSandboxFailure: true,
			},
			want: FailureClassSandboxFailure,
		},
		{
			name: "malformed output",
			input: ClassificationInput{
				HasMalformedOutput: true,
			},
			want: FailureClassMalformedOutput,
		},
		{
			name: "incorrect final output",
			input: ClassificationInput{
				HasLLMFinalAnswerFailure: true,
			},
			want: FailureClassIncorrectFinalOutput,
		},
		{
			name: "insufficient evidence",
			input: ClassificationInput{
				EvidenceTier: EvidenceTierHostedBlackBox,
			},
			want: FailureClassInsufficientEvidence,
		},
		{
			name: "hosted black box without structured signal stays insufficient evidence",
			input: ClassificationInput{
				EvidenceTier: EvidenceTierHostedBlackBox,
				FailedChecks: []string{"policy.filesystem"},
			},
			want: FailureClassInsufficientEvidence,
		},
		{
			name: "fallback other",
			input: ClassificationInput{
				FailedChecks: []string{"unexpected"},
			},
			want: FailureClassOther,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Classify(tt.input); got != tt.want {
				t.Fatalf("Classify() = %s, want %s", got, tt.want)
			}
		})
	}
}
