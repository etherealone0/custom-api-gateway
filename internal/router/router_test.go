package router

import (
	"net/http/httptest"
	"testing"

	"github.com/etherealone0/custom-api-gateway/internal/config"
)

func TestRouter_Match(t *testing.T) {
	cfgRoutes := []config.RouteConfig{
		{
			Path:        "/api",
			StripPrefix: false,
			Strategy:    "round_robin",
			Backends:    []config.BackendConfig{{URL: "http://b1", Weight: 1}},
		},
		{
			Path:        "/api/users",
			StripPrefix: true,
			Strategy:    "round_robin",
			Backends:    []config.BackendConfig{{URL: "http://b2", Weight: 1}},
		},
		{
			Path:        "/api/users/profile",
			StripPrefix: true,
			Strategy:    "round_robin",
			Backends:    []config.BackendConfig{{URL: "http://b3", Weight: 1}},
		},
	}

	rt, err := New(cfgRoutes, nil)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	tests := []struct {
		name          string
		reqPath       string
		wantRoutePath string
		wantFinalPath string
		shouldMatch   bool
	}{
		{
			name:          "Longest Prefix Match - Profile",
			reqPath:       "/api/users/profile/details",
			wantRoutePath: "/api/users/profile/",
			wantFinalPath: "/details", // StripPrefix is true
			shouldMatch:   true,
		},
		{
			name:          "Longest Prefix Match - Users",
			reqPath:       "/api/users/123",
			wantRoutePath: "/api/users/",
			wantFinalPath: "/123", // StripPrefix is true
			shouldMatch:   true,
		},
		{
			name:          "Match Base API",
			reqPath:       "/api/products",
			wantRoutePath: "/api/",
			wantFinalPath: "/api/products", // StripPrefix is false
			shouldMatch:   true,
		},
		{
			name:          "Trailing Slash Normalization - Match",
			reqPath:       "/api",
			wantRoutePath: "/api/",
			wantFinalPath: "/api",
			shouldMatch:   true,
		},
		{
			name:          "Trailing Slash Normalization - Strip Match",
			reqPath:       "/api/users",
			wantRoutePath: "/api/users/",
			wantFinalPath: "/",
			shouldMatch:   true,
		},
		{
			name:        "No Match",
			reqPath:     "/unknown",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.reqPath, nil)
			route := rt.Match(req)

			if !tt.shouldMatch {
				if route != nil {
					t.Errorf("expected no match, got %v", route.Path)
				}
				return
			}

			if route == nil {
				t.Fatalf("expected match, got nil")
			}

			if route.Path != tt.wantRoutePath {
				t.Errorf("expected matched route path %s, got %s", tt.wantRoutePath, route.Path)
			}

			if req.URL.Path != tt.wantFinalPath {
				t.Errorf("expected rewritten req path %s, got %s", tt.wantFinalPath, req.URL.Path)
			}
		})
	}
}
