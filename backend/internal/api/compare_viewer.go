package api

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
)

var compareViewerPageTemplate = template.Must(template.New("compare-viewer").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Run Comparison Viewer</title>
    <style>
      :root {
        color-scheme: light;
        --bg: #f3efe8;
        --panel: rgba(255, 252, 247, 0.94);
        --panel-alt: #f8f2e8;
        --line: #d7c8b5;
        --text: #221d17;
        --muted: #65594d;
        --accent: #0f766e;
        --accent-soft: #ccfbf1;
        --good: #166534;
        --good-soft: #dcfce7;
        --warn: #92400e;
        --warn-soft: #fef3c7;
        --bad: #991b1b;
        --bad-soft: #fee2e2;
      }
      * { box-sizing: border-box; }
      body {
        margin: 0;
        font-family: "Iowan Old Style", Georgia, serif;
        color: var(--text);
        background:
          radial-gradient(circle at top left, rgba(15, 118, 110, 0.14), transparent 22rem),
          radial-gradient(circle at top right, rgba(180, 83, 9, 0.14), transparent 20rem),
          linear-gradient(180deg, #fcfaf6, var(--bg));
      }
      .shell {
        max-width: 1120px;
        margin: 0 auto;
        padding: 28px 18px 56px;
      }
      .hero {
        display: grid;
        gap: 12px;
        margin-bottom: 22px;
      }
      .eyebrow {
        font: 12px "Courier New", monospace;
        letter-spacing: 0.14em;
        text-transform: uppercase;
        color: var(--muted);
      }
      h1 {
        margin: 0;
        font-size: clamp(2rem, 4vw, 3.4rem);
        line-height: 0.95;
        letter-spacing: -0.05em;
      }
      .hero-copy {
        max-width: 52rem;
        color: var(--muted);
        line-height: 1.5;
      }
      .panel {
        border: 1px solid var(--line);
        border-radius: 22px;
        background: var(--panel);
        box-shadow: 0 18px 44px rgba(47, 35, 24, 0.08);
      }
      .panel-body {
        padding: 18px;
      }
      .state {
        display: grid;
        gap: 8px;
        margin-bottom: 18px;
        padding: 18px;
        border: 1px solid var(--line);
        border-radius: 18px;
        background: var(--panel-alt);
      }
      .state.comparable { background: linear-gradient(135deg, var(--good-soft), #fbfffd); }
      .state.partial_evidence { background: linear-gradient(135deg, var(--warn-soft), #fffdf7); }
      .state.not_comparable { background: linear-gradient(135deg, var(--bad-soft), #fff8f8); }
      .state h2 {
        margin: 0;
        font-size: 1.35rem;
      }
      .meta-grid, .delta-grid, .reason-grid {
        display: grid;
        gap: 12px;
      }
      .meta-grid {
        grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
        margin-bottom: 18px;
      }
      .card {
        padding: 15px;
        border: 1px solid var(--line);
        border-radius: 16px;
        background: var(--panel-alt);
      }
      .label {
        margin: 0 0 6px;
        font-size: 0.74rem;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        color: var(--muted);
      }
      .value {
        margin: 0;
        font-size: 1rem;
        line-height: 1.45;
        word-break: break-word;
      }
      .delta-grid {
        grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      }
      .delta-card {
        padding: 16px;
        border-radius: 16px;
        border: 1px solid var(--line);
        background: white;
      }
      .delta-card.better { border-color: #86efac; background: #f0fdf4; }
      .delta-card.worse { border-color: #fca5a5; background: #fef2f2; }
      .delta-card.same { border-color: #d6d3d1; }
      .metric {
        margin: 0 0 10px;
        font-size: 1.1rem;
        text-transform: capitalize;
      }
      .metric-row {
        display: flex;
        justify-content: space-between;
        gap: 10px;
        margin: 4px 0;
        color: var(--muted);
      }
      ul {
        margin: 0;
        padding-left: 18px;
      }
      li + li {
        margin-top: 8px;
      }
      code {
        font-family: "Courier New", monospace;
        font-size: 0.9em;
      }
      .footer-note {
        margin-top: 18px;
        color: var(--muted);
        font-size: 0.95rem;
      }
    </style>
  </head>
  <body>
    <main class="shell">
      <section class="hero">
        <div class="eyebrow">Minimal Compare Surface</div>
        <h1>Baseline vs candidate</h1>
        <div class="hero-copy">
          A first-user walkthrough view for deciding whether the candidate got better or worse, with explicit non-comparable and partial-evidence states.
        </div>
      </section>
      <section class="state" id="state">
        <h2>Loading comparison</h2>
        <div id="state-copy">Fetching baseline and candidate evidence.</div>
      </section>
      <section class="panel">
        <div class="panel-body">
          <div class="meta-grid" id="meta-grid"></div>
          <h2>Key deltas</h2>
          <div class="delta-grid" id="delta-grid"></div>
          <h2>Regression reasons</h2>
          <div class="reason-grid">
            <div class="card">
              <ul id="reason-list"></ul>
            </div>
          </div>
          <div class="footer-note" id="evidence-note"></div>
        </div>
      </section>
    </main>
    <script type="application/json" id="compare-viewer-config">{{.ConfigJSON}}</script>
    <script>
      const config = JSON.parse(document.getElementById("compare-viewer-config").textContent);

      const stateEl = document.getElementById("state");
      const stateCopyEl = document.getElementById("state-copy");
      const metaGridEl = document.getElementById("meta-grid");
      const deltaGridEl = document.getElementById("delta-grid");
      const reasonListEl = document.getElementById("reason-list");
      const evidenceNoteEl = document.getElementById("evidence-note");

      function setState(title, copy, state) {
        stateEl.className = "state " + state;
        stateEl.querySelector("h2").textContent = title;
        stateCopyEl.textContent = copy;
      }

      function addCard(label, value) {
        const card = document.createElement("div");
        card.className = "card";
        card.innerHTML = '<div class="label"></div><p class="value"></p>';
        card.querySelector(".label").textContent = label;
        card.querySelector(".value").textContent = value;
        metaGridEl.appendChild(card);
      }

      function formatNumber(value) {
        if (value === null || value === undefined) return "n/a";
        return Number(value).toFixed(3);
      }

      function renderDeltas(deltas) {
        deltaGridEl.innerHTML = "";
        if (!deltas.length) {
          const empty = document.createElement("div");
          empty.className = "card";
          empty.textContent = "No comparable deltas available.";
          deltaGridEl.appendChild(empty);
          return;
        }
        for (const delta of deltas) {
          const card = document.createElement("div");
          card.className = "delta-card " + delta.outcome;
          card.innerHTML =
            '<h3 class="metric"></h3>' +
            '<div class="metric-row"><span>Baseline</span><strong class="baseline"></strong></div>' +
            '<div class="metric-row"><span>Candidate</span><strong class="candidate"></strong></div>' +
            '<div class="metric-row"><span>Delta</span><strong class="delta"></strong></div>' +
            '<div class="metric-row"><span>Outcome</span><strong class="outcome"></strong></div>';
          card.querySelector(".metric").textContent = delta.metric;
          card.querySelector(".baseline").textContent = formatNumber(delta.baseline_value);
          card.querySelector(".candidate").textContent = formatNumber(delta.candidate_value);
          card.querySelector(".delta").textContent = formatNumber(delta.delta);
          card.querySelector(".outcome").textContent = delta.state === "available" ? delta.outcome : delta.state;
          deltaGridEl.appendChild(card);
        }
      }

      function renderReasons(reasons) {
        reasonListEl.innerHTML = "";
        for (const reason of reasons.length ? reasons : ["No clear regression reason detected."]) {
          const item = document.createElement("li");
          item.textContent = reason;
          reasonListEl.appendChild(item);
        }
      }

      async function load() {
        const response = await fetch(config.api_url, { headers: config.headers });
        const body = await response.json();
        metaGridEl.innerHTML = "";
        addCard("Baseline run", body.baseline_run_id);
        addCard("Candidate run", body.candidate_run_id);
        addCard("State", body.state);
        addCard("Status", body.status);
        if (body.reason_code) addCard("Reason code", body.reason_code);
        setState(
          body.state === "not_comparable" ? "Runs are not comparable" :
          body.state === "partial_evidence" ? "Comparable with partial evidence" :
          "Runs are comparable",
          body.reason_code || "The candidate has enough evidence for a first-user walkthrough.",
          body.state
        );
        renderDeltas(body.key_deltas || []);
        renderReasons(body.regression_reasons || []);
        const missing = (body.evidence_quality && body.evidence_quality.missing_fields) || [];
        const warnings = (body.evidence_quality && body.evidence_quality.warnings) || [];
        evidenceNoteEl.textContent =
          missing.length || warnings.length
            ? "Evidence gaps: " + [...missing, ...warnings].join("; ")
            : "Evidence quality looks complete for this minimal compare view.";
      }

      load().catch((error) => {
        setState("Comparison failed to load", error.message, "not_comparable");
      });
    </script>
  </body>
</html>`))

type compareViewerConfig struct {
	APIURL  string            `json:"api_url"`
	Headers map[string]string `json:"headers"`
}

type compareViewerTemplateData struct {
	ConfigJSON string
}

func getRunComparisonViewerHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := compareInputFromRequest(r); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_compare_request", err.Error())
			return
		}

		configJSON, err := marshalCompareViewerConfig(r)
		if err != nil {
			logger.Error("marshal compare viewer config failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		var rendered bytes.Buffer
		if err := compareViewerPageTemplate.Execute(&rendered, compareViewerTemplateData{ConfigJSON: string(configJSON)}); err != nil {
			logger.Error("render compare viewer failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(rendered.Bytes())
	}
}

func marshalCompareViewerConfig(r *http.Request) ([]byte, error) {
	return json.Marshal(compareViewerConfig{
		APIURL:  "/v1/compare?" + r.URL.Query().Encode(),
		Headers: replayViewerHeadersFromRequest(r),
	})
}
