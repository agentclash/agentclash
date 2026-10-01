package enquiries

import (
	"context"
	"errors"
	"github.com/agentclash/agentclash/backend/internal/email"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
)

type recordingSender struct {
	messages []email.Message
	fail     bool
}

func (s *recordingSender) SendMessage(_ context.Context, m email.Message) (string, error) {
	s.messages = append(s.messages, m)
	if s.fail {
		return "", errors.New("lost acknowledgement")
	}
	return "provider-receipt", nil
}
func TestEnquiryValidation(t *testing.T) {
	r := Request{ClientID: uuid.New(), Email: "person@example.test", Summary: "Build a useful agent"}
	if err := Validate(r); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"invalid", "name <person@example.test>", "person@example.test\r\nBcc: victim@example.test"} {
		r.Email = bad
		if Validate(r) == nil {
			t.Fatal("invalid email accepted")
		}
	}
	r.Email = "person@example.test"
	r.Summary = strings.Repeat("x", 12001)
	if Validate(r) == nil {
		t.Fatal("oversized summary accepted")
	}
}
func TestEnquiryDeliveryRecoveryAndConfigDrift(t *testing.T) {
	dsn := os.Getenv("VIBE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated migrated database required")
	}
	if !strings.Contains(dsn, "vibe_test") {
		t.Fatal("refusing non-test database")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	session := uuid.New()
	actor := "anon:" + uuid.NewString()
	if _, err = db.Exec(ctx, `INSERT INTO vibe_sessions(id,actor,document) VALUES($1,$2,'{}')`, session, actor); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(ctx, `DELETE FROM vibe_sessions WHERE id=$1`, session)
	defer db.Exec(ctx, `DELETE FROM vibe_enquiries WHERE session_id=$1`, session)
	sender := &recordingSender{fail: true}
	s := &Store{DB: db, Recipient: "team@example.test", Sender: sender, TransportID: "test-transport"}
	request := Request{ClientID: uuid.New(), Email: "person@example.test", Summary: "A reviewed project brief"}
	first, err := s.Create(ctx, session, actor, request)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.deliverOne(ctx); err == nil {
		t.Fatal("unknown delivery reported success")
	}
	recovered, err := s.Create(ctx, session, actor, request)
	if err != nil || recovered.ID != first.ID {
		t.Fatal("duplicate receipt", err)
	}
	db.Exec(ctx, `UPDATE vibe_enquiries SET next_attempt_at=now() WHERE id=$1`, first.ID)
	sender.fail = false
	if err = s.deliverOne(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 2 || sender.messages[0].IdempotencyKey != sender.messages[1].IdempotencyKey {
		t.Fatal("retry changed provider identity")
	}
	if sender.messages[1].To != s.Recipient || sender.messages[1].ReplyTo != request.Email || !strings.Contains(sender.messages[1].Text, request.Summary) {
		t.Fatal("wrong reviewed content or recipient")
	}
	recovered, _ = s.Get(ctx, session, request.ClientID)
	if recovered.Status != "provider_accepted" {
		t.Fatal(recovered)
	}
	if err = s.deliverOne(ctx); err != nil || len(sender.messages) != 2 {
		t.Fatal("acknowledged enquiry resent", err)
	}
	for _, reason := range []string{"config", "expired"} {
		request.ClientID = uuid.New()
		r, e := s.Create(ctx, session, actor, request)
		if e != nil {
			t.Fatal(e)
		}
		if reason == "config" {
			db.Exec(ctx, `UPDATE vibe_enquiries SET transport_id='changed' WHERE id=$1`, r.ID)
		} else {
			db.Exec(ctx, `UPDATE vibe_enquiries SET first_attempt_at=now()-interval '21 hours' WHERE id=$1`, r.ID)
		}
		if s.deliverOne(ctx) == nil {
			t.Fatal("operator attention was not reported")
		}
		got, _ := s.Get(ctx, session, request.ClientID)
		if got.Status != "needs_review" || len(sender.messages) != 2 {
			t.Fatal("unsafe automatic resend", got)
		}
	}
}
