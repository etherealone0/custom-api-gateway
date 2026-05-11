package balancer

import "errors"

var ErrNoBackends = errors.New("no healthy backends available")

// Balancer picks next healthy backend
type Balancer interface {
	NextServer() (*Backend, error)
}
