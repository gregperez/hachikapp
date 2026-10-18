package audit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	"github.com/gregperez/hachikapp-backend/internal/infra/audit"
)

func TestSecuritySinkOmitsSecrets(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	sink, err := audit.NewSecuritySink(slog.New(slog.NewJSONHandler(&buf, nil)))
	if err != nil {
		t.Fatal(err)
	}
	sink.Record(context.Background(), authBaseCore.SecurityEvent{
		Type:     authBaseCore.SecurityLoginSuccess,
		ActorID:  "user-1",
		FamilyID: "fam-1",
		Outcome:  "success",
	})
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	if strings.Contains(body, "eyJ") || strings.Contains(strings.ToLower(body), "password") {
		t.Fatalf("leaked material: %s", body)
	}
	if m["type"] != "login_success" || m["actor_id"] != "user-1" {
		t.Fatalf("event=%v", m)
	}
}

func TestFanoutOIDCMapsTypedEvents(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	sink, err := audit.NewSecuritySink(slog.New(slog.NewJSONHandler(&buf, nil)))
	if err != nil {
		t.Fatal(err)
	}
	audit.FanoutOIDC{Sink: sink}.OIDCEvent("success", "google", "user-2", "success")
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["type"] != "oidc_success" || m["provider"] != "google" || m["actor_id"] != "user-2" {
		t.Fatalf("event=%v", m)
	}
}
