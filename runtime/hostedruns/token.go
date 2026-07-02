package hostedruns

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var ErrInvalidCallbackToken = errors.New("invalid hosted callback token")

type CallbackClaims struct {
	RunID      uuid.UUID
	RunAgentID uuid.UUID
}

type CallbackTokenSigner struct {
	secret []byte
}

func NewCallbackTokenSigner(secret string) CallbackTokenSigner {
	return CallbackTokenSigner{secret: []byte(secret)}
}

func (s CallbackTokenSigner) Sign(runID uuid.UUID, runAgentID uuid.UUID) (string, error) {
	if len(s.secret) == 0 {
		return "", errors.New("callback secret is required")
	}
	payload := runID.String() + "." + runAgentID.String()
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature, nil
}

func (s CallbackTokenSigner) Verify(token string) (CallbackClaims, error) {
	if len(s.secret) == 0 {
		return CallbackClaims{}, errors.New("callback secret is required")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return CallbackClaims{}, ErrInvalidCallbackToken
	}
	runID, err := uuid.Parse(parts[0])
	if err != nil {
		return CallbackClaims{}, ErrInvalidCallbackToken
	}
	runAgentID, err := uuid.Parse(parts[1])
	if err != nil {
		return CallbackClaims{}, ErrInvalidCallbackToken
	}
	expected, err := s.Sign(runID, runAgentID)
	if err != nil {
		return CallbackClaims{}, err
	}
	if !hmac.Equal([]byte(expected), []byte(token)) {
		return CallbackClaims{}, ErrInvalidCallbackToken
	}
	return CallbackClaims{RunID: runID, RunAgentID: runAgentID}, nil
}

func BearerToken(header string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("%w: bearer token is required", ErrInvalidCallbackToken)
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", fmt.Errorf("%w: bearer token is required", ErrInvalidCallbackToken)
	}
	return token, nil
}
