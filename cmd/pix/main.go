package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dracma/pix/internal/infrastructure/tigerbeetle"
	"dracma/pix/internal/presentation/httpapi"
)

func main() {
	if err := run(); err != nil {
		slog.Error("exit", "error", err)
		os.Exit(1)
	}
}

func run() error {
	seed := flag.Int64("seed", 0, "create this many demo accounts")
	first := flag.Int64("first", 1, "id of the first demo account")
	balance := flag.Int64("balance", 1000000000000, "initial BRL cents for demo accounts")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ledger, err := tigerbeetle.Open(os.Getenv("TB_ADDRESS"))
	if err != nil {
		return err
	}
	defer ledger.Close()
	boot, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err = ledger.Bootstrap(boot); err != nil {
		return err
	}
	if *seed > 0 {
		return ledger.Seed(ctx, *first, *seed, *balance)
	}
	key := os.Getenv("API_KEY")
	if len(key) < 16 {
		return errors.New("API_KEY must have at least 16 characters")
	}
	api := httpapi.API{Ledger: ledger, Key: key}
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
