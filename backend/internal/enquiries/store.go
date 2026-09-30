// Package enquiries owns reviewed contact requests and notification delivery,
// independently of model runs, credits and business-rule authority.
package enquiries

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/agentclash/agentclash/backend/internal/email"
	"github.com/agentclash/agentclash/backend/internal/vibe/access"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalid = errors.New("Provide a reply email and a summary of at most 12,000 bytes.")
var ErrQuota = errors.New("This project already has three enquiries today. Use your existing receipt or email us directly.")

var ErrDisabled = errors.New("Contact isn't set up yet. You can copy your summary.")
var ErrRevision = errors.New("project revision unavailable")
var ErrUnavailable = errors.New("project or source unavailable")
var ErrConflict = errors.New("This enquiry ID already belongs to another request.")

type Source struct {
	ArtifactID  *uuid.UUID `json:"artifact_id,omitempty"`
	OperationID *uuid.UUID `json:"operation_id,omitempty"`
	Revision    int64      `json:"revision"`
}
type Request struct {
	ClientID uuid.UUID `json:"client_id"`
	Source   Source    `json:"source"`
	Summary  string    `json:"summary"`
	Email    string    `json:"email"`
	Name     string    `json:"name,omitempty"`
	Company  string    `json:"company,omitempty"`
}
type Receipt struct {
	ID        uuid.UUID `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
type Store struct {
	DB          *pgxpool.Pool
	Recipient   string
	Sender      email.MessageSender
	TransportID string
}

func FromEnv(db *pgxpool.Pool) *Store {
	s := &Store{DB: db}
	address := os.Getenv("VIBE_ENQUIRY_EMAIL")
	if ValidEmail(address) {
		s.Recipient = address
	}
	if key, from := os.Getenv("RESEND_API_KEY"), os.Getenv("RESEND_FROM_EMAIL"); key != "" && from != "" && os.Getenv("VIBE_ENQUIRIES_ENABLED") == "true" {
		s.Sender = email.NewResendSender(key, from)
		sum := sha256.Sum256([]byte(address + "\x00" + from))
		s.TransportID = hex.EncodeToString(sum[:])
	}
	return s
}
func ValidEmail(address string) bool {
	a, e := mail.ParseAddress(address)
	return e == nil && a.Address == address && len(address) <= 254 && !strings.ContainsAny(address, "\r\n")
}
func (s *Store) Available() bool { return s != nil && s.Recipient != "" && s.Sender != nil }
func Validate(r Request) error {
	if r.ClientID == uuid.Nil || r.Source.Revision < 0 || !ValidEmail(r.Email) || len(r.Name) > 100 || len(r.Company) > 200 || strings.ContainsAny(r.Name+r.Company, "\r\n\x00") || !utf8.ValidString(r.Summary) || strings.TrimSpace(r.Summary) == "" || len(r.Summary) > 12000 || strings.ContainsRune(r.Summary, 0) {
		return ErrInvalid
	}
	return nil
}
func (s *Store) Get(ctx context.Context, session, client uuid.UUID) (Receipt, error) {
	var r Receipt
	err := s.DB.QueryRow(ctx, `SELECT id,status,created_at FROM vibe_enquiries WHERE session_id=$1 AND client_id=$2`, session, client).Scan(&r.ID, &r.Status, &r.CreatedAt)
	return r, err
}
func (s *Store) Create(ctx context.Context, session uuid.UUID, actor string, r Request) (Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := Validate(r); err != nil {
		return Receipt{}, err
	}
	content, _ := json.Marshal(r)
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Receipt{}, err
	}
	defer tx.Rollback(ctx)
	v, err := access.Lookup(ctx, tx, actor, session, true, true)
	if err != nil {
		return Receipt{}, err
	}
	var oldHash string
	var receipt Receipt
	err = tx.QueryRow(ctx, `SELECT id,status,created_at,request_hash FROM vibe_enquiries WHERE session_id=$1 AND client_id=$2`, session, r.ClientID).Scan(&receipt.ID, &receipt.Status, &receipt.CreatedAt, &oldHash)
	if err == nil {
		if oldHash != hash {
			return Receipt{}, ErrConflict
		}
		return receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, err
	}
	if r.Source.Revision > v.Revision {
		return Receipt{}, ErrRevision
	}
	var valid bool
	if r.Source.ArtifactID != nil {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_sessions s,jsonb_array_elements(COALESCE(s.document->'artifacts','[]'::jsonb)) a WHERE s.id=$1 AND a->>'id'=$2)`, session, r.Source.ArtifactID.String()).Scan(&valid)
		if err != nil {
			return Receipt{}, err
		}
		if !valid {
			return Receipt{}, ErrUnavailable
		}
	}
	if r.Source.OperationID != nil {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND id=$2)`, session, *r.Source.OperationID).Scan(&valid)
		if err != nil {
			return Receipt{}, err
		}
		if !valid {
			return Receipt{}, ErrUnavailable
		}
	}
	if !s.Available() {
		return Receipt{}, ErrDisabled
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM vibe_enquiries WHERE session_id=$1 AND created_at>now()-interval '1 day'`, session).Scan(&count); err != nil {
		return Receipt{}, err
	}
	if count >= 3 {
		return Receipt{}, ErrQuota
	}
	receipt.ID = uuid.New()
	err = tx.QueryRow(ctx, `INSERT INTO vibe_enquiries(id,session_id,client_id,request_hash,content,transport_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING status,created_at`, receipt.ID, session, r.ClientID, hash, content, s.TransportID).Scan(&receipt.Status, &receipt.CreatedAt)
	if err != nil {
		return Receipt{}, err
	}
	return receipt, tx.Commit(ctx)
}
func (s *Store) Run(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.Available() {
				if err := s.deliverOne(ctx); err != nil {
					logger.Error("Project enquiry needs delivery attention", "error", err)
				}
			}
		}
	}
}
func (s *Store) deliverOne(ctx context.Context) error {
	// Resend remembers keys for 24h. Stop well inside that window when an
	// acknowledgement is uncertain; never blindly send it again days later.
	tag, err := s.DB.Exec(ctx, `UPDATE vibe_enquiries SET status='needs_review' WHERE status IN ('sending','received') AND (first_attempt_at<now()-interval '20 hours' OR transport_id<>$1)`, s.TransportID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return fmt.Errorf("%d enquiry acknowledgements require operator review", tag.RowsAffected())
	}
	var id uuid.UUID
	var content []byte
	var lease time.Time
	err = s.DB.QueryRow(ctx, `UPDATE vibe_enquiries SET status='sending',attempts=attempts+1,first_attempt_at=COALESCE(first_attempt_at,now()),lease_until=now()+interval '60 seconds' WHERE id=(SELECT e.id FROM vibe_enquiries e JOIN vibe_sessions s ON s.id=e.session_id WHERE s.deleted_at IS NULL AND ((e.status='received' AND e.next_attempt_at<=now()) OR (e.status='sending' AND e.lease_until<now())) ORDER BY e.created_at FOR UPDATE OF e SKIP LOCKED LIMIT 1) RETURNING id,content,lease_until`).Scan(&id, &content, &lease)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var r Request
	if err = json.Unmarshal(content, &r); err != nil {
		return err
	}
	var active bool
	if err = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_enquiries e JOIN vibe_sessions s ON s.id=e.session_id WHERE e.id=$1 AND e.status='sending' AND e.lease_until=$2 AND s.deleted_at IS NULL)`, id, lease).Scan(&active); err != nil {
		return err
	}
	if !active {
		return nil
	}
	providerID, sendErr := s.Sender.SendMessage(ctx, email.Message{To: s.Recipient, ReplyTo: r.Email, Subject: "AgentClash project enquiry", Text: "Name: " + r.Name + "\nCompany: " + r.Company + "\n\nUser-reviewed project summary:\n" + r.Summary, IdempotencyKey: "vibe-enquiry/" + id.String()})
	if sendErr != nil {
		_, err = s.DB.Exec(ctx, `UPDATE vibe_enquiries SET status='received',next_attempt_at=now()+interval '10 minutes' WHERE id=$1 AND status='sending' AND lease_until=$2`, id, lease)
		if err != nil {
			return err
		}
		return fmt.Errorf("enquiry %s acknowledgement unconfirmed", id)
	}
	_, err = s.DB.Exec(ctx, `UPDATE vibe_enquiries SET status='provider_accepted',provider_id=$3 WHERE id=$1 AND status='sending' AND lease_until=$2`, id, lease, providerID)
	return err
}
