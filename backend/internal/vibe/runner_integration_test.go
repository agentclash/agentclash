package vibe

import (
	"encoding/json"

	"github.com/google/uuid"
)

type repairCompiler struct{}

func (repairCompiler) Draft(a DraftProposal, _ Limits) (json.RawMessage, error) { return raw(a), nil }

func (repairCompiler) Instructions() string {
	return "Create three cases grounded in the supplied policy."
}
func (repairCompiler) Compile(json.RawMessage, string, uuid.UUID, Limits) (Compiled, error) {
	return Compiled{}, nil
}
