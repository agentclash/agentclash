package dbmigrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func writeMigration(t *testing.T, dir, name, up string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".sql"), []byte("-- +goose Up\n"+up+"\n-- +goose Down\nSELECT 1 / 0;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFilenamesAndOrder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"00020_zeta", "00020_alpha", "00019_first"} {
		writeMigration(t, dir, name, "SELECT 1;")
	}
	migrations, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"00019_first", "00020_alpha", "00020_zeta"} {
		if migrations[i].version != want || strings.Contains(migrations[i].sql, "1 / 0") {
			t.Fatalf("unexpected migration at index %d", i)
		}
	}
}

func TestRepositoryMigrationSet(t *testing.T) {
	migrations, err := load("../../db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool)
	prefixes := make(map[string]int)
	for _, migration := range migrations {
		if names[migration.version] {
			t.Fatal("duplicate full basename")
		}
		names[migration.version] = true
		prefixes[migration.version[:5]]++
	}
	for _, prefix := range []string{"00020", "00021", "00036", "00041", "00069"} {
		if prefixes[prefix] < 2 {
			t.Errorf("historical duplicate prefix %s was lost", prefix)
		}
	}
}

func TestExtractUp(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		valid      bool
	}{
		{"normal", "-- +goose Up\nSELECT 1;\n-- +goose Down\nDROP TABLE anything;", true},
		{"crlf", "-- +goose Up\r\nSELECT 1;\r\n-- +goose Down\r\n", true},
		{"missing up", "SELECT 1;\n-- +goose Down", false},
		{"missing down", "-- +goose Up\nSELECT 1;", false},
		{"double up", "-- +goose Up\n-- +goose Up\n-- +goose Down", false},
		{"double down", "-- +goose Up\nSELECT 1;\n-- +goose Down\n-- +goose Down", false},
		{"reversed", "-- +goose Down\n-- +goose Up", false},
		{"no transaction", "-- +goose NO TRANSACTION\n-- +goose Up\nSELECT 1;\n-- +goose Down", false},
		{"empty", "-- +goose Up\n-- comment\n/* nested /* comment */ */;\n-- +goose Down", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractUp(tt.body)
			if (err == nil) != tt.valid {
				t.Fatalf("valid = %v, error = %v", tt.valid, err)
			}
		})
	}
}

func TestValidateSQL(t *testing.T) {
	for _, sql := range []string{
		"CREATE FUNCTION f() RETURNS void LANGUAGE plpgsql AS $body$ BEGIN PERFORM 1; END; $body$;",
		"SELECT 'commit; ''rollback', E'escaped \\' ; commit'; SELECT \"commit;\";",
		"SELECT $$ BEGIN; COMMIT; $$; -- COMMIT\n/* COMMIT; /* nested */ */ SELECT 1;",
		"SELECT column$with$dollars FROM data;",
	} {
		if err := validateSQL(sql); err != nil {
			t.Fatalf("valid SQL rejected: %v", err)
		}
	}
	for _, sql := range []string{
		"BEGIN; SELECT 1; COMMIT;", "SELECT 1; /* note */ cOmMiT;", "END WORK;",
		"SELECT 'safe'; ROLLBACK;", "START TRANSACTION;", "PREPARE TRANSACTION 'x';",
		"ABORT;", "SAVEPOINT x;", "RELEASE x;", "DISCARD ALL;", "\\connect elsewhere",
		"SELECT 'unterminated", "DO $tag$ BEGIN", "/* unterminated", "SELECT 1;\x00",
		"SELECT $$safe$$; COMMIT;", "SELECT E'escaped \\' '; COMMIT;",
	} {
		if err := validateSQL(sql); err == nil {
			t.Errorf("unsafe or malformed SQL accepted: %q", sql)
		}
	}
}

func TestLoadRejectsInvalidInputs(t *testing.T) {
	for _, kind := range []string{"missing", "empty", "filename", "symlink", "directory", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			switch kind {
			case "missing":
				dir = filepath.Join(dir, "private-path")
			case "filename":
				writeMigration(t, dir, "00001_private'path", "SELECT 1;")
			case "symlink":
				if err := os.Symlink("absent", filepath.Join(dir, "00001_link.sql")); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(filepath.Join(dir, "00001_directory.sql"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "malformed":
				writeMigration(t, dir, "00001_bad", "COMMIT;")
			}
			_, err := load(dir)
			if err == nil || strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), "private'path") {
				t.Fatalf("missing or unsanitized validation error: %v", err)
			}
		})
	}
}

func TestSafeErrors(t *testing.T) {
	for _, err := range []error{
		errors.New("private endpoint and password"),
		&pgconn.PgError{Code: "23505", Message: "private row", Detail: "private SQL", Where: "private path"},
		context.Canceled, context.DeadlineExceeded,
	} {
		got := safeError("apply 00001_example", err).Error()
		if strings.Contains(got, "private") || !strings.Contains(got, "apply 00001_example") {
			t.Fatal("error did not preserve safe stage information")
		}
	}
	dir := t.TempDir()
	writeMigration(t, dir, "00001_example", "SELECT 1;")
	for _, url := range []string{"", "://private-password"} {
		err := Run(context.Background(), Config{DatabaseURL: url, Directory: dir, LockTimeout: time.Second, StatementTimeout: time.Second})
		if err == nil || strings.Contains(err.Error(), "private-password") {
			t.Fatalf("missing or unsafe connection validation error: %v", err)
		}
	}
}
