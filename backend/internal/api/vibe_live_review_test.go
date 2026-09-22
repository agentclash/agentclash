package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type liveReviewWave struct {
	StartedAt     time.Time `json:"started_at"`
	BaselineNano  int64     `json:"baseline_account_balance_nano"`
	BaselineCalls int64     `json:"baseline_total_calls"`
	MaxSpendNano  int64     `json:"max_spend_nano"`
	MaxCalls      int64     `json:"max_calls"`
	Campaign      string    `json:"campaign"`
}

type liveReviewBalance struct {
	BalanceNano    int64 `json:"balance_nano"`
	HeldNano       int64 `json:"held_nano"`
	TotalCalls     int64 `json:"total_calls"`
	RemainingCalls int64 `json:"active_reserved_calls_remaining"`
	Active         int64 `json:"active_operations"`
	Disabled       bool  `json:"disabled"`
}

// This deliberately duplicates the admission ceiling, not the estimated
// provider bill. Holds count in full until the real accounting path settles.
func (w liveReviewWave) canAdmit(b liveReviewBalance, maxCost, calls int64) error {
	if w.StartedAt.IsZero() || w.Campaign == "" || w.MaxSpendNano <= 0 || w.MaxSpendNano > 2*vibe.NanoUSD || w.MaxCalls < 1 || w.MaxCalls > 80 || w.BaselineNano < 0 || w.BaselineCalls < 0 {
		return fmt.Errorf("invalid frozen wave allowance")
	}
	if b.Disabled || b.BalanceNano < 0 || b.HeldNano < 0 || b.TotalCalls < w.BaselineCalls || b.BalanceNano > w.BaselineNano || b.RemainingCalls < 0 || calls != 1 || maxCost <= 0 || maxCost > 2*vibe.NanoUSD {
		return fmt.Errorf("campaign accounting or proposed admission is inconsistent")
	}
	spent := w.BaselineNano - b.BalanceNano
	usedCalls := b.TotalCalls - w.BaselineCalls
	if spent > w.MaxSpendNano || b.HeldNano > w.MaxSpendNano-spent || maxCost > w.MaxSpendNano-spent-b.HeldNano || b.HeldNano > b.BalanceNano || maxCost > b.BalanceNano-b.HeldNano {
		return fmt.Errorf("wave spend or available campaign balance cannot cover this complete reservation")
	}
	if usedCalls > w.MaxCalls || b.RemainingCalls > w.MaxCalls-usedCalls || calls > w.MaxCalls-usedCalls-b.RemainingCalls {
		return fmt.Errorf("wave call allowance cannot cover this complete reservation")
	}
	if b.Active != 0 {
		return fmt.Errorf("another campaign operation is active; live wave work must be sequential")
	}
	return nil
}

func readLiveReviewBalance(ctx context.Context, db *pgxpool.Pool, campaign string) (liveReviewBalance, error) {
	var b liveReviewBalance
	// An absent account is an error. This harness never creates a new campaign
	// or grants fresh credits to make a measurement fit.
	err := db.QueryRow(ctx, `SELECT a.balance,a.held,a.disabled,
 COALESCE((SELECT sum(o.model_calls) FROM vibe_operations o JOIN vibe_reservations r ON r.operation_id=o.id WHERE r.account_id=a.id),0),
 COALESCE((SELECT sum(GREATEST(COALESCE((o.input->>'calls')::bigint,0)-o.model_calls,0)) FROM vibe_operations o JOIN vibe_reservations r ON r.operation_id=o.id WHERE r.account_id=a.id AND o.state IN ('QUEUED','RUNNING','AWAITING_APPROVAL','FINALIZING','CANCELLING')),0),
 (SELECT count(*) FROM vibe_operations o JOIN vibe_reservations r ON r.operation_id=o.id WHERE r.account_id=a.id AND o.state IN ('QUEUED','RUNNING','AWAITING_APPROVAL','FINALIZING','CANCELLING'))
 FROM vibe_accounts a WHERE a.id=$1`, "local:"+campaign).Scan(&b.BalanceNano, &b.HeldNano, &b.Disabled, &b.TotalCalls, &b.RemainingCalls, &b.Active)
	return b, err
}

type liveReviewFixture struct {
	ID              string `json:"id"`
	Domain          string `json:"domain"`
	Split           string `json:"split"`
	Source          string `json:"source"`
	Request         string `json:"request"`
	Input           string `json:"input"`
	Expected        string `json:"expected"`
	Criteria        string `json:"shared_criteria"`
	Reference       string `json:"reference_status"`
	ReferenceReason string `json:"reference_reason"`
	Critical        bool   `json:"critical"`
}

type liveReviewResult struct {
	FixtureID   string                        `json:"fixture_id"`
	Domain      string                        `json:"domain"`
	Reference   string                        `json:"proposed_reference_status"`
	Observed    string                        `json:"observed_status"`
	Critical    bool                          `json:"critical"`
	Committed   bool                          `json:"committed"`
	SessionID   uuid.UUID                     `json:"session_id"`
	OperationID uuid.UUID                     `json:"operation_id"`
	ModelCalls  int                           `json:"model_calls"`
	LatencyMS   int64                         `json:"latency_ms"`
	CostNano    *int64                        `json:"actual_cost_nano"`
	Issue       *vibe.Fault                   `json:"issue,omitempty"`
	Problems    []vibe.SuiteValidationProblem `json:"problems,omitempty"`
	ReviewCases []vibe.SuiteCaseReview        `json:"review_cases,omitempty"`
	Consistency *vibe.SuiteConsistencyResult  `json:"consistency,omitempty"`
}

type liveReviewReport struct {
	Provenance           string                    `json:"reference_provenance"`
	Measurement          string                    `json:"measurement_scope"`
	StartedAt            time.Time                 `json:"started_at"`
	FinishedAt           *time.Time                `json:"finished_at,omitempty"`
	Model                string                    `json:"model"`
	ProfileHash          string                    `json:"profile_hash"`
	FixtureHash          string                    `json:"fixture_hash"`
	ValidatorVersion     string                    `json:"validator_version"`
	Wave                 liveReviewWave            `json:"wave"`
	Before               liveReviewBalance         `json:"campaign_before"`
	After                liveReviewBalance         `json:"campaign_after"`
	Results              []liveReviewResult        `json:"results"`
	Confusion            map[string]map[string]int `json:"confusion_reference_to_observed"`
	LatencyMS            []int64                   `json:"latency_ms"`
	TotalCostNano        int64                     `json:"known_actual_cost_nano"`
	CriticalFalseAccepts int                       `json:"critical_false_accepts_vs_proposed_labels"`
	StopReason           string                    `json:"stop_reason,omitempty"`
}

// Explicitly opted-in network experiment, excluded from ordinary test runs.
// Run from backend with the existing Vibe model/budget environment and:
//
//	VIBE_LIVE_REVIEW_ENABLED=true VIBE_LIVE_REVIEW_DATABASE_URL=<local DB>
//	agent-run go test ./internal/api -run '^TestVibeLiveReviewHeldOut$' -count=1 -timeout=90m -v
//
// The caller supplies the frozen shared wave.json; this test never resets it.
// Exactly one real manual-review operation is admitted per held-out fixture.
func TestVibeLiveReviewHeldOut(t *testing.T) {
	runLiveReviewMeasurement(t, false)
}

// Separate fixed corrective controls; this does not rerun or replace the
// original held-out measurement. It needs an explicit additional call allowance
// in consistency-wave.json and retains the original wave's dollar ceiling.
func TestVibeLiveReviewConsistencyRegression(t *testing.T) {
	runLiveReviewMeasurement(t, true)
}

func runLiveReviewMeasurement(t *testing.T, consistency bool) {
	t.Helper()
	flag, waveEnv, reportEnv := "VIBE_LIVE_REVIEW_ENABLED", "VIBE_LIVE_REVIEW_WAVE_FILE", "VIBE_LIVE_REVIEW_REPORT_FILE"
	waveName, reportName := "wave.json", "held-out-review.json"
	version := vibe.SuiteValidatorVersion
	if consistency {
		flag, waveEnv, reportEnv = "VIBE_LIVE_CONSISTENCY_ENABLED", "VIBE_LIVE_CONSISTENCY_WAVE_FILE", "VIBE_LIVE_CONSISTENCY_REPORT_FILE"
		waveName, reportName = "consistency-wave.json", "consistency-review.json"
		version = vibe.LatestSuiteValidatorVersion
	}
	if os.Getenv(flag) != "true" {
		t.Skip("explicit opt-in required for paid held-out review")
	}
	dsn := os.Getenv("VIBE_LIVE_REVIEW_DATABASE_URL")
	if dsn == "" {
		t.Fatal("VIBE_LIVE_REVIEW_DATABASE_URL must explicitly name the local database")
	}
	dbConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid local database configuration")
	}
	if host := dbConfig.ConnConfig.Host; host != "127.0.0.1" && host != "localhost" && host != "::1" && !strings.HasPrefix(host, "/") {
		t.Fatal("live review database must be local")
	}
	cfg, err := vibe.LoadConfig()
	if err != nil {
		t.Fatal("Vibe configuration is not valid for this experiment")
	}
	// A future deployment default cannot silently change the fixed baseline.
	cfg.SuiteReviewVersion = version
	if !cfg.Enabled || !cfg.TestingLocally() || cfg.FreeOnly || !cfg.ReliableAuthoring || cfg.Credential == "" || cfg.DefaultModels().Assistant != "deepseek/deepseek-v4-flash-0731" {
		t.Fatal("requires the existing enabled local DeepSeek paid configuration and reliable authoring")
	}
	profile, err := cfg.Profile(cfg.DefaultModels().Assistant)
	if err != nil {
		t.Fatal(err)
	}
	wavePath := os.Getenv(waveEnv)
	if wavePath == "" {
		// go test sets its working directory to backend/internal/api.
		wavePath = filepath.Join("../../../testing/vibe-local/reliability-20260916", waveName)
	}
	waveData, err := os.ReadFile(wavePath)
	if err != nil {
		t.Fatal("read the pre-existing frozen wave file:", err)
	}
	var wave liveReviewWave
	if err := json.Unmarshal(waveData, &wave); err != nil || wave.Campaign != cfg.Campaign {
		t.Fatal("wave metadata does not match the configured shared campaign")
	}
	if consistency {
		originalData, err := os.ReadFile(filepath.Join(filepath.Dir(wavePath), "wave.json"))
		var original liveReviewWave
		if err != nil || json.Unmarshal(originalData, &original) != nil || wave.MaxCalls > 6 || wave.Campaign != original.Campaign || wave.BaselineNano != original.BaselineNano || wave.MaxSpendNano > original.MaxSpendNano {
			t.Fatal("corrective wave must retain the original campaign/dollar baseline and allow at most six additional calls")
		}
	}
	lock, err := os.OpenFile(wavePath+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("another live wave process holds the campaign lock")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	reportPath := os.Getenv(reportEnv)
	if reportPath == "" {
		reportPath = filepath.Join(filepath.Dir(wavePath), reportName)
	}
	if _, err := os.Stat(reportPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("report already exists or is unavailable; do not retry the held-out sample until green")
	}
	data, err := os.ReadFile("../vibe/testdata/calibration/suite-validity.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Provenance string              `json:"provenance"`
		Fixtures   []liveReviewFixture `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	fixtures := []liveReviewFixture{}
	for _, f := range corpus.Fixtures {
		if f.Split == "held_out" {
			fixtures = append(fixtures, f)
		}
	}
	if len(fixtures) != 60 {
		t.Fatal("the fixed held-out set must contain exactly 60 fixtures")
	}
	measurement := "Single proposed case per suite; one manual review call; machine-authored labels, not independently human-validated; no authoring or repair accuracy claim."
	if consistency {
		fixtures = liveConsistencyFixtures()
		data, err = json.Marshal(fixtures)
		if err != nil {
			t.Fatal(err)
		}
		corpus.Provenance = "Machine-authored six-case corrective controls, including one captured v1 false acceptance; not a new held-out accuracy sample or independent human validation."
		measurement = "Fixed v2 missing-only consistency regression controls; exactly one manual review per control, no repair or retry. Original held-out results remain unchanged."
	}
	ctx := context.Background()
	db, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		t.Fatal("connect to explicit local database")
	}
	defer db.Close()
	before, err := readLiveReviewBalance(ctx, db, wave.Campaign)
	if err != nil {
		t.Fatal("the existing shared campaign account is unavailable")
	}
	limits := cfg.Limits(true) // All synthetic sessions have no workspace.
	if version == vibe.LatestSuiteValidatorVersion {
		limits.OutputTokens = max(limits.OutputTokens, 4096)
	}
	maxCost, err := profile.BoundCost(min(limits.ContextTokens, profile.Context-limits.OutputTokens), limits.OutputTokens)
	if err != nil {
		t.Fatal(err)
	}
	if err := wave.canAdmit(before, maxCost, 1); err != nil {
		t.Fatal(err)
	}
	if before.TotalCalls-wave.BaselineCalls+before.RemainingCalls+int64(len(fixtures)) > wave.MaxCalls {
		t.Fatal("the complete fixed fixture set does not fit the remaining shared wave calls")
	}
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	defer rc.Close()
	store := vibe.NewStore(db, cfg)
	svc := &vibe.Service{Store: store, Config: cfg, Gate: vibe.Gate{Redis: rc}, Compiler: VibePackCompiler{}}
	profileJSON, _ := json.Marshal(profile)
	report := liveReviewReport{Provenance: corpus.Provenance, Measurement: measurement, StartedAt: time.Now().UTC(), Model: profile.ID, ProfileHash: vibe.Hash(profileJSON), FixtureHash: vibe.Hash(data), ValidatorVersion: version, Wave: wave, Before: before, After: before, Results: []liveReviewResult{}, Confusion: map[string]map[string]int{}}
	writeReport := func() {
		t.Helper()
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(reportPath+".tmp", append(encoded, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(reportPath+".tmp", reportPath); err != nil {
			t.Fatal(err)
		}
	}
	writeReport()
	for _, f := range fixtures {
		balance, err := readLiveReviewBalance(ctx, db, wave.Campaign)
		if err != nil {
			report.StopReason = "campaign accounting became unavailable"
			break
		}
		report.After = balance
		if err := wave.canAdmit(balance, maxCost, 1); err != nil {
			report.StopReason = err.Error()
			break
		}
		result, err := runLiveReviewFixture(ctx, svc, f, maxCost)
		report.Results = append(report.Results, result)
		if report.Confusion[f.Reference] == nil {
			report.Confusion[f.Reference] = map[string]int{}
		}
		report.Confusion[f.Reference][result.Observed]++
		report.LatencyMS = append(report.LatencyMS, result.LatencyMS)
		if result.CostNano != nil {
			report.TotalCostNano += *result.CostNano
		}
		if f.Critical && f.Reference != vibe.SuiteSupported && result.Committed {
			report.CriticalFalseAccepts++
		}
		if err != nil {
			report.StopReason = "fixture stopped: " + err.Error()
			break
		}
		writeReport()
		t.Logf("%s: proposed=%s observed=%s calls=%d latency_ms=%d", f.ID, f.Reference, result.Observed, result.ModelCalls, result.LatencyMS)
	}
	if after, err := readLiveReviewBalance(ctx, db, wave.Campaign); err == nil {
		report.After = after
	} else {
		report.StopReason = "final campaign accounting unavailable"
	}
	finished := time.Now().UTC()
	report.FinishedAt = &finished
	writeReport()
	t.Logf("Report: %s; recorded=%d/%d; known_cost_nano=%d; proposed-label critical false accepts=%d", reportPath, len(report.Results), len(fixtures), report.TotalCostNano, report.CriticalFalseAccepts)
	if report.StopReason != "" || len(report.Results) != len(fixtures) {
		t.Fatal("live review stopped; inspect the preserved report; do not automatically retry")
	}
}

func runLiveReviewFixture(ctx context.Context, svc *vibe.Service, f liveReviewFixture, admittedMax int64) (liveReviewResult, error) {
	result := liveReviewResult{FixtureID: f.ID, Domain: f.Domain, Reference: f.Reference, Critical: f.Critical, Observed: vibe.SuiteUnavailable}
	actor := "anon:live-review:" + uuid.NewString()
	session, err := svc.Store.CreateSession(ctx, actor, nil, uuid.New(), svc.Config.DefaultModels())
	if err != nil {
		return result, fmt.Errorf("create synthetic session")
	}
	result.SessionID = session.ID
	sourceID, artifactID := uuid.New(), uuid.New()
	policy := vibe.PolicySnapshot{ID: uuid.New(), ScopeID: artifactID, SourceMessageID: sourceID, Rules: []vibe.PolicyRule{{ID: "supplied-contract", Statement: f.Source, SourceBlockIDs: []string{sourceID.String()}}}}
	blueprint, err := svc.Compiler.Draft(vibe.DraftProposal{TestsOnly: true, Title: f.ID, Summary: "Held-out proposed reference case.", SuccessCriteria: f.Criteria, Scenarios: []vibe.TestScenario{{Input: f.Input, Expected: f.Expected}}}, svc.Config.Limits(session.Anonymous))
	if err != nil {
		return result, fmt.Errorf("fixture compilation failed: %s", f.ID)
	}
	if err := svc.Store.Edit(ctx, actor, session.ID, session.Revision, func(s *vibe.Session) error {
		s.Document.TestJourney = true
		s.Document.Messages = []vibe.Message{{ID: sourceID, Role: "user", Content: f.Source + "\n\n" + f.Request, CreatedAt: time.Now().UTC()}}
		s.Document.Policies = []vibe.PolicySnapshot{policy}
		s.Document.Artifacts = []vibe.Artifact{{ID: artifactID, Kind: "test_suite", Title: f.ID, Provenance: "ai_generated", PolicyID: &policy.ID, Blueprint: blueprint, SourceMessageID: sourceID, CreatedAt: time.Now().UTC()}}
		return nil
	}); err != nil {
		return result, fmt.Errorf("seed synthetic source and candidate")
	}
	session, err = svc.Store.GetSession(ctx, actor, session.ID)
	if err != nil {
		return result, fmt.Errorf("load synthetic candidate")
	}
	op, err := svc.PrepareSuiteEdit(ctx, actor, session.ID, session.Revision, artifactID, blueprint)
	if err != nil {
		return result, fmt.Errorf("manual review admission failed for %s", f.ID)
	}
	result.OperationID = op.ID
	var plan vibe.Plan
	if err := json.Unmarshal(op.Input, &plan); err != nil || plan.Calls != 1 || op.MaxCost != admittedMax || plan.Conversation == nil || plan.Conversation.Manual == nil || plan.Conversation.Policy == nil || plan.ExecutionLimits == nil {
		_ = svc.Store.Stop(ctx, actor, op.ID)
		return result, fmt.Errorf("admitted plan differed from the single-call wave reservation")
	}
	started := time.Now()
	// The live database already has its real outbox dispatcher and Temporal
	// worker. Let that worker own Start/Runner/Gateway/Finish; executing inline
	// here would race the dispatch lease and hide the actual application path.
	waitCtx, cancel := context.WithDeadline(ctx, op.Deadline.Add(time.Minute))
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for !op.State.Terminal() {
		select {
		case <-waitCtx.Done():
			return result, fmt.Errorf("the real worker did not finish before the operation deadline")
		case <-ticker.C:
			current, err := svc.Store.Operation(waitCtx, op.ID)
			if err != nil {
				return result, fmt.Errorf("the real worker receipt became unavailable")
			}
			op = current
		}
	}
	result.LatencyMS = time.Since(started).Milliseconds()
	result.ModelCalls, result.CostNano, result.Issue = op.ModelCalls, op.ActualCost, op.Error
	if op.Completion != nil && op.Completion.ArtifactID != nil {
		result.Committed = true
	}
	var output string
	if err := svc.Store.DB.QueryRow(ctx, "SELECT output FROM vibe_attempts WHERE operation_id=$1 AND step_key='review'", op.ID).Scan(&output); err == nil {
		input, buildErr := vibe.BuildSuiteReviewInput(blueprint, *plan.Conversation.Policy, plan.Conversation.Sources, plan.Conversation.CurrentRequest, 1, *plan.ExecutionLimits)
		if buildErr == nil {
			input.PreviousPolicy = plan.Conversation.Policy
			input.ValidatorVersion = plan.Conversation.ValidatorVersion
			if validation, parseErr := vibe.ParseSuiteReview([]byte(output), input, *plan.ExecutionLimits); parseErr == nil {
				result.Observed, result.Problems = validation.Status, validation.Problems
				result.ReviewCases, result.Consistency = validation.Cases, validation.Consistency
			}
		}
	}
	if result.ModelCalls != 1 || result.CostNano == nil || op.Billing == vibe.Reconciling {
		return result, fmt.Errorf("single-call accounting is incomplete; reservation remains protected")
	}
	return result, nil
}

func liveConsistencyFixtures() []liveReviewFixture {
	returns := "Only unopened items bought within 14 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund."
	visits := "The agent schedules service visits. Ask only for missing service address or appointment date."
	fixture := func(id, domain, source, input, expected, reference, reason string, critical bool) liveReviewFixture {
		return liveReviewFixture{ID: id, Domain: domain, Split: "corrective_controls", Source: source, Request: "Prepare one test using the supplied rules.", Input: input, Expected: expected, Criteria: source, Reference: reference, ReferenceReason: reason, Critical: critical}
	}
	return []liveReviewFixture{
		fixture("captured-returns-07", "returns", returns, "My item was bought 10 days ago; I have not said its condition.", "Ask for both purchase age and condition.", vibe.SuiteContradicted, "The actual expectation requests purchase age even though the input already supplies it.", true),
		fixture("condition-only", "returns", returns, "My item was bought 10 days ago; I have not said its condition.", "Ask only for item condition.", vibe.SuiteSupported, "Condition is missing and the expected request preserves the already supplied purchase age.", false),
		fixture("neither-known", "returns", returns, "I want to return an item.", "Ask for both purchase age and item condition.", vibe.SuiteSupported, "Neither required field is supplied.", false),
		fixture("negated-age-request", "returns", returns, "My item was bought 10 days ago; I have not said its condition.", "Do not ask for purchase age. Ask for item condition.", vibe.SuiteSupported, "The expectation explicitly avoids requesting the present age.", false),
		fixture("service-address-present", "scheduling", visits, "The service address is 10 Main Street. I have not provided an appointment date.", "Ask for both service address and appointment date.", vibe.SuiteContradicted, "The same missing-only invariant applies to an already supplied address in another domain.", true),
		fixture("ambiguous-purchase-age", "returns", returns, "My purchase age is either 10 days or 30 days; I have not said the item condition.", "Ask for purchase age and item condition.", vibe.SuiteUnclear, "The age has unresolved alternatives; this control checks conservative treatment of ambiguous fact mapping.", false),
	}
}

func TestVibeLiveReviewWaveGuard(t *testing.T) {
	w := liveReviewWave{StartedAt: time.Now(), Campaign: "shared-campaign", BaselineNano: 5 * vibe.NanoUSD, BaselineCalls: 100, MaxSpendNano: 2 * vibe.NanoUSD, MaxCalls: 80}
	base := liveReviewBalance{BalanceNano: 4 * vibe.NanoUSD, TotalCalls: 120}
	if err := w.canAdmit(base, vibe.NanoUSD/10, 1); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*liveReviewBalance){
		func(b *liveReviewBalance) { b.HeldNano = vibe.NanoUSD },
		func(b *liveReviewBalance) { b.TotalCalls = 180 },
		func(b *liveReviewBalance) { b.RemainingCalls = 60 },
		func(b *liveReviewBalance) { b.BalanceNano = 6 * vibe.NanoUSD },
		func(b *liveReviewBalance) { b.TotalCalls = 99 },
		func(b *liveReviewBalance) { b.Disabled = true },
		func(b *liveReviewBalance) { b.Active = 1 },
	} {
		bad := base
		mutate(&bad)
		if err := w.canAdmit(bad, vibe.NanoUSD/10, 1); err == nil {
			t.Fatalf("unsafe campaign state admitted: %+v", bad)
		}
	}
	if err := w.canAdmit(base, vibe.NanoUSD/10, 2); err == nil {
		t.Fatal("review reserved more than its single fixed call")
	}
}

func TestVibeLiveReviewHeldOutFixtureCompilation(t *testing.T) {
	data, err := os.ReadFile("../vibe/testdata/calibration/suite-validity.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Fixtures []liveReviewFixture `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	limits := (vibe.Config{LocalTesting: true}).Limits(true)
	count := 0
	for _, f := range corpus.Fixtures {
		if f.Split != "held_out" {
			continue
		}
		count++
		t.Run(f.ID, func(t *testing.T) {
			blueprint, err := (VibePackCompiler{}).Draft(vibe.DraftProposal{TestsOnly: true, Title: f.ID, SuccessCriteria: f.Criteria, Scenarios: []vibe.TestScenario{{Input: f.Input, Expected: f.Expected}}}, limits)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := (VibePackCompiler{}).Compile(blueprint, "deepseek/deepseek-v4-flash-0731", uuid.New(), limits); err != nil {
				t.Fatal(err)
			}
			id, requestID := uuid.New(), uuid.New()
			text := f.Source + "\n\n" + f.Request
			source := vibe.SourceBlock{ID: id.String(), MessageID: id, Text: text, Hash: vibe.Hash([]byte(text))}
			request := vibe.SourceBlock{ID: requestID.String(), MessageID: requestID, Text: "Check these manually edited expected answers. Preserve existing business rules."}
			request.Hash = vibe.Hash([]byte(request.Text))
			policy := vibe.PolicySnapshot{ID: uuid.New(), ScopeID: uuid.New(), SourceMessageID: id, Rules: []vibe.PolicyRule{{ID: "supplied-contract", Statement: f.Source, SourceBlockIDs: []string{source.ID}}}}
			if _, err := vibe.BuildSuiteReviewInput(blueprint, policy, []vibe.SourceBlock{source, request}, request, 1, limits); err != nil {
				t.Fatal(err)
			}
		})
	}
	if count != 60 {
		t.Fatalf("compiled %d held-out fixtures, expected 60", count)
	}
}

func TestVibeLiveReviewConsistencyFixtureCompilation(t *testing.T) {
	fixtures := liveConsistencyFixtures()
	if len(fixtures) != 6 {
		t.Fatal("the corrective controls must remain a fixed six-call set")
	}
	limits := (vibe.Config{LocalTesting: true}).Limits(true)
	limits.OutputTokens = max(limits.OutputTokens, 4096)
	for _, f := range fixtures {
		t.Run(f.ID, func(t *testing.T) {
			blueprint, err := (VibePackCompiler{}).Draft(vibe.DraftProposal{TestsOnly: true, Title: f.ID, SuccessCriteria: f.Criteria, Scenarios: []vibe.TestScenario{{Input: f.Input, Expected: f.Expected}}}, limits)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := (VibePackCompiler{}).Compile(blueprint, "deepseek/deepseek-v4-flash-0731", uuid.New(), limits); err != nil {
				t.Fatal(err)
			}
			id := uuid.New()
			source := vibe.SourceBlock{ID: id.String(), MessageID: id, Text: f.Source, Hash: vibe.Hash([]byte(f.Source))}
			policy := vibe.PolicySnapshot{ID: uuid.New(), ScopeID: uuid.New(), SourceMessageID: id, Rules: []vibe.PolicyRule{{ID: "supplied-contract", Statement: f.Source, SourceBlockIDs: []string{source.ID}}}}
			input, err := vibe.BuildSuiteReviewInput(blueprint, policy, []vibe.SourceBlock{source}, source, 1, limits)
			if err != nil {
				t.Fatal(err)
			}
			input.ValidatorVersion = vibe.LatestSuiteValidatorVersion
			if !vibe.RequiresConsistency(input) {
				t.Fatal("the corrective control does not exercise the v2 ledger")
			}
		})
	}
}
