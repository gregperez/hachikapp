package bootstrap_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/infra/bootstrap"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/appconfig"
)

func TestAccessLogEmitsRequestAttrs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := appconfig.Defaults()
	cfg.Database.Password = "secret-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.Runtime.Environment = "development"
	cfg.CORS.AllowedOrigins = []string{"http://localhost:3000"}
	cfg.Docs.SwaggerEnabled = false

	r := bootstrap.NewEngine(bootstrap.HTTPDeps{
		Config: cfg,
		Gate:   bootstrap.NewReadyGate(nil),
		Logger: lg,
		Register: func(eng *gin.Engine) {
			eng.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
		},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-ID", "rid-access")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}

	var found map[string]any
	dec := json.NewDecoder(&buf)
	for dec.More() {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			t.Fatal(err)
		}
		if m["msg"] == "http" {
			found = m
			break
		}
	}
	if found == nil {
		t.Fatalf("no access log line in %s", buf.String())
	}
	if found["request_id"] != "rid-access" {
		t.Fatalf("request_id=%v", found["request_id"])
	}
	if found["method"] != "GET" || found["path"] != "/ping" {
		t.Fatalf("method/path=%v/%v", found["method"], found["path"])
	}
	if found["status"] != float64(200) {
		t.Fatalf("status=%v", found["status"])
	}
}
