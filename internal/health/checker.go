package health

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/etherealone0/custom-api-gateway/internal/balancer"
)

type Checker struct {
	interval    time.Duration
	timeout     time.Duration
	path        string
	client      *http.Client
	getBackends func() []*balancer.Backend
}

func New(interval, timeout time.Duration, path string, getBackends func() []*balancer.Backend) *Checker {
	return &Checker{
		interval:    interval,
		timeout:     timeout,
		path:        path,
		client:      &http.Client{Timeout: timeout},
		getBackends: getBackends,
	}
}

func (c *Checker) Start(ctx context.Context) {
	slog.Info("health checker started", "interval", c.interval, "path", c.path)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.checkAll(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("health checker stopped")
			return
		case <-ticker.C:
			c.checkAll(ctx)
		}
	}
}

func (c *Checker) checkAll(ctx context.Context) {
	backends := c.getBackends()
	for _, b := range backends {
		c.check(ctx, b)
	}
}

func (c *Checker) check(ctx context.Context, b *balancer.Backend) {
	target := fmt.Sprintf("%s%s", b.URL.String(), c.path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return
	}

	resp, err := c.client.Do(req)
	wasAlive := b.IsAlive()

	if err != nil {
		b.SetAlive(false)
		if wasAlive {
			slog.Warn("backend went down", "url", b.URL.String(), "reason", err.Error())
		}
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b.SetAlive(false)
		if wasAlive {
			slog.Warn("backend went down", "url", b.URL.String(), "status", resp.StatusCode)
		}
		return
	}

	b.SetAlive(true)
	if !wasAlive {
		slog.Info("backend came up", "url", b.URL.String())
	}
}
