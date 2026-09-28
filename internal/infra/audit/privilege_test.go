package audit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/WilsonSayago/base-golang/internal/infra/audit"
)

func TestPrivilegeChangeEmitsAttrs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, nil))
	auditor, err := audit.NewLogger(lg)
	if err != nil {
		t.Fatal(err)
	}
	flag := true
	auditor.PrivilegeChange(context.Background(), "set_admin_flag", "actor-1", "subject-2", []string{"r1"}, &flag)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["msg"] != "privilege change" {
		t.Fatalf("msg=%v", m["msg"])
	}
	if m["action"] != "set_admin_flag" || m["actor_id"] != "actor-1" || m["subject_id"] != "subject-2" {
		t.Fatalf("ids=%v", m)
	}
	if m["is_admin"] != true {
		t.Fatalf("is_admin=%v", m["is_admin"])
	}
}
