package vibe

import "testing"

func TestVibeEvidenceRestoresPastedLayoutWithoutChangingMeaning(t *testing.T) {
	for _, tc := range []struct{ name, source, quote, want string }{
		{"screenshot", "\n     > My agent helps with returns. Ask for missing purchase age or\n     > condition. Never claim to process refunds.", "Ask for missing purchase age or condition.", "Ask for missing purchase age or\n     > condition."},
		{"wrapped plain text", "Ask only for missing\n  purchase age or condition.", "Ask only for missing purchase age or condition.", "Ask only for missing\n  purchase age or condition."},
		{"negation preserved", "> Never claim to process refunds.", "Claim to process refunds.", "Claim to process refunds."},
		{"condition preserved", "> Only unopened items within 30 days qualify.", "Opened items qualify.", "Opened items qualify."},
		{"ambiguous", "Ask for\n details. Ask for\n details.", "Ask for details.", "Ask for details."},
		{"comparison", "Age\n> 30 days is ineligible.", "Age 30 days is ineligible.", "Age 30 days is ineligible."},
		{"nested comparison", "> Age > 30 days is ineligible.", "Age 30 days is ineligible.", "Age 30 days is ineligible."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := restoreEvidenceQuote(tc.quote, tc.source); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestVibeEvidenceRestoresMemoryFactsWithoutAdoptingChat(t *testing.T) {
	text := "\n > Ask for missing purchase age or\n > condition."
	p, o := memoryPlan(t, Document{}, text)
	route := reliableRoute{Intent: "prepare_tests", Reply: "I will prepare tests.", Count: 3, Memory: &memoryUpdate{Facts: []memoryFact{{Kind: "rule", Quote: "Ask for missing purchase age or condition."}}}}
	state, err := proposeConversationState(p, route, o)
	if err != nil {
		t.Fatal("wrapped fact was rejected", err)
	}
	found := false
	for _, fact := range state.Brief.Facts {
		if fact.Status == "stated" && fact.Kind == "rule" && fact.Text != nil && *fact.Text == "Ask for missing purchase age or\n > condition." {
			found = true
		}
	}
	if !found {
		t.Fatal("memory lost original source bytes")
	}
	if route.Memory.Facts[0].Quote != "Ask for missing purchase age or condition." {
		t.Fatal("changed caller-owned route")
	}
	route.Intent = "chat"
	if _, err = proposeConversationState(p, route, o); err == nil {
		t.Fatal("chat acquired policy authority")
	}
	route.Intent = "prepare_tests"
	route.Memory.Facts[0].Quote = "Do not ask for missing purchase age or condition."
	if _, err = proposeConversationState(p, route, o); err == nil {
		t.Fatal("changed meaning accepted")
	}
}
