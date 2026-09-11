package dbmigrate

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Stable, database-scoped application lock keys: ASCII ACLH / MIGR. Do not
// change these between releases; all application migrators must share them.
const lockNamespace, lockID int32 = 0x41434c48, 0x4d494752

type Config struct {
	DatabaseURL      string
	Directory        string
	LockTimeout      time.Duration
	StatementTimeout time.Duration
	Log              func(string)
}

// Run owns exactly one direct connection. Its session lock survives each
// per-migration transaction and is released by closing that connection on every
// exit path. Do not substitute a pool or transaction-pooling proxy here.
func Run(ctx context.Context, cfg Config) error {
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL must be set")
	}
	if cfg.LockTimeout < time.Millisecond || cfg.StatementTimeout < time.Millisecond {
		return errors.New("migration timeouts must be at least 1ms")
	}
	migrations, err := load(cfg.Directory)
	if err != nil {
		return err
	}
	connectionConfig, err := pgx.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid DATABASE_URL")
	}
	connectionConfig.ConnectTimeout = 10 * time.Second
	connectionConfig.RuntimeParams["application_name"] = "agentclash-migrator"
	connectionConfig.RuntimeParams["search_path"] = "public"
	connectionConfig.RuntimeParams["standard_conforming_strings"] = "on"
	connectionConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	conn, err := pgx.ConnectConfig(ctx, connectionConfig)
	if err != nil {
		return safeError("connect", err)
	}
	defer closeConnection(conn)
	log := cfg.Log
	if log == nil {
		log = func(string) {}
	}
	log("waiting for migration lock")
	lockCtx, cancel := context.WithTimeout(ctx, cfg.LockTimeout)
	_, err = conn.Exec(lockCtx, "SELECT pg_advisory_lock($1, $2)", lockNamespace, lockID)
	cancel()
	if err != nil {
		return safeError("acquire migration lock", err)
	}
	log("migration lock acquired")
	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return safeError("initialize migration ledger", err)
	}
	rows, err := conn.Query(ctx, "SELECT version FROM public.schema_migrations")
	if err != nil {
		return safeError("read migration ledger", err)
	}
	versions, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return safeError("read migration ledger", err)
	}
	applied := make(map[string]bool, len(versions))
	for _, version := range versions {
		applied[version] = true
	}
	for _, migration := range migrations {
		if applied[migration.version] {
			log("skipping " + migration.version + " (already applied)")
			continue
		}
		log("applying " + migration.version)
		if err := apply(ctx, conn, migration); err != nil {
			return err
		}
		log("applied " + migration.version)
	}
	log("all migrations applied")
	return nil
}

func apply(ctx context.Context, conn *pgx.Conn, migration migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return safeError("begin "+migration.version, err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	// No arguments: pgx uses the simple query protocol for the complete Up
	// section, including multi-statement functions and dollar-quoted bodies.
	if _, err := tx.Exec(ctx, migration.sql); err != nil {
		return safeError("apply "+migration.version, err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO public.schema_migrations (version) VALUES ($1)", migration.version); err != nil {
		return safeError("record "+migration.version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return safeError("commit "+migration.version+" (outcome unconfirmed; rerun the same image to check the ledger)", err)
	}
	return nil
}

func closeConnection(conn *pgx.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = conn.Close(ctx)
}

// Database errors can contain passwords, endpoints, SQL and row values. Never
// wrap or log their raw text; the stage and SQLSTATE are sufficient public logs.
func safeError(stage string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: timed out", stage)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s: canceled", stage)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return fmt.Errorf("%s: PostgreSQL error (SQLSTATE %s)", stage, pgErr.Code)
	}
	return fmt.Errorf("%s: database operation failed", stage)
}
