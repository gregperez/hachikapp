package bootstrap_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/WilsonSayago/base-golang/internal/infra/bootstrap"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/gin-gonic/gin"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type stubPinger struct {
	err error
}

func (s stubPinger) Ping(context.Context) error { return s.err }

func TestHealthLiveAlwaysOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	gate := bootstrap.NewReadyGate(stubPinger{})
	bootstrap.RegisterHealth(r, gate)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestHealthReadyBeforeAndAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	gate := bootstrap.NewReadyGate(stubPinger{})
	bootstrap.RegisterHealth(r, gate)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("before ready status=%d", w.Code)
	}

	gate.SetReady(true)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("ready status=%d body=%s", w.Code, w.Body.String())
	}

	gate.SetReady(false)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("after unready status=%d", w.Code)
	}
}

func TestHealthReadyFailsWhenDBDown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	gate := bootstrap.NewReadyGate(stubPinger{err: errors.New("down")})
	gate.SetReady(true)
	bootstrap.RegisterHealth(r, gate)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestCORSRejectsDisallowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := appconfig.Defaults()
	cfg.Database.Password = "secret-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.CORS.AllowedOrigins = []string{"http://allowed.example"}
	cfg.CORS.AllowCredentials = true

	r := bootstrap.NewEngine(bootstrap.HTTPDeps{Config: cfg, Gate: bootstrap.NewReadyGate(nil), Logger: discardLogger()})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/health/live", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Access-Control-Request-Method", "GET")
	r.ServeHTTP(w, req)
	if ao := w.Header().Get("Access-Control-Allow-Origin"); ao == "http://evil.example" {
		t.Fatalf("unexpected allow origin %q", ao)
	}
}

func TestSwaggerRespectsFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := appconfig.Defaults()
	cfg.Database.Password = "secret-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.Docs.SwaggerEnabled = false
	cfg.ApplyNonProdCORSDefaults()

	r := bootstrap.NewEngine(bootstrap.HTTPDeps{Config: cfg, Gate: bootstrap.NewReadyGate(nil), Logger: discardLogger()})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("swagger should be absent, status=%d", w.Code)
	}
}

func TestRunServerShutsDownOnCancel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	srv := bootstrap.NewHTTPServer(appconfig.ServerConfig{
		Address:           "127.0.0.1:0",
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		IdleTimeout:       time.Second,
		ShutdownTimeout:   2 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}, r)
	srv.Addr = "127.0.0.1:0"

	ctx, cancel := context.WithCancel(context.Background())
	gate := bootstrap.NewReadyGate(nil)
	gate.SetReady(true)

	errCh := make(chan error, 1)
	go func() {
		errCh <- bootstrap.RunServer(ctx, srv, gate, 2*time.Second)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		// Wait until the listener is up (Ready still true until cancel).
		if gate.Ready() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("RunServer: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for shutdown")
	}
	if gate.Ready() {
		t.Fatal("expected unready after shutdown")
	}
}
