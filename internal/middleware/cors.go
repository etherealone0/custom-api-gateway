package middleware

import (
	"net/http"
	"strings"
)

type CORS struct {
	allowedOrigins map[string]bool
	allowAll       bool
	methods        string
	headers        string
}

func NewCORS(origins, methods, headers []string) *CORS {
	c := &CORS{
		allowedOrigins: make(map[string]bool, len(origins)),
		methods:        strings.Join(methods, ", "),
		headers:        strings.Join(headers, ", "),
	}
	for _, o := range origins {
		if o == "*" {
			c.allowAll = true
			break
		}
		c.allowedOrigins[o] = true
	}
	return c
}

func (c *CORS) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin != "" {
			if c.allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if c.allowedOrigins[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Methods", c.methods)
			w.Header().Set("Access-Control-Allow-Headers", c.headers)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
