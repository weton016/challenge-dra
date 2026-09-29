package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"dracma/pix/internal/httpapi"
	"dracma/pix/internal/postgres"
)

func batchSettings() (int, time.Duration, error) {
	size := 64
	window := 15 * time.Millisecond
	if value := os.Getenv("BATCH_SIZE"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 64 {
			return 0, 0, errors.New("BATCH_SIZE must be from 1 to 64")
		}
		size = parsed
	}
	if value := os.Getenv("BATCH_WINDOW_MS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 50 {
			return 0, 0, errors.New("BATCH_WINDOW_MS must be from 1 to 50")
		}
		window = time.Duration(parsed) * time.Millisecond
	}
	return size, window, nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("exit", "error", err)
		os.Exit(1)
	}
}

func run() error {
	seed := flag.Int64("seed", 0, "create this many demo accounts")
	balance := flag.Int64("balance", 1000000000000, "initial BRL cents for demo accounts")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := postgres.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Pool.Close()
	if err = store.Migrate(ctx); err != nil {
		return err
	}
	if *seed > 0 {
		return store.Seed(ctx, *seed, *balance)
	}
	key := os.Getenv("API_KEY")
	if len(key) < 16 {
		return errors.New("API_KEY must have at least 16 characters")
	}
	api := httpapi.API{Store: store, Key: key}
	if os.Getenv("APP_MODE") != "simulator" {
		size, window, err := batchSettings()
		if err != nil {
			return err
		}
		api.Batch = postgres.NewBatcher(ctx, store, size, window)
		slog.Info("batching", "size", size, "window", window)
	}
	if url := os.Getenv("EXTERNAL_URL"); url != "" {
		go api.RunWorker(ctx, url)
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := http.Server{Addr: addr, Handler: api.Handler(os.Getenv("APP_MODE") == "simulator"), ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	slog.Info("listening", "address", addr)
	err = server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
