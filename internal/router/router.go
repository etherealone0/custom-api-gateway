package router

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/etherealone0/custom-api-gateway/internal/balancer"
	"github.com/etherealone0/custom-api-gateway/internal/config"
)

type Route struct {
	Path        string
	StripPrefix bool
	Balancer    balancer.Balancer
}

type Router struct {
	routes []Route
	pools  []*balancer.ServerPool
}

func New(cfgRoutes []config.RouteConfig, oldRouter *Router) (*Router, error) {
	oldBackends := make(map[string]*balancer.Backend)
	if oldRouter != nil {
		for _, b := range oldRouter.GetAllBackends() {
			oldBackends[b.URL.String()] = b
		}
	}

	routes := make([]Route, len(cfgRoutes))
	pools := make([]*balancer.ServerPool, len(cfgRoutes))

	for i, cr := range cfgRoutes {
		path := cr.Path
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}

		backends := make([]*balancer.Backend, len(cr.Backends))
		for j, cb := range cr.Backends {
			if existing, ok := oldBackends[cb.URL]; ok {
				existing.Weight = cb.Weight // Update weight if it changed
				backends[j] = existing
				continue
			}

			b, err := balancer.NewBackend(cb.URL, cb.Weight)
			if err != nil {
				return nil, fmt.Errorf("route %s: invalid backend URL %q: %w", cr.Path, cb.URL, err)
			}
			backends[j] = b
		}

		pool := balancer.NewServerPool(backends)
		pools[i] = pool

		bal, err := balancer.NewBalancer(cr.Strategy, pool)
		if err != nil {
			return nil, fmt.Errorf("route %s: %w", cr.Path, err)
		}

		routes[i] = Route{
			Path:        path,
			StripPrefix: cr.StripPrefix,
			Balancer:    bal,
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i].Path) > len(routes[j].Path)
	})

	return &Router{routes: routes, pools: pools}, nil
}

func (rt *Router) Match(r *http.Request) *Route {
	reqPath := r.URL.Path
	if !strings.HasSuffix(reqPath, "/") {
		reqPath += "/"
	}

	for i := range rt.routes {
		if strings.HasPrefix(reqPath, rt.routes[i].Path) {
			if rt.routes[i].StripPrefix {
				stripped := strings.TrimPrefix(r.URL.Path, strings.TrimSuffix(rt.routes[i].Path, "/"))
				if stripped == "" {
					stripped = "/"
				}
				r.URL.Path = stripped
			}

			return &rt.routes[i]
		}
	}

	return nil
}

func (rt *Router) GetAllBackends() []*balancer.Backend {
	var all []*balancer.Backend
	for _, pool := range rt.pools {
		all = append(all, pool.GetBackends()...)
	}
	return all
}
