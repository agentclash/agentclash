package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"go.step.sm/crypto/jose"
)

// The Smallstep facade used by Caddy must enforce the vendor's JWK bound,
// including when another dependency still requires go-jose/v3 for its tests.
func TestJWKRejectsOversizedRSA(t *testing.T) {
	modulus := new(big.Int).SetBit(new(big.Int), 4096, 1)
	encoded := base64.RawURLEncoding.EncodeToString(modulus.Bytes())
	var key jose.JSONWebKey
	err := json.Unmarshal([]byte(fmt.Sprintf(`{"kty":"RSA","n":%q,"e":"AQAB"}`, encoded)), &key)
	if err == nil || !strings.Contains(err.Error(), "modulus too large") {
		t.Fatalf("4097-bit JWK must be rejected at parsing, got %v", err)
	}
}

// These are the one-audience Expected literals used by certificates' ACME,
// Azure and OIDC validation. Migration must not accept a wrong or absent aud.
func TestSingleAudienceAndCompactSerialization(t *testing.T) {
	key := make([]byte, 32)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: key}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		audience jose.Audience
		valid    bool
	}{
		{"expected", jose.Audience{"expected"}, true},
		{"expected among multiple", jose.Audience{"expected", "other"}, true},
		{"wrong", jose.Audience{"other"}, false},
		{"missing", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := jose.Signed(signer).Claims(jose.Claims{Audience: tc.audience}).Serialize()
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(raw, ".") != 2 || strings.HasPrefix(raw, "{") {
				t.Fatalf("Serialize must produce a compact JWS, got %q", raw)
			}
			token, err := jose.ParseSigned(raw)
			if err != nil {
				t.Fatal(err)
			}
			var claims jose.Claims
			if err := token.Claims(key, &claims); err != nil {
				t.Fatal(err)
			}
			err = claims.ValidateWithLeeway(jose.Expected{AnyAudience: jose.Audience{"expected"}}, 0)
			if (err == nil) != tc.valid {
				t.Fatalf("audience valid=%v, want %v: %v", err == nil, tc.valid, err)
			}
		})
	}
}
