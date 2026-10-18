package bootstrap

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// ReadyGate tracks whether the process may accept traffic.
type ReadyGate struct {
	ready atomic.Bool
	db    Pinger
}

// Pinger is the readiness DB dependency.
type Pinger interface {
	Ping(ctx context.Context) error
}

func NewReadyGate(db Pinger) *ReadyGate {
	return &ReadyGate{db: db}
}

func (g *ReadyGate) SetReady(v bool) {
	g.ready.Store(v)
}

func (g *ReadyGate) Ready() bool {
	return g.ready.Load()
}

// RegisterHealth mounts /health/live and /health/ready (unversioned).
func RegisterHealth(r *gin.Engine, gate *ReadyGate) {
	r.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "live"})
	})
	r.GET("/health/ready", func(c *gin.Context) {
		if gate == nil || !gate.Ready() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if gate.db != nil {
			if err := gate.db.Ping(ctx); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
