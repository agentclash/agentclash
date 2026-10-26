package vibe

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflowservice/v1"
)

// Availability describes server capability, never an actor's balance or access.
// Admission remains authoritative for capacity, quotas and workspace membership.
type Availability struct {
	Available  bool   `json:"available"`
	ReasonCode string `json:"reason_code,omitempty"`
}

type TaskQueueDescriber interface {
	DescribeTaskQueue(context.Context, string, enumspb.TaskQueueType) (*workflowservice.DescribeTaskQueueResponse, error)
}

// Readiness uses the same database, Redis and Temporal namespace as execution.
// No inference or provider metadata requests are made by these checks.
type Readiness struct {
	DB       *pgxpool.Pool
	Redis    *redis.Client
	Temporal TaskQueueDescriber
}

func (s *Service) Availability(ctx context.Context, cookieReady bool) map[string]Availability {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	base := s.Config.inferenceAvailability()
	if base.Available {
		if s.Readiness == nil {
			base = Availability{ReasonCode: "dependencies_unavailable"}
		} else {
			base = s.Readiness.inference(ctx, s.Config)
		}
	}
	guest := base
	if guest.Available && !cookieReady {
		guest = Availability{ReasonCode: "guest_not_configured"}
	}
	if guest.Available && !(s.Config.TestingLocally() && !s.Config.FreeOnly) && (s.Config.Campaign == "" || s.Config.AnonymousDaily <= 0 || s.Config.AnonymousCampaign <= 0) {
		guest = Availability{ReasonCode: "trial_capacity_reached"}
	}
	if guest.Available && !(s.Config.TestingLocally() && !s.Config.FreeOnly) {
		// Read only existing global pools: discovery never creates grants or returns
		// account balances. Zero-cost routes can still fit an exhausted active pool.
		var exhausted bool
		keys := []string{"subsidy:" + s.Config.Campaign, "subsidy:" + s.Config.Campaign + ":" + timestamp().Format("2006-01-02")}
		err := s.Readiness.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_accounts WHERE id=ANY($1::text[]) AND (disabled OR ($2 AND balance<=held)))`, keys, !s.Config.FreeOnly).Scan(&exhausted)
		if err != nil {
			guest = Availability{ReasonCode: "dependencies_unavailable"}
		} else if exhausted {
			guest = Availability{ReasonCode: "trial_capacity_reached"}
		}
	}
	return map[string]Availability{"guest": guest, "workspace": base}
}

func (c Config) inferenceAvailability() Availability {
	if !c.Enabled {
		return Availability{ReasonCode: "hosted_disabled"}
	}
	if c.Credential == "" {
		return Availability{ReasonCode: "provider_unavailable"}
	}
	// Config discovery must never trigger the optional local metadata verifier.
	c.LocalTesting = false
	if err := c.ValidateModels(c.DefaultModels(), true); err != nil {
		return Availability{ReasonCode: "pricing_unavailable"}
	}
	return Availability{Available: true}
}

// DependencyChecks excludes inference and worker pollers so a disabled provider
// cannot evict the API instance that serves history and accounting recovery.
func (r *Readiness) DependencyChecks(ctx context.Context) map[string]string {
	checks := map[string]string{"vibe_schema": "unavailable", "redis": "unavailable"}
	if r.DB != nil {
		// Resolve required columns as well as relations; a reachable but unmigrated
		// database is not ready. This works with both supported migration runners.
		rows, err := r.DB.Query(ctx, `SELECT s.id,o.state,b.delivered_at,a.reconciliation_evidence,c.balance,
   e.session_id,e.client_id,e.request_hash,e.evidence_id,e.artifact_id,
   sp.session_id,sp.client_id,sp.report_id,sp.source_case_id,sp.source_evidence_hash,sp.request_hash,sp.artifact,sp.created_at,
   sr.id,sr.session_id,sr.actor,sr.workspace_id,sr.artifact_id,sr.external_report_id,sr.request_hash,sr.source_json,sr.content_sha256,sr.body,sr.storage_bytes,sr.created_at,
   g.id,g.session_id,g.client_id,g.request_hash,g.result_kind,g.result_id,g.body,g.created_at,
   v.id,v.session_id,v.operation_id,v.stage_key,v.candidate_hash,v.policy_hash,v.body,v.created_at,
 s.archived_at,f.id,f.session_id,f.artifact_id,f.filename,f.media_type,f.content_sha256,f.content,
 p.draft_id,p.session_id,p.artifact_id,p.workspace_id,p.composition_hash,p.case_keys,p.original_content,p.original_filename,p.original_media_type,p.original_hash,p.advanced,
 k.draft_id,k.artifact_id,k.challenge_pack_version_id,k.case_key,k.input_set_key,k.challenge_key,k.published_case_id
   FROM vibe_sessions s,vibe_operations o,vibe_outbox b,vibe_attempts a,vibe_accounts c,
   vibe_sdk_promotions sp,vibe_sdk_reports sr,vibe_evidence_receipts e,vibe_grade_reviews g,vibe_validation_reviews v,vibe_import_originals f,vibe_pack_sources p,vibe_published_case_bindings k LIMIT 0`)
		if err == nil {
			rows.Close()
			if rows.Err() == nil {
				checks["vibe_schema"] = "ok"
			}
		}
	}
	if (Gate{Redis: r.Redis}).Healthy(ctx) == nil {
		checks["redis"] = "ok"
	}
	return checks
}

func (r *Readiness) inference(ctx context.Context, c Config) Availability {
	for _, value := range r.DependencyChecks(ctx) {
		if value != "ok" {
			return Availability{ReasonCode: "dependencies_unavailable"}
		}
	}
	if r.Temporal == nil {
		return Availability{ReasonCode: "worker_unavailable"}
	}
	for _, kind := range []enumspb.TaskQueueType{enumspb.TASK_QUEUE_TYPE_WORKFLOW, enumspb.TASK_QUEUE_TYPE_ACTIVITY} {
		desc, err := r.Temporal.DescribeTaskQueue(ctx, TaskQueue, kind)
		if err != nil || desc == nil {
			return Availability{ReasonCode: "worker_unavailable"}
		}
		active := false
		for _, p := range desc.Pollers {
			if p.LastAccessTime != nil && time.Since(p.LastAccessTime.AsTime()) < 2*time.Minute {
				active = true
			}
		}
		if !active {
			return Availability{ReasonCode: "worker_unavailable"}
		}
	}
	for _, component := range []string{"outbox", "accounting"} {
		value, err := r.Redis.Get(ctx, readinessKey(c, component)).Result()
		if err != nil || value != "ok" {
			return Availability{ReasonCode: component + "_unavailable"}
		}
	}
	return Availability{Available: true}
}

// Fingerprint prevents a worker with different admission/provider configuration
// from making this API advertise availability. No fingerprint is public.
func readinessKey(c Config, component string) string {
	c.Credential = Hash([]byte(c.Credential))
	b, _ := json.Marshal(c)
	return "vibe:ready:" + Hash(b) + ":" + component
}

func reportWorkerReady(ctx context.Context, rc *redis.Client, c Config, component string) {
	if rc == nil {
		return
	}
	// Expiring evidence, not an indefinite process-start claim. Each loop only
	// renews after its real database pass succeeds; worker shutdown ages it out.
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_ = rc.Set(ctx, readinessKey(c, component), "ok", 2*time.Minute).Err()
}

// uniqueReadinessKey avoids touching any rate/quota key during a Redis probe.
func uniqueReadinessKey() string { return "vibe:readiness:" + uuid.NewString() }
