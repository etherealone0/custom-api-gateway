package router

import (
	"net/http"
	"sort"
	"strings"

	"github.com/Aditya03-D/custom-api-gateway/internal/config"
)

type Route struct {
	Path        string
	StripPrefix bool
	Strategy    string
	Backends    []config.BackendConfig
}

type Router struct {
	routes []Route
}

func New(cfgRoutes []config.RouteConfig) *Router {
	routes := make([]Route, len(cfgRoutes))

	for i, cr := range cfgRoutes {
		path := cr.Path
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}

		routes[i] = Route{
			Path:        path,
			StripPrefix: cr.StripPrefix,
			Strategy:    cr.Strategy,
			Backends:    cr.Backends,
		}
	}

	// Sort by path length descending — longest prefix first.
	// When Match() iterates in order, the first hit is the best match.
	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i].Path) > len(routes[j].Path)
	})

	return &Router{routes: routes}
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
