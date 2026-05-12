package balancer

type LeastConnections struct {
	pool *ServerPool
}

func NewLeastConnections(pool *ServerPool) *LeastConnections {
	return &LeastConnections{pool: pool}
}

func (lc *LeastConnections) NextServer() (*Backend, error) {
	backends := lc.pool.GetBackends()
	if len(backends) == 0 {
		return nil, ErrNoBackends
	}

	var best *Backend
	for _, b := range backends {
		if !b.IsAlive() {
			continue
		}
		if best == nil || b.ActiveConnections.Load() < best.ActiveConnections.Load() {
			best = b
		}
	}

	if best == nil {
		return nil, ErrNoBackends
	}

	return best, nil
}
