package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Aditya03-D/custom-api-gateway/internal/balancer"
	"github.com/Aditya03-D/custom-api-gateway/internal/config"
	"github.com/Aditya03-D/custom-api-gateway/internal/resilience"
)

type ReverseProxy struct {
	timeout   time.Duration
	transport http.RoundTripper
	cbCfg     config.CircuitBreakerConfig
	retryCfg  config.RetryConfig
	breakers  sync.Map
}

func New(timeout time.Duration, cbCfg config.CircuitBreakerConfig, retryCfg config.RetryConfig) *ReverseProxy {
	return &ReverseProxy{
		timeout:   timeout,
		transport: http.DefaultTransport,
		cbCfg:     cbCfg,
		retryCfg:  retryCfg,
	}
}

func (rp *ReverseProxy) getBreaker(backendURL string) *resilience.CircuitBreaker {
	if cb, ok := rp.breakers.Load(backendURL); ok {
		return cb.(*resilience.CircuitBreaker)
	}
	cb := resilience.NewCircuitBreaker(
		backendURL,
		rp.cbCfg.FailureThreshold,
		rp.cbCfg.SuccessThreshold,
		rp.cbCfg.Timeout,
	)
	actual, _ := rp.breakers.LoadOrStore(backendURL, cb)
	return actual.(*resilience.CircuitBreaker)
}

func (rp *ReverseProxy) Forward(w http.ResponseWriter, r *http.Request, bal balancer.Balancer) {
	var lastBackendURL string

	err := resilience.Retry(r.Context(), rp.retryCfg.MaxAttempts, rp.retryCfg.BaseDelay, rp.retryCfg.MaxDelay,
		func(attempt int) error {
			backend, err := bal.NextServer()
			if err != nil {
				return err
			}
			lastBackendURL = backend.URL.String()

			cb := rp.getBreaker(lastBackendURL)
			if err := cb.Allow(); err != nil {
				return err
			}

			backend.ActiveConnections.Add(1)
			defer backend.ActiveConnections.Add(-1)

			resp, proxyErr := rp.doProxy(r, backend.URL)
			if proxyErr != nil {
				cb.RecordFailure()
				slog.Warn("proxy attempt failed",
					"backend", lastBackendURL,
					"attempt", attempt+1,
					"error", proxyErr,
				)
				return proxyErr
			}
			defer resp.Body.Close()

			if isRetryable(resp.StatusCode) {
				cb.RecordFailure()
				io.Copy(io.Discard, resp.Body)
				slog.Warn("retryable response",
					"backend", lastBackendURL,
					"attempt", attempt+1,
					"status", resp.StatusCode,
				)
				return fmt.Errorf("retryable status %d", resp.StatusCode)
			}

			cb.RecordSuccess()
			writeResponse(w, resp)
			return nil
		})

	if err != nil {
		if errors.Is(err, balancer.ErrNoBackends) {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		} else if errors.Is(err, resilience.ErrCircuitOpen) {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		} else if r.Context().Err() != nil {
			http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
		} else {
			slog.Error("all proxy attempts failed", "backend", lastBackendURL, "error", err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		}
	}
}

func (rp *ReverseProxy) doProxy(r *http.Request, target *url.URL) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(r.Context(), rp.timeout)
	defer cancel()

	outReq := r.Clone(ctx)
	outReq.URL.Scheme = target.Scheme
	outReq.URL.Host = target.Host
	outReq.Host = target.Host
	outReq.RequestURI = ""

	resp, err := rp.transport.RoundTrip(outReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("backend timeout after %v", rp.timeout)
		}
		return nil, err
	}
	return resp, nil
}

func isRetryable(statusCode int) bool {
	return statusCode == http.StatusBadGateway ||
		statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusGatewayTimeout
}

func writeResponse(w http.ResponseWriter, resp *http.Response) {
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
