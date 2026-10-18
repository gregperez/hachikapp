package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"
)

func TestPermissionDto_UsesJSONTags(t *testing.T) {
	b, err := json.Marshal(dto.PermissionDto{
		Entity: "USERS",
		Create: true,
		Read:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["entity"]; !ok {
		t.Fatalf("expected json key entity, got %s", b)
	}
	if _, ok := m["create"]; !ok {
		t.Fatalf("expected json key create, got %s", b)
	}
}

func TestRoleDto_MarshalsName(t *testing.T) {
	b, err := json.Marshal(dto.RoleDto{Name: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "{}" || !json.Valid(b) {
		t.Fatalf("unexpected: %s", b)
	}
}
