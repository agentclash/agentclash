package enquiries

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/agentclash/agentclash/backend/internal/mutation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ContactRequest struct {
	ClientID uuid.UUID `json:"client_id"`
	Email    string    `json:"email"`
	Work     string    `json:"work"`
	Company  string    `json:"company,omitempty"`
	Website  string    `json:"website,omitempty"`
}

// Preserve the shared error classifications while giving contact visitors messages
// that do not refer to project summaries or project limits.
type contactError struct {
	cause   error
	message string
}

func (e contactError) Error() string { return e.message }
func (e contactError) Unwrap() error { return e.cause }

func ValidateContact(r ContactRequest) error {
	if r.ClientID == uuid.Nil || !ValidEmail(r.Email) || strings.TrimSpace(r.Work) == "" || !utf8.ValidString(r.Work) || len(r.Work) > 4000 || strings.ContainsRune(r.Work, 0) || len(r.Company) > 200 || !utf8.ValidString(r.Company) || strings.ContainsAny(r.Company, "\r\n\x00") {
		return contactError{ErrInvalid, "Provide a valid reply email, work to automate (up to 4,000 bytes), and a company of at most 200 bytes."}
	}
	return nil
}

func (s *Store) CreateContact(ctx context.Context, actor string, r ContactRequest) (result Receipt, err error) {
	committing := false
	defer func() {
		if err != nil && !committing {
			err = mutation.Reject(err)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ValidateContact(r); err != nil {
		return Receipt{}, err
	}
	if strings.TrimSpace(actor) == "" {
		return Receipt{}, ErrInvalid
	}
	if !s.Available() {
		return Receipt{}, ErrDisabled
	}
	if r.Website != "" {
		return Receipt{ID: uuid.New(), Status: "received", CreatedAt: time.Now().UTC()}, nil
	}
	// Hash the submitted content, as project enquiries do. Kind is persisted so
	// delivery can distinguish the two request shapes without loading a project.
	content, err := json.Marshal(struct {
		Kind string `json:"kind"`
		ContactRequest
	}{Kind: "contact", ContactRequest: r})
	if err != nil {
		return Receipt{}, err
	}
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Receipt{}, err
	}
	defer tx.Rollback(ctx)
	// A project-less actor has no session row to lock. Serialize its quota
	// with an actor-scoped transaction lock, including first use.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "vibe-contact:"+actor); err != nil {
		return Receipt{}, err
	}
	var oldHash string
	var receipt Receipt
	err = tx.QueryRow(ctx, `SELECT id,status,created_at,request_hash FROM vibe_enquiries WHERE session_id IS NULL AND client_id=$1`, r.ClientID).Scan(&receipt.ID, &receipt.Status, &receipt.CreatedAt, &oldHash)
	if err == nil {
		if oldHash != hash {
			return Receipt{}, ErrConflict
		}
		return receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM vibe_enquiries WHERE session_id IS NULL AND actor=$1 AND created_at>now()-interval '24 hours'`, actor).Scan(&count); err != nil {
		return Receipt{}, err
	}
	if count >= 3 {
		return Receipt{}, contactError{ErrQuota, "You have already sent three enquiries in the last 24 hours. Email us directly for more help."}
	}
	receipt.ID = uuid.New()
	if err = tx.QueryRow(ctx, `INSERT INTO vibe_enquiries(id,session_id,actor,client_id,request_hash,content,transport_id) VALUES($1,NULL,$2,$3,$4,$5,$6) ON CONFLICT (client_id) WHERE session_id IS NULL DO NOTHING RETURNING status,created_at`, receipt.ID, actor, r.ClientID, hash, content, s.TransportID).Scan(&receipt.Status, &receipt.CreatedAt); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, err
		}
		// Another actor can win the insert when a first response (and its
		// cookie) is lost. The client ID identifies the request across actors.
		err = tx.QueryRow(ctx, `SELECT id,status,created_at,request_hash FROM vibe_enquiries WHERE session_id IS NULL AND client_id=$1`, r.ClientID).Scan(&receipt.ID, &receipt.Status, &receipt.CreatedAt, &oldHash)
		if err != nil {
			return Receipt{}, err
		}
		if oldHash != hash {
			return Receipt{}, ErrConflict
		}
		return receipt, nil
	}
	committing = true
	return receipt, tx.Commit(ctx)
}
