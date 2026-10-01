package api

import (
	"context"
	"encoding/json"
	"fmt"
	billingpkg "github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type VibeCredits struct {
	Store      *vibe.Store
	Products   []vibe.CreditProduct
	Allowances map[string]int64
	ReturnURL  string
}

func LoadVibeCredits(store *vibe.Store, frontend string) (*VibeCredits, error) {
	c := &VibeCredits{Store: store, Allowances: map[string]int64{}, ReturnURL: strings.TrimRight(frontend, "/") + "/vibe-evals"}
	if b := os.Getenv("VIBE_TOPUP_PRODUCTS_JSON"); b != "" {
		if err := vibe.Decode([]byte(b), vibe.LimitsFor(false), &c.Products); err != nil {
			return nil, err
		}
	}
	if b := os.Getenv("VIBE_PLAN_ALLOWANCES_JSON"); b != "" {
		if err := vibe.Decode([]byte(b), vibe.LimitsFor(false), &c.Allowances); err != nil {
			return nil, err
		}
	}
	return c, nil
}
func (h *VibeHandler) creditBalance(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		vibeError(w, err)
		return
	}
	ws, err := uuid.Parse(r.URL.Query().Get("workspace_id"))
	if err != nil {
		vibeError(w, &vibe.Fault{Code: "workspace_required", Message: "Choose a workspace."})
		return
	}
	if err = h.Service.Store.Authorize(r.Context(), vibe.Session{Actor: actor, WorkspaceID: &ws}, false); err != nil {
		vibeError(w, err)
		return
	}
	var org uuid.UUID
	if err = h.Service.Store.DB.QueryRow(r.Context(), "SELECT organization_id FROM workspaces WHERE id=$1", ws).Scan(&org); err != nil {
		vibeError(w, err)
		return
	}
	b, held, err := h.Service.Store.Balance(r.Context(), "org:"+org.String())
	if err != nil {
		vibeError(w, err)
		return
	}
	products := []vibe.CreditProduct{}
	var underReview bool
	if err = h.Service.Store.DB.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM vibe_accounts WHERE id=$1 AND disabled)", "org:"+org.String()).Scan(&underReview); err != nil {
		vibeError(w, err)
		return
	}
	if h.Billing != nil && h.Billing.vibeCredits != nil {
		products = h.Billing.vibeCredits.Products
	}
	vibeJSON(w, 200, map[string]any{"under_review": underReview, "balance_nano_usd": b, "held_nano_usd": held, "available_nano_usd": b - held, "products": products})
}
func (h *VibeHandler) creditCheckout(w http.ResponseWriter, r *http.Request) {
	if h.Billing == nil || h.Billing.vibeCredits == nil || h.Billing.dodoAPIKey == "" {
		vibeError(w, &vibe.Fault{Code: "hosted_disabled", Message: "Credit top-ups are not configured."})
		return
	}
	caller, err := h.Auth.Authenticate(r)
	if err != nil {
		vibeError(w, &vibe.Fault{Code: "forbidden", Message: "Sign in to purchase workspace credits."})
		return
	}
	var input struct {
		ID          uuid.UUID `json:"id"`
		WorkspaceID uuid.UUID `json:"workspace_id"`
		ProductID   string    `json:"product_id"`
	}
	if err = vibeBody(w, r, false, &input); err != nil {
		vibeError(w, err)
		return
	}
	if err = h.Service.Gate.Check(r.Context(), "user:"+caller.UserID.String(), vibe.LimitsFor(false)); err != nil {
		vibeError(w, err)
		return
	}
	var org uuid.UUID
	if err = h.Service.Store.DB.QueryRow(r.Context(), "SELECT organization_id FROM workspaces WHERE id=$1", input.WorkspaceID).Scan(&org); err != nil {
		vibeError(w, err)
		return
	}
	var product vibe.CreditProduct
	for _, p := range h.Billing.vibeCredits.Products {
		if p.ID == input.ProductID {
			product = p
		}
	}
	checkout, created, err := h.Service.Store.BeginCheckout(r.Context(), caller.UserID, org, input.ID, product)
	if err != nil {
		vibeError(w, err)
		return
	}
	if !created {
		vibeJSON(w, 200, checkout)
		return
	}
	metadata, _ := json.Marshal(map[string]string{"purpose": "vibe_credit_topup", "organization_id": org.String(), "checkout_intent_id": input.ID.String()})
	returnURL := h.Billing.vibeCredits.ReturnURL + "?workspace=" + input.WorkspaceID.String() + "&credit_checkout=" + input.ID.String()
	remoteURL, remoteID, callErr := h.Billing.createDodoCheckoutSession(r.Context(), caller, org, input.ID, billingpkg.Plan{DodoProductIDs: map[string]string{"topup": product.ID}}, CreateBillingCheckoutInput{BillingPeriod: "topup", ReturnURL: returnURL}, metadata)
	if err = h.Service.Store.CheckoutResult(context.WithoutCancel(r.Context()), input.ID, remoteID, remoteURL, callErr != nil); err != nil {
		vibeError(w, err)
		return
	}
	if callErr != nil {
		vibeError(w, &vibe.Fault{Code: "checkout_uncertain", Message: "The payment provider did not confirm the checkout. This purchase attempt will not be automatically repeated."})
		return
	}
	checkout.State = "READY"
	checkout.URL = &remoteURL
	vibeJSON(w, 201, checkout)
}

func (m *BillingManager) applyVibeCreditWebhook(ctx context.Context, e dodoWebhookEnvelope, body []byte) (bool, error) {
	if m.vibeCredits != nil && (e.Type == "refund.succeeded" || strings.HasPrefix(e.Type, "dispute.")) {
		payment := e.DataString("payment_id")
		id := e.DataString("refund_id", "dispute_id")
		if payment == "" || id == "" {
			return true, fmt.Errorf("credit review lacks payment or event identity")
		}
		if err := m.vibeCredits.Store.ReviewCreditPayment(ctx, e.Type+":"+id, payment, e.Type, body); err != nil {
			return true, err
		}
		// Existing subscription processing may also need this verified event.
	}
	purpose := stringFromMap(e.DataObject("metadata"), "purpose")
	if purpose == "" {
		purpose = stringFromMap(e.Metadata, "purpose")
	}
	if purpose != "vibe_credit_topup" {
		return false, nil
	}
	if m.vibeCredits == nil {
		return true, fmt.Errorf("Vibe credit accounting is not configured")
	}
	if e.Type != "payment.succeeded" {
		return true, nil
	}
	var envelope struct {
		Data struct {
			PaymentID  string `json:"payment_id"`
			Status     string `json:"status"`
			Currency   string `json:"currency"`
			Total      int64  `json:"total_amount"`
			Tax        int64  `json:"tax"`
			CheckoutID string `json:"checkout_session_id"`
			Cart       []struct {
				ProductID string `json:"product_id"`
				Quantity  int    `json:"quantity"`
			} `json:"product_cart"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return true, err
	}
	d := envelope.Data
	if d.Status != "succeeded" || len(d.Cart) != 1 || d.Cart[0].Quantity != 1 || d.Tax < 0 || d.Tax >= d.Total {
		return true, fmt.Errorf("invalid credit payment cart or amount")
	}
	org, err := e.OrganizationID()
	if err != nil {
		return true, err
	}
	id := e.CheckoutIntentID()
	if id == nil {
		return true, fmt.Errorf("credit checkout intent is missing")
	}
	return true, m.vibeCredits.Store.ApplyCreditPayment(ctx, *id, org, d.PaymentID, d.CheckoutID, d.Cart[0].ProductID, d.Currency, d.Total-d.Tax, body)
}

// Included credits are keyed by a verified subscription billing period. Granting
// after subscription persistence is recoverable: webhook retries re-enter this
// function and the grant source deduplicates across delivery IDs.
func (m *BillingManager) applyVibeAllowance(ctx context.Context, e dodoWebhookEnvelope) error {
	if m.vibeCredits == nil || (e.Type != "subscription.active" && e.Type != "subscription.renewed") {
		return nil
	}
	product := e.DataString("product_id")
	amount := m.vibeCredits.Allowances[product]
	if amount == 0 {
		return nil
	}
	if amount < 0 || amount > 1000*vibe.NanoUSD {
		return fmt.Errorf("invalid plan credit allowance")
	}
	sub := e.DataString("subscription_id", "id")
	end := e.DataString("next_billing_date")
	org, err := e.OrganizationID()
	if err != nil {
		org, err = m.repo.FindOrganizationByDodoSubscriptionOrCustomer(ctx, sub, e.DataString("customer_id"))
	}
	if err != nil {
		return err
	}
	if sub == "" || end == "" {
		return fmt.Errorf("allowance requires subscription and billing period")
	}
	return m.vibeCredits.Store.Grant(ctx, "org:"+org.String(), "dodo-allowance:"+url.QueryEscape(sub)+":"+url.QueryEscape(end), amount)
}

func (m *BillingManager) WithVibeCredits(store *vibe.Store, frontend string) error {
	c, err := LoadVibeCredits(store, frontend)
	if err == nil {
		m.vibeCredits = c
	}
	return err
}
