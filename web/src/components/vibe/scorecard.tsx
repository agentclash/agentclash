"use client";

import { useEffect, useState, type ReactNode } from "react";
import { ArrowRight, Bookmark, Copy, Lightbulb } from "lucide-react";
import { dollars, terminal, type CaseResult, type Operation } from "@/lib/vibe";
import { resultHeadline } from "./result-headline";
import { verifiedFinding } from "./grounded-finding";
import { CaseEvidence } from "./case-evidence";
import { unresolvedCase } from "@/lib/vibe-result-evidence";
import { testCount } from "@/lib/vibe-presentation";
import { originalInputs } from "./result-prompts";
import { recoveryMessage } from "./recovery-message";
import { VibeButton } from "./vibe-button";

export function VibeScorecard({
  headingLevel = 2,
  findingFirst = false,
  primary = true,
  operation,
  baseline,
  eventCursor,
  loadEvidence,
  onImprove,
  onRetest,
  onNewReplies,
  onDispute,
  onRegrade,
  onNewCheck,
  onSave,
  onRecheck,
  onAskCase,
  busy,
  testJourney = false,
  buildJourney = false,
  more,
}: {
  headingLevel?: 1 | 2;
  findingFirst?: boolean;
  primary?: boolean;
  operation: Operation;
  baseline?: Operation;
  eventCursor?: number;
  loadEvidence: (key: string) => Promise<CaseResult>;
  onImprove: (result?: CaseResult) => void;
  onRetest: () => void;
  onNewReplies?: () => void;
  onDispute?: (rule: string, result: CaseResult) => void;
  onSave?: () => void;
  onRegrade?: () => void;
  onNewCheck?: () => void;
  onRecheck?: () => void;
  onAskCase?: (caseKey: string) => void;
  busy: boolean;
  testJourney?: boolean;
  buildJourney?: boolean;
  more?: ReactNode;
}) {
  const Heading = headingLevel === 1 ? "h1" : "h2";
  const [filter,setFilter]=useState("all");
  const [copiedFinding, setCopiedFinding] = useState("");
  const [concreteFinding,setConcreteFinding]=useState<{id:string;text:string}>();
  const failedKey=operation.results.find(c => c.verdict==="FAIL")?.case_key;
  useEffect(() => {
    if (!findingFirst || !failedKey || !terminal(operation.state)) return;
    let live=true;
    void loadEvidence(failedKey).then(result => {
      const check=result.checks.find(c => c.verdict==="FAIL" && verifiedFinding(result,c));
      if (live && check) setConcreteFinding({id:operation.id,text:check.evidence.slice(0,220)});
    }).catch(() => undefined);
    return () => {live=false};
    // Evidence callbacks are recreated by the parent; immutable run identity controls loading.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  },[findingFirst,failedKey,operation.id,operation.state]);
  const score = operation.scorecard;
  if (!score) return null;
  const done = terminal(operation.state);
  const provided = operation.source?.kind === "provided_conversations";
  const regraded = operation.source?.comparison === "regraded";
  const rechecked = regraded || operation.source?.comparison === "rechecked";
  const comparable = comparableGrades(operation, baseline);

  const oldResults = new Map(
    (comparable ? baseline : undefined)?.results.map((result) => [result.case_key, result]),
  );
  const newFailures = operation.results.filter(
    (result) =>
      result.verdict === "FAIL" &&
      oldResults.get(result.case_key)?.verdict === "PASS",
  );
  const regressions = rechecked ? 0 : newFailures.length;
  const fixed = rechecked
    ? 0
    : operation.results.filter(
        (result) =>
          result.verdict === "PASS" &&
          oldResults.get(result.case_key)?.verdict === "FAIL",
      ).length;
  const unresolved = operation.results.filter(unresolvedCase);
  const unresolvedCount = unresolved.length;
  const overlappingCounts = unresolved.some(result => result.verdict === "FAIL" || result.verdict === "PASS");
  const incomplete = (score.incomplete_cases || 0) > 0 || score.unknown > 0 || unresolvedCount > 0;
  const stopped = ["CANCELLED", "EXPIRED", "FAILED"].includes(operation.state);
  const offerSave =
    !!onSave &&
    (!!baseline || testJourney) &&
    done &&
    !stopped &&
    !rechecked &&
    score.failed === 0 &&
    !incomplete &&
    score.total > 0;
  const retryable = operation.results.some(
    (result) => !!result.error && result.error.code !== "not_evaluated",
  );
  const visible=operation.results.filter(result => filter==="all" || (filter==="unresolved" ? unresolvedCase(result) : result.verdict===(filter==="passing"?"PASS":"FAIL")));
  const leading =
    done && !(testJourney && !score.failed && !incomplete && !stopped)
      ? (!rechecked && newFailures.find(result=>visible.includes(result))) ||
        visible.find((result) => result.verdict === "FAIL") ||
        visible.find(
          (result) =>
            result.verdict === "UNKNOWN" &&
            result.error?.code !== "not_evaluated",
        ) ||
        visible.find((result) => result.verdict === "PASS") ||
        visible[0]
      : undefined;
  const remaining = visible.filter((result) => result !== leading);
  const findingID = `${operation.id}:${leading?.case_key || ""}`;
  const copied = copiedFinding === findingID;
  const headline = stopped
    ? "Run stopped. Results are saved."
    : !done
      ? `Running ${testCount(score.total)}…`
      : score.total === 0
        ? "No tests were run."
        : rechecked
          ? regraded ? "Saved replies regraded" : "Same replies graded again"
          : regressions
            ? `${testCount(regressions, "newly failing")} in this update`
            : score.failed
              ? `${testCount(score.failed)} failed`
              : incomplete
                ? "Some replies still need a closer look"
                : `No issues found in ${score.total === 1 ? "this test" : `these ${testCount(score.total)}`}`;
  const evidenceVersion = `${operation.id}:${operation.state}:${eventCursor || 0}`;
  // The summary line gives the failed and unresolved counts; this note does not repeat them.
  const statusNote = testJourney
    ? done && score.total === 0
      ? "No tests produced a result."
      : done
        ? score.failed || incomplete ? "" : "These replies met the expectations."
        : regraded ? "Regrading the saved replies…" : provided ? "Grading the recorded replies…" : "Generating agent replies and grading them…"
    : done
      ? `These findings cover only ${score.total === 1 ? "this test" : `these ${testCount(score.total)}`}.`
      : "Completed findings are kept while the run continues.";

  if (testJourney && !buildJourney && done) return <article aria-label="Run results" className="space-y-6">
    <Heading className="vibe-results-outcome" aria-live="polite" tabIndex={-1}>{!stopped && !incomplete && score.total > 0 && !rechecked ? resultHeadline(score.passed, score.total) : headline}</Heading>
    {operation.error && <p role="alert">{recoveryMessage(operation)}</p>}
    {baseline && comparable && !rechecked && <p>{fixed} fixed · {regressions} new failures. Same tests and grading model.</p>}
    {incomplete && <p>{unresolvedCount ? `${unresolvedCount} unresolved. ` : ""}Some grading is incomplete. Completed results are kept.</p>}
    {leading && <div className="vibe-panel" aria-label="Leading finding"><CaseEvidence key={findingID} summary={leading} onAsk={onAskCase ? () => onAskCase(leading.case_key) : undefined} onDispute={onDispute} onRegrade={onRegrade} concise exampleFirst evidenceVersion={evidenceVersion} load={loadEvidence} origin={operation.source?.kind} defaultOpen busy={busy} /></div>}
    {remaining.length > 0 && <details className="vibe-other-results"><summary>Other results · {remaining.length}</summary>
      {remaining.map(result => <CaseEvidence key={`${operation.id}:${result.case_key}`} summary={result} onAsk={onAskCase ? () => onAskCase(result.case_key) : undefined} concise evidenceVersion={evidenceVersion} load={loadEvidence} onDispute={onDispute} onRegrade={onRegrade} origin={operation.source?.kind} busy={busy} />)}
    </details>}
    {score.failed > 0 ? <VibeButton variant={primary ? "primary" : "secondary"} disabled={busy} onClick={() => onImprove()}><Lightbulb aria-hidden="true" />Suggest a fix</VibeButton>
      : onSave && !incomplete && !stopped ? <VibeButton variant={primary ? "primary" : "secondary"} disabled={busy} onClick={onSave}><Bookmark aria-hidden="true" />Keep these tests</VibeButton>
      : <VibeButton variant={primary ? "primary" : "secondary"} disabled={busy} onClick={onRetest}>Run tests again</VibeButton>}
    <details className="text-sm vibe-muted"><summary className="cursor-pointer py-3">More</summary><div className="mt-3 space-y-4">
      <p>{score.passed} passed · {score.failed} failed{unresolvedCount ? ` · ${unresolvedCount} unresolved` : ""}</p>
      {overlappingCounts && <p>Unresolved tests have unfinished grading and can also be counted as passed or failed.</p>}
      <p>{regraded ? "Saved replies only. Your agent was not called again." : provided ? "Based on the conversations you provided. Your live app was not called." : "Text replies from your saved instructions. Your live app was not called."}</p>
      {rechecked && <p>These are the same saved replies. A different grade does not show that your agent improved.</p>}
      {baseline && !rechecked && !comparable && <p>These runs do not have matching verified grading settings. Earlier results are kept separately.</p>}
      <label className="vibe-results-filter">Show tests<select aria-label="Filter tests" value={filter} onChange={e => setFilter(e.target.value)}><option value="all">All</option><option value="failing">Failing</option><option value="unresolved">Unresolved</option><option value="passing">Passing</option></select></label>
      <div className="flex flex-wrap gap-2">
        {(score.failed > 0 || onSave && !incomplete && !stopped) && <VibeButton disabled={busy} onClick={onRetest}>Run tests again</VibeButton>}
        {score.failed > 0 && onSave && <VibeButton variant="quiet" disabled={busy} onClick={onSave}>Keep these tests</VibeButton>}
        {provided && onNewReplies && <VibeButton disabled={busy} onClick={onNewReplies}>Test new replies</VibeButton>}
        {onRegrade && <VibeButton variant="quiet" disabled={busy} onClick={onRegrade}>Grade the same replies again</VibeButton>}
        {onNewCheck && <VibeButton variant="quiet" disabled={busy} onClick={onNewCheck}>Start a separate run</VibeButton>}
        {provided && <CopyOriginalInputs key={operation.id} results={operation.results} load={loadEvidence} />}
      </div>
      <p>Grading model: {operation.models.evaluator}</p>
      {!provided && !regraded && <p>Agent model: {operation.models.target}</p>}
      <p>{score.checks_evaluated ?? score.evaluated} of {score.checks_expected ?? score.total} expectations evaluated.</p>
      <p>{operation.billing === "RECONCILING" ? "Provider spend is still being reconciled." : `Provider spend: ${dollars(operation.actual_cost_nano_usd)}.`}</p>
      {more}
      <details><summary>Technical details</summary><p>Origin: {operation.evidence_metadata?.origin || "unverified"}. Application revision: {operation.evidence_metadata?.application_revision || "unknown"}.</p>
        {(["model", "build", "test", "judge", "config"] as const).map(name => <p className="break-all" key={name}>{name}: {operation.evidence_metadata?.fingerprints[name] || "unknown"}</p>)}
        {(["messages", "tools", "retrieval"] as const).map(name => <p key={name}>{name}: {operation.evidence_metadata?.capture_scope[name].scope || "unknown"} · instrumentation {operation.evidence_metadata?.capture_scope[name].instrumentation || "unknown"}</p>)}
      </details>
    </div></details>
  </article>;

  return (
    <article aria-label="Run results" className="space-y-6">
      <div>
        <Heading
          className="vibe-results-outcome"
          aria-live="polite"
          tabIndex={-1}
        >
          {testJourney && done && !stopped && !incomplete && score.total > 0 && !rechecked
            ? resultHeadline(score.passed, score.total)
            : headline}
        </Heading>
        {done && <p className="mt-3 text-sm">{score.passed} passed · {score.failed} failed{unresolvedCount ? ` · ${unresolvedCount} unresolved` : ""}</p>}
        {done && overlappingCounts && <p className="mt-2 text-sm vibe-muted">Unresolved tests have unfinished grading and can also be counted as passed or failed.</p>}
        {findingFirst && concreteFinding?.id === operation.id && leading?.case_key !== failedKey && <p>{concreteFinding.text}</p>}
        {findingFirst && done && !stopped && score.total > 0 && !score.failed && !incomplete && <p className="mt-2 text-sm vibe-muted">All these tests passed. This does not prove reliability beyond them; live integrations and unseen inputs remain untested.</p>}
        {statusNote && <p className="mt-3 text-sm vibe-muted">{statusNote}</p>}
        <p className="mt-2 text-sm vibe-muted">
          {regraded ? "Saved replies only. Your agent was not called again." : provided
            ? "Based on the conversations you provided. Your live app was not called."
            : "Text replies from your saved instructions. Your live app was not called."}
        </p>
        {baseline && done && !rechecked && !comparable && <p className="mt-3 text-sm vibe-muted">These runs do not have matching verified grading settings. Earlier results are kept separately.</p>}
        {rechecked && (
          <p className="mt-3 text-sm vibe-muted">
            These are the same saved replies. A different grade does not show
            that your agent improved.
          </p>
        )}
        {baseline && comparable && done && !rechecked && (
          <p
            className={`mt-3 text-sm${testJourney && regressions ? " text-builder-warn" : ""}`}
          >
            {testJourney ? (
              <>
                {fixed} fixed · {regressions} new failures. Same tests and
                grading model.
              </>
            ) : (
              <>
                {fixed} previously failing{" "}
                {fixed === 1 ? "test" : "tests"}
                {fixed === 1 ? " now passes" : " now pass"} · {regressions} new{" "}
                {regressions === 1 ? "failure" : "failures"}.
                <span className="vibe-muted">
                  {" "}
                  Same expectations and grading model.{" "}
                  {provided
                    ? "Same customer messages."
                    : "Same test messages."}
                </span>
              </>
            )}
          </p>
        )}
        {done && incomplete && (
          <p className="mt-3 text-sm vibe-muted">
            {unresolvedCount > 0
              ? `${unresolvedCount} ${unresolvedCount === 1 ? "result is" : "results are"} unresolved. `
              : "Some grading is incomplete. "}
            An unfinished grade is not a failure by your agent. Completed results are kept.
          </p>
        )}
      </div>

      <label className="vibe-results-filter">Show tests<select aria-label="Filter tests" value={filter} onChange={e=>setFilter(e.target.value)}><option value="all">All</option><option value="failing">Failing</option><option value="unresolved">Unresolved</option><option value="passing">Passing</option></select></label>
      {done && operation.error && <p role="alert" className="text-sm text-builder-warn">{recoveryMessage(operation)}</p>}

      {leading && (
        <div className="vibe-panel" aria-label="Leading finding">
          <CaseEvidence
            key={findingID}
            summary={leading}
            onAsk={onAskCase ? () => onAskCase(leading.case_key) : undefined}
            concise={testJourney}
            evidenceVersion={evidenceVersion}
            load={loadEvidence}
            onDispute={onDispute}
            onRegrade={done ? onRegrade : undefined}
            origin={operation.source?.kind}
            defaultOpen
            allowFixPrompt={!testJourney}
            onFixCopied={() => setCopiedFinding(findingID)}
            busy={busy}
          />
        </div>
      )}

      {done && testJourney && (
        <div className="flex flex-wrap items-center gap-2">
          {score.failed > 0 && (
            <VibeButton
              variant={primary ? "primary" : "secondary"}
              disabled={busy}
              onClick={() => onImprove()}
            >
              <Lightbulb />
              Suggest a fix
            </VibeButton>
          )}
          <VibeButton
            variant={primary && !offerSave && score.failed === 0 ? "primary" : "secondary"}
            disabled={busy}
            onClick={onRetest}
          >
            Run tests again
            <ArrowRight />
          </VibeButton>
          {onSave && (
            <VibeButton
              variant={primary && offerSave ? "primary" : "quiet"}
              disabled={busy}
              onClick={onSave}
            >
              <Bookmark />
              Keep these tests
            </VibeButton>
          )}
        </div>
      )}
      {done && !testJourney && (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            {score.failed > 0 && (
              <VibeButton
                variant={primary && !copied ? "primary" : "secondary"}
                disabled={busy}
                onClick={() => onImprove()}
              >
                <Lightbulb aria-hidden="true" /> Suggest a change
              </VibeButton>
            )}
            {offerSave && (
              <VibeButton variant={primary ? "primary" : "secondary"} disabled={busy} onClick={onSave}>
                <Bookmark aria-hidden="true" /> Save this result
              </VibeButton>
            )}
            {retryable && onRecheck && (
              <VibeButton
                variant={primary && !score.failed ? "primary" : "secondary"}
                disabled={busy}
                onClick={onRecheck}
              >
                Grade these replies again
                <ArrowRight aria-hidden="true" />
              </VibeButton>
            )}
            <VibeButton
              variant={
                primary && !offerSave &&
                (!score.failed || copied) &&
                !(retryable && onRecheck)
                  ? "primary"
                  : "secondary"
              }
              disabled={busy}
              onClick={onNewReplies || onRetest}
            >
              {onNewReplies ? "Test new replies" : "Run tests again"}
              <ArrowRight aria-hidden="true" />
            </VibeButton>
            {operation.results.length > 0 && (
              <CopyOriginalInputs
                key={operation.id}
                results={operation.results}
                load={loadEvidence}
              />
            )}
          </div>
          {score.failed > 0 && (
            <p className="text-sm vibe-muted">
              Fix the behavior in your app, then bring back its new replies to
              the same messages.
            </p>
          )}
        </div>
      )}

      {remaining.length > 0 &&
        (done ? (
          <details className="text-sm">
            <summary className="cursor-pointer py-2 vibe-muted">
              {testJourney && !leading ? "Test results" : "Other results"} ·{" "}
              {remaining.length}
            </summary>
            <div className="vibe-panel mt-3" aria-label="Individual results">
              {remaining.map((result) => (
                <CaseEvidence
                  key={`${operation.id}:${result.case_key}`}
                  summary={result}
                  onAsk={onAskCase ? () => onAskCase(result.case_key) : undefined}
                  concise={testJourney}
                  evidenceVersion={evidenceVersion}
                  load={loadEvidence}
                  onDispute={onDispute}
                  onRegrade={done ? onRegrade : undefined}
                  origin={operation.source?.kind}
                  allowFixPrompt={!testJourney}
                  busy={busy}
                />
              ))}
            </div>
          </details>
        ) : (
          <div className="vibe-panel" aria-label="Individual results">
            {remaining.map((result) => (
              <CaseEvidence
                key={`${operation.id}:${result.case_key}`}
                summary={result}
                onAsk={onAskCase ? () => onAskCase(result.case_key) : undefined}
                concise={testJourney}
                evidenceVersion={evidenceVersion}
                load={loadEvidence}
                onDispute={onDispute}
                onRegrade={done ? onRegrade : undefined}
                origin={operation.source?.kind}
                busy={busy}
              />
            ))}
          </div>
        ))}

      <details className="text-sm vibe-muted">
        <summary className="cursor-pointer py-2">About this run</summary>
        <div className="mt-2 space-y-3">
          <p>
            {score.passed} passed · {score.failed}{" "}
            {score.failed === 1 ? "needs" : "need"} attention · {unresolvedCount}{" "}
            unresolved
            {(score.incomplete_cases || 0) > score.unknown
              ? ` · ${score.incomplete_cases} with incomplete coverage`
              : ""}
          </p>
          <p>
            {regraded ? "Previously saved replies were graded again. The original replies, rules and results are unchanged." : provided
              ? `${operation.source?.label || "Provided conversations"}. Every message in the selected conversations was included. No replies were generated and no live agent was called.`
              : "New text replies generated here from the saved instructions. Each test starts fresh; tools, live data and trial conversations are not included."}
          </p>
          <details><summary>Saved source identities and capture</summary>
            <p>Origin: {operation.evidence_metadata?.origin || "unverified"}. Application revision: {operation.evidence_metadata?.application_revision || "unknown"}.</p>
            {(["model","build","test","judge","config"] as const).map(name=><p className="break-all" key={name}>{name}: {operation.evidence_metadata?.fingerprints[name] || "unknown — no verified original identity"}</p>)}
            {(["messages","tools","retrieval"] as const).map(name=><p key={name}>{name}: {operation.evidence_metadata?.capture_scope[name].scope || "unknown"} · instrumentation {operation.evidence_metadata?.capture_scope[name].instrumentation || "unknown"}</p>)}
          </details>
          <p>
            {score.checks_evaluated ?? score.evaluated} of{" "}
            {score.checks_expected ?? score.total} expectations evaluated.
          </p>
          <p>
            {!provided && !regraded && (
              <>
                Target: {operation.models.target}
                <br />
              </>
            )}
            Grading model: {operation.models.evaluator}
          </p>
          <p>
            {operation.billing === "RECONCILING"
              ? "Provider spend is still being reconciled."
              : `Provider spend: ${dollars(operation.actual_cost_nano_usd)}.`}
          </p>
          {done && onNewCheck && (
            <VibeButton variant="quiet" disabled={busy} onClick={onNewCheck}>
              Start a separate run
            </VibeButton>
          )}
          {done && !testJourney && (onNewReplies || (onSave && !offerSave)) && (
            <div className="flex flex-wrap gap-2">
              {/* Without new replies, "Run tests again" already reruns these tests. */}
              {onNewReplies && (
                <VibeButton variant="quiet" disabled={busy} onClick={onRetest}>
                  {provided ? "Compare an update" : "Run the same tests again"}
                  <ArrowRight aria-hidden="true" />
                </VibeButton>
              )}
              {onSave && !offerSave && (
                <VibeButton variant="quiet" disabled={busy} onClick={onSave}>
                  <Bookmark aria-hidden="true" /> Save this result
                </VibeButton>
              )}
            </div>
          )}
        </div>
      </details>
    </article>
  );
}

function CopyOriginalInputs({
  results,
  load,
}: {
  results: CaseResult[];
  load: (key: string) => Promise<CaseResult>;
}) {
  const [copying, setCopying] = useState(false);
  const [message, setMessage] = useState("");
  const [fallback, setFallback] = useState("");
  const copy = async () => {
    setCopying(true);
    setMessage("");
    setFallback("");
    try {
      const evidence = await Promise.all(
        results.map((result) => load(result.case_key)),
      );
      if (
        evidence.some(
          (result, index) =>
            result.case_key !== results[index].case_key ||
            result.version !== results[index].version,
        )
      )
        throw new Error(
          "The saved evidence did not match these results. Reload before copying the original inputs.",
        );
      const text = originalInputs(evidence);
      try {
        await navigator.clipboard.writeText(text);
        setMessage(
          "Original inputs copied. Keep them unchanged and replace only the agent replies.",
        );
      } catch {
        setFallback(text);
        setMessage("Select the original inputs below to copy them.");
      }
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "The original inputs could not be loaded.",
      );
    } finally {
      setCopying(false);
    }
  };
  return (
    <div className="contents">
      <VibeButton
        variant="quiet"
        disabled={copying}
        loading={copying}
        onClick={() => void copy()}
      >
        <Copy aria-hidden="true" /> Copy original inputs
      </VibeButton>
      {message && (
        <p role="status" className="w-full text-sm vibe-muted">
          {message}
        </p>
      )}
      {fallback && (
        <textarea
          className="vibe-textarea min-h-40"
          aria-label="Original inputs to copy"
          readOnly
          value={fallback}
          onFocus={(event) => event.currentTarget.select()}
        />
      )}
    </div>
  );
}

// Historical runs remain readable, but model names alone cannot establish that
// the same grading contract was used. Hashes come from the admitted server plan.
export function comparableGrades(operation: Operation, baseline?: Operation) {
  return !!baseline && operation.baseline_id === baseline.id &&
    (operation.grading?.version === 1 || operation.grading?.version === 2) && baseline.grading?.version === operation.grading.version &&
    !!operation.grading.hash && operation.grading.hash === baseline.grading.hash &&
    operation.source?.kind === baseline.source?.kind;
}
