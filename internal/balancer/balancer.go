package balancer

import "github.com/Aditya03-D/custom-api-gateway/internal/config"

// Balancer picks next healthy backend
type Balancer interface {
	NextServer() (*config.BackendConfig, error)
}
