package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/osamah22/payments-microservice/internal/platform/postgres"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	dns := os.Getenv("WALLET_DB_URL")
	if dns == "" {
		// Local development: fine to commit.
		dns = "postgres://payments:payments@localhost:5432/wallet?sslmode=disable"
	}
	pool, err := postgres.NewPool(context.Background(), dns)
	if err != nil {
		logger.Error("connect to postgres", "err", err)
		os.Exit(1)
	}
	logger.Info("connected to postgres", "max_conns", pool.Config().MaxConns)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("health check", "remote", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{status:ok}` + "\n"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")

		if err := pool.Ping(ctx); err != nil {
			logger.Warn("not ready: database unreachable", "err", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{status:unavailable}` + "\n"))
			return
		}
		w.Write([]byte(`{status:ready}` + "\n"))
	})

	server := &http.Server{
		Addr:        ":8081",
		Handler:     mux,
		ReadTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("wallet service listing", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down gracefull")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "err", err)
	}
	logger.Info("server stopped")

	// close the pool last: requests that were still finishing needed it.
	pool.Close()
	logger.Info("database pool closed")
}
