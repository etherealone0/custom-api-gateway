package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

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

	slog.Info("starting gateway", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
