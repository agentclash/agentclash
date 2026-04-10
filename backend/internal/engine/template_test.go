package engine

import (
	"reflect"
	"testing"
)

func TestResolveTemplateMap_SubstitutesParametersAndNestedPaths(t *testing.T) {
	resolved, err := resolveTemplateMap(map[string]any{
		"url":     "https://api.example.com/${customer.id}",
		"message": "Hello ${name}",
		"nested": map[string]any{
			"value": "${customer.tier}",
		},
	}, templateResolutionOptions{
		parameters: map[string]any{
			"name": "Ada",
			"customer": map[string]any{
				"id":   "cust-1",
				"tier": "enterprise",
			},
		},
		declaredParams:       map[string]struct{}{"name": {}, "customer": {}},
		errorOnMissingParams: true,
	})
	if err != nil {
		t.Fatalf("resolveTemplateMap returned error: %v", err)
	}

	if resolved["url"] != "https://api.example.com/cust-1" {
		t.Fatalf("url = %#v, want substituted value", resolved["url"])
	}
	if resolved["message"] != "Hello Ada" {
		t.Fatalf("message = %#v, want substituted value", resolved["message"])
	}
	nested, ok := resolved["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested = %T, want map[string]any", resolved["nested"])
	}
	if nested["value"] != "enterprise" {
		t.Fatalf("nested.value = %#v, want enterprise", nested["value"])
	}
}

func TestResolveTemplateMap_ResolvesSecrets(t *testing.T) {
	resolved, err := resolveTemplateMap(map[string]any{
		"headers": map[string]any{
			"Authorization": "Bearer ${secrets.API_KEY}",
		},
	}, templateResolutionOptions{
		secrets:              map[string]string{"API_KEY": "secret-token"},
		errorOnMissingSecret: true,
	})
	if err != nil {
		t.Fatalf("resolveTemplateMap returned error: %v", err)
	}

	headers := resolved["headers"].(map[string]any)
	if headers["Authorization"] != "Bearer secret-token" {
		t.Fatalf("authorization = %#v, want resolved secret", headers["Authorization"])
	}
}

func TestResolveTemplateMap_ExactParametersPlaceholderReplacesStructurally(t *testing.T) {
	resolved, err := resolveTemplateMap(map[string]any{
		"payload": "${parameters}",
	}, templateResolutionOptions{
		parameters: map[string]any{
			"sku": "WIDGET-42",
			"filters": map[string]any{
				"active": true,
			},
		},
		declaredParams: map[string]struct{}{"sku": {}, "filters": {}},
	})
	if err != nil {
		t.Fatalf("resolveTemplateMap returned error: %v", err)
	}

	payload, ok := resolved["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload = %T, want map[string]any", resolved["payload"])
	}
	if payload["sku"] != "WIDGET-42" {
		t.Fatalf("payload.sku = %#v, want WIDGET-42", payload["sku"])
	}
	filters := payload["filters"].(map[string]any)
	if filters["active"] != true {
		t.Fatalf("payload.filters.active = %#v, want true", filters["active"])
	}
}

func TestResolveTemplateMap_EmbeddedParametersPlaceholderSerializesJSON(t *testing.T) {
	resolved, err := resolveTemplateMap(map[string]any{
		"body": "payload=${parameters}",
	}, templateResolutionOptions{
		parameters: map[string]any{
			"sku": "WIDGET-42",
		},
		declaredParams: map[string]struct{}{"sku": {}},
	})
	if err != nil {
		t.Fatalf("resolveTemplateMap returned error: %v", err)
	}

	if resolved["body"] != `payload={"sku":"WIDGET-42"}` {
		t.Fatalf("body = %#v, want JSON-serialized parameters", resolved["body"])
	}
}

func TestResolveTemplateMap_LeavesUnknownPlaceholdersLiteral(t *testing.T) {
	resolved, err := resolveTemplateMap(map[string]any{
		"message": "Hello ${unknown}",
	}, templateResolutionOptions{
		parameters:     map[string]any{"name": "Ada"},
		declaredParams: map[string]struct{}{"name": {}},
	})
	if err != nil {
		t.Fatalf("resolveTemplateMap returned error: %v", err)
	}
	if resolved["message"] != "Hello ${unknown}" {
		t.Fatalf("message = %#v, want unresolved placeholder literal", resolved["message"])
	}
}

func TestResolveTemplateMap_ErrorsOnMissingDeclaredPath(t *testing.T) {
	_, err := resolveTemplateMap(map[string]any{
		"value": "${order.shipping.address}",
	}, templateResolutionOptions{
		parameters: map[string]any{
			"order": map[string]any{},
		},
		declaredParams:       map[string]struct{}{"order": {}},
		errorOnMissingParams: true,
	})
	if err == nil {
		t.Fatal("expected missing path error")
	}
	if got := err.Error(); got != `cannot resolve path "order.shipping.address": key "shipping" not found` {
		t.Fatalf("error = %q, want missing shipping path", got)
	}
}

func TestResolveTemplateMap_DoesNotRecursivelyResolveInjectedPlaceholderValues(t *testing.T) {
	resolved, err := resolveTemplateMap(map[string]any{
		"value": "${name}",
	}, templateResolutionOptions{
		parameters: map[string]any{
			"name": "${secrets.API_KEY}",
		},
		declaredParams:       map[string]struct{}{"name": {}},
		secrets:              map[string]string{"API_KEY": "secret-token"},
		errorOnMissingParams: true,
		errorOnMissingSecret: true,
	})
	if err != nil {
		t.Fatalf("resolveTemplateMap returned error: %v", err)
	}
	if resolved["value"] != "${secrets.API_KEY}" {
		t.Fatalf("value = %#v, want literal injected placeholder", resolved["value"])
	}
}

func TestValidateTemplateReferences_RejectsUnknownPlaceholder(t *testing.T) {
	err := validateTemplateReferences(map[string]any{
		"url": "https://api.example.com/${unknown}",
	}, "args", map[string]struct{}{"sku": {}})
	if err == nil {
		t.Fatal("expected unknown placeholder error")
	}
	if got := err.Error(); got != `unknown placeholder at args.url: "${unknown}"` {
		t.Fatalf("error = %q, want unknown placeholder", got)
	}
}

func TestCloneTemplateValue_ClonesMapsAndSlices(t *testing.T) {
	original := map[string]any{
		"items": []any{
			map[string]any{"name": "Ada"},
		},
	}

	cloned := cloneTemplateValue(original).(map[string]any)
	items := cloned["items"].([]any)
	items[0].(map[string]any)["name"] = "Grace"

	if reflect.DeepEqual(original, cloned) {
		t.Fatal("expected clone mutation not to affect original")
	}
	if original["items"].([]any)[0].(map[string]any)["name"] != "Ada" {
		t.Fatalf("original mutated unexpectedly: %#v", original)
	}
}
