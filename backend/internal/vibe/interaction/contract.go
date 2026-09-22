// Package interaction freezes the next Vibe chat wire contracts. It deliberately
// does not persist state, resolve consent, or execute actions; those belong to
// the session's versioned application workflow.
package interaction

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
)

const Version = 1
const MaxBytes = 65536

//go:embed v1.schema.json
var schemaBytes []byte

// Schema returns a copy so callers cannot mutate the process-wide contract.
func Schema() []byte { return bytes.Clone(schemaBytes) }

type Envelope struct {
	Version int             `json:"version"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

var resolved = sync.OnceValues(func() (*jsonschema.Resolved, error) {
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil, err
	}
	return schema.Resolve(nil)
})

var ErrInvalid = errors.New("invalid Vibe interaction contract")

// Decode rejects unknown fields/versions, duplicate keys and ambiguous unions.
// Errors never echo customer content. Semantic/source authorization still needs
// the saved session, the actual source bytes and the expected revision.
func Decode(b []byte) (Envelope, error) {
	var e Envelope
	if len(b) > MaxBytes {
		return e, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if err := uniqueJSON(d, 0); err != nil {
		return e, ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return e, ErrInvalid
	}
	var value any
	if json.Unmarshal(b, &value) != nil {
		return e, ErrInvalid
	}
	schema, err := resolved()
	if err != nil {
		return e, errors.New("Vibe interaction schema unavailable")
	}
	if schema.Validate(value) != nil {
		return e, ErrInvalid
	}
	if json.Unmarshal(b, &e) != nil {
		return Envelope{}, ErrInvalid
	}
	if err := validateRelations(e); err != nil {
		return Envelope{}, err
	}
	return e, nil
}

func uniqueJSON(d *json.Decoder, depth int) error {
	if depth > 32 {
		return ErrInvalid
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	if t == json.Delim('{') {
		seen := map[string]bool{}
		for d.More() {
			t, err := d.Token()
			if err != nil {
				return err
			}
			k, ok := t.(string)
			if !ok || seen[k] {
				return ErrInvalid
			}
			seen[k] = true
			if err := uniqueJSON(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	if t == json.Delim('[') {
		for d.More() {
			if err := uniqueJSON(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	return nil
}

func validateRelations(e Envelope) error {
	var p map[string]any
	_ = json.Unmarshal(e.Payload, &p) // Schema already establishes all types.
	switch e.Kind {
	case "brief":
		ids := map[string]bool{}
		for _, raw := range p["facts"].([]any) {
			f := raw.(map[string]any)
			id := f["id"].(string)
			if ids[id] {
				return ErrInvalid
			}
			ids[id] = true
			status := f["status"].(string)
			if status == "unknown" {
				if f["text"] != nil || f["adoption_message_id"] != nil || len(f["sources"].([]any)) != 0 {
					return ErrInvalid
				}
			} else if f["text"] == nil {
				return ErrInvalid
			}
			if (status == "stated" || status == "accepted") && len(f["sources"].([]any)) == 0 {
				return ErrInvalid
			}
			if (status == "accepted") != (f["adoption_message_id"] != nil) && status != "superseded" {
				return ErrInvalid
			}
			if f["supersedes_id"] == id {
				return ErrInvalid
			}
		}
	case "question":
		ids := map[string]bool{}
		for _, raw := range p["options"].([]any) {
			id := raw.(map[string]any)["id"].(string)
			if ids[id] {
				return ErrInvalid
			}
			ids[id] = true
		}
		if len(ids) > 0 && int(p["max_selections"].(float64)) > len(ids) {
			return ErrInvalid
		}
		if (p["proposal_id"] == nil) != (p["proposal_revision"] == nil) {
			return ErrInvalid
		}
		if p["purpose"] == "adopt_proposal" && p["proposal_id"] == nil {
			return ErrInvalid
		}
	case "card":
		if p["kind"] == "progress" && p["completed"].(float64) > p["total"].(float64) {
			return ErrInvalid
		}
		if p["kind"] == "evidence" && (p["finding"] == "quote") != (p["quote"] != nil) {
			return ErrInvalid
		}
	case "action":
		n := len(p["option_ids"].([]any))
		if p["kind"] == "answer_question" {
			if (n > 0) == (p["text"] != nil) {
				return ErrInvalid
			}
		} else if n > 0 || p["text"] != nil {
			return ErrInvalid
		}
	}
	return nil
}
