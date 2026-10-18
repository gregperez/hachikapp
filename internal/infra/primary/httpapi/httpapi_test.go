package httpapi_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	userout "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/outputs/users"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gin-gonic/gin"
)

func TestBindJSON_Table(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type req struct {
		Email string `json:"email" binding:"required,email"`
		Name  string `json:"name" binding:"required"`
	}

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "empty", body: "", wantStatus: 400, wantCode: httpapi.CodeBadRequest},
		{name: "truncated", body: `{"email":`, wantStatus: 400, wantCode: httpapi.CodeBadRequest},
		{name: "wrong type", body: `{"email":1,"name":"a"}`, wantStatus: 400, wantCode: httpapi.CodeBadRequest},
		{name: "unknown field", body: `{"email":"a@b.co","name":"a","isAdmin":true}`, wantStatus: 400, wantCode: httpapi.CodeBadRequest},
		{name: "validation", body: `{"email":"bad","name":""}`, wantStatus: 422, wantCode: httpapi.CodeValidation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set("request_id", "test-rid")

			var dst req
			ok := httpapi.BindAndWrite(c, &dst)
			if ok {
				t.Fatalf("expected bind failure")
			}
			if w.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			var p httpapi.Problem
			if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if p.Code != tt.wantCode {
				t.Fatalf("code=%s want=%s", p.Code, tt.wantCode)
			}
			if p.RequestID != "test-rid" {
				t.Fatalf("requestId=%q", p.RequestID)
			}
		})
	}
}

func TestBindJSON_BodyTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	big := `{"email":"a@b.co","name":"` + strings.Repeat("x", 2048) + `"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	c.Request.Header.Set("Content-Type", "application/json")

	var dst struct {
		Email string `json:"email" binding:"required,email"`
		Name  string `json:"name" binding:"required"`
	}
	err := httpapi.BindJSONLimited(c, &dst, 64)
	if !errors.Is(err, httpapi.ErrBodyTooLarge) {
		t.Fatalf("err=%v want ErrBodyTooLarge", err)
	}
	p := httpapi.MapError(err, "rid")
	if p.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", p.Status)
	}
}

func TestMapError_Table(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrForbidden, 403, httpapi.CodeForbidden},
		{domain.ErrNotFound, 404, httpapi.CodeNotFound},
		{authBaseCore.ErrNotFound, 404, httpapi.CodeNotFound},
		{domain.ErrLastAdmin, 409, httpapi.CodeConflict},
		{domain.ErrConflict, 409, httpapi.CodeConflict},
		{domain.ErrStaleVersion, 409, httpapi.CodeConflict},
		{domain.ErrAdminExists, 409, httpapi.CodeConflict},
		{domain.ErrWeakPassword, 422, httpapi.CodeValidation},
		{domain.ErrInvalidSettings, 422, httpapi.CodeValidation},
		{authBaseCore.ErrInvalidCredentials, 401, httpapi.CodeUnauthorized},
		{authBaseCore.ErrInvalidRefresh, 401, httpapi.CodeUnauthorized},
		{httpapi.ErrInvalidJSON, 400, httpapi.CodeBadRequest},
		{errors.New("sql: connection refused"), 500, httpapi.CodeInternal},
	}
	for _, tt := range tests {
		p := httpapi.MapError(tt.err, "rid")
		if p.Status != tt.status || p.Code != tt.code {
			t.Fatalf("err=%v -> %d/%s want %d/%s detail=%q", tt.err, p.Status, p.Code, tt.status, tt.code, p.Detail)
		}
		if tt.err == domain.ErrWeakPassword && strings.Contains(strings.ToLower(p.Detail), "complexity") {
			t.Fatalf("weak password message still mentions complexity: %q", p.Detail)
		}
		if tt.status == 500 && (strings.Contains(strings.ToLower(p.Detail), "sql") || strings.Contains(p.Detail, "connection")) {
			t.Fatalf("500 leaked internals: %q", p.Detail)
		}
	}
}

func TestCursorPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	key := authBaseDomain.CursorKey{
		CreatedAt: time.Date(2026, time.September, 24, 12, 34, 56, 123, time.UTC),
		ID:        "00000000-0000-0000-0000-000000000001",
	}
	cursor, err := httpapi.EncodeCursor(key)
	if err != nil {
		t.Fatalf("EncodeCursor: %v", err)
	}

	okCases := []struct {
		query string
		limit int
		after bool
	}{
		{"", 10, false},
		{"limit=25", 25, false},
		{"limit=100&cursor=" + cursor, 100, true},
	}
	for _, tt := range okCases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil)
		p, err := httpapi.ParseCursorPagination(c)
		if err != nil {
			t.Fatalf("query=%q err=%v", tt.query, err)
		}
		if p.Limit != tt.limit || (p.After != nil) != tt.after {
			t.Fatalf("query=%q got %+v", tt.query, p)
		}
		if tt.after && (p.After.ID != key.ID || !p.After.CreatedAt.Equal(key.CreatedAt)) {
			t.Fatalf("query=%q cursor=%+v", tt.query, p.After)
		}
	}

	badPayload := base64.RawURLEncoding.EncodeToString([]byte(
		`{"v":1,"createdAt":"2026-09-24T12:34:56Z","id":"bad"}`,
	))
	unknownVersion := base64.RawURLEncoding.EncodeToString([]byte(
		`{"v":2,"createdAt":"2026-09-24T12:34:56Z","id":"00000000-0000-0000-0000-000000000001"}`,
	))
	invalidTimestamp := base64.RawURLEncoding.EncodeToString([]byte(
		`{"v":1,"createdAt":"not-a-time","id":"00000000-0000-0000-0000-000000000001"}`,
	))
	unknownField := base64.RawURLEncoding.EncodeToString([]byte(
		`{"v":1,"createdAt":"2026-09-24T12:34:56Z","id":"00000000-0000-0000-0000-000000000001","extra":true}`,
	))
	bad := []string{
		"limit=0",
		"limit=-1",
		"limit=101",
		"limit=abc",
		"cursor=not-base64!",
		"cursor=" + badPayload,
		"cursor=" + unknownVersion,
		"cursor=" + invalidTimestamp,
		"cursor=" + unknownField,
		"cursor=" + strings.Repeat("a", 1025),
	}
	for _, q := range bad {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/?"+q, nil)
		if _, err := httpapi.ParseCursorPagination(c); err == nil {
			t.Fatalf("query=%q expected error", q)
		}
	}
}

func TestUserResponseNeverSerializesPassword(t *testing.T) {
	u := userout.UserResponse{
		Name:  "Alice",
		Email: "a@b.co",
	}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bytes.ToLower(b), []byte("password")) {
		t.Fatalf("password key present: %s", b)
	}
	if bytes.Contains(bytes.ToLower(b), []byte("hash")) {
		t.Fatalf("hash key present: %s", b)
	}
}

func TestRecovery_ReturnsProblemJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("request_id", "rid-1"); c.Next() })
	r.Use(httpapi.Recovery(lg))
	r.GET("/boom", func(c *gin.Context) { panic("boom") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != 500 {
		t.Fatalf("status=%d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, httpapi.ContentTypeProblemJSON) {
		t.Fatalf("content-type=%q", ct)
	}
	var p httpapi.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Code != httpapi.CodeInternal || strings.Contains(p.Detail, "boom") {
		t.Fatalf("problem=%+v", p)
	}

	var logLine map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logLine); err != nil {
		t.Fatalf("log json: %v body=%s", err, buf.String())
	}
	if logLine["msg"] != "http panic recovered" {
		t.Fatalf("msg=%v", logLine["msg"])
	}
	if logLine["request_id"] != "rid-1" {
		t.Fatalf("request_id=%v", logLine["request_id"])
	}
	if logLine["path"] != "/boom" || logLine["method"] != "GET" {
		t.Fatalf("path/method=%v/%v", logLine["path"], logLine["method"])
	}
	if _, ok := logLine["stack"]; !ok {
		t.Fatal("expected stack attr")
	}
	if strings.Contains(strings.ToLower(p.Detail), "stack") {
		t.Fatalf("stack leaked to client: %q", p.Detail)
	}
}

func TestPermissionDtoJSONTags(t *testing.T) {
	// Ensure primary PermissionDto marshals with json tags (regression for bson).
	type probe struct {
		Entity string `json:"entity,omitempty"`
		Create bool   `json:"create,omitempty"`
	}
	b, _ := json.Marshal(probe{Entity: "USERS", Create: true})
	if !bytes.Contains(b, []byte(`"entity"`)) {
		t.Fatalf("unexpected: %s", b)
	}
}

func TestSessionCursorRoundTrip(t *testing.T) {
	lastSeen := time.Date(2026, time.September, 28, 18, 0, 0, 0, time.UTC)
	encoded, err := httpapi.EncodeSessionCursor(lastSeen, "family-base64url_id")
	if err != nil {
		t.Fatal(err)
	}
	gotTime, gotFamily, err := httpapi.DecodeSessionCursor(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !gotTime.Equal(lastSeen) || gotFamily != "family-base64url_id" {
		t.Fatalf("decoded=%s %s", gotTime, gotFamily)
	}
	if _, _, err := httpapi.DecodeSessionCursor("%%%"); err == nil {
		t.Fatal("expected invalid cursor error")
	}
}
