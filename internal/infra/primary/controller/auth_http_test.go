package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	authBasePort "github.com/WilsonSayago/authBase/v4/core/port"
	authBaseServices "github.com/WilsonSayago/authBase/v4/core/services"
	"github.com/WilsonSayago/authBase/v4/infra/config/properties"
	"github.com/WilsonSayago/authBase/v4/infra/secundary"
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	usersin "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/inputs/users"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
)

func TestBindRejectsOmittedPrivilegedBools(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		body string
		dst  any
	}{
		{"setActive empty object", `{}`, &usersin.SetActiveRequest{}},
		{"setAdmin empty object", `{}`, &usersin.SetAdminFlagRequest{}},
		{"setActive null", `{"active":null}`, &usersin.SetActiveRequest{}},
		{"setAdmin null", `{"isAdmin":null}`, &usersin.SetAdminFlagRequest{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set("request_id", "rid-bool")

			if httpapi.BindAndWrite(c, tc.dst) {
				t.Fatal("expected bind failure for omitted/null bool")
			}
			if w.Code != http.StatusUnprocessableEntity && w.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			var p httpapi.Problem
			if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if p.Code != httpapi.CodeValidation && p.Code != httpapi.CodeBadRequest {
				t.Fatalf("code=%s", p.Code)
			}
		})
	}

	t.Run("setActive explicit false", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(`{"active":false}`))
		c.Request.Header.Set("Content-Type", "application/json")
		var dst usersin.SetActiveRequest
		if !httpapi.BindAndWrite(c, &dst) {
			t.Fatalf("bind failed: %s", w.Body.String())
		}
		if dst.Active == nil || *dst.Active {
			t.Fatalf("active=%v", dst.Active)
		}
	})
}

type stubAuthUC struct {
	revokeErr error
}

func (s stubAuthUC) Login(context.Context, string, string) (string, string, error) {
	return "", "", errors.New("unused")
}
func (s stubAuthUC) RefreshToken(context.Context, string) (string, string, error) {
	return "", "", errors.New("unused")
}
func (s stubAuthUC) ValidateToken(context.Context, string) (authBaseDomain.IUserGeneric, error) {
	return nil, errors.New("unused")
}
func (s stubAuthUC) RevokeRefreshFamily(context.Context, string) error {
	return s.revokeErr
}

var _ authBaseCore.AuthenticationUseCase = stubAuthUC{}

type memIdentity struct {
	mu    sync.Mutex
	users map[string]authBaseDomain.UserGeneric
	creds map[string]authBasePort.CredentialRecord
}

func newMemIdentity() *memIdentity {
	return &memIdentity{
		users: map[string]authBaseDomain.UserGeneric{},
		creds: map[string]authBasePort.CredentialRecord{},
	}
}

func (m *memIdentity) add(u authBaseDomain.UserGeneric, user, hash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.GetId()] = u
	m.creds[user] = authBasePort.CredentialRecord{
		UserID: u.GetId(), Username: user, PasswordHash: hash, Active: u.GetActive(),
	}
}

func (m *memIdentity) FindByID(_ context.Context, id string) (authBaseDomain.UserGeneric, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return authBaseDomain.UserGeneric{}, authBaseCore.ErrNotFound
	}
	return u, nil
}

func (m *memIdentity) FindCredentialsByUsername(_ context.Context, username string) (authBasePort.CredentialRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.creds[username]
	if !ok {
		return authBasePort.CredentialRecord{}, authBaseCore.ErrNotFound
	}
	return c, nil
}

type memRefresh struct {
	mu       sync.Mutex
	byHash   map[[32]byte]authBaseDomain.RefreshSession
	consumed map[[32]byte]struct{}
	revoked  map[string]struct{}
}

func newMemRefresh() *memRefresh {
	return &memRefresh{
		byHash:   map[[32]byte]authBaseDomain.RefreshSession{},
		consumed: map[[32]byte]struct{}{},
		revoked:  map[string]struct{}{},
	}
}

func (m *memRefresh) Create(_ context.Context, s authBaseDomain.RefreshSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byHash[s.TokenHash] = s
	return nil
}

func (m *memRefresh) Rotate(_ context.Context, currentHash [32]byte, next authBaseDomain.RefreshSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.byHash[currentHash]
	if !ok {
		return authBaseCore.ErrInvalidRefresh
	}
	if _, r := m.revoked[cur.FamilyID]; r {
		return authBaseCore.ErrInvalidRefresh
	}
	if _, c := m.consumed[currentHash]; c {
		return authBaseCore.ErrInvalidRefresh
	}
	m.consumed[currentHash] = struct{}{}
	m.byHash[next.TokenHash] = next
	return nil
}

func (m *memRefresh) RevokeFamily(_ context.Context, familyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revoked[familyID] = struct{}{}
	return nil
}

func logoutJWT() properties.Jwt {
	return properties.Jwt{
		SecretKey:        strings.Repeat("a", 32),
		RefreshSecret:    strings.Repeat("b", 32),
		ExpirationTime:   1,
		RefreshTokenTime: 24,
		Issuer:           "logout-test-iss",
		Audience:         "logout-test-aud",
		LeewaySeconds:    0,
	}
}

func mintRefreshToken(t *testing.T) (refresh string, tm *authBaseServices.TokenManager) {
	t.Helper()
	jwtCfg := logoutJWT()
	id := newMemIdentity()
	store := newMemRefresh()
	validate := secundary.NewValidationService()
	hash, err := validate.HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	id.add(authBaseDomain.NewUserGeneric("u1", "n", "u@example.com", nil, false, true), "u@example.com", hash)
	svc, err := authBaseServices.NewAuthenticationService[authBaseDomain.UserGeneric](id, id, store, validate, jwtCfg)
	if err != nil {
		t.Fatal(err)
	}
	_, refresh, err = svc.Login(context.Background(), "u@example.com", "correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	tm, err = authBaseServices.NewTokenManager(jwtCfg)
	if err != nil {
		t.Fatal(err)
	}
	return refresh, tm
}

func TestLogoutPropagatesRevokeInfrastructureError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	refresh, tm := mintRefreshToken(t)
	ctrl := &AuthController{
		authenticationService: stubAuthUC{revokeErr: errors.New("postgres unavailable")},
		tokens:                tm,
	}

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.POST("/v1/auth/logout", ctrl.Logout())
	body := `{"refreshToken":"` + refresh + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s want 500", w.Code, w.Body.String())
	}
	var p httpapi.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Code != httpapi.CodeInternal {
		t.Fatalf("code=%s", p.Code)
	}
	if strings.Contains(strings.ToLower(p.Detail), "postgres") {
		t.Fatalf("leaked internals: %q", p.Detail)
	}
}

func TestLogoutIdempotentOnInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tm, err := authBaseServices.NewTokenManager(logoutJWT())
	if err != nil {
		t.Fatal(err)
	}
	ctrl := &AuthController{
		authenticationService: stubAuthUC{},
		tokens:                tm,
	}

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.POST("/v1/auth/logout", ctrl.Logout())
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", strings.NewReader(`{"refreshToken":"not-a-jwt"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthzMatrixMissingAndForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/v1/users", func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			httpapi.WriteProblem(c, httpapi.Unauthorized("Missing or invalid bearer token", httpapi.RequestID(c)))
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		switch token {
		case "guest":
			httpapi.WriteProblem(c, httpapi.MapError(domain.ErrForbidden, httpapi.RequestID(c)))
		case "bad":
			httpapi.WriteProblem(c, httpapi.Unauthorized("Invalid credentials", httpapi.RequestID(c)))
		default:
			c.Status(http.StatusOK)
		}
	})

	cases := []struct {
		name   string
		header string
		status int
		code   string
	}{
		{"missing", "", 401, httpapi.CodeUnauthorized},
		{"malformed", "Token abc", 401, httpapi.CodeUnauthorized},
		{"invalid claims", "Bearer bad", 401, httpapi.CodeUnauthorized},
		{"forbidden role", "Bearer guest", 403, httpapi.CodeForbidden},
		{"allowed", "Bearer admin", 200, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
			if tc.code == "" {
				return
			}
			var p httpapi.Problem
			if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if p.Code != tc.code {
				t.Fatalf("code=%s want=%s", p.Code, tc.code)
			}
		})
	}
}
