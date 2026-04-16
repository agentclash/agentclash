package api

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

var replayViewerPageTemplate = template.Must(template.New("replay-viewer").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>RunAgent Replay Viewer</title>
    <style>
      :root {
        color-scheme: light;
        --bg: #f4efe6;
        --panel: #fffdf8;
        --panel-alt: #f7f1e7;
        --border: #d7ccb7;
        --text: #1f1b16;
        --muted: #675f54;
        --accent: #9a3412;
        --accent-soft: #fed7aa;
        --good: #166534;
        --good-soft: #dcfce7;
        --warn: #92400e;
        --warn-soft: #fef3c7;
        --bad: #991b1b;
        --bad-soft: #fee2e2;
        --shadow: rgba(66, 38, 8, 0.08);
        --radius: 18px;
      }

      * { box-sizing: border-box; }
      body {
        margin: 0;
        font-family: "Iowan Old Style", "Palatino Linotype", "Book Antiqua", Georgia, serif;
        background:
          radial-gradient(circle at top left, rgba(154, 52, 18, 0.12), transparent 24rem),
          radial-gradient(circle at top right, rgba(120, 113, 108, 0.15), transparent 22rem),
          linear-gradient(180deg, #fbf7f0, var(--bg));
        color: var(--text);
      }

      a { color: inherit; }

      .shell {
        max-width: 1180px;
        margin: 0 auto;
        padding: 32px 20px 56px;
      }

      .hero {
        display: grid;
        gap: 14px;
        margin-bottom: 24px;
      }

      .eyebrow {
        font-family: "Courier New", monospace;
        font-size: 12px;
        letter-spacing: 0.14em;
        text-transform: uppercase;
        color: var(--muted);
      }

      h1 {
        margin: 0;
        font-size: clamp(2rem, 5vw, 3.6rem);
        line-height: 0.95;
        letter-spacing: -0.05em;
      }

      .hero-copy {
        max-width: 48rem;
        color: var(--muted);
        font-size: 1rem;
        line-height: 1.5;
      }

      .hero-meta {
        display: flex;
        flex-wrap: wrap;
        gap: 10px;
      }

      .pill {
        display: inline-flex;
        align-items: center;
        gap: 8px;
        padding: 9px 12px;
        border-radius: 999px;
        border: 1px solid var(--border);
        background: rgba(255, 253, 248, 0.72);
        font-size: 0.92rem;
      }

      .layout {
        display: grid;
        gap: 20px;
      }

      .panel {
        background: color-mix(in srgb, var(--panel) 92%, white);
        border: 1px solid var(--border);
        border-radius: var(--radius);
        box-shadow: 0 16px 40px var(--shadow);
      }

      .panel-body {
        padding: 18px;
      }

      .summary-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
        gap: 12px;
      }

      .summary-card {
        padding: 14px;
        border-radius: 14px;
        background: var(--panel-alt);
        border: 1px solid var(--border);
      }

      .summary-label {
        margin: 0 0 6px;
        font-size: 0.82rem;
        color: var(--muted);
        text-transform: uppercase;
        letter-spacing: 0.08em;
      }

      .summary-value {
        margin: 0;
        font-size: 1.1rem;
        font-weight: 700;
      }

      .status-banner {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
        padding: 18px;
        border-radius: 16px;
        border: 1px solid var(--border);
        background: var(--panel-alt);
      }

      .status-banner.ready { background: linear-gradient(135deg, var(--good-soft), #f8fffb); }
      .status-banner.pending { background: linear-gradient(135deg, var(--warn-soft), #fffdf7); }
      .status-banner.errored { background: linear-gradient(135deg, var(--bad-soft), #fff8f8); }
      .status-banner.empty { background: linear-gradient(135deg, #ede9fe, #faf8ff); }

      .status-title {
        margin: 0 0 4px;
        font-size: 1.2rem;
      }

      .status-copy {
        margin: 0;
        color: var(--muted);
      }

      .actions {
        display: flex;
        flex-wrap: wrap;
        gap: 10px;
      }

      button {
        appearance: none;
        border: 1px solid var(--border);
        border-radius: 999px;
        padding: 10px 14px;
        background: var(--panel);
        color: var(--text);
        font: inherit;
        cursor: pointer;
      }

      button.primary {
        background: var(--accent);
        color: white;
        border-color: var(--accent);
      }

      button[disabled] {
        opacity: 0.45;
        cursor: not-allowed;
      }

      .steps {
        display: grid;
        gap: 14px;
      }

      .step {
        padding: 16px;
        border: 1px solid var(--border);
        border-radius: 16px;
        background: linear-gradient(180deg, rgba(255,255,255,0.78), rgba(247,241,231,0.92));
      }

      .step-head {
        display: flex;
        flex-wrap: wrap;
        justify-content: space-between;
        gap: 10px;
        margin-bottom: 12px;
      }

      .step-title {
        margin: 0;
        font-size: 1.05rem;
      }

      .badge-row {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
      }

      .badge {
        display: inline-flex;
        align-items: center;
        border-radius: 999px;
        padding: 4px 9px;
        font-size: 0.78rem;
        letter-spacing: 0.04em;
        text-transform: uppercase;
        border: 1px solid var(--border);
        background: white;
      }

      .badge.boundary-provider { background: #dbeafe; }
      .badge.boundary-tool { background: #ede9fe; }
      .badge.boundary-system { background: #e5e7eb; }
      .badge.boundary-scoring { background: #fce7f3; }
      .badge.status-completed { background: var(--good-soft); color: var(--good); }
      .badge.status-running { background: var(--warn-soft); color: var(--warn); }
      .badge.status-failed { background: var(--bad-soft); color: var(--bad); }

      .details {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
        gap: 10px 14px;
      }

      .detail-label {
        display: block;
        margin-bottom: 4px;
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 0.08em;
        color: var(--muted);
      }

      .detail-value {
        margin: 0;
        line-height: 1.4;
        word-break: break-word;
      }

      pre {
        margin: 0;
        white-space: pre-wrap;
        word-break: break-word;
        font-family: "Courier New", monospace;
        font-size: 0.9rem;
      }

      .meta-block {
        display: grid;
        gap: 12px;
      }

      .footer-note {
        margin-top: 18px;
        color: var(--muted);
        font-size: 0.92rem;
      }

      @media (max-width: 760px) {
        .shell { padding: 24px 14px 40px; }
        .panel-body { padding: 15px; }
      }
    </style>
  </head>
  <body>
    <main class="shell">
      <section class="hero">
        <div class="eyebrow">Minimal RunAgent Replay Viewer</div>
        <h1>Inspect what the agent actually did.</h1>
        <p class="hero-copy">
          This page stays on the replay read model from Step 6a.8. It fetches the paginated replay API,
          makes provider, tool, and system boundaries legible, and keeps pending or failed materialization obvious.
        </p>
        <div class="hero-meta">
          <span class="pill"><strong>RunAgent</strong> <span>{{.RunAgentID}}</span></span>
          <span class="pill"><strong>Page Size</strong> <span id="page-limit">{{.Limit}}</span></span>
          <span class="pill"><strong>Cursor</strong> <span id="page-cursor">{{.Cursor}}</span></span>
        </div>
      </section>

      <div class="layout">
        <section class="panel">
          <div class="panel-body">
            <div id="status-banner" class="status-banner pending">
              <div>
                <h2 class="status-title">Loading replay</h2>
                <p class="status-copy">Fetching /v1/replays/{{.RunAgentID}} with the current pagination window.</p>
              </div>
              <div class="actions">
                <button id="refresh-button" class="primary" type="button">Refresh</button>
                <button id="prev-button" type="button">Previous Page</button>
                <button id="next-button" type="button">Next Page</button>
              </div>
            </div>
          </div>
        </section>

        <section class="panel">
          <div class="panel-body meta-block">
            <div class="summary-grid" id="summary-grid"></div>
            <div id="terminal-state"></div>
          </div>
        </section>

        <section class="panel">
          <div class="panel-body">
            <div id="steps" class="steps"></div>
            <p class="footer-note">Use this view for first-user testing. It is intentionally minimal, but it should already make regressions inspectable.</p>
          </div>
        </section>
      </div>
    </main>

    <script id="replay-viewer-config" type="application/json">{{.ConfigJSON}}</script>
    <script>
      const config = JSON.parse(document.getElementById("replay-viewer-config").textContent);
      const state = { cursor: config.cursor, limit: config.limit };

      const statusBanner = document.getElementById("status-banner");
      const summaryGrid = document.getElementById("summary-grid");
      const terminalState = document.getElementById("terminal-state");
      const stepsEl = document.getElementById("steps");
      const refreshButton = document.getElementById("refresh-button");
      const prevButton = document.getElementById("prev-button");
      const nextButton = document.getElementById("next-button");
      const cursorLabel = document.getElementById("page-cursor");
      const limitLabel = document.getElementById("page-limit");

      refreshButton.addEventListener("click", () => loadReplay(false));
      prevButton.addEventListener("click", () => {
        state.cursor = Math.max(0, state.cursor - state.limit);
        loadReplay(true);
      });
      nextButton.addEventListener("click", () => {
        if (!nextButton.dataset.nextCursor) return;
        state.cursor = Number(nextButton.dataset.nextCursor);
        loadReplay(true);
      });

      function updateURL() {
        const url = new URL(window.location.href);
        url.searchParams.set("limit", String(state.limit));
        url.searchParams.set("cursor", String(state.cursor));
        window.history.replaceState({}, "", url);
      }

      function fetchURL() {
        const url = new URL(config.apiURL, window.location.origin);
        url.searchParams.set("limit", String(state.limit));
        url.searchParams.set("cursor", String(state.cursor));
        return url.toString();
      }

      function setLoading() {
        statusBanner.className = "status-banner pending";
        statusBanner.querySelector(".status-title").textContent = "Loading replay";
        statusBanner.querySelector(".status-copy").textContent = "Fetching " + fetchURL();
        refreshButton.disabled = true;
        prevButton.disabled = true;
        nextButton.disabled = true;
      }

      function boundaryLabel(step) {
        if (step.type === "model_call") return "provider";
        if (step.type === "tool_call" || step.type === "sandbox_command" || step.type === "sandbox_file") return "tool";
        if (step.type === "scoring" || step.type === "scoring_metric") return "scoring";
        return "system";
      }

      function badge(label, className) {
        return '<span class="badge ' + className + '">' + escapeHTML(label) + '</span>';
      }

      function detail(label, value, asPre) {
        if (value === undefined || value === null || value === "") return "";
        const content = asPre ? '<pre>' + escapeHTML(String(value)) + '</pre>' : '<p class="detail-value">' + escapeHTML(String(value)) + '</p>';
        return '<div><span class="detail-label">' + escapeHTML(label) + '</span>' + content + '</div>';
      }

      function escapeHTML(value) {
        return value
          .replaceAll("&", "&amp;")
          .replaceAll("<", "&lt;")
          .replaceAll(">", "&gt;")
          .replaceAll('"', "&quot;")
          .replaceAll("'", "&#39;");
      }

      function renderStatus(kind, title, copy) {
        statusBanner.className = "status-banner " + kind;
        statusBanner.querySelector(".status-title").textContent = title;
        statusBanner.querySelector(".status-copy").textContent = copy;
      }

      function renderSummary(data) {
        const summary = data.replay && data.replay.summary ? data.replay.summary : {};
        const counts = summary.counts || {};
        const cards = [
          ["Replay state", data.state],
          ["RunAgent status", data.run_agent_status],
          ["Headline", summary.headline || data.message || "No summary yet"],
          ["Summary status", summary.status || "n/a"],
          ["Events", data.replay ? data.replay.event_count : 0],
          ["Replay steps", data.pagination ? data.pagination.total_steps : 0],
          ["Model calls", counts.model_calls || 0],
          ["Tool calls", counts.tool_calls || 0],
          ["Agent steps", counts.agent_steps || 0],
          ["Outputs", counts.outputs || 0],
          ["Latest sequence", data.replay && data.replay.latest_sequence_number !== null ? data.replay.latest_sequence_number : "n/a"],
          ["Artifact refs", summary.artifact_ids ? summary.artifact_ids.length : 0]
        ];

        summaryGrid.innerHTML = cards.map(([label, value]) =>
          '<article class="summary-card"><p class="summary-label">' + escapeHTML(label) + '</p><p class="summary-value">' + escapeHTML(String(value)) + '</p></article>'
        ).join("");

        const terminal = summary.terminal_state;
        if (!terminal) {
          terminalState.innerHTML = "";
          return;
        }

        terminalState.innerHTML =
          '<article class="summary-card">' +
            '<p class="summary-label">Terminal state</p>' +
            '<div class="details">' +
              detail("Headline", terminal.headline) +
              detail("Status", terminal.status) +
              detail("Event type", terminal.event_type) +
              detail("Source", terminal.source) +
              detail("Sequence", terminal.sequence_number) +
              detail("Occurred at", terminal.occurred_at) +
              detail("Error", terminal.error_message, true) +
            '</div>' +
          '</article>';
      }

      function renderSteps(data) {
        const steps = Array.isArray(data.steps) ? data.steps : [];
        if (data.state === "pending") {
          stepsEl.innerHTML = '<article class="step"><h3 class="step-title">Replay generation is pending</h3><p class="detail-value">The run agent is still executing or the replay builder has not materialized a summary yet.</p></article>';
          return;
        }
        if (data.state === "errored") {
          stepsEl.innerHTML = '<article class="step"><h3 class="step-title">Replay unavailable</h3><p class="detail-value">' + escapeHTML(data.message || "Replay generation failed or replay data is unavailable.") + '</p></article>';
          return;
        }
        if (steps.length === 0) {
          stepsEl.innerHTML = '<article class="step"><h3 class="step-title">Replay is empty</h3><p class="detail-value">The replay summary is ready, but this page window does not contain any steps yet.</p></article>';
          return;
        }

        stepsEl.innerHTML = steps.map((step, index) => {
          const boundary = boundaryLabel(step);
          const status = step.status || "running";
          const details = [
            detail("Boundary", boundary),
            detail("Source", step.source),
            detail("Type", step.type),
            detail("Provider", step.provider_key),
            detail("Model", step.provider_model_id),
            detail("Tool", step.tool_name),
            detail("Subagent key", step.subagent_key),
            detail("Subagent label", step.subagent_label),
            detail("Sandbox action", step.sandbox_action),
            detail("Metric", step.metric_key),
            detail("Started sequence", step.started_sequence),
            detail("Completed sequence", step.completed_sequence),
            detail("Occurred at", step.occurred_at),
            detail("Completed at", step.completed_at),
            detail("Events in step", step.event_count),
            detail("Artifact IDs", Array.isArray(step.artifact_ids) && step.artifact_ids.length ? step.artifact_ids.join(", ") : ""),
            detail("Final output", step.final_output, true),
            detail("Error", step.error_message, true)
          ].join("");

          return (
            '<article class="step">' +
              '<div class="step-head">' +
                '<div>' +
                  '<h3 class="step-title">' + escapeHTML((state.cursor + index + 1) + ". " + (step.headline || "Replay step")) + '</h3>' +
                  '<div class="badge-row">' +
                    badge(boundary, "boundary-" + boundary) +
                    badge(status, "status-" + status) +
                    badge(step.type || "step", "") +
                  '</div>' +
                '</div>' +
              '</div>' +
              '<div class="details">' + details + '</div>' +
            '</article>'
          );
        }).join("");
      }

      async function loadReplay(updateHistory) {
        setLoading();
        cursorLabel.textContent = String(state.cursor);
        limitLabel.textContent = String(state.limit);

        try {
          const response = await fetch(fetchURL(), {
            headers: config.headers
          });
          const data = await response.json();

          if (updateHistory) updateURL();

          renderSummary(data);
          renderSteps(data);

          const empty = data.state === "ready" && (!data.steps || data.steps.length === 0) && data.pagination && data.pagination.total_steps === 0;
          if (empty) {
            renderStatus("empty", "Replay is ready but empty", "No replay steps have been materialized for this run agent yet.");
          } else if (data.state === "ready") {
            renderStatus("ready", data.replay && data.replay.summary && data.replay.summary.headline ? data.replay.summary.headline : "Replay ready", "Provider, tool, and system steps are shown below using the paginated replay API.");
          } else if (data.state === "pending") {
            renderStatus("pending", "Replay pending", data.message || "Replay generation is still pending.");
          } else {
            renderStatus("errored", "Replay unavailable", data.message || "Replay generation failed or replay data is unavailable.");
          }

          const nextCursor = data.pagination && data.pagination.next_cursor ? data.pagination.next_cursor : "";
          nextButton.dataset.nextCursor = nextCursor;
          refreshButton.disabled = false;
          prevButton.disabled = state.cursor === 0;
          nextButton.disabled = !nextCursor;
        } catch (error) {
          renderStatus("errored", "Replay fetch failed", error instanceof Error ? error.message : "unknown fetch error");
          summaryGrid.innerHTML = "";
          terminalState.innerHTML = "";
          stepsEl.innerHTML = '<article class="step"><h3 class="step-title">Viewer could not load the replay</h3><p class="detail-value">Check the server logs and auth headers, then refresh this page.</p></article>';
          refreshButton.disabled = false;
          prevButton.disabled = state.cursor === 0;
          nextButton.disabled = true;
        }
      }

      loadReplay(false);
    </script>
  </body>
</html>`))

type replayViewerPageData struct {
	RunAgentID string
	Cursor     int
	Limit      int
	ConfigJSON string
}

type replayViewerConfig struct {
	APIURL  string            `json:"apiURL"`
	Headers map[string]string `json:"headers"`
	Cursor  int               `json:"cursor"`
	Limit   int               `json:"limit"`
}

func getRunAgentReplayViewerHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := CallerFromContext(r.Context()); err != nil {
			writeAuthzError(w, err)
			return
		}

		runAgentID, err := runAgentIDFromURLParam("runAgentID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_agent_id", err.Error())
			return
		}
		page, err := replayStepPageParamsFromRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_replay_pagination", err.Error())
			return
		}

		configJSON, err := marshalReplayViewerConfig(runAgentID, page, r)
		if err != nil {
			logger.Error("marshal replay viewer config failed",
				"method", r.Method,
				"path", r.URL.Path,
				"run_agent_id", runAgentID,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		data := replayViewerPageData{
			RunAgentID: runAgentID.String(),
			Cursor:     page.Cursor,
			Limit:      normalizedReplayPageLimit(page.Limit),
			ConfigJSON: string(configJSON),
		}

		var rendered bytes.Buffer
		if err := replayViewerPageTemplate.Execute(&rendered, data); err != nil {
			logger.Error("render replay viewer failed",
				"method", r.Method,
				"path", r.URL.Path,
				"run_agent_id", runAgentID,
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

func marshalReplayViewerConfig(runAgentID uuid.UUID, page ReplayStepPageParams, r *http.Request) ([]byte, error) {
	return json.Marshal(replayViewerConfig{
		APIURL:  "/v1/replays/" + runAgentID.String(),
		Headers: replayViewerHeadersFromRequest(r),
		Cursor:  page.Cursor,
		Limit:   normalizedReplayPageLimit(page.Limit),
	})
}

func replayViewerHeadersFromRequest(r *http.Request) map[string]string {
	headers := make(map[string]string)
	for _, headerName := range []string{
		headerUserID,
		headerWorkOSUserID,
		headerUserEmail,
		headerUserDisplayName,
		headerWorkspaceMemberships,
	} {
		if value := r.Header.Get(headerName); value != "" {
			headers[headerName] = value
		}
	}
	return headers
}
