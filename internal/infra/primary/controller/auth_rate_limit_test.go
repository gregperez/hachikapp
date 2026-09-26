package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	authBaseServices "github.com/WilsonSayago/authBase/v4/core/services"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/ratelimit"
	"github.com/gin-gonic/gin"
)

type rateAuthUC struct{}

func (rateAuthUC) Login(_ context.Context, _, password string) (string, string, error) {
	if password == "accepted" {
		return "access", "refresh", nil
	}
	return "", "", authBaseCore.ErrInvalidCredentials
}

func (rateAuthUC) RefreshToken(context.Context, string) (string, string, error) {
	return "", "", authBaseCore.ErrInvalidRefresh
}

func (rateAuthUC) ValidateToken(context.Context, string) (authBaseDomain.IUserGeneric, error) {
	return nil, errors.New("unused")
}

func (rateAuthUC) RevokeRefreshFamily(context.Context, string) error {
	return errors.New("unused")
}

func TestLoginRateLimitBoundaryRetryAfterAndSuccessReset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := rateLimitController(t, nil, 3, 100, 3)
	engine := gin.New()
	engine.POST("/v1/auth/login", ctrl.Login())

	request := func(email, password, remote string) *httptest.ResponseRecorder {
		body := `{"email":"` + email + `","password":"` + password + `"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = remote
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}

	if got := request("a@example.com", "bad", "192.0.2.1:1000").Code; got != http.StatusUnauthorized {
		t.Fatalf("first status=%d", got)
	}
	if got := request("a@example.com", "accepted", "192.0.2.1:1000").Code; got != http.StatusOK {
		t.Fatalf("success status=%d", got)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		if got := request("a@example.com", "bad", "192.0.2.1:1000").Code; got != http.StatusUnauthorized {
			t.Fatalf("post-reset attempt %d status=%d", attempt, got)
		}
	}
	blocked := request("a@example.com", "bad", "192.0.2.1:1000")
	if blocked.Code != http.StatusTooManyRequests || blocked.Header().Get("Retry-After") == "" {
		t.Fatalf("blocked status=%d retry=%q body=%s", blocked.Code, blocked.Header().Get("Retry-After"), blocked.Body.String())
	}

	if got := request("b@example.com", "bad", "192.0.2.1:1000").Code; got != http.StatusUnauthorized {
		t.Fatalf("independent account status=%d", got)
	}
}

func TestLoginOriginLimitsAreIndependentAndIgnoreUntrustedForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := rateLimitController(t, nil, 100, 1, 100)
	engine := gin.New()
	engine.POST("/v1/auth/login", ctrl.Login())

	request := func(email, remote, forwarded string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(
			`{"email":"`+email+`","password":"bad"}`,
		))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", forwarded)
		req.RemoteAddr = remote
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response.Code
	}

	if got := request("a@example.com", "192.0.2.1:1", "198.51.100.1"); got != http.StatusUnauthorized {
		t.Fatalf("first origin status=%d", got)
	}
	if got := request("b@example.com", "192.0.2.1:2", "198.51.100.2"); got != http.StatusTooManyRequests {
		t.Fatalf("spoofed forwarding status=%d", got)
	}
	if got := request("b@example.com", "192.0.2.2:1", "198.51.100.2"); got != http.StatusUnauthorized {
		t.Fatalf("independent origin status=%d", got)
	}
}

func TestClientOriginResolverUsesForwardingOnlyForTrustedPeer(t *testing.T) {
	t.Parallel()
	resolver, err := NewClientOriginResolver([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.RemoteAddr = "10.1.2.3:443"
	request.Header.Set("X-Forwarded-For", "203.0.113.9, 10.2.3.4")
	if got := resolver.Origin(request); got != "203.0.113.9" {
		t.Fatalf("trusted proxy origin=%q", got)
	}

	request.RemoteAddr = "192.0.2.10:443"
	if got := resolver.Origin(request); got != "192.0.2.10" {
		t.Fatalf("untrusted peer origin=%q", got)
	}
}

func TestRefreshLimitsVerifiedFamilyOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	refresh, tokens := mintRefreshToken(t)
	ctrl := rateLimitController(t, tokens, 100, 100, 1)
	engine := gin.New()
	engine.POST("/v1/auth/refresh-token", ctrl.RefreshToken())

	request := func(token string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh-token", strings.NewReader(
			`{"refreshToken":"`+token+`"}`,
		))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.0.2.1:1"
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response.Code
	}

	if got := request("not-a-token"); got != http.StatusUnauthorized {
		t.Fatalf("invalid token status=%d", got)
	}
	if got := request(refresh); got != http.StatusUnauthorized {
		t.Fatalf("first verified family status=%d", got)
	}
	if got := request(refresh); got != http.StatusTooManyRequests {
		t.Fatalf("limited verified family status=%d", got)
	}
}

func rateLimitController(
	t *testing.T,
	tokens *authBaseServices.TokenManager,
	accountLimit, originLimit, familyLimit int,
) *AuthController {
	t.Helper()
	limiter, err := ratelimit.NewMemory(ratelimit.Config{
		Scopes: map[ports.LoginLimitScope]ratelimit.ScopeConfig{
			ports.LoginLimitAccount:  {Limit: accountLimit, Window: time.Minute},
			ports.LoginLimitOrigin:   {Limit: originLimit, Window: time.Minute},
			ports.RefreshLimitFamily: {Limit: familyLimit, Window: time.Minute},
		},
		MaxEntries: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := NewClientOriginResolver(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &AuthController{
		authenticationService: rateAuthUC{},
		tokens:                tokens,
		limiter:               limiter,
		originResolver:        resolver,
	}
}
