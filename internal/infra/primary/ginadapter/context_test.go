package ginadapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestContext_RequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	type key struct{}
	ctx := context.WithValue(req.Context(), key{}, "v")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	got := Wrap(c).RequestContext()
	if got.Value(key{}) != "v" {
		t.Fatalf("RequestContext did not preserve values")
	}
}

func TestContext_GetSetStringKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	ac := Wrap(c)
	ac.Set("user", "alice")
	got, ok := ac.Get("user")
	if !ok || got != "alice" {
		t.Fatalf("Get = (%v, %v)", got, ok)
	}
}

func TestMiddlewareAndHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var sawAdapter bool
	h := Middleware(Handler(func(c *gin.Context) {
		if Wrap(c).RequestContext() == nil {
			t.Fatal("nil request context")
		}
		sawAdapter = true
	}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	h(c)
	if !sawAdapter {
		t.Fatal("handler not invoked")
	}
}
