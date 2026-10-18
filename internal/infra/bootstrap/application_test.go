package bootstrap_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WilsonSayago/authBase/v4/infra/config/properties"
	"github.com/WilsonSayago/base-golang/internal/infra/bootstrap"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/test/testdb"
	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func testJWT() properties.Jwt {
	return properties.Jwt{
		SecretKey:        strings.Repeat("a", 32),
		RefreshSecret:    strings.Repeat("b", 32),
		ExpirationTime:   1,
		RefreshTokenTime: 24,
		Issuer:           "composition-test",
		Audience:         "composition-test",
	}
}

func testConfig() appconfig.Config {
	cfg := appconfig.Defaults()
	cfg.Database.Password = "secret-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.Runtime.Environment = "test"
	cfg.Server.Address = "127.0.0.1:0"
	cfg.Server.ShutdownTimeout = 2 * time.Second
	cfg.CORS.AllowedOrigins = []string{"http://localhost:3000"}
	cfg.Docs.SwaggerEnabled = false
	return cfg
}

func testLogger(w io.Writer) *slog.Logger {
	if w == nil {
		w = io.Discard
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func compose(t *testing.T, logger *slog.Logger, mutate func(*bootstrap.Options)) *bootstrap.Application {
	t.Helper()
	opts := bootstrap.Options{
		Config:           testConfig(),
		JWT:              testJWT(),
		Logger:           logger,
		DB:               database.NewPostgresPgxForTest(nil),
		TranslationFiles: []string{"testdata/en.json", "testdata/es.json"},
		SkipMigrate:      true,
		SkipPurger:       true,
		SkipListen:       true,
		SkipDatabaseLife: true,
	}
	if mutate != nil {
		mutate(&opts)
	}
	app, err := bootstrap.NewApplication(opts)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	return app
}

func TestCompositionSmokeDoesNotReadSingletons(t *testing.T) {
	t.Parallel()
	app := compose(t, testLogger(nil), nil)
	if app.Engine == nil || app.Container == nil || app.App == nil || app.Users == nil {
		t.Fatal("expected engine, container, app, and users")
	}
	if database.GetDatabasePostgresPgx() != nil {
		t.Fatal("composition must not install a database singleton")
	}
	if strings.Contains(strings.Join(app.LifecycleNames(), ","), "http") {
		t.Fatalf("smoke composition should not listen: %v", app.LifecycleNames())
	}
}

func TestNewApplicationRequiresLoggerAndJWT(t *testing.T) {
	t.Parallel()
	_, err := bootstrap.NewApplication(bootstrap.Options{
		Config:           testConfig(),
		JWT:              testJWT(),
		DB:               database.NewPostgresPgxForTest(nil),
		SkipMigrate:      true,
		SkipPurger:       true,
		SkipListen:       true,
		SkipDatabaseLife: true,
	})
	if err == nil || !strings.Contains(err.Error(), "logger is required") {
		t.Fatalf("err=%v", err)
	}

	badJWT := testJWT()
	badJWT.SecretKey = "short"
	_, err = bootstrap.NewApplication(bootstrap.Options{
		Config:           testConfig(),
		JWT:              badJWT,
		Logger:           testLogger(nil),
		DB:               database.NewPostgresPgxForTest(nil),
		SkipMigrate:      true,
		SkipPurger:       true,
		SkipListen:       true,
		SkipDatabaseLife: true,
	})
	if err == nil {
		t.Fatal("expected jwt validation error")
	}
}

func TestTwoApplicationsStayIsolated(t *testing.T) {
	t.Parallel()
	var aBuf, bBuf bytes.Buffer
	appA := compose(t, testLogger(&aBuf), nil)
	appB := compose(t, testLogger(&bBuf), nil)
	if appA.Engine == appB.Engine || appA.Container == appB.Container || appA.App == appB.App {
		t.Fatal("applications must not share engine, container, or app")
	}

	serve := func(app *bootstrap.Application, requestID string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		req.Header.Set("X-Request-ID", requestID)
		app.Engine.ServeHTTP(w, req)
		return w.Code
	}
	if got := serve(appA, "rid-a"); got != http.StatusOK {
		t.Fatalf("app A status=%d", got)
	}
	if got := serve(appB, "rid-b"); got != http.StatusOK {
		t.Fatalf("app B status=%d", got)
	}
	if !strings.Contains(aBuf.String(), "rid-a") || strings.Contains(aBuf.String(), "rid-b") {
		t.Fatalf("logger A leaked: %s", aBuf.String())
	}
	if !strings.Contains(bBuf.String(), "rid-b") || strings.Contains(bBuf.String(), "rid-a") {
		t.Fatalf("logger B leaked: %s", bBuf.String())
	}
}

func TestLifecycleOrderAndStartRollback(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var order []string
	record := func(name string) *orderedLifecycle {
		return &orderedLifecycle{name: name, order: &order, mu: &mu}
	}
	failing := record("http")
	failing.startErr = errors.New("listen failed")
	started := record("database")

	app := initModules.NewApp()
	app.Register(started)
	app.Register(failing)
	err := app.RunContext(context.Background(), initModules.RunOptions{
		RunLifecycles: true,
		StopTimeout:   time.Second,
	})
	if err == nil {
		t.Fatal("expected start failure")
	}
	mu.Lock()
	got := append([]string(nil), order...)
	mu.Unlock()
	want := "start:database,start:http,stop:database"
	if strings.Join(got, ",") != want {
		t.Fatalf("order=%v want=%s", got, want)
	}
}

func TestHTTPShutdownDrainsInFlightAndReleasesPort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := compose(t, testLogger(nil), func(opts *bootstrap.Options) {
		opts.SkipListen = false
	})
	started := make(chan struct{})
	app.Engine.GET("/slow", func(c *gin.Context) {
		close(started)
		time.Sleep(150 * time.Millisecond)
		c.Status(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for app.HTTP.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	addr := app.HTTP.Addr()
	if addr == "" {
		cancel()
		t.Fatal("listener did not start")
	}

	clientErr := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/slow")
		if err != nil {
			clientErr <- err
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			clientErr <- errors.New(resp.Status)
			return
		}
		clientErr <- nil
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("in-flight request did not start")
	}
	if !app.Gate.Ready() {
		cancel()
		t.Fatal("expected ready while serving")
	}
	cancel()

	select {
	case err := <-clientErr:
		if err != nil {
			t.Fatalf("in-flight request: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request was not drained")
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("application did not stop")
	}
	if app.Gate.Ready() {
		t.Fatal("expected unready after shutdown")
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("port not released: %v", err)
	}
	_ = ln.Close()
}

func TestNewApplicationClosesOwnedPoolOnWireError(t *testing.T) {
	dsn, err := testdb.RequireURL()
	if err != nil {
		t.Skip(err.Error())
	}
	appName := "owned-pool-" + strings.ReplaceAll(t.Name(), "/", "-")
	cfg := testConfig()
	cfg.Database.URL = dsn + "&application_name=" + appName
	cfg.Database.SSLMode = "disable"
	cfg.Auth.TokenKeys.Algorithm = "ed25519"
	cfg.Auth.TokenKeys.AccessKID = "access-1"
	cfg.Auth.TokenKeys.RefreshKID = "refresh-1"
	cfg.Auth.TokenKeys.AccessKeyPath = filepath.Join(t.TempDir(), "missing-access.pem")
	cfg.Auth.TokenKeys.RefreshKeyPath = filepath.Join(t.TempDir(), "missing-refresh.pem")

	probe := openProbePool(t, dsn)
	_, err = bootstrap.NewApplication(bootstrap.Options{
		Config:           cfg,
		JWT:              testJWT(),
		Logger:           testLogger(nil),
		SkipMigrate:      true,
		SkipPurger:       true,
		SkipListen:       true,
		SkipDatabaseLife: true,
	})
	if err == nil {
		t.Fatal("expected wire error")
	}
	if n := countNamedBackends(t, probe, appName); n != 0 {
		t.Fatalf("owned pool leaked %d backends named %s: %v", n, appName, err)
	}
}

func TestNewApplicationDoesNotCloseInjectedPoolOnWireError(t *testing.T) {
	dsn, err := testdb.RequireURL()
	if err != nil {
		t.Skip(err.Error())
	}
	pool := openProbePool(t, dsn)
	db := database.NewPostgresPgxForTest(pool)
	cfg := testConfig()
	cfg.Database.URL = dsn
	cfg.Database.SSLMode = "disable"
	cfg.Auth.TokenKeys.Algorithm = "ed25519"
	cfg.Auth.TokenKeys.AccessKID = "access-1"
	cfg.Auth.TokenKeys.RefreshKID = "refresh-1"
	cfg.Auth.TokenKeys.AccessKeyPath = filepath.Join(t.TempDir(), "missing-access.pem")
	cfg.Auth.TokenKeys.RefreshKeyPath = filepath.Join(t.TempDir(), "missing-refresh.pem")

	_, err = bootstrap.NewApplication(bootstrap.Options{
		Config:           cfg,
		JWT:              testJWT(),
		Logger:           testLogger(nil),
		DB:               db,
		SkipMigrate:      true,
		SkipPurger:       true,
		SkipListen:       true,
		SkipDatabaseLife: true,
	})
	if err == nil {
		t.Fatal("expected wire error")
	}
	if pingErr := db.Ping(context.Background()); pingErr != nil {
		t.Fatalf("injected pool was closed: %v", pingErr)
	}
}

func openProbePool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func countNamedBackends(t *testing.T, pool *pgxpool.Pool, appName string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE application_name = $1`, appName).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

type orderedLifecycle struct {
	name     string
	order    *[]string
	mu       *sync.Mutex
	startErr error
}

func (l *orderedLifecycle) Start(ctx context.Context) error {
	l.mu.Lock()
	*l.order = append(*l.order, "start:"+l.name)
	l.mu.Unlock()
	return l.startErr
}

func (l *orderedLifecycle) Stop(ctx context.Context) error {
	l.mu.Lock()
	*l.order = append(*l.order, "stop:"+l.name)
	l.mu.Unlock()
	return nil
}

var _ initModules.Lifecycle = (*orderedLifecycle)(nil)
