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
	authBasePorts "github.com/WilsonSayago/authBase/v4/core/port"
	"github.com/WilsonSayago/authBase/v4/infra/secundary"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	"github.com/WilsonSayago/base-golang/internal/core/services"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/httpapi"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/oidccrypto"
	"github.com/gin-gonic/gin"
)

func TestOIDCLoginRedirectSetsHttpOnlyCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, mem := testOIDCService(t)
	mem.principal = domain.ExternalPrincipal{
		Provider: "google", Issuer: "https://accounts.example.test", Subject: "sub-1", EmailVerified: true,
	}
	ctrl := NewOIDCController(svc, nil, gin.New(), nil, nil, nil, appconfig.OIDCConfig{
		CookieName: "oidc_tx",
		CookiePath: "/v1/auth/oidc",
	}, true)
	ctrl.r.GET("/v1/auth/oidc/google/login", ctrl.Login())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/google/login?return_to=https://app.example.com/auth/oidc", nil)
	ctrl.r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if strings.Contains(strings.ToLower(loc), "eyj") {
		t.Fatalf("jwt in location: %s", loc)
	}
	cookie := w.Result().Cookies()
	if len(cookie) != 1 || !cookie[0].HttpOnly || !cookie[0].Secure || cookie[0].Path != "/v1/auth/oidc" {
		t.Fatalf("cookie=%+v", cookie)
	}
	if cookie[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("samesite=%v", cookie[0].SameSite)
	}
}

func TestOIDCCallbackRejectsUnknownState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := testOIDCService(t)
	ctrl := NewOIDCController(svc, nil, gin.New(), nil, nil, nil, appconfig.OIDCConfig{
		CookieName: "oidc_tx",
		CookiePath: "/v1/auth/oidc",
	}, false)
	ctrl.r.GET("/v1/auth/oidc/google/callback", ctrl.Callback())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/google/callback?state=abc&code=x", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_tx", Value: "abc"})
	ctrl.r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var p httpapi.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Code != httpapi.CodeUnauthorized {
		t.Fatalf("code=%s", p.Code)
	}
}

func TestOIDCDisabledReturns503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := NewOIDCController(nil, nil, gin.New(), nil, nil, nil, appconfig.OIDCConfig{CookieName: "oidc_tx"}, false)
	ctrl.r.GET("/v1/auth/oidc/google/login", ctrl.Login())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/google/login", nil)
	ctrl.r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func testOIDCService(t *testing.T) (*services.OIDCService, *oidcMem) {
	t.Helper()
	mem := &oidcMem{}
	box, err := oidccrypto.NewBox(bytesRepeat(0x44))
	if err != nil {
		t.Fatal(err)
	}
	svc, err := services.NewOIDCService(
		services.OIDCServiceConfig{Enabled: true, TransactionTTL: time.Minute, GrantTTL: time.Minute},
		[]ports.OIDCProvider{mem},
		map[string][]string{"google": {"https://app.example.com/auth/oidc"}},
		mem, mem, mem, box, oidcSessions{},
		secundary.NewValidationService(),
		ports.NoopOIDCAuditor{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return svc, mem
}

func bytesRepeat(b byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = b
	}
	return out
}

type oidcMem struct {
	txns      map[string]domain.OIDCTransaction
	principal domain.ExternalPrincipal
}

func (m *oidcMem) Create(_ context.Context, tx domain.OIDCTransaction) error {
	if m.txns == nil {
		m.txns = map[string]domain.OIDCTransaction{}
	}
	tx.ID = string(tx.StateHash)
	m.txns[string(tx.StateHash)] = tx
	return nil
}
func (m *oidcMem) ConsumeByStateHash(_ context.Context, stateHash []byte, at time.Time) (domain.OIDCTransaction, error) {
	tx, ok := m.txns[string(stateHash)]
	if !ok {
		return domain.OIDCTransaction{}, domain.ErrOIDCInvalidState
	}
	if tx.ConsumedAt != nil {
		return domain.OIDCTransaction{}, domain.ErrOIDCReplay
	}
	if !tx.ExpiresAt.After(at) {
		return domain.OIDCTransaction{}, domain.ErrOIDCExpired
	}
	now := at
	tx.ConsumedAt = &now
	m.txns[string(stateHash)] = tx
	return tx, nil
}
func (m *oidcMem) AttachGrant(context.Context, string, []byte, string, time.Time) error {
	return nil
}
func (m *oidcMem) ConsumeGrant(context.Context, []byte, time.Time) (domain.OIDCTransaction, error) {
	return domain.OIDCTransaction{}, domain.ErrOIDCInvalidGrant
}
func (m *oidcMem) PurgeExpired(context.Context, time.Time) (int64, error) { return 0, nil }
func (m *oidcMem) FindByIssuerSubject(context.Context, string, string) (domain.ExternalIdentity, error) {
	return domain.ExternalIdentity{}, domain.ErrNotFound
}
func (m *oidcMem) Link(context.Context, domain.ExternalIdentity) error { return nil }
func (m *oidcMem) MarkLogin(context.Context, string, time.Time) error  { return nil }
func (m *oidcMem) Unlink(context.Context, string, string) error        { return domain.ErrNotFound }
func (m *oidcMem) FindByID(context.Context, string) (domain.User, error) {
	return domain.User{UserGeneric: authBaseDomain.NewUserGeneric("u1", "Ada", "ada@example.com", nil, false, true)}, nil
}
func (m *oidcMem) FindCredentialsByUsername(context.Context, string) (authBasePorts.CredentialRecord, error) {
	return authBasePorts.CredentialRecord{}, domain.ErrNotFound
}
func (m *oidcMem) Name() string   { return "google" }
func (m *oidcMem) Issuer() string { return "https://accounts.example.test" }
func (m *oidcMem) AuthorizationURL(state, nonce, challenge string) (string, error) {
	return "https://accounts.example.test/auth?state=" + state, nil
}
func (m *oidcMem) Exchange(context.Context, string, string, string) (domain.ExternalPrincipal, error) {
	return m.principal, nil
}

type oidcSessions struct{}

func (oidcSessions) EstablishSession(context.Context, string) (string, string, error) {
	return "access", "refresh", nil
}
