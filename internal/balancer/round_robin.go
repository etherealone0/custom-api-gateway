package balancer

import "sync/atomic"

type RoundRobin struct {
	pool    *ServerPool
	counter atomic.Uint64
}

func NewRoundRobin(pool *ServerPool) *RoundRobin {
	return &RoundRobin{pool: pool}
}

func (rr *RoundRobin) NextServer() (*Backend, error) {
	backends := rr.pool.GetBackends()
	total := len(backends)
	if total == 0 {
		return nil, ErrNoBackends
	}

	// Try at most `total` backends before giving up
	for i := 0; i < total; i++ {
		idx := rr.counter.Add(1) % uint64(total)
		backend := backends[idx]
		if backend.IsAlive() {
			return backend, nil
		}
	}

	return nil, ErrNoBackends
}
