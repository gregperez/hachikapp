package docs_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenAPIContract(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	raw, err := os.ReadFile(filepath.Join(root, "docs", "swagger.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}

	basePath, _ := doc["basePath"].(string)
	if basePath != "/" && basePath != "" {
		t.Fatalf("basePath=%q want /", basePath)
	}

	secDefs, _ := doc["securityDefinitions"].(map[string]any)
	if _, ok := secDefs["BearerAuth"]; !ok {
		t.Fatalf("missing BearerAuth securityDefinitions: %#v", secDefs)
	}
	if _, ok := secDefs["ApiKeyAuth"]; ok {
		t.Fatal("ApiKeyAuth must not be present")
	}

	paths, _ := doc["paths"].(map[string]any)
	required := []string{
		"/v1/auth/login",
		"/v1/auth/refresh-token",
		"/v1/auth/logout",
		"/v1/auth/oidc/google/login",
		"/v1/auth/oidc/google/callback",
		"/v1/auth/oidc/google/exchange",
		"/v1/auth/oidc/google/link",
		"/v1/auth/oidc/google",
		"/v1/users",
		"/v1/users/{id}",
		"/v1/users/me",
		"/v1/roles",
		"/v1/roles/{id}",
		"/v1/roles/{id}/change-status",
	}
	for _, p := range required {
		if _, ok := paths[p]; !ok {
			t.Fatalf("missing path %s", p)
		}
	}

	publicNoAuth := map[string]bool{
		"post /v1/auth/login":                true,
		"post /v1/auth/refresh-token":        true,
		"post /v1/auth/logout":               true,
		"get /v1/auth/oidc/google/login":     true,
		"get /v1/auth/oidc/google/callback":  true,
		"post /v1/auth/oidc/google/exchange": true,
	}
	for path, methods := range paths {
		methodMap, _ := methods.(map[string]any)
		for method, opAny := range methodMap {
			op, _ := opAny.(map[string]any)
			key := strings.ToLower(method) + " " + path
			hasBearer := false
			if sec, ok := op["security"].([]any); ok {
				for _, s := range sec {
					m, _ := s.(map[string]any)
					if _, ok := m["BearerAuth"]; ok {
						hasBearer = true
					}
				}
			}
			if publicNoAuth[key] {
				if hasBearer {
					t.Fatalf("public route %s must not require BearerAuth", key)
				}
				continue
			}
			if strings.HasPrefix(path, "/v1/users") || strings.HasPrefix(path, "/v1/roles") ||
				key == "post /v1/auth/oidc/google/link" || key == "delete /v1/auth/oidc/google" {
				if !hasBearer {
					t.Fatalf("private route %s missing BearerAuth", key)
				}
			}
		}
	}

	defs, _ := doc["definitions"].(map[string]any)
	for name, defAny := range defs {
		def, _ := defAny.(map[string]any)
		props, _ := def["properties"].(map[string]any)
		if _, ok := props["password"]; !ok {
			continue
		}
		// password allowed only on request DTOs, never on response models.
		lname := strings.ToLower(name)
		if strings.Contains(lname, "response") || strings.Contains(lname, "userresponse") ||
			strings.Contains(lname, "pageresponse") || strings.Contains(lname, "authdto") {
			t.Fatalf("response schema %s exposes password", name)
		}
	}

	// RoleDto must not be empty.
	for name, defAny := range defs {
		if !strings.HasSuffix(name, "RoleDto") && !strings.Contains(name, "dto.RoleDto") {
			continue
		}
		def, _ := defAny.(map[string]any)
		props, _ := def["properties"].(map[string]any)
		if len(props) == 0 {
			t.Fatalf("RoleDto schema empty: %s", name)
		}
	}
}
