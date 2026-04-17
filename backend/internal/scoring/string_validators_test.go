package scoring

import (
	"encoding/json"
	"math"
	"testing"
)

// --- levenshteinDistance ---

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"identical", "hello", "hello", 0},
		{"empty_both", "", "", 0},
		{"empty_a", "", "abc", 3},
		{"empty_b", "abc", "", 3},
		{"single_insert", "kitten", "kittens", 1},
		{"substitution_and_insert", "kitten", "sitting", 3},
		{"completely_different", "abc", "xyz", 3},
		{"unicode_identical", "日本語", "日本語", 0},
		{"unicode_one_diff", "日本語", "日本人", 1},
		{"case_difference", "Hello", "hello", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := levenshteinDistance([]rune(tt.a), []rune(tt.b))
			if got != tt.want {
				t.Fatalf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
			// Verify symmetry.
			rev := levenshteinDistance([]rune(tt.b), []rune(tt.a))
			if rev != got {
				t.Fatalf("levenshteinDistance is not symmetric: (%q,%q)=%d vs (%q,%q)=%d", tt.a, tt.b, got, tt.b, tt.a, rev)
			}
		})
	}
}

// --- validateFuzzyMatch ---

func TestValidateFuzzyMatch(t *testing.T) {
	tests := []struct {
		name        string
		actual      string
		expected    string
		config      string
		wantVerdict string
		wantAbove   float64 // normalizedScore must be >= this
		wantBelow   float64 // normalizedScore must be <= this
	}{
		{
			name:        "exact_match_passes",
			actual:      "hello world",
			expected:    "hello world",
			config:      `{}`,
			wantVerdict: "pass",
			wantAbove:   1.0,
			wantBelow:   1.0,
		},
		{
			name:        "similar_strings_pass_default_threshold",
			actual:      "hello world",
			expected:    "hello worle",
			config:      `{}`,
			wantVerdict: "pass",
			wantAbove:   0.8,
			wantBelow:   1.0,
		},
		{
			name:        "dissimilar_strings_fail",
			actual:      "hello",
			expected:    "goodbye world",
			config:      `{}`,
			wantVerdict: "fail",
			wantAbove:   0.0,
			wantBelow:   0.8,
		},
		{
			// With the standard sum-based ratio, completely different strings of
			// equal length get similarity 0.5: (3+3-3)/(3+3) = 0.5.
			name:        "custom_low_threshold_passes",
			actual:      "abc",
			expected:    "xyz",
			config:      `{"threshold": 0.0}`,
			wantVerdict: "pass",
			wantAbove:   0.0,
			wantBelow:   0.5,
		},
		{
			name:        "case_insensitive_match",
			actual:      "Hello World",
			expected:    "hello world",
			config:      `{"case_insensitive": true}`,
			wantVerdict: "pass",
			wantAbove:   1.0,
			wantBelow:   1.0,
		},
		{
			name:        "normalize_collapses_whitespace",
			actual:      "  hello   world  ",
			expected:    "hello world",
			config:      `{"normalize": true}`,
			wantVerdict: "pass",
			wantAbove:   1.0,
			wantBelow:   1.0,
		},
		{
			name:        "both_empty_returns_pass_with_similarity_1",
			actual:      "",
			expected:    "",
			config:      `{}`,
			wantVerdict: "pass",
			wantAbove:   1.0,
			wantBelow:   1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := validateFuzzyMatch(tt.actual, tt.expected, json.RawMessage(tt.config))
			if outcome.verdict != tt.wantVerdict {
				t.Fatalf("verdict = %q, want %q (reason: %s)", outcome.verdict, tt.wantVerdict, outcome.reason)
			}
			if outcome.normalizedScore == nil {
				t.Fatal("normalizedScore is nil")
			}
			score := *outcome.normalizedScore
			if score < tt.wantAbove || score > tt.wantBelow {
				t.Fatalf("normalizedScore = %f, want [%f, %f]", score, tt.wantAbove, tt.wantBelow)
			}
			if outcome.evidence == nil {
				t.Fatal("evidence is nil")
			}
		})
	}
}

func TestValidateFuzzyMatch_InvalidConfig(t *testing.T) {
	outcome := validateFuzzyMatch("a", "b", json.RawMessage(`{bad json`))
	if outcome.verdict != "error" {
		t.Fatalf("verdict = %q, want error", outcome.verdict)
	}
}

func TestValidateFuzzyMatch_InputTooLarge(t *testing.T) {
	large := make([]byte, maxFuzzyMatchRunes+1)
	for i := range large {
		large[i] = 'a'
	}
	outcome := validateFuzzyMatch(string(large), "b", nil)
	if outcome.verdict != "error" {
		t.Fatalf("verdict = %q, want error", outcome.verdict)
	}
}

// --- extractNumber ---

func TestExtractNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"plain_integer", "42", 42, false},
		{"plain_float", "3.14", 3.14, false},
		{"negative", "-7.5", -7.5, false},
		{"positive_sign", "+42", 42, false},
		{"leading_decimal", ".5", 0.5, false},
		{"with_currency", "$1,234.56", 1234.56, false},
		{"with_euro", "€99.99 EUR", 99.99, false},
		{"with_percent", "85.5%", 85.5, false},
		{"scientific_notation", "1.5e10", 1.5e10, false},
		{"surrounded_by_text", "the answer is 42 ok", 42, false},
		{"no_number", "no numbers here", 0, true},
		{"with_commas", "1,000,000", 1000000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractNumber(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("extractNumber(%q) returned nil error, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("extractNumber(%q) returned error: %v", tt.input, err)
			}
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("extractNumber(%q) = %g, want %g", tt.input, got, tt.want)
			}
		})
	}
}

// --- roundToSignificantDigits ---

func TestRoundToSignificantDigits(t *testing.T) {
	tests := []struct {
		name   string
		val    float64
		digits int
		want   float64
	}{
		{"zero", 0, 3, 0},
		{"round_3_sig_figs", 123456, 3, 123000},
		{"round_2_sig_figs", 0.001234, 2, 0.0012},
		{"negative", -9876, 2, -9900},
		{"already_exact", 1.5, 2, 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := roundToSignificantDigits(tt.val, tt.digits)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("roundToSignificantDigits(%g, %d) = %g, want %g", tt.val, tt.digits, got, tt.want)
			}
		})
	}
}

// --- validateNumericMatch ---

func TestValidateNumericMatch(t *testing.T) {
	tests := []struct {
		name        string
		actual      string
		expected    string
		config      string
		wantVerdict string
	}{
		{
			name:        "exact_equal",
			actual:      "42",
			expected:    "42",
			config:      `{}`,
			wantVerdict: "pass",
		},
		{
			name:        "exact_not_equal",
			actual:      "42",
			expected:    "43",
			config:      `{}`,
			wantVerdict: "fail",
		},
		{
			name:        "within_absolute_tolerance",
			actual:      "10.05",
			expected:    "10.0",
			config:      `{"absolute_tolerance": 0.1}`,
			wantVerdict: "pass",
		},
		{
			name:        "outside_absolute_tolerance",
			actual:      "10.2",
			expected:    "10.0",
			config:      `{"absolute_tolerance": 0.1}`,
			wantVerdict: "fail",
		},
		{
			name:        "within_relative_tolerance",
			actual:      "105",
			expected:    "100",
			config:      `{"relative_tolerance": 0.1}`,
			wantVerdict: "pass",
		},
		{
			name:        "outside_relative_tolerance",
			actual:      "120",
			expected:    "100",
			config:      `{"relative_tolerance": 0.1}`,
			wantVerdict: "fail",
		},
		{
			name:        "extract_number_from_text",
			actual:      "The total cost is $42.50",
			expected:    "42.5",
			config:      `{"extract_number": true}`,
			wantVerdict: "pass",
		},
		{
			name:        "extract_number_from_expected_text",
			actual:      "42.5",
			expected:    "The total cost is $42.50",
			config:      `{"extract_number": true}`,
			wantVerdict: "pass",
		},
		{
			name:        "extract_number_from_both_sides_with_plus_sign",
			actual:      "Answer: +42",
			expected:    "The answer is .42e2",
			config:      `{"extract_number": true}`,
			wantVerdict: "pass",
		},
		{
			name:        "significant_digits_rounding",
			actual:      "3.14159",
			expected:    "3.14",
			config:      `{"significant_digits": 3}`,
			wantVerdict: "pass",
		},
		{
			name:        "dual_tolerance_or_logic_abs_passes",
			actual:      "10.05",
			expected:    "10.0",
			config:      `{"absolute_tolerance": 0.1, "relative_tolerance": 0.001}`,
			wantVerdict: "pass",
		},
		{
			name:        "dual_tolerance_or_logic_rel_passes",
			actual:      "110",
			expected:    "100",
			config:      `{"absolute_tolerance": 0.001, "relative_tolerance": 0.15}`,
			wantVerdict: "pass",
		},
		{
			name:        "relative_tolerance_expected_zero_actual_nonzero_fails",
			actual:      "99999",
			expected:    "0",
			config:      `{"relative_tolerance": 0.01}`,
			wantVerdict: "fail",
		},
		{
			name:        "relative_tolerance_both_zero_passes",
			actual:      "0",
			expected:    "0",
			config:      `{"relative_tolerance": 0.01}`,
			wantVerdict: "pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := validateNumericMatch(tt.actual, tt.expected, json.RawMessage(tt.config))
			if outcome.verdict != tt.wantVerdict {
				t.Fatalf("verdict = %q, want %q (reason: %s)", outcome.verdict, tt.wantVerdict, outcome.reason)
			}
			if outcome.normalizedScore == nil {
				t.Fatal("normalizedScore is nil")
			}
			if outcome.evidence == nil {
				t.Fatal("evidence is nil")
			}
		})
	}
}

func TestValidateNumericMatch_Errors(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		expected string
		config   string
	}{
		{"invalid_config", "1", "1", `{bad json`},
		{"non_numeric_expected", "1", "abc", `{}`},
		{"non_numeric_actual", "abc", "1", `{}`},
		{"extract_no_number", "no numbers", "1", `{"extract_number": true}`},
		{"extract_no_number_in_expected", "1", "no numbers", `{"extract_number": true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := validateNumericMatch(tt.actual, tt.expected, json.RawMessage(tt.config))
			if outcome.verdict != "error" {
				t.Fatalf("verdict = %q, want error", outcome.verdict)
			}
		})
	}
}

func TestValidateNumericMatch_EvidenceIncludesRawAndParsedValues(t *testing.T) {
	outcome := validateNumericMatch("Answer: +42", "The answer is 42", json.RawMessage(`{"extract_number": true}`))
	if outcome.verdict != "pass" {
		t.Fatalf("verdict = %q, want pass", outcome.verdict)
	}

	if got := outcome.evidence["actual_raw"]; got != "Answer: +42" {
		t.Fatalf("actual_raw = %#v, want %q", got, "Answer: +42")
	}
	if got := outcome.evidence["expected_raw"]; got != "The answer is 42" {
		t.Fatalf("expected_raw = %#v, want %q", got, "The answer is 42")
	}
	if got := outcome.evidence["actual_parsed"]; got != "+42" {
		t.Fatalf("actual_parsed = %#v, want %q", got, "+42")
	}
	if got := outcome.evidence["expected_parsed"]; got != "42" {
		t.Fatalf("expected_parsed = %#v, want %q", got, "42")
	}
}

// --- validateNormalizedMatch ---

func TestValidateNormalizedMatch(t *testing.T) {
	tests := []struct {
		name        string
		actual      string
		expected    string
		config      string
		wantVerdict string
	}{
		{
			name:        "default_pipeline_trims_and_lowercases",
			actual:      "  Hello  World  ",
			expected:    "hello world",
			config:      `{}`,
			wantVerdict: "pass",
		},
		{
			name:        "default_pipeline_mismatch",
			actual:      "hello",
			expected:    "world",
			config:      `{}`,
			wantVerdict: "fail",
		},
		{
			name:        "strip_punctuation",
			actual:      "hello, world!",
			expected:    "hello world",
			config:      `{"pipeline": ["strip_punctuation", "trim", "collapse_whitespace"]}`,
			wantVerdict: "pass",
		},
		{
			name:        "strip_currency",
			actual:      "$100 USD",
			expected:    "100 ",
			config:      `{"pipeline": ["strip_currency"]}`,
			wantVerdict: "pass",
		},
		{
			name:        "strip_formatting",
			actual:      "1,000 (net)",
			expected:    "1000 net",
			config:      `{"pipeline": ["strip_formatting"]}`,
			wantVerdict: "pass",
		},
		{
			name:        "sort_words",
			actual:      "world hello",
			expected:    "hello world",
			config:      `{"pipeline": ["sort_words"]}`,
			wantVerdict: "pass",
		},
		{
			name:        "sort_lines",
			actual:      "banana\napple",
			expected:    "apple\nbanana",
			config:      `{"pipeline": ["sort_lines"]}`,
			wantVerdict: "pass",
		},
		{
			name:        "normalize_unicode",
			actual:      "\u00e9",
			expected:    "e\u0301",
			config:      `{"pipeline": ["normalize_unicode"]}`,
			wantVerdict: "pass",
		},
		{
			name:        "remove_articles",
			actual:      "the quick brown fox",
			expected:    "quick brown fox",
			config:      `{"pipeline": ["remove_articles", "collapse_whitespace", "trim"]}`,
			wantVerdict: "pass",
		},
		{
			name:        "full_pipeline",
			actual:      "  Hello,  World!  ",
			expected:    "hello world",
			config:      `{"pipeline": ["trim", "lowercase", "strip_punctuation", "collapse_whitespace"]}`,
			wantVerdict: "pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := validateNormalizedMatch(tt.actual, tt.expected, json.RawMessage(tt.config))
			if outcome.verdict != tt.wantVerdict {
				t.Fatalf("verdict = %q, want %q (reason: %s)", outcome.verdict, tt.wantVerdict, outcome.reason)
			}
			if outcome.normalizedScore == nil {
				t.Fatal("normalizedScore is nil")
			}
			if outcome.evidence == nil {
				t.Fatal("evidence is nil")
			}
		})
	}
}

func TestValidateNormalizedMatch_Errors(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{"invalid_config", `{bad json`},
		{"unknown_step", `{"pipeline": ["trim", "unknown_step"]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := validateNormalizedMatch("a", "b", json.RawMessage(tt.config))
			if outcome.verdict != "error" {
				t.Fatalf("verdict = %q, want error", outcome.verdict)
			}
		})
	}
}

// --- validateTokenF1 ---

func TestValidateTokenF1(t *testing.T) {
	tests := []struct {
		name          string
		actual        string
		expected      string
		config        string
		wantVerdict   string
		wantScore     float64
		wantPrecision float64
		wantRecall    float64
	}{
		{
			name:          "exact_match_scores_one",
			actual:        "eiffel tower in paris",
			expected:      "eiffel tower in paris",
			config:        `{}`,
			wantVerdict:   "pass",
			wantScore:     1.0,
			wantPrecision: 1.0,
			wantRecall:    1.0,
		},
		{
			name:          "partial_overlap_scores_between_zero_and_one",
			actual:        "eiffel tower",
			expected:      "eiffel tower paris",
			config:        `{"threshold": 0.9}`,
			wantVerdict:   "fail",
			wantScore:     0.8,
			wantPrecision: 1.0,
			wantRecall:    2.0 / 3.0,
		},
		{
			name:          "duplicate_tokens_use_bag_overlap_not_set_overlap",
			actual:        "paris paris paris",
			expected:      "paris tower",
			config:        `{"threshold": 0.5}`,
			wantVerdict:   "fail",
			wantScore:     0.4,
			wantPrecision: 1.0 / 3.0,
			wantRecall:    0.5,
		},
		{
			name:          "no_overlap_scores_zero",
			actual:        "louvre museum",
			expected:      "eiffel tower",
			config:        `{}`,
			wantVerdict:   "fail",
			wantScore:     0.0,
			wantPrecision: 0.0,
			wantRecall:    0.0,
		},
		{
			name:          "normalization_removes_articles_and_punctuation",
			actual:        "The Eiffel Tower!",
			expected:      "eiffel tower",
			config:        `{"threshold": 1.0, "normalize": true, "remove_articles": true, "remove_punctuation": true}`,
			wantVerdict:   "pass",
			wantScore:     1.0,
			wantPrecision: 1.0,
			wantRecall:    1.0,
		},
		{
			name:          "empty_prediction_scores_zero",
			actual:        "",
			expected:      "eiffel tower",
			config:        `{}`,
			wantVerdict:   "fail",
			wantScore:     0.0,
			wantPrecision: 0.0,
			wantRecall:    0.0,
		},
		{
			name:          "both_empty_scores_one",
			actual:        "",
			expected:      "",
			config:        `{}`,
			wantVerdict:   "pass",
			wantScore:     1.0,
			wantPrecision: 1.0,
			wantRecall:    1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := validateTokenF1(tt.actual, tt.expected, json.RawMessage(tt.config))
			if outcome.verdict != tt.wantVerdict {
				t.Fatalf("verdict = %q, want %q (reason: %s)", outcome.verdict, tt.wantVerdict, outcome.reason)
			}
			if outcome.normalizedScore == nil {
				t.Fatal("normalizedScore is nil")
			}
			if math.Abs(*outcome.normalizedScore-tt.wantScore) > 1e-9 {
				t.Fatalf("normalizedScore = %f, want %f", *outcome.normalizedScore, tt.wantScore)
			}
			if got := outcome.evidence["precision"].(float64); math.Abs(got-tt.wantPrecision) > 1e-9 {
				t.Fatalf("precision = %f, want %f", got, tt.wantPrecision)
			}
			if got := outcome.evidence["recall"].(float64); math.Abs(got-tt.wantRecall) > 1e-9 {
				t.Fatalf("recall = %f, want %f", got, tt.wantRecall)
			}
		})
	}
}

func TestValidateTokenF1_Errors(t *testing.T) {
	outcome := validateTokenF1("a", "b", json.RawMessage(`{bad json`))
	if outcome.verdict != "error" {
		t.Fatalf("verdict = %q, want error", outcome.verdict)
	}
}

// --- applyNormalizationPipeline ---

func TestApplyNormalizationPipeline(t *testing.T) {
	result, err := applyNormalizationPipeline("  HELLO   world  ", []string{"trim", "lowercase", "collapse_whitespace"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Fatalf("result = %q, want %q", result, "hello world")
	}
}

func TestApplyNormalizationPipeline_UnknownStep(t *testing.T) {
	_, err := applyNormalizationPipeline("test", []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown pipeline step")
	}
}

// --- helper functions ---

func TestCollapseWhitespace(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello  world", "hello world"},
		{"a\t\nb", "a b"},
		{"  spaces  ", " spaces "},
		{"no-change", "no-change"},
	}
	for _, tt := range tests {
		got := collapseWhitespace(tt.input)
		if got != tt.want {
			t.Fatalf("collapseWhitespace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStripPunctuation(t *testing.T) {
	got := stripPunctuation("hello, world! How's it?")
	want := "hello world Hows it"
	if got != want {
		t.Fatalf("stripPunctuation = %q, want %q", got, want)
	}
}

func TestStripCurrency(t *testing.T) {
	got := stripCurrency("$100 USD €200 EUR £50 GBP")
	want := "100  200  50 "
	if got != want {
		t.Fatalf("stripCurrency = %q, want %q", got, want)
	}
}

func TestStripFormatting(t *testing.T) {
	got := stripFormatting("1,000 (net) [note]")
	want := "1000 net note"
	if got != want {
		t.Fatalf("stripFormatting = %q, want %q", got, want)
	}
}

func TestRemoveArticles(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		// "the" → " ", preserving the space after → double space (collapse_whitespace cleans this).
		{"the quick brown fox", "  quick brown fox"},
		{"a cat and an owl", "  cat and   owl"},
		{"there is nothing", "there is nothing"},
		{"atheist", "atheist"},
	}
	for _, tt := range tests {
		got := removeArticles(tt.input)
		if got != tt.want {
			t.Fatalf("removeArticles(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeUnicodeUsesNFKC(t *testing.T) {
	// NFKC collapses compatibility variants that NFC does not.
	// Fullwidth A (U+FF21) should become standard A.
	result, err := applyNormalizationPipeline("\uff21\uff22\uff23", []string{"normalize_unicode"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ABC" {
		t.Fatalf("normalize_unicode NFKC result = %q, want %q", result, "ABC")
	}
}
