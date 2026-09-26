package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestJWKSHeadersAndEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerJWKS(r, []byte(`{"keys":[{"kty":"OKP","kid":"access-1"}]}`))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "max-age=300") || w.Header().Get("ETag") == "" {
		t.Fatalf("headers=%v", w.Header())
	}
	if strings.Contains(w.Body.String(), `"d"`) {
		t.Fatal("private material in JWKS")
	}

	empty := gin.New()
	registerJWKS(empty, nil)
	w = httptest.NewRecorder()
	empty.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("empty jwks status=%d", w.Code)
	}
}
