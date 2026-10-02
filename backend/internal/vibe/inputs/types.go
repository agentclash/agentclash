// Package inputs owns private, immutable task material. It does not interpret
// instructions or grant documents authority over business rules.
package inputs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxPDFBytes = 10_000_000
const MaxPages = 30
const MaxTextCharacters = 60_000
const GuestLifetime = 7 * 24 * time.Hour

var ErrUnavailable = errors.New("material unavailable")
var ErrConflict = errors.New("request ID was used for different material")

type Page struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}
type Binding struct {
	ID            uuid.UUID `json:"input_id"`
	Hash          string    `json:"content_hash"`
	Usage         string    `json:"usage"`
	AcceptPartial bool      `json:"accept_partial,omitempty"`
}
type Record struct {
	PageCount   int        `json:"page_count,omitempty"`
	ID          uuid.UUID  `json:"id"`
	SessionID   uuid.UUID  `json:"session_id"`
	ClientID    uuid.UUID  `json:"client_id"`
	Kind        string     `json:"kind"`
	Name        string     `json:"name"`
	Size        int64      `json:"size_bytes"`
	Status      string     `json:"status"`
	Hash        string     `json:"content_hash"`
	Pages       []Page     `json:"pages,omitempty"`
	Warnings    []string   `json:"warnings"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	ObjectKey   string     `json:"-"`
	RequestHash string     `json:"-"`
}
type Extraction struct {
	Pages    []Page   `json:"pages"`
	Warnings []string `json:"warnings"`
	Version  string   `json:"version"`
}

func Hash(b []byte) string            { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func ContentHash(pages []Page) string { b, _ := json.Marshal(pages); return Hash(b) }
func ValidateText(s string) error {
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) || strings.TrimSpace(s) == "" {
		return Invalid("Supply readable text.")
	}
	if utf8.RuneCountInString(s) > MaxTextCharacters {
		return Invalid("Use at most %d text characters; no text was shortened.", MaxTextCharacters)
	}
	return nil
}
func (r Record) Text() string {
	var b strings.Builder
	for _, p := range r.Pages {
		fmt.Fprintf(&b, "[Page %d]\n%s\n", p.Number, p.Text)
	}
	return b.String()
}
func (r Record) Usable(now time.Time) bool {
	return r.Status == "ready" && (r.ExpiresAt == nil || now.Before(*r.ExpiresAt))
}
func ValidateExtraction(e Extraction) error {
	if len(e.Pages) == 0 || len(e.Pages) > MaxPages {
		return Invalid("Use a PDF with 1 to %d pages.", MaxPages)
	}
	count, nonempty := 0, false
	for i, p := range e.Pages {
		if p.Number != i+1 || !utf8.ValidString(p.Text) || strings.ContainsRune(p.Text, 0) {
			return Invalid("The PDF could not be read reliably. Paste its text instead.")
		}
		count += utf8.RuneCountInString(p.Text)
		nonempty = nonempty || strings.TrimSpace(p.Text) != ""
	}
	if !nonempty {
		return Invalid("This PDF has no readable text. Paste the text instead; scanned PDFs are not supported yet.")
	}
	if count > MaxTextCharacters {
		return Invalid("This PDF contains too much text. Use a smaller document or paste a shorter section; nothing was truncated.")
	}
	return nil
}

// ValidationError contains only user-safe validation messages, never storage errors.
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string       { return e.Message }
func Invalid(format string, args ...any) error { return &ValidationError{fmt.Sprintf(format, args...)} }

var ErrParserUnavailable = errors.New("PDF reading is unavailable. Paste the text instead.")
