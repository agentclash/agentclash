package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCommandConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		env  map[string]string
		code int
		want string
	}{
		{"help", []string{"--help"}, nil, 0, "DATABASE_URL"},
		{"no URL", nil, nil, 1, "DATABASE_URL must be set"},
		{"arguments", []string{"private-value"}, nil, 2, "arguments are unsupported"},
		{"bad lock", nil, map[string]string{"MIGRATION_LOCK_TIMEOUT": "private-value"}, 2, "MIGRATION_LOCK_TIMEOUT"},
		{"zero statement", nil, map[string]string{"MIGRATION_STATEMENT_TIMEOUT": "0s"}, 2, "MIGRATION_STATEMENT_TIMEOUT"},
		{"negative total", nil, map[string]string{"MIGRATION_TIMEOUT": "-1s"}, 2, "MIGRATION_TIMEOUT"},
		{"tiny statement", nil, map[string]string{"MIGRATION_STATEMENT_TIMEOUT": "1ns"}, 2, "MIGRATION_STATEMENT_TIMEOUT"},
		{"private directory", nil, map[string]string{"DATABASE_URL": "private-value", "MIGRATION_DIR": "/absent/private-value"}, 1, "cannot read MIGRATION_DIR"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			code := run(context.Background(), tt.args, func(key string) string { return tt.env[key] }, &output)
			if code != tt.code || !strings.Contains(output.String(), tt.want) || strings.Contains(output.String(), "private-value") {
				t.Fatalf("unexpected exit/output: code=%d output=%s", code, output.String())
			}
		})
	}
}
