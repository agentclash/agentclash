package enquiries

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/email"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func contactRequest() ContactRequest {
	return ContactRequest{ClientID: uuid.New(), Email: "person@example.test", Work: "Automate return emails", Company: "Northwind"}
}

func TestContactValidation(t *testing.T) {
	good := contactRequest()
	for name, edit := range map[string]func(*ContactRequest){
		"missing ID":      func(r *ContactRequest) { r.ClientID = uuid.Nil },
		"email":           func(r *ContactRequest) { r.Email = "name <person@example.test>" },
		"email injection": func(r *ContactRequest) { r.Email += "\r\nBcc: other@example.test" },
		"blank work":      func(r *ContactRequest) { r.Work = " \n " },
		"large work":      func(r *ContactRequest) { r.Work = strings.Repeat("é", 2001) },
		"work UTF8":       func(r *ContactRequest) { r.Work = "bad\xff" },
		"work NUL":        func(r *ContactRequest) { r.Work += "\x00" },
		"large company":   func(r *ContactRequest) { r.Company = strings.Repeat("é", 101) },
		"company CR":      func(r *ContactRequest) { r.Company += "\r" },
		"company LF":      func(r *ContactRequest) { r.Company += "\n" },
		"company NUL":     func(r *ContactRequest) { r.Company += "\x00" },
	} {
		t.Run(name, func(t *testing.T) {
			r := good
			edit(&r)
			if !errors.Is(ValidateContact(r), ErrInvalid) {
				t.Fatal("invalid request accepted")
			}
		})
	}
	good.Work = strings.Repeat("é", 2000)
	good.Company = strings.Repeat("é", 100)
	if err := ValidateContact(good); err != nil {
		t.Fatal(err)
	}
}

func TestContactHoneypot(t *testing.T) {
	s := &Store{Recipient: "team@example.test", Sender: &recordingSender{}}
	r := contactRequest()
	r.Website = "https://bot.example.test"
	receipt, err := s.CreateContact(context.Background(), "anon:bot", r)
	if err != nil || receipt.ID == uuid.Nil || receipt.Status != "received" || receipt.CreatedAt.IsZero() {
		t.Fatalf("receipt: %+v, %v", receipt, err)
	}
	if len(s.Sender.(*recordingSender).messages) != 0 {
		t.Fatal("honeypot sent mail")
	}
	// No DB is configured: reaching persistence would panic.
}

func TestContactDisabled(t *testing.T) {
	var s *Store
	if _, err := s.CreateContact(context.Background(), "anon:test", contactRequest()); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
}

func contactDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("VIBE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated migrated database required")
	}
	if !strings.Contains(dsn, "vibe_test") {
		t.Fatal("refusing non-test database")
	}
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}
func contactStore(t *testing.T) (*Store, string) {
	t.Helper()
	db := contactDatabase(t)
	actor := "anon:" + uuid.NewString()
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), `DELETE FROM vibe_enquiries WHERE actor=$1`, actor) })
	return &Store{DB: db, Recipient: "team@example.test", Sender: &recordingSender{}, TransportID: "test-transport"}, actor
}
func TestContactCreateIdempotencyQuota(t *testing.T) {
	ctx := context.Background()
	s, actor := contactStore(t)
	r := contactRequest()
	first, err := s.CreateContact(ctx, actor, r)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.CreateContact(ctx, actor, r)
	if err != nil || first != again {
		t.Fatalf("retry: %+v %v", again, err)
	}
	changed := r
	changed.Work = "Different work"
	if _, err = s.CreateContact(ctx, actor, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("expected conflict", err)
	}
	var kind, owner string
	var noSession bool
	if err = s.DB.QueryRow(ctx, `SELECT content->>'kind',actor,session_id IS NULL FROM vibe_enquiries WHERE id=$1`, first.ID).Scan(&kind, &owner, &noSession); err != nil || kind != "contact" || owner != actor || !noSession {
		t.Fatalf("contact row: %s %s %v %v", kind, owner, noSession, err)
	}
	for i := 0; i < 2; i++ {
		r.ClientID = uuid.New()
		if _, err = s.CreateContact(ctx, actor, r); err != nil {
			t.Fatal(err)
		}
	}
	r.ClientID = uuid.New()
	if _, err = s.CreateContact(ctx, actor, r); !errors.Is(err, ErrQuota) || strings.Contains(err.Error(), "project") {
		t.Fatal("expected contact quota", err)
	}
	otherActor := "anon:" + uuid.NewString()
	t.Cleanup(func() { _, _ = s.DB.Exec(ctx, `DELETE FROM vibe_enquiries WHERE actor=$1`, otherActor) })
	independentRequest := changed
	independentRequest.ClientID = uuid.New()
	independent, err := s.CreateContact(ctx, otherActor, independentRequest)
	if err != nil || independent.ID == first.ID {
		t.Fatal("actors share quota", err)
	}
	// Idempotency still succeeds at quota, and a row older than 24h releases a slot.
	r.ClientID = changed.ClientID
	r.Work = contactRequest().Work
	if _, err = s.CreateContact(ctx, actor, r); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(ctx, `UPDATE vibe_enquiries SET created_at=now()-interval '25 hours' WHERE id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	r.ClientID = uuid.New()
	if _, err = s.CreateContact(ctx, actor, r); err != nil {
		t.Fatal(err)
	}
}
func TestContactConcurrentQuotaAndIdempotency(t *testing.T) {
	ctx := context.Background()
	s, actor := contactStore(t)
	r := contactRequest()
	receipts := make(chan Receipt, 8)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); v, err := s.CreateContact(ctx, actor, r); receipts <- v; errs <- err }()
	}
	wg.Wait()
	close(receipts)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first uuid.UUID
	for v := range receipts {
		if first == uuid.Nil {
			first = v.ID
		}
		if v.ID != first {
			t.Fatal("duplicate concurrent receipt")
		}
	}
	errs = make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			next := r
			next.ClientID = uuid.New()
			_, err := s.CreateContact(ctx, actor, next)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrQuota) {
			t.Fatal(err)
		}
	}
	if successes != 2 {
		t.Fatalf("quota allowed %d additional enquiries", successes)
	}
}
func TestContactAndProjectDelivery(t *testing.T) {
	ctx := context.Background()
	s, actor := contactStore(t)
	r := contactRequest()
	contact, err := s.CreateContact(ctx, actor, r)
	if err != nil {
		t.Fatal(err)
	}
	session := uuid.New()
	if _, err = s.DB.Exec(ctx, `INSERT INTO vibe_sessions(id,actor,document) VALUES($1,$2,'{}')`, session, actor); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.DB.Exec(ctx, `DELETE FROM vibe_enquiries WHERE session_id=$1`, session)
		_, _ = s.DB.Exec(ctx, `DELETE FROM vibe_sessions WHERE id=$1`, session)
	})
	project := Request{ClientID: uuid.New(), Email: "project@example.test", Summary: "Reviewed project", Company: "Project Co", Name: "Owner"}
	projectReceipt, err := s.Create(ctx, session, actor, project)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = s.deliverOne(ctx); err != nil {
			t.Fatal(err)
		}
		// Check immediately, before the next maintenance pass can erase it.
		var status, content string
		if err = s.DB.QueryRow(ctx, `SELECT status,content::text FROM vibe_enquiries WHERE id=$1`, contact.ID).Scan(&status, &content); err != nil {
			t.Fatal(err)
		}
		if status == "provider_accepted" && content != "{}" {
			t.Fatalf("acceptance retained contact content: %s", content)
		}
	}
	messages := s.Sender.(*recordingSender).messages
	if len(messages) != 2 {
		t.Fatalf("expected two messages, got %d", len(messages))
	}
	byKey := map[string]email.Message{}
	for _, m := range messages {
		byKey[m.IdempotencyKey] = m
		if m.To != s.Recipient {
			t.Fatal("wrong recipient")
		}
	}
	m := byKey["vibe-enquiry/"+contact.ID.String()]
	if m.Subject != "AgentClash: agents for a team" || m.ReplyTo != r.Email || m.Text != "Company: Northwind\n\nWork to automate:\nAutomate return emails" {
		t.Fatalf("contact: %+v", m)
	}
	m = byKey["vibe-enquiry/"+projectReceipt.ID.String()]
	if m.Subject != "AgentClash project enquiry" || m.ReplyTo != project.Email || m.Text != "Name: Owner\nCompany: Project Co\n\nUser-reviewed project summary:\nReviewed project" {
		t.Fatalf("project: %+v", m)
	}

	var contactContent, projectContent string
	if err = s.DB.QueryRow(ctx, `SELECT content::text FROM vibe_enquiries WHERE id=$1`, contact.ID).Scan(&contactContent); err != nil || contactContent != "{}" {
		t.Fatalf("accepted contact retained content: %s %v", contactContent, err)
	}
	if err = s.DB.QueryRow(ctx, `SELECT content->>'summary' FROM vibe_enquiries WHERE id=$1`, projectReceipt.ID).Scan(&projectContent); err != nil || projectContent != project.Summary {
		t.Fatalf("project content changed: %s %v", projectContent, err)
	}
	again, err := s.CreateContact(ctx, "anon:"+uuid.NewString(), r)
	if err != nil || again.ID != contact.ID || again.Status != "provider_accepted" {
		t.Fatalf("redacted retry: %+v %v", again, err)
	}
	changed := r
	changed.Work = "Different work"
	if _, err = s.CreateContact(ctx, actor, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("redacted conflict", err)
	}
	if err = s.deliverOne(ctx); err != nil || len(s.Sender.(*recordingSender).messages) != 2 {
		t.Fatal("accepted enquiry resent", err)
	}
	project.ClientID = uuid.New()
	if _, err = s.Create(ctx, session, actor, project); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(ctx, `UPDATE vibe_sessions SET deleted_at=now() WHERE id=$1`, session); err != nil {
		t.Fatal(err)
	}
	if err = s.deliverOne(ctx); err != nil || len(s.Sender.(*recordingSender).messages) != 2 {
		t.Fatal("deleted project delivered", err)
	}
}

func TestContactIdempotencyAcrossActors(t *testing.T) {
	ctx := context.Background()
	s, actor := contactStore(t)
	r := contactRequest()
	t.Cleanup(func() {
		_, _ = s.DB.Exec(ctx, `DELETE FROM vibe_enquiries WHERE session_id IS NULL AND client_id=$1`, r.ClientID)
	})
	first, err := s.CreateContact(ctx, actor, r)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.CreateContact(ctx, "anon:"+uuid.NewString(), r)
	if err != nil || again != first {
		t.Fatalf("first-contact retry: %+v %v; want %+v", again, err, first)
	}
	var count int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM vibe_enquiries WHERE session_id IS NULL AND client_id=$1`, r.ClientID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("stored %d rows: %v", count, err)
	}
	r.Work = "Different work"
	if _, err = s.CreateContact(ctx, "anon:"+uuid.NewString(), r); !errors.Is(err, ErrConflict) {
		t.Fatal("expected cross-actor conflict", err)
	}
}

func TestContactConcurrentIdempotencyAcrossActors(t *testing.T) {
	ctx := context.Background()
	s, _ := contactStore(t)
	r := contactRequest()
	t.Cleanup(func() {
		_, _ = s.DB.Exec(ctx, `DELETE FROM vibe_enquiries WHERE session_id IS NULL AND client_id=$1`, r.ClientID)
	})
	type result struct {
		receipt Receipt
		err     error
	}
	results := make(chan result, 8)
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			<-start
			receipt, err := s.CreateContact(ctx, "anon:"+uuid.NewString(), r)
			results <- result{receipt, err}
		}()
	}
	close(start)
	var gotResults []result
	for i := 0; i < 8; i++ {
		gotResults = append(gotResults, <-results)
	}
	var first Receipt
	for i, got := range gotResults {
		if got.err != nil {
			t.Fatal(got.err)
		}
		if i == 0 {
			first = got.receipt
		}
		if got.receipt != first {
			t.Fatalf("duplicate receipt: %+v, want %+v", got.receipt, first)
		}
	}
	var count int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM vibe_enquiries WHERE session_id IS NULL AND client_id=$1`, r.ClientID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("stored %d rows: %v", count, err)
	}
}

func TestContactContentRetentionMaintenance(t *testing.T) {
	ctx := context.Background()
	s, actor := contactStore(t)
	session := uuid.New()
	if _, err := s.DB.Exec(ctx, `INSERT INTO vibe_sessions(id,actor,document) VALUES($1,$2,'{}')`, session, actor); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.DB.Exec(ctx, `DELETE FROM vibe_enquiries WHERE session_id=$1`, session)
		_, _ = s.DB.Exec(ctx, `DELETE FROM vibe_sessions WHERE id=$1`, session)
	})
	for _, project := range []bool{false, true} {
		for _, status := range []string{"received", "sending", "provider_accepted", "needs_review", "cancelled"} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("project=%v/status=%s/old=%v", project, status, old), func(t *testing.T) {
					id := uuid.New()
					var sessionID *uuid.UUID
					if project {
						sessionID = &session
					}
					if _, err := s.DB.Exec(ctx, `INSERT INTO vibe_enquiries(id,session_id,actor,client_id,request_hash,content,transport_id,status,created_at,next_attempt_at,lease_until) VALUES($1,$2,$3,$4,'retained-hash','{"email":"person@example.test","work":"private"}',$5,$6,now()-CASE WHEN $7 THEN interval '31 days' ELSE interval '29 days' END,now()+interval '1 day',now()+interval '1 day')`, id, sessionID, actor, uuid.New(), s.TransportID, status, old); err != nil {
						t.Fatal(err)
					}
					var before string
					if err := s.DB.QueryRow(ctx, `SELECT (to_jsonb(e)-'content')::text FROM vibe_enquiries e WHERE id=$1`, id).Scan(&before); err != nil {
						t.Fatal(err)
					}
					if err := s.deliverOne(ctx); err != nil {
						t.Fatal(err)
					}
					var content, after string
					if err := s.DB.QueryRow(ctx, `SELECT content::text,(to_jsonb(e)-'content')::text FROM vibe_enquiries e WHERE id=$1`, id).Scan(&content, &after); err != nil {
						t.Fatal(err)
					}
					if (content == "{}") != (!project && (old || status == "provider_accepted")) {
						t.Fatalf("unexpected retained content: %s", content)
					}
					if before != after {
						t.Fatalf("receipt metadata changed: %s -> %s", before, after)
					}
				})
			}
		}
	}
	if len(s.Sender.(*recordingSender).messages) != 0 {
		t.Fatal("retention maintenance sent mail")
	}
}
