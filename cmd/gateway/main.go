package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Aditya03-D/custom-api-gateway/internal/config"
	"github.com/Aditya03-D/custom-api-gateway/internal/middleware"
	"github.com/Aditya03-D/custom-api-gateway/internal/proxy"
	"github.com/Aditya03-D/custom-api-gateway/internal/router"
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

	rt := router.New(cfg.Routes)
	rp := proxy.New(cfg.Server.WriteTimeout)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := rt.Match(r)
		if route == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		// No balancer yet
		target := route.Backends[0].URL
		rp.Forward(w, r, target)
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      middleware.Logger(handler),
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

	// Block until we receive SIGINT or SIGTERM 
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutdown signal received", "signal", sig.String())

	// Give in-flight requests 30 seconds to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("gateway stopped gracefully")
}
