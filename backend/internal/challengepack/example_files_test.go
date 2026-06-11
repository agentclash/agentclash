package challengepack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExampleChallengePacksParse(t *testing.T) {
	examplesDir := filepath.Join("..", "..", "..", "examples", "challenge-packs")
	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(examplesDir, entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read example: %v", err)
			}
			if _, err := ParseYAML(data); err != nil {
				t.Fatalf("ParseYAML returned error: %v", err)
			}
		})
	}
}
