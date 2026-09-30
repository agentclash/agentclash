package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/email"
	"github.com/agentclash/agentclash/backend/internal/enquiries"

	"github.com/agentclash/agentclash/backend/internal/vibe/access"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testEnquirySender struct{}

func (*testEnquirySender) SendMessage(context.Context, email.Message) (string, error) {
	return "fake-delivery", nil
}

type queryCounter struct{ count atomic.Int32 }

func (q *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	q.count.Add(1)
	return ctx
}
func (*queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestVibeFileAndSnapshotQueriesAreBounded(t *testing.T) {
	h := newReliabilityHarness(t, 1)
	cfg, err := pgxpool.ParseConfig(os.Getenv("VIBE_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	counter := &queryCounter{}
	cfg.ConnConfig.Tracer = counter
	db, err := pgxpool.NewWithConfig(h.ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
 defer db.Exec(h.ctx,`DELETE FROM vibe_enquiries WHERE session_id=$1`,h.session.ID)
	h.svc.Store.DB = db
	h.svc.Store.Inputs.DB = db
	contact := &enquiries.Store{DB: db, Recipient: "team@example.test", Sender: &testEnquirySender{}}
	handler := (&VibeHandler{Service: h.svc, Auth: NewDevelopmentAuthenticator(), Enquiries: contact}).Routes()
	request := func(method, path string, body []byte) (int, int) {
		counter.count.Store(0)
		r := httptest.NewRequest(method, "/sessions/"+h.session.ID.String()+path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set(headerUserID, h.user.String())
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if path == "" && bytes.Contains(w.Body.Bytes(), []byte("hidden-actual-reply")) {
			t.Fatal("snapshot exposed full evidence")
		}
		if w.Code >= 400 {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Code, int(counter.count.Load())
	}
	createHistory := func(count int) {
		_, err := db.Exec(h.ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,deadline) SELECT gen_random_uuid(),$1,$2,gen_random_uuid(),'fixture','message','FAILED','RELEASED',$3,'{}',0,now() FROM generate_series(1,$4)`, h.session.ID, h.actor, mustJSON(t, h.svc.Config.DefaultModels()), count)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(h.ctx, `INSERT INTO vibe_case_results(operation_id,case_key,version,result) SELECT id,'one','fixture','{"case_key":"one","title":"Situation","verdict":"PASS","output":"hidden-actual-reply"}' FROM vibe_operations WHERE session_id=$1 ON CONFLICT DO NOTHING`, h.session.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	createHistory(1)
	var baseline map[string]int
	for _, size := range []int{1, 100} {
		if size == 100 {
			createHistory(99)
		}
		input, err := h.svc.Store.Inputs.Create(h.ctx, h.session.ID, h.actor, uuid.New(), "text", "Notes", []byte("Useful supplied material"))
		if err != nil {
			t.Fatal(err)
		}
		observations := map[string]int{}
		for _, path := range []string{"/inputs", "/inputs/" + input.ID.String(), "/inputs/" + input.ID.String() + "/download"} {
			_, n := request(http.MethodGet, path, nil)
			key := path
			if path != "/inputs" {
				key = "read"
				if len(path) > len("/inputs/")+36 {
					key = "download"
				}
			}
			observations[key] = n
		}
		_, observations["snapshot"] = request(http.MethodGet, "", nil)
		_, observations["delete"] = request(http.MethodDelete, "/inputs/"+input.ID.String(), nil)
		body, _ := json.Marshal(enquiries.Request{ClientID: uuid.New(), Email: "person@example.test", Summary: "Reviewed project"})
		_, observations["contact"] = request(http.MethodPost, "/enquiries", body)
		if size == 1 {
			baseline = observations
		} else {
			for key, n := range observations {
				if n != baseline[key] {
					t.Errorf("%s queries grew: 1=%d 100=%d", key, baseline[key], n)
				}
			}
		}
		if observations["snapshot"] > 12 {
			t.Fatalf("unbounded snapshot: %v", observations)
		}
		t.Logf("%d operations: %v", size, observations)
	}
	// Revocation is checked inside writes too, not only by the HTTP precheck.
	if _, err = db.Exec(h.ctx, `UPDATE users SET archived_at=now() WHERE id=$1`, h.user); err != nil {
		t.Fatal(err)
	}
	if _, err = h.svc.Store.SessionAccess(h.ctx, h.actor, h.session.ID, false); err == nil {
		t.Fatal("revoked access admitted")
	}
	if err = h.svc.Store.Inputs.Delete(h.ctx, h.session.ID, uuid.New(), h.actor); !errors.Is(err, access.ErrForbidden) {
		t.Fatalf("file write missed revocation: %v", err)
	}
	if _, err = contact.Create(h.ctx, h.session.ID, h.actor, enquiries.Request{ClientID: uuid.New(), Email: "person@example.test", Summary: "Reviewed project"}); !errors.Is(err, access.ErrForbidden) {
		t.Fatalf("contact write missed revocation: %v", err)
	}
}
