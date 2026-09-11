// db-migrate is a one-off application schema job, never an API startup hook.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentclash/agentclash/backend/internal/dbmigrate"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Getenv, os.Stderr))
}

func run(ctx context.Context, args []string, getenv func(string) string, output io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(output, "Usage: db-migrate (no arguments)\nEnvironment: DATABASE_URL, MIGRATION_DIR (default /migrations), MIGRATION_LOCK_TIMEOUT (60s), MIGRATION_STATEMENT_TIMEOUT (10m), MIGRATION_TIMEOUT (30m).\nApply application Up migrations only; credentials must not be passed as arguments.")
		return 0
	}
	if len(args) != 0 {
		fmt.Fprintln(output, "[migrate] arguments are unsupported; use --help")
		return 2
	}
	durations := []time.Duration{time.Minute, 10 * time.Minute, 30 * time.Minute}
	for i, name := range []string{"MIGRATION_LOCK_TIMEOUT", "MIGRATION_STATEMENT_TIMEOUT", "MIGRATION_TIMEOUT"} {
		if raw := getenv(name); raw != "" {
			value, err := time.ParseDuration(raw)
			if err != nil || value < time.Millisecond {
				fmt.Fprintf(output, "[migrate] %s must be a duration of at least 1ms\n", name)
				return 2
			}
			durations[i] = value
		}
	}
	dir := getenv("MIGRATION_DIR")
	if dir == "" {
		dir = "/migrations"
	}
	ctx, cancel := context.WithTimeout(ctx, durations[2])
	defer cancel()
	err := dbmigrate.Run(ctx, dbmigrate.Config{
		DatabaseURL: getenv("DATABASE_URL"), Directory: dir,
		LockTimeout: durations[0], StatementTimeout: durations[1],
		Log: func(message string) { fmt.Fprintln(output, "[migrate] "+message) },
	})
	if err != nil {
		fmt.Fprintln(output, "[migrate] "+err.Error())
		return 1
	}
	return 0
}
