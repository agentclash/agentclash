package interaction

// These types implement the frozen v1 wire schema. Source hashes refer to the
// complete original message, not a generated summary or an extracted quote.
type Source struct {
	MessageID string `json:"message_id"`
	Quote     string `json:"quote"`
	SHA256    string `json:"sha256"`
}
type Fact struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	Status            string   `json:"status"`
	Text              *string  `json:"text"`
	Sources           []Source `json:"sources"`
	AdoptionMessageID *string  `json:"adoption_message_id"`
	SupersedesID      *string  `json:"supersedes_id"`
}
type Brief struct {
	ScopeID  string `json:"scope_id"`
	Revision int64  `json:"revision"`
	Facts    []Fact `json:"facts"`
}
type Option struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
type Question struct {
	ID               string   `json:"id"`
	ScopeID          string   `json:"scope_id"`
	Revision         int64    `json:"revision"`
	OriginMessageID  string   `json:"origin_message_id"`
	Purpose          string   `json:"purpose"`
	Status           string   `json:"status"`
	Text             string   `json:"text"`
	Options          []Option `json:"options"`
	MaxSelections    int      `json:"max_selections"`
	ProposalID       *string  `json:"proposal_id"`
	ProposalRevision *int64   `json:"proposal_revision"`
}
