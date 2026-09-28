package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/authz"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto"
	settingsdto "github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto/outputs/settings"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/ginadapter"
	"github.com/gin-gonic/gin"
)

type stubSettingsUC struct {
	current domain.AppSettings
	err     error
	patch   domain.AppSettingsPatch
	actor   string
}

func (s *stubSettingsUC) Get(context.Context) (domain.AppSettings, error) {
	if s.err != nil {
		return domain.AppSettings{}, s.err
	}
	return s.current, nil
}

func (s *stubSettingsUC) Update(_ context.Context, actorID string, patch domain.AppSettingsPatch) (domain.AppSettings, error) {
	s.actor = actorID
	s.patch = patch
	if s.err != nil {
		return domain.AppSettings{}, s.err
	}
	next := s.current
	next.DisplayName = patch.DisplayName
	next.LogoURL = patch.LogoURL
	next.DefaultLocale = patch.DefaultLocale
	next.Version = patch.Version + 1
	next.UpdatedBy = actorID
	s.current = next
	return next, nil
}

func settingsUser() domain.User {
	return domain.User{UserGeneric: authBaseDomain.NewUserGeneric("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Ada", "ada@example.com", nil, true, true)}
}

func TestSettingsPublicOmitsVersionAndSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := &stubSettingsUC{current: domain.AppSettings{
		DisplayName: "Acme", LogoURL: "/logo.svg", DefaultLocale: "en", Version: 4,
		UpdatedAt: time.Date(2026, 9, 28, 18, 0, 0, 0, time.UTC), UpdatedBy: "actor-1",
	}}
	ctrl := NewSettingsController(guardAuthz{}, uc, gin.New(), nil)
	w := httptest.NewRecorder()
	engine := gin.New()
	engine.GET("/v1/settings/public", ctrl.Public())
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/settings/public", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("ETag"), "4") {
		t.Fatalf("etag=%q", w.Header().Get("ETag"))
	}
	body := w.Body.String()
	for _, leaked := range []string{"version", "updatedBy", "jwt", "postgres://", "ada@example.com"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("leaked %q in %s", leaked, body)
		}
	}
	var envelope dto.ResponseDto
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(envelope.Data)
	var pub settingsdto.PublicSettingsResponse
	if err := json.Unmarshal(raw, &pub); err != nil {
		t.Fatal(err)
	}
	if pub.DisplayName != "Acme" || pub.DefaultLocale != "en" {
		t.Fatalf("public=%+v", pub)
	}
}

func TestSettingsUpdateForbiddenWithoutPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := NewSettingsController(guardAuthz{user: settingsUser(), deny: true}, &stubSettingsUC{current: domain.DefaultAppSettings()}, gin.New(), nil)
	w := httptest.NewRecorder()
	engine := gin.New()
	engine.PUT("/v1/settings", ginadapter.Middleware(ctrl.authorization.AuthorizeJWT()), ginadapter.Middleware(ctrl.authorization.PoliciesGuard(ginadapter.Handler(ctrl.Update()), nil, authz.EntitySettings, authz.Update)))
	req := httptest.NewRequest(http.MethodPut, "/v1/settings", strings.NewReader(`{"version":1,"displayName":"Acme","defaultLocale":"es"}`))
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSettingsGetRequiresPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := NewSettingsController(guardAuthz{user: settingsUser(), deny: true}, &stubSettingsUC{current: domain.DefaultAppSettings()}, gin.New(), nil)
	w := httptest.NewRecorder()
	engine := gin.New()
	engine.GET("/v1/settings", ginadapter.Middleware(ctrl.authorization.AuthorizeJWT()), ginadapter.Middleware(ctrl.authorization.PoliciesGuard(ginadapter.Handler(ctrl.Get()), nil, authz.EntitySettings, authz.Read)))
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/settings", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSettingsUpdateRejectsUnknownAndProtectedKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := &stubSettingsUC{current: domain.DefaultAppSettings()}
	ctrl := NewSettingsController(guardAuthz{user: settingsUser()}, uc, gin.New(), nil)
	engine := gin.New()
	engine.PUT("/v1/settings", ctrl.Update())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/settings", strings.NewReader(`{"version":1,"displayName":"Acme","jwtSecret":"s3cret","defaultLocale":"es"}`))
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("protected status=%d body=%s", w.Code, w.Body.String())
	}
	if uc.patch.DisplayName != "" {
		t.Fatal("protected payload must not reach the use case")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/v1/settings", strings.NewReader(`{"version":1,"displayName":"Acme","featureFlag":true,"defaultLocale":"es"}`))
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSettingsUpdateConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := &stubSettingsUC{current: domain.DefaultAppSettings(), err: domain.ErrStaleVersion}
	ctrl := NewSettingsController(guardAuthz{user: settingsUser()}, uc, gin.New(), nil)
	engine := gin.New()
	engine.PUT("/v1/settings", ctrl.Update())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/settings", strings.NewReader(`{"version":1,"displayName":"Acme","logoUrl":"","defaultLocale":"es"}`))
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSettingsUpdateSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := &stubSettingsUC{current: domain.DefaultAppSettings()}
	ctrl := NewSettingsController(guardAuthz{user: settingsUser()}, uc, gin.New(), nil)
	engine := gin.New()
	engine.PUT("/v1/settings", ctrl.Update())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/settings", strings.NewReader(`{"version":1,"displayName":"Acme","logoUrl":"https://cdn.example.com/a.png","defaultLocale":"en"}`))
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if uc.actor == "" || uc.patch.DisplayName != "Acme" {
		t.Fatalf("patch=%+v actor=%s", uc.patch, uc.actor)
	}
	if strings.Contains(w.Body.String(), "https://cdn.example.com/a.png") == false {
		t.Fatalf("missing logo: %s", w.Body.String())
	}
}
