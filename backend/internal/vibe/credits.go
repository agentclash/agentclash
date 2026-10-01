package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CreditProduct struct {
	ID         string `json:"id"`
	Credits    int64  `json:"credits_nano_usd"`
	PriceMinor int64  `json:"price_minor"`
	Currency   string `json:"currency"`
}
type Checkout struct {
	ID             uuid.UUID     `json:"id"`
	OrganizationID uuid.UUID     `json:"organization_id"`
	Product        CreditProduct `json:"product"`
	State          string        `json:"state"`
	URL            *string       `json:"checkout_url"`
	RemoteID       *string       `json:"remote_id"`
}

func (s *Store) BeginCheckout(ctx context.Context, user, org, id uuid.UUID, p CreditProduct) (Checkout, bool, error) {
	var c Checkout
	created := false
	if id == uuid.Nil || p.ID == "" || p.Credits <= 0 || p.Credits > 1000*NanoUSD || p.PriceMinor <= 0 || p.PriceMinor > 1_000_000 || p.Currency != "USD" {
		return c, false, fault("invalid_product", "The credit product is not configured.")
	}
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := lockScopes(ctx, tx, "checkout:"+id.String()); err != nil {
			return err
		}
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_memberships m JOIN organizations g ON g.id=m.organization_id AND g.archived_at IS NULL JOIN users u ON u.id=m.user_id AND u.archived_at IS NULL WHERE m.organization_id=$1 AND m.user_id=$2 AND m.role='org_admin' AND m.membership_status='active')`, org, user).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return fault("forbidden", "Only an organization administrator can purchase credits.")
		}
		var frozen bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_accounts WHERE id=$1 AND disabled)", "org:"+org.String()).Scan(&frozen); err != nil {
			return err
		}
		if frozen {
			return fault("accounting_unavailable", "This credit account is under review. Purchases are paused until accounting is resolved.")
		}
		err := tx.QueryRow(ctx, "SELECT organization_id,product_id,credits,price_minor,currency,state,checkout_url,remote_id FROM vibe_credit_checkouts WHERE id=$1", id).Scan(&c.OrganizationID, &c.Product.ID, &c.Product.Credits, &c.Product.PriceMinor, &c.Product.Currency, &c.State, &c.URL, &c.RemoteID)
		c.ID = id
		if err == nil {
			if c.OrganizationID != org || c.Product != p {
				return fault("idempotency_conflict", "This checkout ID was already used for a different purchase.")
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO vibe_credit_checkouts(id,organization_id,created_by,product_id,credits,price_minor,currency,state) VALUES($1,$2,$3,$4,$5,$6,$7,'DISPATCHING')", id, org, user, p.ID, p.Credits, p.PriceMinor, p.Currency)
		created = err == nil
		c = Checkout{ID: id, OrganizationID: org, Product: p, State: "DISPATCHING"}
		return err
	})
	return c, created, err
}
func (s *Store) CheckoutResult(ctx context.Context, id uuid.UUID, remote, url string, failed bool) error {
	state := "READY"
	if failed {
		state = "UNCERTAIN"
	}
	_, err := s.DB.Exec(ctx, "UPDATE vibe_credit_checkouts SET remote_id=NULLIF($2,''),checkout_url=NULLIF($3,''),state=CASE WHEN state='PAID' THEN state ELSE $4 END WHERE id=$1", id, remote, url, state)
	return err
}

// ApplyCreditPayment receives a signature-verified Dodo payment. Checkout amount,
// currency, cart and destination must all match the durable pre-checkout intent.
// A payment ID can credit exactly one intent, even with different webhook IDs.
func (s *Store) ApplyCreditPayment(ctx context.Context, id, org uuid.UUID, payment, remote, product, currency string, subtotal int64, payload json.RawMessage) error {
	if payment == "" || len(payment) > 256 || subtotal <= 0 {
		return fmt.Errorf("invalid payment evidence")
	}
	return s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := lockScopes(ctx, tx, "payment:"+payment); err != nil {
			return err
		}
		var c Checkout
		if err := tx.QueryRow(ctx, "SELECT organization_id,product_id,credits,price_minor,currency,remote_id FROM vibe_credit_checkouts WHERE id=$1 FOR UPDATE", id).Scan(&c.OrganizationID, &c.Product.ID, &c.Product.Credits, &c.Product.PriceMinor, &c.Product.Currency, &c.RemoteID); err != nil {
			return err
		}
		if c.OrganizationID != org || c.Product.ID != product || c.Product.PriceMinor != subtotal || c.Product.Currency != currency || (c.RemoteID != nil && *c.RemoteID != remote) {
			return fault("payment_mismatch", "Payment does not match its credit checkout.")
		}
		var old uuid.UUID
		err := tx.QueryRow(ctx, "SELECT checkout_id FROM vibe_credit_payment_events WHERE payment_id=$1", payment).Scan(&old)
		if err == nil {
			if old != id {
				return fault("payment_mismatch", "Payment already belongs to another checkout.")
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err = grant(ctx, tx, "org:"+org.String(), "dodo-payment:"+payment, c.Product.Credits); err != nil {
			return err
		}
		// Refund/dispute events may arrive before payment.succeeded. Preserve the
		// verified review marker and freeze the eventual grant atomically.
		if _, err = tx.Exec(ctx, "UPDATE vibe_accounts SET disabled=true WHERE id=$1 AND EXISTS(SELECT 1 FROM vibe_credit_reviews WHERE payment_id=$2)", "org:"+org.String(), payment); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO vibe_credit_payment_events(payment_id,checkout_id,organization_id,amount,payload) VALUES($1,$2,$3,$4,$5)", payment, id, org, c.Product.Credits, payload)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "UPDATE vibe_credit_checkouts SET state='PAID',payment_id=$2 WHERE id=$1", id, payment)
		return err
	})
}

// ReviewCreditPayment is accounting-only. A verified refund/dispute freezes the
// funded wallet; it does not auto-refund, discard evidence, or release holds.
func (s *Store) ReviewCreditPayment(ctx context.Context, source, payment, kind string, payload json.RawMessage) error {
	if source == "" || payment == "" || len(source) > 512 || len(payment) > 256 {
		return fmt.Errorf("invalid credit review evidence")
	}
	return s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := lockScopes(ctx, tx, "payment:"+payment); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO vibe_credit_reviews(source,payment_id,event_type,payload) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", source, payment, kind, payload); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "UPDATE vibe_accounts SET disabled=true WHERE id IN (SELECT 'org:'||organization_id::text FROM vibe_credit_payment_events WHERE payment_id=$1)", payment)
		return err
	})
}
