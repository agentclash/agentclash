package scoring

import (
	"encoding/json"
	"math/big"
	"testing"
)

func TestParseMathEquivalenceConfig(t *testing.T) {
	cfg, err := parseMathEquivalenceConfig(nil)
	if err != nil {
		t.Fatalf("parseMathEquivalenceConfig returned error: %v", err)
	}
	if cfg.ComparisonMode != mathComparisonModeSymbolic {
		t.Fatalf("comparison mode = %q, want %q", cfg.ComparisonMode, mathComparisonModeSymbolic)
	}
	if !cfg.numericFallback() {
		t.Fatal("numericFallback = false, want true")
	}

	cfg, err = parseMathEquivalenceConfig(json.RawMessage(`{"extract_answer": true}`))
	if err != nil {
		t.Fatalf("parseMathEquivalenceConfig returned error: %v", err)
	}
	if cfg.AnswerDelimiter != "####" {
		t.Fatalf("answer delimiter = %q, want ####", cfg.AnswerDelimiter)
	}
}

func TestNormalizeMathExpression(t *testing.T) {
	cfg := mathEquivalenceConfig{ExtractAnswer: true, AnswerDelimiter: "####"}
	got, err := normalizeMathExpression(`The answer is $\boxed{\frac{1}{2}}$ #### \boxed{\frac{1}{2}}`, cfg)
	if err != nil {
		t.Fatalf("normalizeMathExpression returned error: %v", err)
	}
	want := "(((1)/(2)))"
	if got != want {
		t.Fatalf("normalizeMathExpression = %q, want %q", got, want)
	}

	got, err = normalizeMathExpression(`√2`, mathEquivalenceConfig{})
	if err != nil {
		t.Fatalf("normalizeMathExpression returned error: %v", err)
	}
	if got != "sqrt(2)" {
		t.Fatalf("normalizeMathExpression = %q, want %q", got, "sqrt(2)")
	}
}

func TestValidateMathEquivalence(t *testing.T) {
	tests := []struct {
		name        string
		actual      string
		expected    string
		config      string
		wantVerdict string
		wantMode    string
	}{
		{
			name:        "fraction_decimal_equivalence",
			actual:      "0.5",
			expected:    "1/2",
			config:      `{}`,
			wantVerdict: "pass",
			wantMode:    mathComparisonModeSymbolic,
		},
		{
			name:        "fraction_ratio_equivalence",
			actual:      "2/4",
			expected:    "1/2",
			config:      `{}`,
			wantVerdict: "pass",
			wantMode:    mathComparisonModeSymbolic,
		},
		{
			name:        "latex_fraction_equivalence",
			actual:      `\frac{1}{2}`,
			expected:    "0.5",
			config:      `{}`,
			wantVerdict: "pass",
			wantMode:    mathComparisonModeSymbolic,
		},
		{
			name:        "boxed_answer_equivalence",
			actual:      `\boxed{42}`,
			expected:    "42",
			config:      `{}`,
			wantVerdict: "pass",
			wantMode:    mathComparisonModeSymbolic,
		},
		{
			name:        "extracts_answer_after_delimiter",
			actual:      "Reasoning... #### 42",
			expected:    "42",
			config:      `{"extract_answer": true, "answer_delimiter": "####"}`,
			wantVerdict: "pass",
			wantMode:    mathComparisonModeSymbolic,
		},
		{
			name:        "sqrt_equivalence",
			actual:      `\sqrt{2}`,
			expected:    "2^(1/2)",
			config:      `{}`,
			wantVerdict: "pass",
			wantMode:    mathComparisonModeSymbolic,
		},
		{
			name:        "commutative_symbolic_equivalence",
			actual:      "x + 1",
			expected:    "1 + x",
			config:      `{}`,
			wantVerdict: "pass",
			wantMode:    mathComparisonModeSymbolic,
		},
		{
			name:        "scientific_notation_and_percentage",
			actual:      "5e-1",
			expected:    "50%",
			config:      `{}`,
			wantVerdict: "pass",
			wantMode:    mathComparisonModeSymbolic,
		},
		{
			name:        "different_answers_fail",
			actual:      "41",
			expected:    "42",
			config:      `{}`,
			wantVerdict: "fail",
		},
		{
			name:        "numeric_mode_uses_tolerance",
			actual:      "0.3333334",
			expected:    "1/3",
			config:      `{"comparison_mode":"numeric","tolerance":0.0001}`,
			wantVerdict: "pass",
			wantMode:    mathComparisonModeNumeric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := validateMathEquivalence(tt.actual, tt.expected, json.RawMessage(tt.config))
			if outcome.verdict != tt.wantVerdict {
				t.Fatalf("verdict = %q, want %q (reason: %s)", outcome.verdict, tt.wantVerdict, outcome.reason)
			}
			if tt.wantMode != "" && outcome.evidence["mode_used"] != tt.wantMode {
				t.Fatalf("mode_used = %#v, want %q", outcome.evidence["mode_used"], tt.wantMode)
			}
		})
	}
}

func TestValidateMathEquivalence_InvalidConfig(t *testing.T) {
	outcome := validateMathEquivalence("1", "1", json.RawMessage(`{"comparison_mode":"approximate"}`))
	if outcome.verdict != "error" {
		t.Fatalf("verdict = %q, want error", outcome.verdict)
	}
}

func TestRaiseRatToIntegerPowerRejectsLargeExponent(t *testing.T) {
	if got, ok := raiseRatToIntegerPower(big.NewRat(2, 1), big.NewInt(maxExactPowerExponent+1)); ok || got != nil {
		t.Fatalf("raiseRatToIntegerPower accepted exponent beyond safety limit: got=%v ok=%v", got, ok)
	}
}
