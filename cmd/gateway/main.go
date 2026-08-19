package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/etherealone0/custom-api-gateway/internal/balancer"
	"github.com/etherealone0/custom-api-gateway/internal/config"
	"github.com/etherealone0/custom-api-gateway/internal/health"
	"github.com/etherealone0/custom-api-gateway/internal/middleware"
	"github.com/etherealone0/custom-api-gateway/internal/proxy"
	"github.com/etherealone0/custom-api-gateway/internal/router"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	slog.Info("config loaded", "port", cfg.Server.Port, "routes", len(cfg.Routes))

	rt, err := router.New(cfg.Routes, nil)
	if err != nil {
		slog.Error("failed to build router", "error", err)
		os.Exit(1)
	}
	rp := proxy.New(cfg.Server.WriteTimeout, cfg.CircuitBreaker, cfg.Retry)

	var mu sync.RWMutex

	healthCtx, healthCancel := context.WithCancel(context.Background())
	hc := health.New(
		cfg.HealthCheck.Interval,
		cfg.HealthCheck.Timeout,
		cfg.HealthCheck.Path,
		func() []*balancer.Backend {
			mu.RLock()
			currentRouter := rt
			mu.RUnlock()
			return currentRouter.GetAllBackends()
		},
	)
	go hc.Start(healthCtx)

	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		currentRouter := rt
		currentProxy := rp
		mu.RUnlock()

		route := currentRouter.Match(r)
		if route == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		currentProxy.Forward(w, r, route.Balancer)
	})

	rl := middleware.NewRateLimiter(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Burst)
	corsHandler := middleware.NewCORS(cfg.CORS.AllowedOrigins, cfg.CORS.AllowedMethods, cfg.CORS.AllowedHeaders)
	jwtAuth := middleware.JWTAuth(cfg.Auth.JWTSecret)

	var apiHandler http.Handler = proxyHandler
	apiHandler = rl.Middleware(apiHandler)
	apiHandler = jwtAuth(apiHandler)
	apiHandler = corsHandler.Middleware(apiHandler)

	mux := http.NewServeMux()
	mux.Handle("/metrics", middleware.MetricsHandler())
	mux.Handle("/", apiHandler)

	var finalHandler http.Handler = mux
	finalHandler = middleware.Metrics(finalHandler)
	finalHandler = middleware.Logger(finalHandler)

	cfgWatcher, err := config.NewWatcher("config.yaml", func(newCfg *config.Config) {
		mu.RLock()
		currentRouter := rt
		mu.RUnlock()

		newRouter, err := router.New(newCfg.Routes, currentRouter)
		if err != nil {
			slog.Error("failed to build router from reloaded config", "error", err)
			return
		}

		mu.RLock()
		oldBackends := rt.GetAllBackends()
		mu.RUnlock()
		newBackends := newRouter.GetAllBackends()

		removed := balancer.FindRemovedBackends(oldBackends, newBackends)
		for _, b := range removed {
			b.MarkDraining()
		}

		mu.Lock()
		rt = newRouter
		rp = proxy.New(newCfg.Server.WriteTimeout, newCfg.CircuitBreaker, newCfg.Retry)
		mu.Unlock()

		slog.Info("router and proxy updated from reloaded config")

		if len(removed) > 0 {
			slog.Info("draining removed backends", "count", len(removed))
			go balancer.DrainAll(removed, 30*time.Second)
		}
	})
	if err != nil {
		slog.Error("failed to start config watcher", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      finalHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		var err error
		if cfg.Server.TLS.Enabled {
			slog.Info("starting gateway with TLS", "addr", server.Addr)
			err = server.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
		} else {
			slog.Info("starting gateway", "addr", server.Addr)
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutdown signal received", "signal", sig.String())

	healthCancel()
	cfgWatcher.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("gateway stopped gracefully")
}
