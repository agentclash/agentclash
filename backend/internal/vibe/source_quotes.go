package vibe

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Providers sometimes reflow pasted blockquotes or change quotation styles.
// Restore the original bytes only for a unique formatting-equivalent match;
// never accept paraphrases, missing words, changed negation, or ambiguous spans.
// The restored evidence still goes through exact validation and semantic review.
func restoreEvidenceQuote(quote, source string) string {
	if quote == "" || strings.Contains(source, quote) {
		return quote
	}
	needle, _ := foldEvidenceQuotes(quote)
	haystack, offsets := foldEvidenceQuotes(source)
	start := strings.Index(haystack, needle)
	if start < 0 || strings.Contains(haystack[start+1:], needle) {
		return quote
	}
	return source[offsets[start]:offsets[start+len(needle)]]
}

func foldEvidenceQuotes(text string) (string, []int) {
	var folded strings.Builder
	offsets := make([]int, 0, len(text)+1)
	var previous rune
	linePrefix := true
	blockquote := strings.HasPrefix(strings.TrimSpace(text), "> ")
	word := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }
	for i, r := range text {
		_, size := utf8.DecodeRuneInString(text[i:])
		next, _ := utf8.DecodeRuneInString(text[i+size:])
		canonical := r
		if blockquote && linePrefix && r == '>' && unicode.IsSpace(next) {
			linePrefix = false
			continue
		}
		if unicode.IsSpace(r) {
			if r == '\n' || r == '\r' {
				linePrefix = true
			}
			canonical = ' '
			if previous == ' ' {
				continue
			}
		} else {
			linePrefix = false
		}
		switch r {
		case '“', '”':
			canonical = '"'
		case '\'', '‘', '’':
			canonical = '\''
			// A contraction's apostrophe is not a quotation delimiter.
			if !word(previous) || !word(next) {
				canonical = '"'
			}
		}
		before := folded.Len()
		folded.WriteRune(canonical)
		for n := folded.Len() - before; n > 0; n-- {
			offsets = append(offsets, i)
		}
		previous = canonical
	}
	return folded.String(), append(offsets, len(text))
}
