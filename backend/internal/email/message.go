package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// MessageSender is intentionally separate from invite delivery. NoopSender
// cannot satisfy this interface or claim that an enquiry was sent.
type MessageSender interface {
	SendMessage(context.Context, Message) (string, error)
}
type Message struct{ To, ReplyTo, Subject, Text, IdempotencyKey string }

func (s *ResendSender) SendMessage(ctx context.Context, m Message) (string, error) {
	body, err := json.Marshal(map[string]any{"from": s.fromEmail, "to": []string{m.To}, "reply_to": m.ReplyTo, "subject": m.Subject, "text": m.Text})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", m.IdempotencyKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("email provider status %d", resp.StatusCode)
	}
	var receipt struct {
		ID string `json:"id"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&receipt)
	if err != nil || receipt.ID == "" {
		return "", fmt.Errorf("email provider acknowledgement unavailable")
	}
	return receipt.ID, nil
}
