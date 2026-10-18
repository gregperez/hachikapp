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
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/core/services"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/appconfig"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/oidccrypto"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/ratelimit"
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

func TestOIDCRateLimitAppliesToLoginCallbackAndExchange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := testOIDCService(t)
	limiter, err := ratelimit.NewMemory(ratelimit.Config{
		Scopes: map[ports.LoginLimitScope]ratelimit.ScopeConfig{
			ports.LoginLimitOrigin: {Limit: 1, Window: time.Minute},
		},
		MaxEntries: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := NewClientOriginResolver(nil)
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	ctrl := NewOIDCController(svc, nil, engine, nil, limiter, resolver, appconfig.OIDCConfig{
		CookieName: "oidc_tx",
		CookiePath: "/v1/auth/oidc",
	}, true)
	ctrl.RunController()

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/google/login?return_to=https://app.example.com/auth/oidc", nil)
	req.RemoteAddr = "192.0.2.9:1000"
	engine.ServeHTTP(first, req)
	if first.Code != http.StatusFound {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}

	assert429 := func(method, path, body string) {
		t.Helper()
		w := httptest.NewRecorder()
		var next *http.Request
		if body == "" {
			next = httptest.NewRequest(method, path, nil)
		} else {
			next = httptest.NewRequest(method, path, strings.NewReader(body))
			next.Header.Set("Content-Type", "application/json")
		}
		next.RemoteAddr = "192.0.2.9:1000"
		engine.ServeHTTP(w, next)
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("%s %s status=%d body=%s", method, path, w.Code, w.Body.String())
		}
	}
	assert429(http.MethodGet, "/v1/auth/oidc/google/login?return_to=https://app.example.com/auth/oidc", "")
	assert429(http.MethodGet, "/v1/auth/oidc/google/callback?state=abc&code=x", "")
	assert429(http.MethodPost, "/v1/auth/oidc/google/exchange", `{"grant":"opaque"}`)
}

func TestOIDCCallbackAndExchangeOmitTokensFromLocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, mem := testOIDCService(t)
	mem.principal = domain.ExternalPrincipal{
		Provider: "google", Issuer: "https://accounts.example.test", Subject: "sub-1", EmailVerified: true,
	}
	mem.identity = &domain.ExternalIdentity{
		ID: "id-1", Provider: "google", Issuer: "https://accounts.example.test", Subject: "sub-1", UserID: "u1",
	}
	engine := gin.New()
	ctrl := NewOIDCController(svc, nil, engine, nil, nil, nil, appconfig.OIDCConfig{
		CookieName: "oidc_tx",
		CookiePath: "/v1/auth/oidc",
	}, true)
	ctrl.RunController()

	login := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/google/login?return_to=https://app.example.com/auth/oidc", nil)
	engine.ServeHTTP(login, loginReq)
	if login.Code != http.StatusFound {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	if strings.Contains(strings.ToLower(login.Header().Get("Location")), "eyj") || strings.Contains(login.Body.String(), "access") {
		t.Fatalf("tokens leaked on login: loc=%s body=%s", login.Header().Get("Location"), login.Body.String())
	}
	var state string
	for _, cookie := range login.Result().Cookies() {
		if cookie.Name == "oidc_tx" {
			state = cookie.Value
		}
	}
	if state == "" {
		t.Fatal("missing state cookie")
	}

	cb := httptest.NewRecorder()
	cbReq := httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/google/callback?state="+state+"&code=code-1", nil)
	cbReq.AddCookie(&http.Cookie{Name: "oidc_tx", Value: state})
	engine.ServeHTTP(cb, cbReq)
	if cb.Code != http.StatusFound {
		t.Fatalf("callback status=%d body=%s", cb.Code, cb.Body.String())
	}
	loc := cb.Header().Get("Location")
	if strings.Contains(strings.ToLower(loc), "eyj") || strings.Contains(loc, "access") || strings.Contains(loc, "refresh") {
		t.Fatalf("tokens in location: %s", loc)
	}
	idx := strings.Index(loc, "grant=")
	if idx < 0 {
		t.Fatalf("missing grant in %s", loc)
	}
	grant := loc[idx+len("grant="):]
	if grant == "" || strings.Contains(grant, ".") {
		t.Fatalf("grant=%q", grant)
	}

	ex := httptest.NewRecorder()
	exReq := httptest.NewRequest(http.MethodPost, "/v1/auth/oidc/google/exchange", strings.NewReader(`{"grant":"`+grant+`"}`))
	exReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(ex, exReq)
	if ex.Code != http.StatusOK {
		t.Fatalf("exchange status=%d body=%s", ex.Code, ex.Body.String())
	}
	if strings.Contains(strings.ToLower(ex.Header().Get("Location")), "eyj") {
		t.Fatalf("tokens in exchange location: %s", ex.Header().Get("Location"))
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
	grants    map[string]string
	principal domain.ExternalPrincipal
	identity  *domain.ExternalIdentity
	user      domain.User
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
func (m *oidcMem) AttachGrant(_ context.Context, id string, grantHash []byte, userID string, expiresAt time.Time) error {
	tx, ok := m.txns[id]
	if !ok {
		return domain.ErrOIDCInvalidState
	}
	tx.GrantHash = grantHash
	tx.GrantExpiresAt = &expiresAt
	tx.ResolvedUserID = userID
	m.txns[id] = tx
	if m.grants == nil {
		m.grants = map[string]string{}
	}
	m.grants[string(grantHash)] = id
	return nil
}
func (m *oidcMem) ConsumeGrant(_ context.Context, grantHash []byte, at time.Time) (domain.OIDCTransaction, error) {
	id, ok := m.grants[string(grantHash)]
	if !ok {
		return domain.OIDCTransaction{}, domain.ErrOIDCInvalidGrant
	}
	tx := m.txns[id]
	if tx.GrantConsumedAt != nil {
		return domain.OIDCTransaction{}, domain.ErrOIDCReplay
	}
	if tx.GrantExpiresAt == nil || !tx.GrantExpiresAt.After(at) {
		return domain.OIDCTransaction{}, domain.ErrOIDCExpired
	}
	now := at
	tx.GrantConsumedAt = &now
	m.txns[id] = tx
	return tx, nil
}
func (m *oidcMem) PurgeExpired(context.Context, time.Time) (int64, error) { return 0, nil }
func (m *oidcMem) FindByIssuerSubject(context.Context, string, string) (domain.ExternalIdentity, error) {
	if m.identity == nil {
		return domain.ExternalIdentity{}, domain.ErrNotFound
	}
	return *m.identity, nil
}
func (m *oidcMem) Link(context.Context, domain.ExternalIdentity) error { return nil }
func (m *oidcMem) MarkLogin(context.Context, string, time.Time) error  { return nil }
func (m *oidcMem) Unlink(context.Context, string, string) error        { return domain.ErrNotFound }
func (m *oidcMem) FindByID(context.Context, string) (domain.User, error) {
	if m.user.GetId() != "" {
		return m.user, nil
	}
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
