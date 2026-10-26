// Package connectors defines frozen application invocation contracts. It contains
// no workspace lookup, provider credential fallback, or billing implementation.
package connectors

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

const MaxBodyBytes = 1 << 20
const MaxTimeoutSeconds = 300
const Native = "native"
const HTTPJSON = "http_json"
const Hosted = "hosted"

// InputFields maps each top-level outbound JSON field to a JSON pointer in the
// canonical case payload. LiteralJSON supplies explicit static JSON fields.
// There is no string templating or implicit serialization of workspace state.
type Config struct {
	Adapter         string                     `json:"adapter"`
	EndpointURL     string                     `json:"endpoint_url"`
	SecretKey       string                     `json:"secret_key"`
	Auth            string                     `json:"auth"` // bearer or api_key (X-API-Key)
	InputFields     map[string]string          `json:"input_fields,omitempty"`
	LiteralJSON     map[string]json.RawMessage `json:"literal_json,omitempty"`
	ResponsePointer string                     `json:"response_pointer"`
	TimeoutSeconds  int                        `json:"timeout_seconds"`
}
type Binding struct {
	SnapshotID     uuid.UUID `json:"snapshot_id"`
	DeploymentID   uuid.UUID `json:"deployment_id"`
	WorkspaceID    uuid.UUID `json:"workspace_id"`
	Config         Config    `json:"config"`
	TargetOrigin   string    `json:"target_origin"`
	SecretID       uuid.UUID `json:"secret_id"`
	SecretRevision int64     `json:"secret_revision"`
	Approved       bool      `json:"approved"`
}

func (c Config) Validate() error {
	if c.Adapter != HTTPJSON && c.Adapter != Hosted {
		return errors.New("adapter must be http_json or hosted")
	}
	if _, err := provider.NormalizeBaseURL(c.EndpointURL); err != nil {
		return err
	}
	if c.SecretKey == "" || len(c.SecretKey) > 128 || strings.ContainsAny(c.SecretKey, "/: \\@\n\r") {
		return errors.New("a workspace secret key is required")
	}
	if c.Auth != "bearer" && c.Auth != "api_key" {
		return errors.New("auth must be bearer or api_key")
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > MaxTimeoutSeconds {
		return errors.New("timeout_seconds must be between 1 and 300")
	}
	if c.Adapter == HTTPJSON {
		if len(c.InputFields)+len(c.LiteralJSON) == 0 {
			return errors.New("explicit JSON input mapping is required")
		}
		if err := validatePointer(c.ResponsePointer); err != nil {
			return err
		}
		for field, p := range c.InputFields {
			if field == "" {
				return errors.New("input field must not be empty")
			}
			if _, ok := c.LiteralJSON[field]; ok {
				return errors.New("input fields and literal JSON must not overlap")
			}
			if err := validatePointer(p); err != nil {
				return err
			}
		}
		for _, v := range c.LiteralJSON {
			if !json.Valid(v) {
				return errors.New("literal_json values must be JSON")
			}
		}
	} else if len(c.InputFields)+len(c.LiteralJSON) > 0 || c.ResponsePointer != "" {
		return errors.New("hosted uses the signed /agentclash/runs protocol without JSON mappings")
	}
	b, _ := json.Marshal(c)
	if len(b) > MaxBodyBytes {
		return errors.New("configuration too large")
	}
	return nil
}
func Origin(raw string) (string, error) {
	n, err := provider.NormalizeBaseURL(raw)
	if err != nil {
		return "", err
	}
	u, _ := url.Parse(n)
	return u.Scheme + "://" + u.Host, nil
}
func validatePointer(p string) error {
	if p != "" && !strings.HasPrefix(p, "/") {
		return errors.New("JSON pointer must be empty or start with /")
	}
	for i := 0; i < len(p); i++ {
		if p[i] == '~' {
			if i+1 >= len(p) || (p[i+1] != '0' && p[i+1] != '1') {
				return errors.New("invalid JSON pointer escape")
			}
			i++
		}
	}
	return nil
}
func Decode(body []byte) (any, error) {
	if len(body) > MaxBodyBytes {
		return nil, errors.New("JSON body exceeds 1 MiB")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, errors.New("invalid JSON body")
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, errors.New("JSON body must contain exactly one value")
	}
	return v, nil
}
func Pointer(value any, p string) (any, error) {
	if err := validatePointer(p); err != nil {
		return nil, err
	}
	if p == "" {
		return value, nil
	}
	for _, part := range strings.Split(p[1:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch v := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = v[part]
			if !ok {
				return nil, errors.New("JSON pointer field is missing")
			}
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(v) || (len(part) > 1 && part[0] == '0') {
				return nil, errors.New("JSON pointer array index is invalid")
			}
			value = v[i]
		default:
			return nil, errors.New("JSON pointer traverses a scalar")
		}
	}
	return value, nil
}
func (c Config) Request(payload json.RawMessage) ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	source, err := Decode(payload)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for k, v := range c.LiteralJSON {
		out[k] = v
	}
	for k, p := range c.InputFields {
		v, err := Pointer(source, p)
		if err != nil {
			return nil, fmt.Errorf("input mapping %s: %w", k, err)
		}
		out[k] = v
	}
	b, err := json.Marshal(out)
	if len(b) > MaxBodyBytes {
		return nil, errors.New("mapped request exceeds 1 MiB")
	}
	return b, err
}
