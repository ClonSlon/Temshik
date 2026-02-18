package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/bludnic/temchik"
	"github.com/bludnic/temchik/internal/db"
	"github.com/bludnic/temchik/internal/trpc"
)

type Config struct {
	Host          string
	Port          int
	AdminPassword string
}

func Run(ctx context.Context, cfg Config) error {
	frontend, err := temchik.FrontendFS()
	if err != nil {
		return fmt.Errorf("embed frontend: %w", err)
	}

	sqlDB, err := openAndMigrateDB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	return runWithFS(ctx, cfg, frontend, sqlDB)
}

func runWithFS(ctx context.Context, cfg Config, frontend fs.FS, sqlDB *sql.DB) error {
	mux := http.NewServeMux()
	mux.Handle("/api/trpc/", trpc.NewHandler(cfg.AdminPassword, sqlDB))
	mux.Handle("/", newSPAHandler(frontend))

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("HTTP server listening", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func openAndMigrateDB() (*sql.DB, error) {
	migrationsFS, err := temchik.MigrationsFS()
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.Open()
	if err != nil {
		return nil, err
	}

	if err := db.ApplyPrismaMigrations(sqlDB, migrationsFS); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := db.Seed(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	return sqlDB, nil
}
