//go:build migrationintegration

package dbmigrate

import (
	"context"
	"crypto/rand"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Use scripts/db/test-migrator.py. This suite intentionally refuses a missing
// URL or any endpoint other than the disposable local control database.
func databaseConfig(t *testing.T) (Config, *pgx.Conn) {
	t.Helper()
	databaseURL := os.Getenv("MIGRATOR_TEST_DATABASE_URL")
	parsed, err := pgx.ParseConfig(databaseURL)
	if err != nil || databaseURL == "" || parsed.Database != "migrator_test_control" || parsed.Host != "127.0.0.1" {
		t.Fatal("run this suite through scripts/db/test-migrator.py using disposable PostgreSQL")
	}
	ctx := t.Context()
	admin, err := pgx.ConnectConfig(ctx, parsed)
	if err != nil {
		t.Fatal(safeError("connect test controller", err))
	}
	name := "migrator_test_" + strings.ToLower(rand.Text()[:16])
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		closeConnection(admin)
		t.Fatal(err)
	}
	parsed.Database = name
	conn, err := pgx.ConnectConfig(ctx, parsed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeConnection(conn)
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = admin.Exec(cleanup, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		closeConnection(admin)
	})
	// pgx.ConnString retains the original parsed URL after changing Database.
	// Update the URL explicitly; do not format the credentials in diagnostics.
	target, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal("invalid test configuration")
	}
	target.Path = "/" + name
	return Config{DatabaseURL: target.String(), Directory: "../../db/migrations", LockTimeout: 5 * time.Second, StatementTimeout: 10 * time.Second}, conn
}

func execTest(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func ledger(t *testing.T, conn *pgx.Conn) map[string]time.Time {
	t.Helper()
	rows, err := conn.Query(t.Context(), "SELECT version, applied_at FROM public.schema_migrations")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := make(map[string]time.Time)
	for rows.Next() {
		var name string
		var timestamp time.Time
		if err := rows.Scan(&name, &timestamp); err != nil {
			t.Fatal(err)
		}
		result[name] = timestamp
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertBool(t *testing.T, conn *pgx.Conn, sql string, want bool) {
	t.Helper()
	var got bool
	if err := conn.QueryRow(t.Context(), sql).Scan(&got); err != nil || got != want {
		t.Fatalf("database assertion: got=%v want=%v err=%v", got, want, err)
	}
}

func waitFor(t *testing.T, conn *pgx.Conn, sql string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for {
		var ready bool
		if err := conn.QueryRow(ctx, sql).Scan(&ready); err != nil {
			t.Fatal(err)
		}
		if ready {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for database state")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func startRun(t *testing.T, cfg Config) (<-chan error, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	result := make(chan error, 1)
	go func() { result <- Run(ctx, cfg) }()
	return result, cancel
}

func resultOf(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("migrator failed to finish")
		return nil
	}
}

func TestIntegrationFreshAndRepeat(t *testing.T) {
	cfg, conn := databaseConfig(t)
	if err := Run(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	before := ledger(t, conn)
	migrations, err := load(cfg.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(migrations) {
		t.Fatalf("ledger has %d versions, expected %d", len(before), len(migrations))
	}
	for _, migration := range migrations {
		if _, ok := before[migration.version]; !ok {
			t.Fatalf("missing full filename %s", migration.version)
		}
	}
	assertBool(t, conn, "SELECT to_regclass('public.runs') IS NOT NULL AND to_regclass('public.workspaces') IS NOT NULL", true)
	if err := Run(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, ledger(t, conn)) {
		t.Fatal("repeat changed existing ledger entries or timestamps")
	}
	t.Logf("applied and repeated %d real filename migrations", len(migrations))
}

func TestIntegrationExistingLedger(t *testing.T) {
	cfg, conn := databaseConfig(t)
	cfg.Directory = t.TempDir()
	writeMigration(t, cfg.Directory, "00020_existing", "SELECT 1 / 0;")
	writeMigration(t, cfg.Directory, "00020_new", "CREATE TABLE new_table(id integer);")
	execTest(t, conn, "CREATE TABLE schema_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now()); INSERT INTO schema_migrations VALUES ('00020_existing', '2020-01-01T00:00:00Z');")
	before := ledger(t, conn)
	if err := Run(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	after := ledger(t, conn)
	if len(after) != 2 || !before["00020_existing"].Equal(after["00020_existing"]) {
		t.Fatal("existing filename ledger was not preserved")
	}
}

func TestIntegrationConcurrentSessionLock(t *testing.T) {
	cfg, conn := databaseConfig(t)
	cfg.Directory = t.TempDir()
	writeMigration(t, cfg.Directory, "00001_first", "CREATE TABLE sessions(pid integer); INSERT INTO sessions VALUES (pg_backend_pid());")
	writeMigration(t, cfg.Directory, "00002_second", "SELECT pg_advisory_xact_lock(1, 2); INSERT INTO sessions VALUES (pg_backend_pid());")
	execTest(t, conn, "SELECT pg_advisory_lock(1, 2)")
	first, _ := startRun(t, cfg)
	waitFor(t, conn, "SELECT EXISTS (SELECT 1 FROM pg_locks WHERE locktype='advisory' AND classid=1 AND objid=2 AND NOT granted)")
	// The first migration committed; the next migration is blocked in the same
	// session. The advisory lock must still protect the complete migration run.
	assertBool(t, conn, "SELECT count(*)=1 FROM schema_migrations", true)
	second, _ := startRun(t, cfg)
	waitFor(t, conn, "SELECT count(*)=2 AND count(*) FILTER (WHERE granted)=1 FROM pg_locks WHERE locktype='advisory' AND classid=1094929480 AND objid=1296648018")
	assertBool(t, conn, "SELECT EXISTS (SELECT 1 FROM pg_locks JOIN sessions USING(pid) WHERE locktype='advisory' AND classid=1094929480 AND objid=1296648018 AND granted)", true)
	execTest(t, conn, "SELECT pg_advisory_unlock(1, 2)")
	for _, result := range []<-chan error{first, second} {
		if err := resultOf(t, result); err != nil {
			t.Fatal(err)
		}
	}
	assertBool(t, conn, "SELECT count(*)=2 AND count(DISTINCT pid)=1 FROM sessions", true)
	assertBool(t, conn, "SELECT count(*)=2 FROM schema_migrations", true)
}

func TestIntegrationFailureAndRetry(t *testing.T) {
	for _, failure := range []string{"body", "ledger"} {
		t.Run(failure, func(t *testing.T) {
			cfg, conn := databaseConfig(t)
			cfg.Directory = t.TempDir()
			writeMigration(t, cfg.Directory, "00001_first", "CREATE TABLE example(id integer);")
			body := "ALTER TABLE example ADD COLUMN marker text;"
			if failure == "body" {
				body += " SELECT 1 / 0;"
			} else {
				execTest(t, conn, "CREATE TABLE schema_migrations(version text PRIMARY KEY CHECK (version <> '00002_failing'), applied_at timestamptz NOT NULL DEFAULT now())")
			}
			writeMigration(t, cfg.Directory, "00002_failing", body)
			writeMigration(t, cfg.Directory, "00003_later", "CREATE TABLE later(id integer);")
			err := Run(t.Context(), cfg)
			if err == nil || !strings.Contains(err.Error(), "00002_failing") {
				t.Fatalf("expected failing migration: %v", err)
			}
			assertBool(t, conn, "SELECT count(*)=1 FROM schema_migrations WHERE version='00001_first'", true)
			assertBool(t, conn, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='example' AND column_name='marker')", false)
			assertBool(t, conn, "SELECT to_regclass('public.later') IS NOT NULL", false)
			if len(ledger(t, conn)) != 1 {
				t.Fatal("failing or later migration was recorded")
			}
			if failure == "ledger" {
				execTest(t, conn, "ALTER TABLE schema_migrations DROP CONSTRAINT schema_migrations_version_check")
			}
			writeMigration(t, cfg.Directory, "00002_failing", "ALTER TABLE example ADD COLUMN marker text;")
			if err := Run(t.Context(), cfg); err != nil {
				t.Fatal(err)
			}
			assertBool(t, conn, "SELECT count(*)=3 FROM schema_migrations", true)
		})
	}
}

func TestIntegrationLockTimeout(t *testing.T) {
	cfg, conn := databaseConfig(t)
	cfg.Directory = t.TempDir()
	writeMigration(t, cfg.Directory, "00001_first", "CREATE TABLE example(id integer);")
	cfg.LockTimeout = 80 * time.Millisecond
	execTest(t, conn, "SELECT pg_advisory_lock($1, $2)", lockNamespace, lockID)
	if err := Run(t.Context(), cfg); err == nil || !strings.Contains(err.Error(), "acquire migration lock") {
		t.Fatalf("expected lock timeout: %v", err)
	}
	assertBool(t, conn, "SELECT to_regclass('public.schema_migrations') IS NOT NULL", false)
	execTest(t, conn, "SELECT pg_advisory_unlock($1, $2)", lockNamespace, lockID)
	if err := Run(t.Context(), cfg); err != nil {
		t.Fatalf("lock was not released after timeout: %v", err)
	}
}

func TestIntegrationInterruptedMigration(t *testing.T) {
	for _, failure := range []string{"statement timeout", "cancellation", "connection lost"} {
		t.Run(failure, func(t *testing.T) {
			cfg, conn := databaseConfig(t)
			cfg.Directory = t.TempDir()
			writeMigration(t, cfg.Directory, "00001_slow", "CREATE TABLE example(id integer); SELECT pg_sleep(5);")
			if failure == "statement timeout" {
				cfg.StatementTimeout = 80 * time.Millisecond
			}
			result, cancel := startRun(t, cfg)
			if failure != "statement timeout" {
				waitFor(t, conn, "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND application_name='agentclash-migrator' AND wait_event='PgSleep')")
				if failure == "cancellation" {
					cancel()
				} else {
					execTest(t, conn, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=current_database() AND application_name='agentclash-migrator'")
				}
			}
			if err := resultOf(t, result); err == nil {
				t.Fatal("interrupted migration succeeded")
			}
			assertBool(t, conn, "SELECT to_regclass('public.example') IS NOT NULL", false)
			if len(ledger(t, conn)) != 0 {
				t.Fatal("interrupted migration was recorded")
			}
			writeMigration(t, cfg.Directory, "00001_slow", "CREATE TABLE example(id integer);")
			if err := Run(t.Context(), cfg); err != nil {
				t.Fatalf("retry after interruption failed: %v", err)
			}
		})
	}
}

func TestIntegrationPreflightBeforeWrites(t *testing.T) {
	cfg, conn := databaseConfig(t)
	cfg.Directory = t.TempDir()
	writeMigration(t, cfg.Directory, "00001_first", "CREATE TABLE example(id integer);")
	writeMigration(t, cfg.Directory, "00002_invalid", "COMMIT;")
	if err := Run(t.Context(), cfg); err == nil {
		t.Fatal("malformed input was accepted")
	}
	assertBool(t, conn, "SELECT to_regclass('public.schema_migrations') IS NOT NULL OR to_regclass('public.example') IS NOT NULL", false)
}
