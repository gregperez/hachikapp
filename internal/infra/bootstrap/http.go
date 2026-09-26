package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/httpapi"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// HTTPDeps wires optional docs and route registration into the engine.
type HTTPDeps struct {
	Config   appconfig.Config
	Register func(r *gin.Engine)
	Gate     *ReadyGate
	Logger   *slog.Logger
	JWKS     []byte
}

// NewEngine builds the Gin engine with middleware, health, CORS, and routes.
// Routes are registered before the caller starts listening.
func NewEngine(deps HTTPDeps) *gin.Engine {
	if deps.Logger == nil {
		panic("bootstrap: logger is required")
	}
	if gin.Mode() != gin.TestMode {
		gin.SetMode(deps.Config.EffectiveGinMode())
	}
	lg := deps.Logger
	r := gin.New()
	r.Use(httpapi.Recovery(lg))
	r.Use(requestIDMiddleware())
	r.Use(httpapi.LimitRequestBody(httpapi.DefaultMaxBodyBytes))
	r.Use(accessLogMiddleware(lg))

	corsCfg := cors.Config{
		AllowOrigins:     deps.Config.CORS.AllowedOrigins,
		AllowMethods:     deps.Config.CORS.AllowedMethods,
		AllowHeaders:     deps.Config.CORS.AllowedHeaders,
		AllowCredentials: deps.Config.CORS.AllowCredentials,
		MaxAge:           12 * time.Hour,
	}
	r.Use(cors.New(corsCfg))

	RegisterHealth(r, deps.Gate)
	registerJWKS(r, deps.JWKS)

	if deps.Config.Docs.SwaggerEnabled {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	if deps.Register != nil {
		deps.Register(r)
	}
	return r
}

// NewHTTPServer builds an http.Server with configured timeouts.
func NewHTTPServer(cfg appconfig.ServerConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}
}

// RunServer serves HTTP until ctx is cancelled or ListenAndServe fails.
// On shutdown signal it marks the gate unready, Shuts down with deadline, then returns.
func RunServer(ctx context.Context, srv *http.Server, gate *ReadyGate, shutdownTimeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	case <-ctx.Done():
		if gate != nil {
			gate.SetReady(false)
		}
		stopCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(stopCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		if err := <-errCh; err != nil {
			return err
		}
		return nil
	}
}

// SignalContext cancels on SIGINT/SIGTERM/SIGHUP/SIGQUIT.
func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}
		c.Writer.Header().Set("X-Request-ID", id)
		c.Set("request_id", id)
		c.Next()
	}
}

func accessLogMiddleware(logger *slog.Logger) gin.HandlerFunc {
	lg := logger
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		lg.Info("http",
			"request_id", c.GetString("request_id"),
			"status", c.Writer.Status(),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"latency_ms", time.Since(start).Milliseconds(),
		)
	}
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
