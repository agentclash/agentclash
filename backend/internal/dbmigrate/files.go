// Package dbmigrate applies the application's filename-ledger migrations.
// It is deliberately separate from Temporal's schema tooling.
package dbmigrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type migration struct {
	version string
	sql     string
}

var migrationName = regexp.MustCompile(`^[0-9]{5}_[a-z0-9_]+\.sql$`)

// ReadDir sorts filenames lexically, independent of the caller's locale. Load
// the entire set before connecting so malformed later files cannot cause a
// partially applied run. Never convert the numeric prefix to a version number.
func load(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read MIGRATION_DIR")
	}
	var migrations []migration
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		if !migrationName.MatchString(entry.Name()) || !entry.Type().IsRegular() {
			return nil, fmt.Errorf("MIGRATION_DIR contains an invalid SQL filename or non-regular file")
		}
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("cannot read migration %s", entry.Name())
		}
		up, err := extractUp(string(body))
		if err != nil {
			return nil, fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		migrations = append(migrations, migration{strings.TrimSuffix(entry.Name(), ".sql"), up})
	}
	if len(migrations) == 0 {
		return nil, fmt.Errorf("MIGRATION_DIR contains no SQL migrations")
	}
	return migrations, nil
}

func extractUp(body string) (string, error) {
	var up strings.Builder
	section := 0
	for _, line := range strings.Split(body, "\n") {
		marker := strings.TrimSpace(line)
		switch marker {
		case "-- +goose Up":
			if section != 0 {
				return "", fmt.Errorf("duplicate or misplaced Up section")
			}
			section = 1
		case "-- +goose Down":
			if section != 1 {
				return "", fmt.Errorf("duplicate or misplaced Down section")
			}
			section = 2
		default:
			if strings.HasPrefix(marker, "-- +goose") {
				return "", fmt.Errorf("unsupported Goose directive")
			}
			if section == 1 {
				up.WriteString(line)
				up.WriteByte('\n')
			}
		}
	}
	if section != 2 {
		return "", fmt.Errorf("exactly one Up and one Down section are required")
	}
	if err := validateSQL(up.String()); err != nil {
		return "", err
	}
	return up.String(), nil
}
