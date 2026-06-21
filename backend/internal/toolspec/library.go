package toolspec

// The tool library: a curated, backend-owned catalog of ready-made tools that a
// non-engineer can add to a workspace in one click. Each entry compiles to a
// normal `primitive` tool definition (validated by ValidateDefinition — see
// library_test.go), so an added tool is a fully editable workspace tool.
//
// Delivery is "hybrid": tools that delegate to a sandbox primitive or a no-key
// public API run for real (Delivery == "live"); SaaS tools that need an API key
// ship as realistic mocks (Delivery == "mock") so they work instantly for
// rehearsing evals, with the real HTTP call bundled in Live to switch on later.

import "encoding/json"

// Library categories, surfaced as sections in the gallery.
const (
	CatFiles = "Files & code"
	CatWeb   = "Web & data"
	CatComms = "Communication"
	CatDev   = "Dev & SaaS"
	CatCRM   = "CRM & enterprise"
	CatLegal = "Legal & documents"
	CatAI    = "AI & utilities"
)

var libraryCategories = []string{CatFiles, CatWeb, CatComms, CatDev, CatCRM, CatLegal, CatAI}

// LibraryCategories returns the catalog categories in display order.
func LibraryCategories() []string { return append([]string(nil), libraryCategories...) }

// Delivery values describe whether the default Definition runs for real.
const (
	DeliveryLive = "live" // Definition executes against a primitive / no-key API
	DeliveryMock = "mock" // Definition returns a canned response; Live may hold the real call
)

// LibraryEntry is one ready-made tool in the catalog.
type LibraryEntry struct {
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	Tags        []string        `json:"tags"`
	ToolKind    string          `json:"tool_kind"`
	Delivery    string          `json:"delivery"`
	// RequiresSecret is the workspace secret the live variant needs ("" if none).
	RequiresSecret string `json:"requires_secret,omitempty"`
	// Definition is the canonical, instantly-usable tool definition.
	Definition json.RawMessage `json:"definition"`
	// Live is the real http_request definition for mock SaaS tools ("" if none).
	Live json.RawMessage `json:"live,omitempty"`
}

// HasLive reports whether a real (live) variant is bundled.
func (e LibraryEntry) HasLive() bool { return len(e.Live) > 0 }

// --- small builders: every entry is valid by construction --------------------

type fields = map[string]string // param name -> JSON Schema type

func object(required []string, f fields) json.RawMessage {
	if required == nil {
		required = []string{}
	}
	props := map[string]any{}
	for name, typ := range f {
		props[name] = map[string]string{"type": typ}
	}
	return mustJSON(map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	})
}

func delegateDef(params json.RawMessage, primitive string, args map[string]any) json.RawMessage {
	return mustJSON(map[string]any{
		"tool_type":   ToolTypePrimitive,
		"description":  "",
		"parameters":  params,
		"implementation": map[string]any{"mode": ModeDelegate, "primitive": primitive, "args": args},
	})
}

func mockDef(params json.RawMessage, response map[string]any) json.RawMessage {
	return mustJSON(map[string]any{
		"tool_type":   ToolTypePrimitive,
		"description":  "",
		"parameters":  params,
		"implementation": map[string]any{"mode": ModeMock, "mock": map[string]any{"strategy": "static", "response": response}},
	})
}

func httpGet(params json.RawMessage, url string) json.RawMessage {
	return delegateDef(params, PrimitiveHTTPRequest, map[string]any{"method": "GET", "url": url})
}

func httpSend(params json.RawMessage, method, url string, headers map[string]string, body string) json.RawMessage {
	args := map[string]any{"method": method, "url": url}
	if len(headers) > 0 {
		args["headers"] = headers
	}
	if body != "" {
		args["body"] = body
	}
	return delegateDef(params, PrimitiveHTTPRequest, args)
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic("toolspec: building library entry: " + err.Error())
	}
	return b
}

func injectDescription(def json.RawMessage, desc string) json.RawMessage {
	if len(def) == 0 {
		return def
	}
	var m map[string]any
	if err := json.Unmarshal(def, &m); err != nil {
		return def
	}
	m["description"] = desc
	return mustJSON(m)
}

// --- public accessors --------------------------------------------------------

var libraryEntries = buildLibrary()

var libraryIndex = func() map[string]LibraryEntry {
	m := make(map[string]LibraryEntry, len(libraryEntries))
	for _, e := range libraryEntries {
		m[e.Slug] = e
	}
	return m
}()

// Library returns the full catalog (descriptions injected into definitions).
func Library() []LibraryEntry { return libraryEntries }

// LibraryBySlug looks up one catalog entry by its slug.
func LibraryBySlug(slug string) (LibraryEntry, bool) {
	e, ok := libraryIndex[slug]
	return e, ok
}

func buildLibrary() []LibraryEntry {
	entries := rawLibrary()
	for i := range entries {
		entries[i].Definition = injectDescription(entries[i].Definition, entries[i].Description)
		if len(entries[i].Live) > 0 {
			entries[i].Live = injectDescription(entries[i].Live, entries[i].Description)
		}
	}
	return entries
}

// rawLibrary is the authored catalog (descriptions injected by buildLibrary).
func rawLibrary() []LibraryEntry {
	live := func(slug, name, cat, desc, secret string, tags []string, mockResp map[string]any, liveDef json.RawMessage) LibraryEntry {
		// Pull the parameters out of liveDef so the mock shares the same input shape.
		return LibraryEntry{
			Slug: slug, Name: name, Category: cat, Description: desc, Tags: tags,
			ToolKind: ToolTypePrimitive, Delivery: DeliveryMock, RequiresSecret: secret,
			Definition: mockDef(paramsOf(liveDef), mockResp), Live: liveDef,
		}
	}

	return []LibraryEntry{
		// --- Files & code (real, delegate to sandbox primitives) -------------
		{Slug: "read-file", Name: "Read a file", Category: CatFiles, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Read the contents of a file from the agent's workspace.", Tags: []string{"files", "read"},
			Definition: delegateDef(object([]string{"path"}, fields{"path": "string"}), PrimitiveReadFile, map[string]any{"path": "${path}"})},
		{Slug: "write-file", Name: "Write a file", Category: CatFiles, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Create or overwrite a file in the agent's workspace.", Tags: []string{"files", "write"},
			Definition: delegateDef(object([]string{"path", "content"}, fields{"path": "string", "content": "string"}), PrimitiveWriteFile, map[string]any{"path": "${path}", "content": "${content}"})},
		{Slug: "list-files", Name: "List files", Category: CatFiles, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "List files in the workspace, optionally under a path prefix.", Tags: []string{"files"},
			Definition: delegateDef(object(nil, fields{"prefix": "string"}), PrimitiveListFiles, map[string]any{"prefix": "${prefix}"})},
		{Slug: "find-files", Name: "Find files by name", Category: CatFiles, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Find files in the workspace by name or glob pattern.", Tags: []string{"files", "search"},
			Definition: delegateDef(object([]string{"pattern"}, fields{"pattern": "string"}), PrimitiveSearchFiles, map[string]any{"pattern": "${pattern}"})},
		{Slug: "search-code", Name: "Search file contents", Category: CatFiles, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Search the text inside workspace files with a regular expression.", Tags: []string{"files", "search", "code"},
			Definition: delegateDef(object([]string{"pattern"}, fields{"pattern": "string"}), PrimitiveSearchText, map[string]any{"pattern": "${pattern}"})},
		{Slug: "run-tests", Name: "Run the test suite", Category: CatFiles, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Run the project's tests (auto-detected, or pass a command).", Tags: []string{"code", "tests", "ci"},
			Definition: delegateDef(object(nil, fields{"command": "string"}), PrimitiveRunTests, map[string]any{"command": "${command}"})},
		{Slug: "build-project", Name: "Build the project", Category: CatFiles, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Build the project (auto-detected, or pass a command).", Tags: []string{"code", "build", "ci"},
			Definition: delegateDef(object(nil, fields{"command": "string"}), PrimitiveBuild, map[string]any{"command": "${command}"})},
		{Slug: "run-command", Name: "Run a shell command", Category: CatFiles, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Run a shell command in the agent's workspace.", Tags: []string{"shell", "exec"},
			Definition: delegateDef(object([]string{"command"}, fields{"command": "string"}), PrimitiveExec, map[string]any{"command": []any{"bash", "-lc", "${command}"}})},

		// --- Web & data (real: no-key public APIs + data primitives) ---------
		{Slug: "fetch-url", Name: "Fetch a web page", Category: CatWeb, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Fetch the raw contents of a URL over HTTP.", Tags: []string{"http", "web", "fetch"},
			Definition: httpGet(object([]string{"url"}, fields{"url": "string"}), "${url}")},
		{Slug: "web-search", Name: "Search the web", Category: CatWeb, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Search the web and get instant-answer results (DuckDuckGo, no key needed).", Tags: []string{"search", "web"},
			Definition: httpGet(object([]string{"query"}, fields{"query": "string"}), "https://api.duckduckgo.com/?q=${query}&format=json&no_html=1&skip_disambig=1")},
		{Slug: "wikipedia-lookup", Name: "Look up Wikipedia", Category: CatWeb, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Get the summary of a Wikipedia article by title.", Tags: []string{"search", "reference"},
			Definition: httpGet(object([]string{"title"}, fields{"title": "string"}), "https://en.wikipedia.org/api/rest_v1/page/summary/${title}")},
		{Slug: "get-weather", Name: "Get the weather", Category: CatWeb, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Get the current weather for a latitude/longitude (Open-Meteo, no key needed).", Tags: []string{"weather", "api"},
			Definition: httpGet(object([]string{"latitude", "longitude"}, fields{"latitude": "string", "longitude": "string"}), "https://api.open-meteo.com/v1/forecast?latitude=${latitude}&longitude=${longitude}&current=temperature_2m,wind_speed_10m")},
		{Slug: "geocode-place", Name: "Find coordinates for a place", Category: CatWeb, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Look up the latitude/longitude of a place by name.", Tags: []string{"maps", "geocode"},
			Definition: httpGet(object([]string{"name"}, fields{"name": "string"}), "https://geocoding-api.open-meteo.com/v1/search?name=${name}&count=1")},
		{Slug: "convert-currency", Name: "Convert currency", Category: CatWeb, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Get exchange rates for a base currency (open.er-api, no key needed).", Tags: []string{"finance", "currency"},
			Definition: httpGet(object([]string{"base"}, fields{"base": "string"}), "https://open.er-api.com/v6/latest/${base}")},
		{Slug: "country-info", Name: "Look up country info", Category: CatWeb, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Get facts about a country by name (REST Countries, no key needed).", Tags: []string{"reference", "geo"},
			Definition: httpGet(object([]string{"name"}, fields{"name": "string"}), "https://restcountries.com/v3.1/name/${name}")},
		{Slug: "query-sql", Name: "Query a database (SQL)", Category: CatWeb, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Run a read-only SQL query against a SQLite database in the workspace.", Tags: []string{"data", "sql", "database"},
			Definition: delegateDef(object([]string{"query"}, fields{"query": "string", "database_path": "string"}), PrimitiveQuerySQL, map[string]any{"engine": "sqlite", "query": "${query}", "database_path": "${database_path}"})},
		{Slug: "query-json", Name: "Query JSON data", Category: CatWeb, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Query or transform a JSON document with a jq expression.", Tags: []string{"data", "json", "jq"},
			Definition: delegateDef(object([]string{"query", "json"}, fields{"query": "string", "json": "string"}), PrimitiveQueryJSON, map[string]any{"query": "${query}", "json": "${json}"})},

		// --- Communication (mock by default, live HTTP bundled) --------------
		live("slack-message", "Send a Slack message", CatComms,
			"Post a message to a Slack channel.", "SLACK_BOT_TOKEN", []string{"slack", "chat", "notify"},
			map[string]any{"ok": true, "channel": "C0123456789", "ts": "1718900000.000100"},
			httpSend(object([]string{"channel", "text"}, fields{"channel": "string", "text": "string"}), "POST", "https://slack.com/api/chat.postMessage",
				map[string]string{"Authorization": "Bearer ${secrets.SLACK_BOT_TOKEN}", "Content-Type": "application/json"},
				`{"channel":"${channel}","text":"${text}"}`)),
		live("send-email", "Send an email", CatComms,
			"Send a transactional email (Resend).", "RESEND_API_KEY", []string{"email", "notify"},
			map[string]any{"id": "re_123456789", "status": "queued"},
			httpSend(object([]string{"to", "subject", "html"}, fields{"to": "string", "subject": "string", "html": "string"}), "POST", "https://api.resend.com/emails",
				map[string]string{"Authorization": "Bearer ${secrets.RESEND_API_KEY}", "Content-Type": "application/json"},
				`{"from":"agent@example.com","to":"${to}","subject":"${subject}","html":"${html}"}`)),
		live("send-sms", "Send an SMS", CatComms,
			"Send a text message (Twilio).", "TWILIO_AUTH", []string{"sms", "notify"},
			map[string]any{"sid": "SM0123456789", "status": "queued"},
			httpSend(object([]string{"to", "body"}, fields{"to": "string", "body": "string"}), "POST", "https://api.twilio.com/2010-04-01/Messages.json",
				map[string]string{"Authorization": "Basic ${secrets.TWILIO_AUTH}", "Content-Type": "application/x-www-form-urlencoded"},
				`To=${to}&Body=${body}`)),
		live("discord-message", "Post to Discord", CatComms,
			"Post a message to a Discord channel via webhook.", "DISCORD_WEBHOOK_URL", []string{"discord", "chat", "notify"},
			map[string]any{"id": "112233445566", "type": 0},
			httpSend(object([]string{"content"}, fields{"content": "string"}), "POST", "${secrets.DISCORD_WEBHOOK_URL}",
				map[string]string{"Content-Type": "application/json"}, `{"content":"${content}"}`)),
		live("telegram-message", "Send a Telegram message", CatComms,
			"Send a message from a Telegram bot.", "TELEGRAM_BOT_TOKEN", []string{"telegram", "chat", "notify"},
			map[string]any{"ok": true, "result": map[string]any{"message_id": 42}},
			httpSend(object([]string{"chat_id", "text"}, fields{"chat_id": "string", "text": "string"}), "POST", "https://api.telegram.org/bot${secrets.TELEGRAM_BOT_TOKEN}/sendMessage",
				map[string]string{"Content-Type": "application/json"}, `{"chat_id":"${chat_id}","text":"${text}"}`)),
		{Slug: "create-calendar-event", Name: "Create a calendar event", Category: CatComms, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Create an event on a calendar.", Tags: []string{"calendar", "schedule"}, RequiresSecret: "GOOGLE_OAUTH_TOKEN",
			Definition: mockDef(object([]string{"summary", "start", "end"}, fields{"summary": "string", "start": "string", "end": "string"}),
				map[string]any{"id": "evt_123456789", "status": "confirmed", "htmlLink": "https://calendar.google.com/event?eid=evt_123456789"})},
		{Slug: "push-notification", Name: "Send a push notification", Category: CatComms, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Send a push notification to a device.", Tags: []string{"push", "notify"},
			Definition: mockDef(object([]string{"title", "message"}, fields{"title": "string", "message": "string"}),
				map[string]any{"delivered": true, "id": "ntf_123456789"})},

		// --- Dev & SaaS (mock by default, live HTTP bundled) -----------------
		live("github-create-issue", "Create a GitHub issue", CatDev,
			"Open an issue in a GitHub repository.", "GITHUB_TOKEN", []string{"github", "issues", "dev"},
			map[string]any{"number": 101, "state": "open", "html_url": "https://github.com/acme/repo/issues/101"},
			httpSend(object([]string{"owner", "repo", "title", "body"}, fields{"owner": "string", "repo": "string", "title": "string", "body": "string"}), "POST", "https://api.github.com/repos/${owner}/${repo}/issues",
				map[string]string{"Authorization": "Bearer ${secrets.GITHUB_TOKEN}", "Accept": "application/vnd.github+json"},
				`{"title":"${title}","body":"${body}"}`)),
		live("github-search-code", "Search code on GitHub", CatDev,
			"Search code across GitHub repositories.", "GITHUB_TOKEN", []string{"github", "search", "dev"},
			map[string]any{"total_count": 2, "items": []any{map[string]any{"name": "main.go", "repository": map[string]any{"full_name": "acme/repo"}}}},
			httpSend(object([]string{"query"}, fields{"query": "string"}), "GET", "https://api.github.com/search/code?q=${query}",
				map[string]string{"Authorization": "Bearer ${secrets.GITHUB_TOKEN}", "Accept": "application/vnd.github+json"}, "")),
		live("github-open-pr", "Open a GitHub pull request", CatDev,
			"Open a pull request in a GitHub repository.", "GITHUB_TOKEN", []string{"github", "pr", "dev"},
			map[string]any{"number": 55, "state": "open", "html_url": "https://github.com/acme/repo/pull/55"},
			httpSend(object([]string{"owner", "repo", "title", "head", "base"}, fields{"owner": "string", "repo": "string", "title": "string", "head": "string", "base": "string"}), "POST", "https://api.github.com/repos/${owner}/${repo}/pulls",
				map[string]string{"Authorization": "Bearer ${secrets.GITHUB_TOKEN}", "Accept": "application/vnd.github+json"},
				`{"title":"${title}","head":"${head}","base":"${base}"}`)),
		{Slug: "gitlab-create-issue", Name: "Create a GitLab issue", Category: CatDev, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Open an issue in a GitLab project.", Tags: []string{"gitlab", "issues", "dev"}, RequiresSecret: "GITLAB_TOKEN",
			Definition: mockDef(object([]string{"project_id", "title"}, fields{"project_id": "string", "title": "string", "description": "string"}),
				map[string]any{"iid": 12, "state": "opened", "web_url": "https://gitlab.com/acme/repo/-/issues/12"})},
		live("jira-create-issue", "Create a Jira issue", CatDev,
			"Create an issue in Jira.", "JIRA_TOKEN", []string{"jira", "issues", "dev"},
			map[string]any{"id": "10042", "key": "ENG-42", "self": "https://acme.atlassian.net/rest/api/3/issue/10042"},
			httpSend(object([]string{"project", "summary", "issue_type"}, fields{"project": "string", "summary": "string", "issue_type": "string"}), "POST", "https://acme.atlassian.net/rest/api/3/issue",
				map[string]string{"Authorization": "Basic ${secrets.JIRA_TOKEN}", "Content-Type": "application/json"},
				`{"fields":{"project":{"key":"${project}"},"summary":"${summary}","issuetype":{"name":"${issue_type}"}}}`)),
		{Slug: "jira-search", Name: "Search Jira issues (JQL)", Category: CatDev, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Search Jira issues with a JQL query.", Tags: []string{"jira", "search", "dev"}, RequiresSecret: "JIRA_TOKEN",
			Definition: mockDef(object([]string{"jql"}, fields{"jql": "string"}),
				map[string]any{"total": 1, "issues": []any{map[string]any{"key": "ENG-42", "fields": map[string]any{"summary": "Fix login bug", "status": map[string]any{"name": "In Progress"}}}}})},
		{Slug: "linear-create-issue", Name: "Create a Linear issue", Category: CatDev, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Create an issue in Linear.", Tags: []string{"linear", "issues", "dev"}, RequiresSecret: "LINEAR_API_KEY",
			Definition: mockDef(object([]string{"team_id", "title"}, fields{"team_id": "string", "title": "string", "description": "string"}),
				map[string]any{"success": true, "issue": map[string]any{"id": "lin_123", "identifier": "ENG-7", "url": "https://linear.app/acme/issue/ENG-7"}})},
		{Slug: "confluence-create-page", Name: "Create a Confluence page", Category: CatDev, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Create a page in Confluence.", Tags: []string{"confluence", "docs", "dev"}, RequiresSecret: "CONFLUENCE_TOKEN",
			Definition: mockDef(object([]string{"space", "title", "body"}, fields{"space": "string", "title": "string", "body": "string"}),
				map[string]any{"id": "987654", "status": "current", "_links": map[string]any{"webui": "/spaces/ENG/pages/987654"}})},
		live("notion-create-page", "Create a Notion page", CatDev,
			"Create a page in a Notion database.", "NOTION_TOKEN", []string{"notion", "docs"},
			map[string]any{"object": "page", "id": "page_123456789", "url": "https://notion.so/page_123456789"},
			httpSend(object([]string{"parent_id", "title"}, fields{"parent_id": "string", "title": "string"}), "POST", "https://api.notion.com/v1/pages",
				map[string]string{"Authorization": "Bearer ${secrets.NOTION_TOKEN}", "Notion-Version": "2022-06-28", "Content-Type": "application/json"},
				`{"parent":{"database_id":"${parent_id}"},"properties":{"title":{"title":[{"text":{"content":"${title}"}}]}}}`)),
		{Slug: "google-sheets-append", Name: "Append a row to Google Sheets", Category: CatDev, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Append a row of values to a Google Sheet.", Tags: []string{"sheets", "data"}, RequiresSecret: "GOOGLE_OAUTH_TOKEN",
			Definition: mockDef(object([]string{"spreadsheet_id", "values"}, fields{"spreadsheet_id": "string", "values": "string"}),
				map[string]any{"updates": map[string]any{"updatedRows": 1, "updatedRange": "Sheet1!A2:C2"}})},
		live("stripe-create-charge", "Create a Stripe charge", CatDev,
			"Charge a customer with Stripe.", "STRIPE_API_KEY", []string{"stripe", "payments", "billing"},
			map[string]any{"id": "ch_123456789", "status": "succeeded", "amount": 2000, "currency": "usd"},
			httpSend(object([]string{"amount", "currency", "source"}, fields{"amount": "string", "currency": "string", "source": "string"}), "POST", "https://api.stripe.com/v1/charges",
				map[string]string{"Authorization": "Bearer ${secrets.STRIPE_API_KEY}", "Content-Type": "application/x-www-form-urlencoded"},
				`amount=${amount}&currency=${currency}&source=${source}`)),
		{Slug: "stripe-refund", Name: "Refund a Stripe payment", Category: CatDev, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Issue a refund for a Stripe charge.", Tags: []string{"stripe", "payments", "billing"}, RequiresSecret: "STRIPE_API_KEY",
			Definition: mockDef(object([]string{"charge_id"}, fields{"charge_id": "string", "amount": "string"}),
				map[string]any{"id": "re_123456789", "status": "succeeded", "charge": "ch_123456789"})},
		{Slug: "airtable-create-record", Name: "Create an Airtable record", Category: CatDev, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Add a record to an Airtable base.", Tags: []string{"airtable", "data"}, RequiresSecret: "AIRTABLE_TOKEN",
			Definition: mockDef(object([]string{"base_id", "table", "fields_json"}, fields{"base_id": "string", "table": "string", "fields_json": "string"}),
				map[string]any{"id": "rec123456789", "createdTime": "2026-01-01T00:00:00.000Z"})},
		{Slug: "pagerduty-alert", Name: "Trigger a PagerDuty alert", Category: CatDev, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Trigger an incident alert in PagerDuty.", Tags: []string{"pagerduty", "oncall", "alerting"}, RequiresSecret: "PAGERDUTY_ROUTING_KEY",
			Definition: mockDef(object([]string{"summary", "severity"}, fields{"summary": "string", "severity": "string"}),
				map[string]any{"status": "success", "dedup_key": "pd_123456789"})},

		// --- CRM & enterprise (mock) -----------------------------------------
		{Slug: "salesforce-query", Name: "Query Salesforce (SOQL)", Category: CatCRM, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Run a SOQL query against Salesforce.", Tags: []string{"salesforce", "crm", "query"}, RequiresSecret: "SALESFORCE_TOKEN",
			Definition: mockDef(object([]string{"soql"}, fields{"soql": "string"}),
				map[string]any{"totalSize": 1, "records": []any{map[string]any{"Id": "001xx0000ABCDEF", "Name": "Acme Corp"}}})},
		{Slug: "salesforce-create-lead", Name: "Create a Salesforce lead", Category: CatCRM, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Create a new lead in Salesforce.", Tags: []string{"salesforce", "crm", "lead"}, RequiresSecret: "SALESFORCE_TOKEN",
			Definition: mockDef(object([]string{"last_name", "company"}, fields{"last_name": "string", "company": "string", "email": "string"}),
				map[string]any{"id": "00Qxx0000ABCDEF", "success": true})},
		{Slug: "hubspot-create-contact", Name: "Create a HubSpot contact", Category: CatCRM, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Create a contact in HubSpot.", Tags: []string{"hubspot", "crm", "contact"}, RequiresSecret: "HUBSPOT_TOKEN",
			Definition: mockDef(object([]string{"email"}, fields{"email": "string", "firstname": "string", "lastname": "string"}),
				map[string]any{"id": "151515", "properties": map[string]any{"email": "jane@acme.com"}})},
		{Slug: "zendesk-create-ticket", Name: "Create a Zendesk ticket", Category: CatCRM, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Open a support ticket in Zendesk.", Tags: []string{"zendesk", "support", "ticket"}, RequiresSecret: "ZENDESK_TOKEN",
			Definition: mockDef(object([]string{"subject", "comment"}, fields{"subject": "string", "comment": "string"}),
				map[string]any{"ticket": map[string]any{"id": 35436, "status": "open", "url": "https://acme.zendesk.com/tickets/35436"}})},
		{Slug: "intercom-message", Name: "Send an Intercom message", Category: CatCRM, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Send a message to a user in Intercom.", Tags: []string{"intercom", "support", "chat"}, RequiresSecret: "INTERCOM_TOKEN",
			Definition: mockDef(object([]string{"user_id", "body"}, fields{"user_id": "string", "body": "string"}),
				map[string]any{"type": "admin_message", "id": "im_123456789"})},
		{Slug: "shopify-get-order", Name: "Look up a Shopify order", Category: CatCRM, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Look up an order by id in Shopify.", Tags: []string{"shopify", "ecommerce", "order"}, RequiresSecret: "SHOPIFY_TOKEN",
			Definition: mockDef(object([]string{"order_id"}, fields{"order_id": "string"}),
				map[string]any{"order": map[string]any{"id": 450789469, "financial_status": "paid", "fulfillment_status": "fulfilled", "total_price": "199.00"}})},
		{Slug: "stripe-get-customer", Name: "Look up a Stripe customer", Category: CatCRM, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Look up a customer by id in Stripe.", Tags: []string{"stripe", "billing", "customer"}, RequiresSecret: "STRIPE_API_KEY",
			Definition: mockDef(object([]string{"customer_id"}, fields{"customer_id": "string"}),
				map[string]any{"id": "cus_123456789", "email": "jane@acme.com", "balance": 0, "delinquent": false})},

		// --- Legal & documents (mock, for rehearsing evals) ------------------
		{Slug: "extract-contract-clauses", Name: "Extract contract clauses", Category: CatLegal, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Pull out key clauses (term, liability, termination) from a contract.", Tags: []string{"legal", "contracts", "extract"},
			Definition: mockDef(object([]string{"text"}, fields{"text": "string"}),
				map[string]any{"clauses": []any{
					map[string]any{"type": "term", "text": "This Agreement begins on the Effective Date and continues for 12 months."},
					map[string]any{"type": "limitation_of_liability", "text": "Liability is capped at fees paid in the prior 12 months."},
				}})},
		{Slug: "case-law-search", Name: "Search case law", Category: CatLegal, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Search for relevant case law by query.", Tags: []string{"legal", "research", "search"},
			Definition: mockDef(object([]string{"query"}, fields{"query": "string"}),
				map[string]any{"results": []any{map[string]any{"name": "Carlill v Carbolic Smoke Ball Co", "citation": "[1893] 1 QB 256", "court": "Court of Appeal"}}})},
		{Slug: "pdf-extract-text", Name: "Extract text from a PDF", Category: CatLegal, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Extract the plain text from a PDF document.", Tags: []string{"documents", "pdf", "extract"},
			Definition: mockDef(object([]string{"url"}, fields{"url": "string"}),
				map[string]any{"pages": 3, "text": "Page 1 ... extracted document text ..."})},
		{Slug: "compare-documents", Name: "Compare two documents (redline)", Category: CatLegal, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Produce a redline diff of two document versions.", Tags: []string{"documents", "diff", "redline"},
			Definition: mockDef(object([]string{"before", "after"}, fields{"before": "string", "after": "string"}),
				map[string]any{"changes": []any{map[string]any{"op": "replace", "before": "30 days", "after": "60 days"}}})},
		{Slug: "esignature-send", Name: "Send for e-signature", Category: CatLegal, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Send a document for electronic signature (DocuSign).", Tags: []string{"documents", "esign", "docusign"}, RequiresSecret: "DOCUSIGN_TOKEN",
			Definition: mockDef(object([]string{"recipient_email", "document_url"}, fields{"recipient_email": "string", "document_url": "string"}),
				map[string]any{"envelopeId": "env_123456789", "status": "sent"})},
		{Slug: "extract-pii", Name: "Find personal data (PII)", Category: CatLegal, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Detect personal data (emails, phone numbers, names) in text.", Tags: []string{"legal", "privacy", "pii"},
			Definition: mockDef(object([]string{"text"}, fields{"text": "string"}),
				map[string]any{"findings": []any{map[string]any{"type": "email", "value": "jane@acme.com"}, map[string]any{"type": "phone", "value": "+1-415-555-0172"}}})},
		{Slug: "summarize-contract", Name: "Summarize a contract", Category: CatLegal, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Produce a plain-language summary of a contract.", Tags: []string{"legal", "contracts", "summary"},
			Definition: mockDef(object([]string{"text"}, fields{"text": "string"}),
				map[string]any{"summary": "A 12-month services agreement with a liability cap and 60-day termination for convenience."})},

		// --- AI & utilities --------------------------------------------------
		{Slug: "summarize-text", Name: "Summarize text", Category: CatAI, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Summarize a block of text into a few sentences.", Tags: []string{"ai", "nlp", "summary"},
			Definition: mockDef(object([]string{"text"}, fields{"text": "string"}),
				map[string]any{"summary": "A concise summary of the provided text."})},
		{Slug: "classify-text", Name: "Classify text", Category: CatAI, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Assign a label to text from a set of categories.", Tags: []string{"ai", "nlp", "classify"},
			Definition: mockDef(object([]string{"text", "labels"}, fields{"text": "string", "labels": "string"}),
				map[string]any{"label": "billing", "confidence": 0.92})},
		{Slug: "extract-entities", Name: "Extract entities from text", Category: CatAI, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Pull named entities (people, orgs, dates) out of text.", Tags: []string{"ai", "nlp", "ner"},
			Definition: mockDef(object([]string{"text"}, fields{"text": "string"}),
				map[string]any{"entities": []any{map[string]any{"type": "ORG", "text": "Acme Corp"}, map[string]any{"type": "DATE", "text": "January 2026"}}})},
		{Slug: "translate-text", Name: "Translate text", Category: CatAI, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Translate text into a target language.", Tags: []string{"ai", "nlp", "translate"},
			Definition: mockDef(object([]string{"text", "target_language"}, fields{"text": "string", "target_language": "string"}),
				map[string]any{"translation": "Bonjour le monde", "detected_source_language": "en"})},
		{Slug: "sentiment-analysis", Name: "Analyze sentiment", Category: CatAI, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Score the sentiment of a piece of text.", Tags: []string{"ai", "nlp", "sentiment"},
			Definition: mockDef(object([]string{"text"}, fields{"text": "string"}),
				map[string]any{"sentiment": "positive", "score": 0.78})},
		{Slug: "knowledge-base-search", Name: "Search a knowledge base", Category: CatAI, Delivery: DeliveryMock, ToolKind: ToolTypePrimitive,
			Description: "Semantic search over a knowledge base (RAG retrieval).", Tags: []string{"ai", "rag", "search"},
			Definition: mockDef(object([]string{"query"}, fields{"query": "string", "top_k": "integer"}),
				map[string]any{"matches": []any{map[string]any{"id": "doc_12", "score": 0.83, "text": "Refunds are processed within 5 business days."}}})},
		{Slug: "current-datetime", Name: "Get the current date and time", Category: CatAI, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Return the current UTC date and time.", Tags: []string{"utility", "time"},
			Definition: delegateDef(object(nil, fields{}), PrimitiveExec, map[string]any{"command": []any{"date", "-u", "+%Y-%m-%dT%H:%M:%SZ"}})},
		{Slug: "calculator", Name: "Evaluate a math expression", Category: CatAI, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Evaluate an arithmetic expression and return the result.", Tags: []string{"utility", "math"},
			Definition: delegateDef(object([]string{"expression"}, fields{"expression": "string"}), PrimitiveExec, map[string]any{"command": []any{"python3", "-c", "import sys; print(eval(sys.argv[1]))", "${expression}"}})},
		{Slug: "generate-uuid", Name: "Generate a UUID", Category: CatAI, Delivery: DeliveryLive, ToolKind: ToolTypePrimitive,
			Description: "Generate a random UUID.", Tags: []string{"utility", "id"},
			Definition: delegateDef(object(nil, fields{}), PrimitiveExec, map[string]any{"command": []any{"uuidgen"}})},
	}
}

// paramsOf extracts the "parameters" object from a definition so a mock and its
// bundled live variant share the same input shape.
func paramsOf(def json.RawMessage) json.RawMessage {
	var m struct {
		Parameters json.RawMessage `json:"parameters"`
	}
	if err := json.Unmarshal(def, &m); err != nil || len(m.Parameters) == 0 {
		return object(nil, fields{})
	}
	return m.Parameters
}
