package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Sender delivers transactional emails.
type Sender interface {
	SendInvite(ctx context.Context, input InviteEmail) error
}

// InviteEmail contains everything needed to send a membership invitation.
type InviteEmail struct {
	To           string
	ResourceName string
	ResourceKind string
	InviterEmail string
	Role         string
	AcceptURL    string
}

// ResendSender sends emails via the Resend API.
type ResendSender struct {
	apiKey    string
	fromEmail string
	client    *http.Client
}

func NewResendSender(apiKey, fromEmail string) *ResendSender {
	return &ResendSender{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *ResendSender) SendInvite(ctx context.Context, input InviteEmail) error {
	subject := fmt.Sprintf("You've been invited to %s on AgentClash", input.ResourceName)
	html := buildInviteHTML(input)

	body, err := json.Marshal(map[string]any{
		"from":    s.fromEmail,
		"to":      []string{input.To},
		"subject": subject,
		"html":    html,
	})
	if err != nil {
		return fmt.Errorf("marshal resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("resend returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func buildInviteHTML(input InviteEmail) string {
	resourceKind := strings.TrimSpace(input.ResourceKind)
	if resourceKind == "" {
		resourceKind = "workspace"
	}
	resourceKind = html.EscapeString(resourceKind)
	resourceName := html.EscapeString(input.ResourceName)
	inviterEmail := html.EscapeString(input.InviterEmail)
	role := html.EscapeString(input.Role)
	acceptURL := html.EscapeString(input.AcceptURL)

	return fmt.Sprintf(`<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:480px;margin:0 auto;padding:32px 0">
  <h2 style="font-size:20px;font-weight:600;margin:0 0 16px">You've been invited to %s</h2>
  <p style="color:#555;font-size:14px;line-height:1.6;margin:0 0 8px">
    <strong>%s</strong> invited you to this %s as <strong>%s</strong> on AgentClash.
  </p>
  <p style="color:#555;font-size:14px;line-height:1.6;margin:0 0 24px">
    Click the button below to accept the invitation. This invite expires in 7 days.
  </p>
  <a href="%s" style="display:inline-block;background:#000;color:#fff;font-size:14px;font-weight:500;padding:10px 24px;border-radius:6px;text-decoration:none">
    Accept Invitation
  </a>
  <p style="color:#777;font-size:12px;line-height:1.6;margin:24px 0 0">
    If the button does not work, copy and paste this link into your browser:
  </p>
  <p style="font-size:12px;line-height:1.6;margin:6px 0 0;word-break:break-all">
    <a href="%s" style="color:#000">%s</a>
  </p>
  <p style="color:#999;font-size:12px;margin:24px 0 0">
    If you didn't expect this invitation, you can ignore this email.
  </p>
</div>`, resourceName, inviterEmail, resourceKind, role, acceptURL, acceptURL, acceptURL)
}

// NoopSender logs invite emails without sending them.
type NoopSender struct{}

func (NoopSender) SendInvite(_ context.Context, input InviteEmail) error {
	slog.Default().Info("invite email (noop)", "to", input.To, "resource_kind", input.ResourceKind, "resource", input.ResourceName, "role", input.Role, "accept_url", input.AcceptURL)
	return nil
}
