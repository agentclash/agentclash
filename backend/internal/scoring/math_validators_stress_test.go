package scoring

import (
	"encoding/json"
	"testing"
)

// TestMathEquivalenceStress exercises every edge case in the math equivalence
// validator. Run with:
//   cd backend && go test -short -race -count=1 ./internal/scoring -run TestMathEquivalenceStress
func TestMathEquivalenceStress(t *testing.T) {
	symbolicConfig := json.RawMessage(`{}`)
	symbolicStrictConfig := json.RawMessage(`{"comparison_mode":"symbolic","numeric_fallback":false}`)
	numericConfig := json.RawMessage(`{"comparison_mode":"numeric","tolerance":0.0001}`)
	extractConfig := json.RawMessage(`{"extract_answer":true,"answer_delimiter":"####"}`)

	tests := []struct {
		name        string
		actual      string
		expected    string
		config      json.RawMessage
		wantVerdict string // "pass", "fail", or "error"
		wantMode    string // optional: "symbolic", "numeric", "numeric_fallback"
		note        string // documents why
	}{
		// ── Group 1: Basic equivalences ──────────────────────────────
		{
			name: "fraction_decimal", actual: "0.5", expected: "1/2",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "0.5 = 1/2 in big.Rat",
		},
		{
			name: "fraction_reduction", actual: "2/4", expected: "1/2",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "big.Rat auto-reduces 2/4 → 1/2",
		},
		{
			name: "negative_fraction", actual: "-3/4", expected: "-0.75",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "negative fractions",
		},
		{
			name: "integer_power", actual: "2^3", expected: "8",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "power reduced: 2^3 → 8",
		},
		{
			name: "zero_power_number", actual: "5^0", expected: "1",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "x^0 = 1 when x is a number",
		},
		{
			name: "scientific_notation", actual: "3e2", expected: "300",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "scientific notation parsed to big.Rat",
		},
		{
			name: "percentage", actual: "25%", expected: "0.25",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "25% → 0.25",
		},
		{
			name: "sci_pct_cross", actual: "5e-1", expected: "50%",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "5e-1 = 0.5 = 50%",
		},

		// ── Group 2: LaTeX normalization ─────────────────────────────
		{
			name: "latex_frac", actual: `\frac{3}{4}`, expected: "0.75",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: `\frac{3}{4} → ((3)/(4)) → 3/4`,
		},
		{
			name: "latex_boxed", actual: `\boxed{42}`, expected: "42",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: `\boxed{42} → (42)`,
		},
		{
			name: "latex_sqrt", actual: `\sqrt{2}`, expected: "2^(1/2)",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: `\sqrt{2} → sqrt(2) → 2^(1/2)`,
		},
		{
			name: "latex_nested_frac", actual: `\frac{1}{\frac{2}{3}}`, expected: "3/2",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "nested frac: 1/(2/3) = 3/2",
		},
		{
			name: "latex_sqrt_frac", actual: `\sqrt{\frac{1}{4}}`, expected: "1/2",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "numeric_fallback",
			note: "sqrt(1/4) = 0.5 — symbolic may not reduce, numeric fallback handles it",
		},
		{
			name: "latex_dollar_wrap", actual: "$42$", expected: "42",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "dollar signs stripped",
		},
		{
			name: "latex_double_dollar", actual: `$$\frac{1}{2}$$`, expected: "0.5",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "double dollar signs stripped",
		},
		{
			name: "latex_boxed_sqrt", actual: `$\boxed{\sqrt{2}}$`, expected: "2^(1/2)",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "boxed sqrt in dollar delimiters",
		},
		{
			name: "latex_nth_root", actual: `\sqrt[3]{8}`, expected: "2",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "numeric_fallback",
			note: `\sqrt[3]{8} → 8^(1/3) = 2 via numeric fallback`,
		},
		{
			name: "latex_left_right", actual: `\left(\frac{1}{2}\right)`, expected: "0.5",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: `\left \right stripped`,
		},

		// ── Group 3: Symbolic equivalences ───────────────────────────
		{
			name: "commutative_add", actual: "x + 1", expected: "1 + x",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "canonical sort: num before sym",
		},
		{
			name: "commutative_mul", actual: "a * b", expected: "b * a",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "canonical sort: sym:a before sym:b",
		},
		{
			name: "associative_add", actual: "(a + b) + c", expected: "a + b + c",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "nested add is flattened",
		},
		{
			name: "constant_folding_add", actual: "1 + 2 + x", expected: "3 + x",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "1+2 folded to 3",
		},
		{
			name: "constant_folding_mul", actual: "2 * 3 * x", expected: "6 * x",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "2*3 folded to 6",
		},

		// ── Group 4: Unicode normalization ───────────────────────────
		{
			name: "unicode_minus", actual: "5 \u2212 3", expected: "2",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "U+2212 MINUS SIGN → hyphen",
		},
		{
			name: "unicode_multiply", actual: "3 \u00d7 4", expected: "12",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "U+00D7 MULTIPLICATION SIGN → *",
		},
		{
			name: "unicode_cdot", actual: "3 \u00b7 4", expected: "12",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "U+00B7 MIDDLE DOT → *",
		},
		{
			name: "unicode_divide", actual: "6 \u00f7 2", expected: "3",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "U+00F7 DIVISION SIGN → /",
		},
		{
			name: "unicode_sqrt", actual: "\u221a2", expected: "2^(1/2)",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "U+221A SQUARE ROOT → sqrt(2)",
		},

		// ── Group 5: Answer extraction ───────────────────────────────
		{
			name: "extract_after_reasoning", actual: "Let me think... the answer is 42 because... #### 42", expected: "42",
			config: extractConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "text after last #### is extracted",
		},
		{
			name: "extract_multiple_delimiters", actual: "Step 1 #### intermediate #### 7", expected: "7",
			config: extractConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "LastIndex finds the final ####",
		},

		// ── Group 6: Expected failures ───────────────────────────────
		{
			name: "wrong_answer", actual: "41", expected: "42",
			config: symbolicConfig, wantVerdict: "fail",
			note: "41 ≠ 42",
		},
		{
			name: "off_by_one_symbolic", actual: "x + 1", expected: "x + 2",
			config: symbolicConfig, wantVerdict: "fail",
			note: "different constants",
		},

		// ── Group 7: Limitation probes ───────────────────────────────
		{
			name: "distributive_law_KNOWN_GAP", actual: "2*(x+y)", expected: "2*x + 2*y",
			config: symbolicConfig, wantVerdict: "fail",
			note: "KNOWN GAP: no expansion/factoring. Numeric fallback also fails (symbols).",
		},
		{
			name: "large_exponent_numeric_fallback", actual: "2^50", expected: "1125899906842624",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "numeric_fallback",
			note: "exponent 50 > 32 bound, so symbolic won't reduce, but numeric fallback works",
		},
		{
			name: "nested_power_identity_KNOWN_GAP", actual: "(x^2)^3", expected: "x^6",
			config: symbolicConfig, wantVerdict: "fail",
			note: "KNOWN GAP: no power-of-power simplification",
		},
		{
			name: "implicit_multiplication_KNOWN_GAP", actual: "2x", expected: "2*x",
			config: symbolicConfig, wantVerdict: "error",
			note: "KNOWN GAP: '2x' fails to parse — no implicit multiplication",
		},
		{
			name: "unsupported_function_sin", actual: "sin(0)", expected: "0",
			config: symbolicConfig, wantVerdict: "error",
			note: "KNOWN GAP: only sqrt() is supported as a function",
		},
		{
			name: "case_sensitive_symbols_KNOWN_GAP", actual: "X + 1", expected: "x + 1",
			config: symbolicConfig, wantVerdict: "fail",
			note: "KNOWN GAP: symbols are case-sensitive",
		},
		{
			name: "subtraction_normalized", actual: "a - b", expected: "a + (-1) * b",
			config: symbolicStrictConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "subtraction → add(a, mul(-1, b))",
		},
		{
			name: "double_negative", actual: "--5", expected: "5",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "unary minus applied twice",
		},
		{
			name: "multiplicative_identity", actual: "1*x", expected: "x",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "product=1 is omitted when other factors exist",
		},
		{
			name: "additive_identity", actual: "0+x", expected: "x",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "sum=0 is omitted when other terms exist",
		},
		{
			name: "mixed_fraction_decimal_sum", actual: "1/2 + 0.5", expected: "1",
			config: symbolicConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "constant folding: 1/2 + 1/2 = 1",
		},

		// ── Numeric-only mode ────────────────────────────────────────
		{
			name: "numeric_tolerance_pass", actual: "0.3333334", expected: "1/3",
			config: numericConfig, wantVerdict: "pass", wantMode: "numeric",
			note: "within tolerance 0.0001",
		},
		{
			name: "numeric_tolerance_fail", actual: "0.5", expected: "1/3",
			config: numericConfig, wantVerdict: "fail", wantMode: "numeric",
			note: "outside tolerance",
		},

		// ── Symbolic-strict mode (no fallback) ───────────────────────
		{
			name: "strict_no_fallback_pass", actual: "1/2", expected: "0.5",
			config: symbolicStrictConfig, wantVerdict: "pass", wantMode: "symbolic",
			note: "rational equivalence works symbolically",
		},
		{
			name: "strict_no_fallback_nth_root_fail", actual: `\sqrt[3]{8}`, expected: "2",
			config: symbolicStrictConfig, wantVerdict: "fail",
			note: "8^(1/3) can't be evaluated symbolically (exponent is non-integer), and no fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := validateMathEquivalence(tt.actual, tt.expected, tt.config)
			if outcome.verdict != tt.wantVerdict {
				t.Fatalf("verdict = %q, want %q\n  note: %s\n  reason: %s\n  evidence: %v",
					outcome.verdict, tt.wantVerdict, tt.note, outcome.reason, outcome.evidence)
			}
			if tt.wantMode != "" {
				got, _ := outcome.evidence["mode_used"].(string)
				if got != tt.wantMode {
					t.Fatalf("mode_used = %q, want %q\n  note: %s\n  evidence: %v",
						got, tt.wantMode, tt.note, outcome.evidence)
				}
			}
		})
	}
}
