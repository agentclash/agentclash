package scoring

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	mathComparisonModeSymbolic = "symbolic"
	mathComparisonModeNumeric  = "numeric"
	maxExactPowerExponent      = 32
	maxExactPowerOperandBits   = 256
)

type mathEquivalenceConfig struct {
	ExtractAnswer   bool     `json:"extract_answer"`
	AnswerDelimiter string   `json:"answer_delimiter"`
	ComparisonMode  string   `json:"comparison_mode"`
	NumericFallback *bool    `json:"numeric_fallback"`
	Tolerance       *float64 `json:"tolerance"`
}

type mathExprKind string

const (
	mathExprNumber mathExprKind = "number"
	mathExprSymbol mathExprKind = "symbol"
	mathExprAdd    mathExprKind = "add"
	mathExprMul    mathExprKind = "mul"
	mathExprPow    mathExprKind = "pow"
)

type mathExpr struct {
	kind     mathExprKind
	value    *big.Rat
	symbol   string
	children []*mathExpr
}

type mathTokenType string

const (
	mathTokenEOF    mathTokenType = "eof"
	mathTokenNumber mathTokenType = "number"
	mathTokenIdent  mathTokenType = "ident"
	mathTokenOp     mathTokenType = "op"
	mathTokenLParen mathTokenType = "lparen"
	mathTokenRParen mathTokenType = "rparen"
)

type mathToken struct {
	typ   mathTokenType
	text  string
	value *big.Rat
}

type mathParser struct {
	tokens []mathToken
	pos    int
}

func validateMathEquivalence(actual string, expected string, rawConfig json.RawMessage) validatorOutcome {
	config, err := parseMathEquivalenceConfig(rawConfig)
	if err != nil {
		return validatorError("parse math_equivalence config", err, nil)
	}
	if config.ComparisonMode != mathComparisonModeSymbolic && config.ComparisonMode != mathComparisonModeNumeric {
		return validatorError("parse math_equivalence config", fmt.Errorf(`comparison_mode must be "symbolic" or "numeric"`), nil)
	}

	normalizedActual, err := normalizeMathExpression(actual, config)
	if err != nil {
		return validatorError("normalize actual math expression", err, map[string]any{
			"actual_raw": actual,
		})
	}
	normalizedExpected, err := normalizeMathExpression(expected, config)
	if err != nil {
		return validatorError("normalize expected math expression", err, map[string]any{
			"expected_raw": expected,
		})
	}

	tolerance := config.effectiveTolerance()
	evidence := map[string]any{
		"actual_raw":          actual,
		"expected_raw":        expected,
		"normalized_actual":   normalizedActual,
		"normalized_expected": normalizedExpected,
		"comparison_mode":     config.ComparisonMode,
		"extract_answer":      config.ExtractAnswer,
		"answer_delimiter":    config.AnswerDelimiter,
		"numeric_fallback":    config.numericFallback(),
		"tolerance":           tolerance,
	}

	actualExpr, actualParseErr := parseMathExpression(normalizedActual)
	expectedExpr, expectedParseErr := parseMathExpression(normalizedExpected)

	if config.ComparisonMode == mathComparisonModeSymbolic && actualParseErr == nil && expectedParseErr == nil {
		canonicalActual := canonicalizeMathExpr(actualExpr)
		canonicalExpected := canonicalizeMathExpr(expectedExpr)
		actualKey := mathExprKey(canonicalActual)
		expectedKey := mathExprKey(canonicalExpected)
		evidence["canonical_actual"] = actualKey
		evidence["canonical_expected"] = expectedKey

		if actualKey == expectedKey {
			evidence["mode_used"] = mathComparisonModeSymbolic
			return validatorOutcome{
				verdict:         "pass",
				normalizedScore: floatPtr(1),
				evidence:        evidence,
			}
		}
	}

	if actualParseErr != nil {
		evidence["actual_parse_error"] = actualParseErr.Error()
	}
	if expectedParseErr != nil {
		evidence["expected_parse_error"] = expectedParseErr.Error()
	}

	if config.ComparisonMode == mathComparisonModeNumeric || config.numericFallback() {
		actualValue, errActualNumeric := evaluateMathExpression(normalizedActual)
		expectedValue, errExpectedNumeric := evaluateMathExpression(normalizedExpected)
		if errActualNumeric == nil && errExpectedNumeric == nil {
			diff := math.Abs(actualValue - expectedValue)
			evidence["numeric_actual"] = actualValue
			evidence["numeric_expected"] = expectedValue
			evidence["absolute_difference"] = diff

			modeUsed := mathComparisonModeNumeric
			if config.ComparisonMode == mathComparisonModeSymbolic {
				modeUsed = "numeric_fallback"
			}
			evidence["mode_used"] = modeUsed

			if diff <= tolerance {
				return validatorOutcome{
					verdict:         "pass",
					normalizedScore: floatPtr(1),
					evidence:        evidence,
				}
			}
			return validatorOutcome{
				verdict:         "fail",
				normalizedScore: floatPtr(0),
				reason:          fmt.Sprintf("numeric values differ by %.10g which exceeds tolerance %.10g", diff, tolerance),
				evidence:        evidence,
			}
		}

		if errActualNumeric != nil {
			evidence["actual_numeric_error"] = errActualNumeric.Error()
		}
		if errExpectedNumeric != nil {
			evidence["expected_numeric_error"] = errExpectedNumeric.Error()
		}

		if config.ComparisonMode == mathComparisonModeNumeric || actualParseErr != nil || expectedParseErr != nil {
			return validatorError("evaluate math_equivalence expressions", joinMathErrors(actualParseErr, expectedParseErr, errActualNumeric, errExpectedNumeric), evidence)
		}
	}

	if actualParseErr != nil || expectedParseErr != nil {
		return validatorError("parse math_equivalence expressions", joinMathErrors(actualParseErr, expectedParseErr), evidence)
	}

	return validatorOutcome{
		verdict:         "fail",
		normalizedScore: floatPtr(0),
		reason:          "symbolic forms are not equivalent",
		evidence:        evidence,
	}
}

func parseMathEquivalenceConfig(rawConfig json.RawMessage) (mathEquivalenceConfig, error) {
	config := mathEquivalenceConfig{
		ComparisonMode: mathComparisonModeSymbolic,
	}
	if len(rawConfig) > 0 {
		if err := decodeStrictJSON(rawConfig, &config); err != nil {
			return mathEquivalenceConfig{}, err
		}
	}
	config.ComparisonMode = strings.TrimSpace(config.ComparisonMode)
	if config.ComparisonMode == "" {
		config.ComparisonMode = mathComparisonModeSymbolic
	}
	config.AnswerDelimiter = strings.TrimSpace(config.AnswerDelimiter)
	if config.ExtractAnswer && config.AnswerDelimiter == "" {
		config.AnswerDelimiter = "####"
	}
	return config, nil
}

func (c mathEquivalenceConfig) effectiveTolerance() float64 {
	if c.Tolerance == nil {
		return 1e-6
	}
	return *c.Tolerance
}

func (c mathEquivalenceConfig) numericFallback() bool {
	if c.ComparisonMode == mathComparisonModeNumeric {
		return true
	}
	if c.NumericFallback == nil {
		return true
	}
	return *c.NumericFallback
}

func normalizeMathExpression(raw string, config mathEquivalenceConfig) (string, error) {
	s := strings.TrimSpace(raw)
	if config.ExtractAnswer {
		s = extractMathAnswer(s, config.AnswerDelimiter)
	}
	s = stripOuterMathDelimiters(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "−", "-")
	s = strings.ReplaceAll(s, "–", "-")
	s = strings.ReplaceAll(s, "×", "*")
	s = strings.ReplaceAll(s, "·", "*")
	s = strings.ReplaceAll(s, "÷", "/")
	s = strings.ReplaceAll(s, `\left`, "")
	s = strings.ReplaceAll(s, `\right`, "")
	s = strings.ReplaceAll(s, `\cdot`, "*")
	s = strings.ReplaceAll(s, `\times`, "*")
	s = strings.ReplaceAll(s, `\!`, "")
	s = strings.ReplaceAll(s, `\,`, "")
	s = strings.ReplaceAll(s, `\;`, "")
	s = strings.ReplaceAll(s, `\:`, "")
	s = strings.TrimSpace(s)

	var err error
	s, err = unwrapLatexCommand(s, `\boxed`)
	if err != nil {
		return "", err
	}
	s, err = replaceLatexFrac(s)
	if err != nil {
		return "", err
	}
	s, err = replaceLatexSqrt(s)
	if err != nil {
		return "", err
	}
	s = replaceUnicodeSqrt(s)
	s = strings.ReplaceAll(s, "{", "(")
	s = strings.ReplaceAll(s, "}", ")")
	s = collapseWhitespace(s)
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("expression is empty")
	}
	return s, nil
}

func extractMathAnswer(raw string, delimiter string) string {
	if delimiter == "" {
		return strings.TrimSpace(raw)
	}
	idx := strings.LastIndex(raw, delimiter)
	if idx == -1 {
		return strings.TrimSpace(raw)
	}
	return strings.TrimSpace(raw[idx+len(delimiter):])
}

func stripOuterMathDelimiters(s string) string {
	for {
		trimmed := strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(trimmed, "$$") && strings.HasSuffix(trimmed, "$$") && len(trimmed) >= 4:
			s = strings.TrimSpace(trimmed[2 : len(trimmed)-2])
		case strings.HasPrefix(trimmed, "$") && strings.HasSuffix(trimmed, "$") && len(trimmed) >= 2:
			s = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		default:
			return trimmed
		}
	}
}

func unwrapLatexCommand(s string, command string) (string, error) {
	for {
		idx := strings.Index(s, command)
		if idx == -1 {
			return s, nil
		}
		start := idx + len(command)
		for start < len(s) && unicode.IsSpace(rune(s[start])) {
			start++
		}
		if start >= len(s) || s[start] != '{' {
			return "", fmt.Errorf("%s must be followed by braces", command)
		}
		content, end, err := extractBalancedGroup(s, start, '{', '}')
		if err != nil {
			return "", err
		}
		s = s[:idx] + "(" + content + ")" + s[end:]
	}
}

func replaceLatexFrac(s string) (string, error) {
	for {
		idx := strings.Index(s, `\frac`)
		if idx == -1 {
			return s, nil
		}
		numStart := idx + len(`\frac`)
		for numStart < len(s) && unicode.IsSpace(rune(s[numStart])) {
			numStart++
		}
		if numStart >= len(s) || s[numStart] != '{' {
			return "", fmt.Errorf(`\frac must have a numerator group`)
		}
		numerator, afterNum, err := extractBalancedGroup(s, numStart, '{', '}')
		if err != nil {
			return "", err
		}
		denStart := afterNum
		for denStart < len(s) && unicode.IsSpace(rune(s[denStart])) {
			denStart++
		}
		if denStart >= len(s) || s[denStart] != '{' {
			return "", fmt.Errorf(`\frac must have a denominator group`)
		}
		denominator, end, err := extractBalancedGroup(s, denStart, '{', '}')
		if err != nil {
			return "", err
		}
		replacement := "((" + numerator + ")/(" + denominator + "))"
		s = s[:idx] + replacement + s[end:]
	}
}

func replaceLatexSqrt(s string) (string, error) {
	for {
		idx := strings.Index(s, `\sqrt`)
		if idx == -1 {
			return s, nil
		}
		pos := idx + len(`\sqrt`)
		for pos < len(s) && unicode.IsSpace(rune(s[pos])) {
			pos++
		}

		rootIndex := ""
		if pos < len(s) && s[pos] == '[' {
			var err error
			rootIndex, pos, err = extractBalancedGroup(s, pos, '[', ']')
			if err != nil {
				return "", err
			}
			for pos < len(s) && unicode.IsSpace(rune(s[pos])) {
				pos++
			}
		}
		if pos >= len(s) || s[pos] != '{' {
			return "", fmt.Errorf(`\sqrt must have a radicand group`)
		}
		radicand, end, err := extractBalancedGroup(s, pos, '{', '}')
		if err != nil {
			return "", err
		}

		replacement := "sqrt(" + radicand + ")"
		if strings.TrimSpace(rootIndex) != "" {
			replacement = "(((" + radicand + ")^(1/(" + rootIndex + "))))"
		}
		s = s[:idx] + replacement + s[end:]
	}
}

func extractBalancedGroup(s string, start int, open byte, close byte) (string, int, error) {
	if start >= len(s) || s[start] != open {
		return "", start, fmt.Errorf("expected %q at offset %d", open, start)
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[start+1 : i], i + 1, nil
			}
		}
	}
	return "", start, fmt.Errorf("unbalanced %q %q group", open, close)
}

func replaceUnicodeSqrt(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r != '√' {
			b.WriteRune(r)
			i += size
			continue
		}

		j := i + size
		for j < len(s) {
			next, nextSize := utf8.DecodeRuneInString(s[j:])
			if !unicode.IsSpace(next) {
				break
			}
			j += nextSize
		}
		if j >= len(s) {
			b.WriteString("sqrt")
			i += size
			continue
		}

		next, nextSize := utf8.DecodeRuneInString(s[j:])
		if next == '(' || next == '{' {
			b.WriteString("sqrt")
			i += size
			continue
		}

		tokenStart := j
		for j < len(s) {
			next, nextSize = utf8.DecodeRuneInString(s[j:])
			if unicode.IsSpace(next) || strings.ContainsRune("+-*/^(){}", next) {
				break
			}
			j += nextSize
		}
		if tokenStart == j {
			b.WriteString("sqrt")
			i += size
			continue
		}
		b.WriteString("sqrt(")
		b.WriteString(strings.TrimSpace(s[tokenStart:j]))
		b.WriteString(")")
		i = j
	}
	return b.String()
}

func parseMathExpression(raw string) (*mathExpr, error) {
	tokens, err := tokenizeMathExpression(raw)
	if err != nil {
		return nil, err
	}
	parser := mathParser{tokens: tokens}
	expr, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	if parser.current().typ != mathTokenEOF {
		return nil, fmt.Errorf("unexpected token %q", parser.current().text)
	}
	return expr, nil
}

func evaluateMathExpression(raw string) (float64, error) {
	expr, err := parseMathExpression(raw)
	if err != nil {
		return 0, err
	}
	return evalMathExpr(expr)
}

func tokenizeMathExpression(raw string) ([]mathToken, error) {
	tokens := make([]mathToken, 0, len(raw)+1)
	for i := 0; i < len(raw); {
		r, size := utf8.DecodeRuneInString(raw[i:])
		if unicode.IsSpace(r) {
			i += size
			continue
		}

		switch r {
		case '(', '{':
			tokens = append(tokens, mathToken{typ: mathTokenLParen, text: string(r)})
			i += size
			continue
		case ')', '}':
			tokens = append(tokens, mathToken{typ: mathTokenRParen, text: string(r)})
			i += size
			continue
		case '+', '-', '*', '/', '^':
			tokens = append(tokens, mathToken{typ: mathTokenOp, text: string(r)})
			i += size
			continue
		}

		if unicode.IsDigit(r) || r == '.' {
			start := i
			i += size
			seenDot := r == '.'
			seenExp := false
			for i < len(raw) {
				next, nextSize := utf8.DecodeRuneInString(raw[i:])
				switch {
				case unicode.IsDigit(next):
					i += nextSize
				case next == '.' && !seenDot && !seenExp:
					seenDot = true
					i += nextSize
				case (next == 'e' || next == 'E') && !seenExp:
					seenExp = true
					i += nextSize
					if i < len(raw) {
						sign, signSize := utf8.DecodeRuneInString(raw[i:])
						if sign == '+' || sign == '-' {
							i += signSize
						}
					}
				default:
					goto numberDone
				}
			}
		numberDone:
			numberText := raw[start:i]
			value, err := parseMathNumber(numberText)
			if err != nil {
				return nil, err
			}
			if i < len(raw) {
				next, nextSize := utf8.DecodeRuneInString(raw[i:])
				if next == '%' {
					value = new(big.Rat).Quo(value, big.NewRat(100, 1))
					i += nextSize
				}
			}
			tokens = append(tokens, mathToken{typ: mathTokenNumber, text: numberText, value: value})
			continue
		}

		if unicode.IsLetter(r) || r == '_' {
			start := i
			i += size
			for i < len(raw) {
				next, nextSize := utf8.DecodeRuneInString(raw[i:])
				if unicode.IsLetter(next) || unicode.IsDigit(next) || next == '_' {
					i += nextSize
					continue
				}
				break
			}
			tokens = append(tokens, mathToken{typ: mathTokenIdent, text: raw[start:i]})
			continue
		}

		return nil, fmt.Errorf("unexpected character %q", r)
	}
	tokens = append(tokens, mathToken{typ: mathTokenEOF})
	return tokens, nil
}

func parseMathNumber(raw string) (*big.Rat, error) {
	if strings.ContainsAny(raw, "eE") {
		return parseScientificRat(raw)
	}
	value, ok := new(big.Rat).SetString(raw)
	if !ok {
		return nil, fmt.Errorf("invalid number %q", raw)
	}
	return value, nil
}

func parseScientificRat(raw string) (*big.Rat, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == 'e' || r == 'E' })
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid scientific notation %q", raw)
	}
	mantissa := parts[0]
	exponent, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid exponent in %q", raw)
	}

	sign := ""
	if strings.HasPrefix(mantissa, "+") || strings.HasPrefix(mantissa, "-") {
		sign = mantissa[:1]
		mantissa = mantissa[1:]
	}

	fracDigits := 0
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		fracDigits = len(mantissa) - dot - 1
		mantissa = mantissa[:dot] + mantissa[dot+1:]
	}
	mantissa = strings.TrimLeft(mantissa, "0")
	if mantissa == "" {
		return big.NewRat(0, 1), nil
	}

	intValue, ok := new(big.Int).SetString(sign+mantissa, 10)
	if !ok {
		return nil, fmt.Errorf("invalid mantissa in %q", raw)
	}
	scale := exponent - fracDigits
	if scale >= 0 {
		factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
		return new(big.Rat).SetInt(new(big.Int).Mul(intValue, factor)), nil
	}

	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-scale)), nil)
	return new(big.Rat).SetFrac(intValue, denominator), nil
}

func (p *mathParser) parseExpression() (*mathExpr, error) {
	return p.parseAddSub()
}

func (p *mathParser) parseAddSub() (*mathExpr, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}
	for {
		token := p.current()
		if token.typ != mathTokenOp || (token.text != "+" && token.text != "-") {
			return left, nil
		}
		p.pos++
		right, err := p.parseMulDiv()
		if err != nil {
			return nil, err
		}
		if token.text == "-" {
			right = negateMathExpr(right)
		}
		left = &mathExpr{kind: mathExprAdd, children: []*mathExpr{left, right}}
	}
}

func (p *mathParser) parseMulDiv() (*mathExpr, error) {
	left, err := p.parsePower()
	if err != nil {
		return nil, err
	}
	for {
		token := p.current()
		if token.typ != mathTokenOp || (token.text != "*" && token.text != "/") {
			return left, nil
		}
		p.pos++
		right, err := p.parsePower()
		if err != nil {
			return nil, err
		}
		if token.text == "/" {
			right = invertMathExpr(right)
		}
		left = &mathExpr{kind: mathExprMul, children: []*mathExpr{left, right}}
	}
}

func (p *mathParser) parsePower() (*mathExpr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	if token := p.current(); token.typ == mathTokenOp && token.text == "^" {
		p.pos++
		right, err := p.parsePower()
		if err != nil {
			return nil, err
		}
		left = &mathExpr{kind: mathExprPow, children: []*mathExpr{left, right}}
	}
	return left, nil
}

func (p *mathParser) parseUnary() (*mathExpr, error) {
	token := p.current()
	if token.typ == mathTokenOp && token.text == "+" {
		p.pos++
		return p.parseUnary()
	}
	if token.typ == mathTokenOp && token.text == "-" {
		p.pos++
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return negateMathExpr(expr), nil
	}
	return p.parsePrimary()
}

func (p *mathParser) parsePrimary() (*mathExpr, error) {
	token := p.current()
	switch token.typ {
	case mathTokenNumber:
		p.pos++
		return &mathExpr{kind: mathExprNumber, value: new(big.Rat).Set(token.value)}, nil
	case mathTokenIdent:
		p.pos++
		if p.current().typ == mathTokenLParen {
			p.pos++
			arg, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if p.current().typ != mathTokenRParen {
				return nil, fmt.Errorf("expected closing parenthesis after %s", token.text)
			}
			p.pos++
			switch token.text {
			case "sqrt":
				return &mathExpr{
					kind: mathExprPow,
					children: []*mathExpr{
						arg,
						{kind: mathExprNumber, value: big.NewRat(1, 2)},
					},
				}, nil
			default:
				return nil, fmt.Errorf("unsupported function %q", token.text)
			}
		}
		return &mathExpr{kind: mathExprSymbol, symbol: token.text}, nil
	case mathTokenLParen:
		p.pos++
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.current().typ != mathTokenRParen {
			return nil, fmt.Errorf("expected closing parenthesis")
		}
		p.pos++
		return expr, nil
	default:
		return nil, fmt.Errorf("unexpected token %q", token.text)
	}
}

func (p *mathParser) current() mathToken {
	if p.pos >= len(p.tokens) {
		return mathToken{typ: mathTokenEOF}
	}
	return p.tokens[p.pos]
}

func canonicalizeMathExpr(expr *mathExpr) *mathExpr {
	switch expr.kind {
	case mathExprNumber:
		return &mathExpr{kind: mathExprNumber, value: new(big.Rat).Set(expr.value)}
	case mathExprSymbol:
		return &mathExpr{kind: mathExprSymbol, symbol: expr.symbol}
	case mathExprAdd:
		terms := make([]*mathExpr, 0, len(expr.children))
		sum := new(big.Rat)
		for _, child := range expr.children {
			canonical := canonicalizeMathExpr(child)
			if canonical.kind == mathExprAdd {
				terms = append(terms, canonical.children...)
				continue
			}
			if canonical.kind == mathExprNumber {
				sum.Add(sum, canonical.value)
				continue
			}
			terms = append(terms, canonical)
		}
		if sum.Sign() != 0 || len(terms) == 0 {
			terms = append(terms, &mathExpr{kind: mathExprNumber, value: sum})
		}
		sortMathExprs(terms)
		if len(terms) == 1 {
			return terms[0]
		}
		return &mathExpr{kind: mathExprAdd, children: terms}
	case mathExprMul:
		factors := make([]*mathExpr, 0, len(expr.children))
		product := big.NewRat(1, 1)
		for _, child := range expr.children {
			canonical := canonicalizeMathExpr(child)
			if canonical.kind == mathExprMul {
				factors = append(factors, canonical.children...)
				continue
			}
			if canonical.kind == mathExprNumber {
				product.Mul(product, canonical.value)
				continue
			}
			factors = append(factors, canonical)
		}
		if product.Sign() == 0 {
			return &mathExpr{kind: mathExprNumber, value: big.NewRat(0, 1)}
		}
		if product.Cmp(big.NewRat(1, 1)) != 0 || len(factors) == 0 {
			factors = append(factors, &mathExpr{kind: mathExprNumber, value: product})
		}
		sortMathExprs(factors)
		if len(factors) == 1 {
			return factors[0]
		}
		return &mathExpr{kind: mathExprMul, children: factors}
	case mathExprPow:
		base := canonicalizeMathExpr(expr.children[0])
		exponent := canonicalizeMathExpr(expr.children[1])
		if exponent.kind == mathExprNumber {
			if exponent.value.Sign() == 0 {
				return &mathExpr{kind: mathExprNumber, value: big.NewRat(1, 1)}
			}
			if exponent.value.Cmp(big.NewRat(1, 1)) == 0 {
				return base
			}
			if base.kind == mathExprNumber && exponent.value.IsInt() {
				powered, ok := raiseRatToIntegerPower(base.value, exponent.value.Num())
				if ok {
					return &mathExpr{kind: mathExprNumber, value: powered}
				}
			}
		}
		return &mathExpr{kind: mathExprPow, children: []*mathExpr{base, exponent}}
	default:
		return expr
	}
}

func raiseRatToIntegerPower(value *big.Rat, exponent *big.Int) (*big.Rat, bool) {
	if exponent == nil {
		return nil, false
	}
	if exponent.BitLen() > 62 {
		return nil, false
	}
	exp := exponent.Int64()
	if exp == 0 {
		return big.NewRat(1, 1), true
	}
	absExp := exp
	if absExp < 0 {
		absExp = -absExp
	}
	if absExp > maxExactPowerExponent {
		return nil, false
	}
	if value.Num().BitLen() > maxExactPowerOperandBits || value.Denom().BitLen() > maxExactPowerOperandBits {
		return nil, false
	}

	numerator := new(big.Int).Exp(value.Num(), big.NewInt(absExp), nil)
	denominator := new(big.Int).Exp(value.Denom(), big.NewInt(absExp), nil)
	if exp < 0 {
		numerator, denominator = denominator, numerator
	}
	return new(big.Rat).SetFrac(numerator, denominator), true
}

func sortMathExprs(exprs []*mathExpr) {
	sort.Slice(exprs, func(i, j int) bool {
		return mathExprKey(exprs[i]) < mathExprKey(exprs[j])
	})
}

func mathExprKey(expr *mathExpr) string {
	switch expr.kind {
	case mathExprNumber:
		return "num:" + expr.value.RatString()
	case mathExprSymbol:
		return "sym:" + expr.symbol
	case mathExprAdd:
		parts := make([]string, 0, len(expr.children))
		for _, child := range expr.children {
			parts = append(parts, mathExprKey(child))
		}
		return "add(" + strings.Join(parts, ",") + ")"
	case mathExprMul:
		parts := make([]string, 0, len(expr.children))
		for _, child := range expr.children {
			parts = append(parts, mathExprKey(child))
		}
		return "mul(" + strings.Join(parts, ",") + ")"
	case mathExprPow:
		return "pow(" + mathExprKey(expr.children[0]) + "," + mathExprKey(expr.children[1]) + ")"
	default:
		return string(expr.kind)
	}
}

func evalMathExpr(expr *mathExpr) (float64, error) {
	switch expr.kind {
	case mathExprNumber:
		value, _ := expr.value.Float64()
		return value, nil
	case mathExprSymbol:
		return 0, fmt.Errorf("symbol %q is not numeric", expr.symbol)
	case mathExprAdd:
		total := 0.0
		for _, child := range expr.children {
			value, err := evalMathExpr(child)
			if err != nil {
				return 0, err
			}
			total += value
		}
		return total, nil
	case mathExprMul:
		total := 1.0
		for _, child := range expr.children {
			value, err := evalMathExpr(child)
			if err != nil {
				return 0, err
			}
			total *= value
		}
		return total, nil
	case mathExprPow:
		base, err := evalMathExpr(expr.children[0])
		if err != nil {
			return 0, err
		}
		exponent, err := evalMathExpr(expr.children[1])
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exponent), nil
	default:
		return 0, fmt.Errorf("unsupported expression kind %q", expr.kind)
	}
}

func negateMathExpr(expr *mathExpr) *mathExpr {
	if expr.kind == mathExprNumber {
		return &mathExpr{kind: mathExprNumber, value: new(big.Rat).Neg(expr.value)}
	}
	return &mathExpr{
		kind: mathExprMul,
		children: []*mathExpr{
			{kind: mathExprNumber, value: big.NewRat(-1, 1)},
			expr,
		},
	}
}

func invertMathExpr(expr *mathExpr) *mathExpr {
	if expr.kind == mathExprNumber && expr.value.Sign() != 0 {
		return &mathExpr{kind: mathExprNumber, value: new(big.Rat).Inv(expr.value)}
	}
	return &mathExpr{
		kind: mathExprPow,
		children: []*mathExpr{
			expr,
			{kind: mathExprNumber, value: big.NewRat(-1, 1)},
		},
	}
}

func joinMathErrors(errs ...error) error {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		parts = append(parts, err.Error())
	}
	if len(parts) == 0 {
		return fmt.Errorf("unknown math equivalence error")
	}
	return errors.New(strings.Join(parts, "; "))
}
