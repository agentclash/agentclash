package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRepositoryBillingCheckoutIntentExpiresAndLatePaymentCompletes(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	createIntent := func(age time.Duration) uuid.UUID {
		t.Helper()
		intent, err := repo.CreateBillingCheckoutIntent(ctx, repository.BillingCheckoutIntentInput{
			OrganizationID:   fixture.organizationID,
			CreatedByUserID:  fixture.userID,
			RequestedPlanKey: billing.PlanPro,
			BillingPeriod:    billing.PeriodMonthly,
			SeatQuantity:     1,
			ReturnURL:        "https://app.example.com/billing",
			CheckoutURL:      "https://checkout.example.com/" + uuid.NewString(),
			DodoCheckoutID:   "cks_" + uuid.NewString(),
		})
		if err != nil {
			t.Fatalf("CreateBillingCheckoutIntent returned error: %v", err)
		}
		if _, err := db.Exec(ctx, `UPDATE billing_checkout_intents SET created_at = $2 WHERE id = $1`, intent.ID, time.Now().Add(-age)); err != nil {
			t.Fatalf("backdate checkout intent returned error: %v", err)
		}
		return intent.ID
	}
	statusOf := func(id uuid.UUID) string {
		t.Helper()
		var status string
		if err := db.QueryRow(ctx, `SELECT status FROM billing_checkout_intents WHERE id = $1`, id).Scan(&status); err != nil {
			t.Fatalf("select checkout intent status returned error: %v", err)
		}
		return status
	}
	sweep := func(want int64) {
		t.Helper()
		expired, err := repo.ExpireStaleBillingCheckoutIntents(ctx, time.Now())
		if err != nil {
			t.Fatalf("ExpireStaleBillingCheckoutIntents returned error: %v", err)
		}
		if expired != want {
			t.Fatalf("sweep expired %d intents, want %d", expired, want)
		}
	}
	completeByWebhook := func(intentID uuid.UUID) {
		t.Helper()
		duplicate, err := repo.ApplyBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
			WebhookID: "wh_late_payment_" + uuid.NewString(),
			EventType: "payment.succeeded",
			Payload:   []byte(`{"type":"payment.succeeded"}`),
		}, repository.BillingWebhookApplication{CheckoutIntentID: &intentID})
		if err != nil {
			t.Fatalf("ApplyBillingWebhookEvent returned error: %v", err)
		}
		if duplicate {
			t.Fatal("a new webhook id must not be a duplicate")
		}
	}

	// The user left checkout 25 hours ago. The Dodo session is gone.
	abandoned := createIntent(25 * time.Hour)
	sweep(1)
	if got := statusOf(abandoned); got != string(repository.BillingCheckoutIntentStatusExpired) {
		t.Fatalf("abandoned intent status = %q, want expired", got)
	}
	overview, err := repo.GetBillingOverview(ctx, fixture.organizationID)
	if err != nil {
		t.Fatalf("GetBillingOverview returned error: %v", err)
	}
	if overview.LatestCheckoutIntent == nil || overview.LatestCheckoutIntent.ID != abandoned {
		t.Fatalf("latest checkout intent = %+v, want %s", overview.LatestCheckoutIntent, abandoned)
	}
	if overview.LatestCheckoutIntent.Status != string(repository.BillingCheckoutIntentStatusExpired) {
		t.Fatalf("overview checkout status = %q, want expired", overview.LatestCheckoutIntent.Status)
	}

	// A checkout from 23 hours ago is still open.
	open := createIntent(23 * time.Hour)
	sweep(0)
	if got := statusOf(open); got != string(repository.BillingCheckoutIntentStatusCreated) {
		t.Fatalf("open intent status = %q, want created", got)
	}

	// A late payment still completes the expired intent.
	completeByWebhook(abandoned)
	if got := statusOf(abandoned); got != string(repository.BillingCheckoutIntentStatusCompleted) {
		t.Fatalf("late payment intent status = %q, want completed", got)
	}
	// A second webhook for the same checkout is a no-op, not an error.
	completeByWebhook(abandoned)
	if got := statusOf(abandoned); got != string(repository.BillingCheckoutIntentStatusCompleted) {
		t.Fatalf("intent status after repeat webhook = %q, want completed", got)
	}
	// The sweep never touches a completed intent.
	sweep(0)
	if got := statusOf(abandoned); got != string(repository.BillingCheckoutIntentStatusCompleted) {
		t.Fatalf("intent status after second sweep = %q, want completed", got)
	}
}

func TestRepositoryBillingCheckoutIntentStatusCheckMatchesLifecycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)

	insert := func(status string) error {
		_, err := db.Exec(ctx, `
			INSERT INTO billing_checkout_intents (
				organization_id, requested_plan_key, billing_period, seat_quantity,
				return_url, checkout_url, status
			)
			VALUES ($1, 'pro', 'monthly', 1, 'https://app.example.com/billing', $2, $3)
		`, fixture.organizationID, "https://checkout.example.com/"+uuid.NewString(), status)
		return err
	}

	for _, status := range []repository.BillingCheckoutIntentStatus{
		repository.BillingCheckoutIntentStatusCreated,
		repository.BillingCheckoutIntentStatusExpired,
		repository.BillingCheckoutIntentStatusCompleted,
	} {
		if err := insert(string(status)); err != nil {
			t.Errorf("insert status %q: %v, want no error", status, err)
		}
	}
	for _, status := range []string{"pending", "cancelled", "bogus"} {
		err := insert(status)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("insert status %q: err = %v, want SQLSTATE 23514", status, err)
		}
	}
}

// The SQL guard of each writer must name the same old statuses as the Go
// transition table.
func TestRepositoryBillingCheckoutIntentGuardsMatchTransitionTable(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	all := []repository.BillingCheckoutIntentStatus{
		repository.BillingCheckoutIntentStatusCreated,
		repository.BillingCheckoutIntentStatusExpired,
		repository.BillingCheckoutIntentStatusCompleted,
	}

	insert := func(status repository.BillingCheckoutIntentStatus, createdAt time.Time) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := db.QueryRow(ctx, `
			INSERT INTO billing_checkout_intents (
				organization_id, requested_plan_key, billing_period, seat_quantity,
				return_url, checkout_url, status, created_at
			)
			VALUES ($1, 'pro', 'monthly', 1, 'https://app.example.com/billing', $2, $3, $4)
			RETURNING id
		`, fixture.organizationID, "https://checkout.example.com/"+uuid.NewString(), string(status), createdAt).Scan(&id); err != nil {
			t.Fatalf("insert checkout intent %q returned error: %v", status, err)
		}
		return id
	}
	statusOf := func(id uuid.UUID) repository.BillingCheckoutIntentStatus {
		t.Helper()
		var status string
		if err := db.QueryRow(ctx, `SELECT status FROM billing_checkout_intents WHERE id = $1`, id).Scan(&status); err != nil {
			t.Fatalf("select checkout intent status returned error: %v", err)
		}
		return repository.BillingCheckoutIntentStatus(status)
	}

	// The sweep: old rows in each status.
	old := make(map[repository.BillingCheckoutIntentStatus]uuid.UUID)
	for _, status := range all {
		old[status] = insert(status, time.Now().Add(-48*time.Hour))
	}
	if _, err := repo.ExpireStaleBillingCheckoutIntents(ctx, time.Now()); err != nil {
		t.Fatalf("ExpireStaleBillingCheckoutIntents returned error: %v", err)
	}
	for _, from := range all {
		want := from
		if from.CanTransitionTo(repository.BillingCheckoutIntentStatusExpired) {
			want = repository.BillingCheckoutIntentStatusExpired
		}
		if got := statusOf(old[from]); got != want {
			t.Errorf("sweep: %s -> %s, want %s", from, got, want)
		}
	}

	// The webhook complete: new rows in each status.
	for _, from := range all {
		id := insert(from, time.Now())
		if _, err := repo.ApplyBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
			WebhookID: "wh_guard_" + uuid.NewString(),
			EventType: "payment.succeeded",
			Payload:   []byte(`{"type":"payment.succeeded"}`),
		}, repository.BillingWebhookApplication{CheckoutIntentID: &id}); err != nil {
			t.Fatalf("ApplyBillingWebhookEvent for %s intent returned error: %v", from, err)
		}
		want := from
		if from.CanTransitionTo(repository.BillingCheckoutIntentStatusCompleted) {
			want = repository.BillingCheckoutIntentStatusCompleted
		}
		if got := statusOf(id); got != want {
			t.Errorf("complete: %s -> %s, want %s", from, got, want)
		}
	}
}
