package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type ReverseProxy struct {
	timeout time.Duration
}

func New(timeout time.Duration) *ReverseProxy {
	return &ReverseProxy{timeout: timeout}
}

func (rp *ReverseProxy) Forward(w http.ResponseWriter, r *http.Request, target string) {
	targetURL, err := url.Parse(target)
	if err != nil {
		slog.Error("invalid backend URL", "url", target, "error", err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), rp.timeout)
	defer cancel()
	r = r.WithContext(ctx)

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.Host = targetURL.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if ctx.Err() == context.DeadlineExceeded {
				slog.Warn("backend timeout", "url", target, "timeout", rp.timeout)
				http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
				return
			}
			slog.Error("proxy error", "url", target, "error", err)
			http.Error(w, fmt.Sprintf("bad gateway: %v", err), http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}
