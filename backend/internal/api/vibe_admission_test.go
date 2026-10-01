package api

import (
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/backend/internal/mutation"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"net/http/httptest"
	"testing"
)

func TestVibeAdmissionMetadataRequiresProof(t *testing.T) {
	for _, tc := range []struct {
		err      error
		rejected bool
	}{
		{mutation.Reject(&vibe.Fault{Code: "invalid_input", Message: "Invalid attachment"}), true},
		{mutation.Reject(&vibe.Fault{Code: "input_unavailable", Message: "Deleted attachment"}), true},
		{&vibe.Fault{Code: "forbidden", Message: "Permission changed"}, false},
		{errors.New("commit outcome unknown"), false},
	} {
		w := httptest.NewRecorder()
		vibeError(w, tc.err)
		var body struct {
			Error struct {
				Admission string `json:"admission"`
			}
		}
		if e := json.Unmarshal(w.Body.Bytes(), &body); e != nil {
			t.Fatal(e)
		}
		if (body.Error.Admission == "rejected") != tc.rejected {
			t.Fatal("unproven admission status", w.Body.String())
		}
	}
}
