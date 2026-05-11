package balancer

import (
	"net/url"
	"sync"
	"sync/atomic"
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
