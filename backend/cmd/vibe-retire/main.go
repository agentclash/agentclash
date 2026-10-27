// vibe-retire inventories execution before explicitly retiring pre-V1 content.
// Dry run is the default. Run per deployment before upgrading old workers.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/agentclash/agentclash/backend/internal/temporalutil"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/api/workflowservice/v1"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "retirement failed:", err)
		os.Exit(1)
	}
}
func run() error {
	apply := flag.Bool("apply", false, "retire pre-V1 content after all execution is drained")
	host := flag.String("temporal", "127.0.0.1:7233", "Temporal host:port for this deployment")
	namespace := flag.String("namespace", "default", "Temporal namespace")
	flag.Parse()
	if os.Getenv("DATABASE_URL") == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("database unavailable")
	}
	defer db.Close()
	store := vibe.NewStore(db, vibe.Config{})
	inventory, err := store.RetirementInventory(ctx)
	if err != nil {
		return err
	}
	connection, err := temporalutil.LoadConnectionConfigFromEnv(os.Getenv("APP_ENV"))
	if err != nil {
		return fmt.Errorf("invalid Temporal connection: %w", err)
	}
	temporal, err := temporalutil.NewClient(*host, *namespace, connection)
	if err != nil {
		return fmt.Errorf("Temporal inventory unavailable: %w", err)
	}
	defer temporal.Close()
	workflows, err := temporal.CountWorkflow(ctx, &workflowservice.CountWorkflowExecutionsRequest{Namespace: *namespace, Query: "ExecutionStatus = 'Running' AND TaskQueue = 'vibe-evals'"})
	if err != nil {
		return err
	}
	report := struct {
		Inventory        vibe.RetirementInventory `json:"inventory"`
		RunningWorkflows int64                    `json:"running_vibe_workflows"`
		Apply            bool                     `json:"apply"`
	}{inventory, workflows.Count, *apply}
	if err = json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return err
	}
	if !*apply {
		return nil
	}
	if workflows.Count != 0 || inventory.PendingOperations != 0 || inventory.PendingOutbox != 0 {
		return fmt.Errorf("drain execution first; no content was removed")
	}
	retired, err := store.RetirePreV1(ctx)
	if err != nil {
		return err
	}
	// The normal input worker removes blobs marked deleted. This command does
	// not guess storage credentials or bypass pending physical file cleanup.
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"retired": retired, "cleanup": "eligible content scrubbed; unresolved billing/blob removal continues through the worker"})
}
