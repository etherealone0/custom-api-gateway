package balancer

import "sync"

type WeightedRoundRobin struct {
	pool           *ServerPool
	mu             sync.Mutex
	currentWeights []int
}

func NewWeightedRoundRobin(pool *ServerPool) *WeightedRoundRobin {
	backends := pool.GetBackends()
	return &WeightedRoundRobin{
		pool:           pool,
		currentWeights: make([]int, len(backends)),
	}
}

func (wrr *WeightedRoundRobin) NextServer() (*Backend, error) {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	backends := wrr.pool.GetBackends()
	total := len(backends)
	if total == 0 {
		return nil, ErrNoBackends
	}

	// Resize currentWeights if pool changed (hot-reload)
	if len(wrr.currentWeights) != total {
		wrr.currentWeights = make([]int, total)
	}

	totalWeight := 0
	for _, b := range backends {
		if b.IsAlive() {
			totalWeight += b.Weight
		}
	}

	if totalWeight == 0 {
		return nil, ErrNoBackends
	}

	bestIdx := -1
	for i, b := range backends {
		if !b.IsAlive() {
			wrr.currentWeights[i] = 0
			continue
		}

		wrr.currentWeights[i] += b.Weight
		if bestIdx == -1 || wrr.currentWeights[i] > wrr.currentWeights[bestIdx] {
			bestIdx = i
		}
	}

	wrr.currentWeights[bestIdx] -= totalWeight

	return backends[bestIdx], nil
}
