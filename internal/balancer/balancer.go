package balancer

import (
	"errors"
	"fmt"
)

var ErrNoBackends = errors.New("no healthy backends available")

type Balancer interface {
	NextServer() (*Backend, error)
}

func NewBalancer(strategy string, pool *ServerPool) (Balancer, error) {
	switch strategy {
	case "round_robin":
		return NewRoundRobin(pool), nil
	case "weighted_round_robin":
		return NewWeightedRoundRobin(pool), nil
	case "least_conn":
		return NewLeastConnections(pool), nil
	default:
		return nil, fmt.Errorf("unknown balancer strategy: %s", strategy)
	}
}
