package api

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestVibeSessionEntryOpenAPI(t *testing.T) {
	b, err := os.ReadFile("../../../docs/api-server/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	field := func(path ...string) any {
		t.Helper()
		var value any = doc
		for _, key := range path {
			object, ok := value.(map[string]any)
			if !ok {
				t.Fatalf("expected object before %q in %v", key, path)
			}
			value = object[key]
		}
		return value
	}
	t.Run("optional authentication", func(t *testing.T) {
		want := []any{map[string]any{}, map[string]any{"bearerJWT": []any{}}}
		if got := field("paths", "/v1/vibe/sessions", "post", "security"); !reflect.DeepEqual(got, want) {
			t.Fatalf("security=%v, want anonymous or bearer JWT", got)
		}
	})
	t.Run("trimmed title length", func(t *testing.T) {
		path := []string{"paths", "/v1/vibe/sessions", "post", "requestBody", "content", "application/json", "schema", "properties", "title"}
		if got := field(append(path, "maxLength")...); got != nil {
			t.Errorf("raw title maxLength=%v rejects valid padded titles", got)
		}
		description, _ := field(append(path, "description")...).(string)
		if !strings.Contains(description, "trimmed before the 80-character limit is checked") {
			t.Errorf("title description must explain the trimmed limit: %q", description)
		}
	})
}
