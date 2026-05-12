package balancer

import (
	"log/slog"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type Backend struct {
	URL               *url.URL
	Weight            int
	alive             atomic.Bool
	ActiveConnections atomic.Int64
}

func NewBackend(rawURL string, weight int) (*Backend, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	b := &Backend{
		URL:    u,
		Weight: weight,
	}
	b.alive.Store(true)
	return b, nil
}

func (b *Backend) IsAlive() bool {
	return b.alive.Load()
}

func (b *Backend) SetAlive(alive bool) {
	b.alive.Store(alive)
}

// Thread-safe collection of backends
type ServerPool struct {
	mu       sync.RWMutex
	backends []*Backend
}

func NewServerPool(backends []*Backend) *ServerPool {
	return &ServerPool{backends: backends}
}

func (sp *ServerPool) GetBackends() []*Backend {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	out := make([]*Backend, len(sp.backends))
	copy(out, sp.backends)
	return out
}

func (sp *ServerPool) SetBackends(backends []*Backend) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.backends = backends
}

func (sp *ServerPool) Len() int {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return len(sp.backends)
}

func (b *Backend) MarkDraining() {
	b.alive.Store(false)
}

func (b *Backend) DrainConnections(timeout time.Duration) bool {
	if b.ActiveConnections.Load() == 0 {
		return true
	}

	deadline := time.After(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return false
		case <-ticker.C:
			if b.ActiveConnections.Load() == 0 {
				return true
			}
		}
	}
}

func FindRemovedBackends(old, current []*Backend) []*Backend {
	currentURLs := make(map[string]bool)
	for _, b := range current {
		currentURLs[b.URL.String()] = true
	}

	var removed []*Backend
	for _, b := range old {
		if !currentURLs[b.URL.String()] {
			removed = append(removed, b)
		}
	}
	return removed
}

func DrainAll(backends []*Backend, timeout time.Duration) {
	var wg sync.WaitGroup
	for _, b := range backends {
		wg.Add(1)
		go func(b *Backend) {
			defer wg.Done()
			if b.DrainConnections(timeout) {
				slog.Info("backend drained", "backend", b.URL.String())
			} else {
				slog.Warn("drain timeout, forcing removal",
					"backend", b.URL.String(),
					"remaining", b.ActiveConnections.Load(),
				)
			}
		}(b)
	}
	wg.Wait()
}
