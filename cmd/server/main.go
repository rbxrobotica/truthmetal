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

	"connectrpc.com/connect"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/ldamasio/truthmetal/gen/truthmetal/v1/truthmetalv1connect"
	"github.com/ldamasio/truthmetal/internal/api"
	"github.com/ldamasio/truthmetal/internal/cache"
	"github.com/ldamasio/truthmetal/internal/consensus"
	"github.com/ldamasio/truthmetal/internal/ledger"
	"github.com/ldamasio/truthmetal/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg := loadConfig()

	// Database
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db connect: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}
	slog.Info("database connected")

	// Migrations
	if err := runMigrations(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	slog.Info("migrations applied")

	// Redis
	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("redis url: %w", err)
	}
	redisClient := redis.NewClient(redisOpt)
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	slog.Info("redis connected")

	// Wire up services
	pgStore := store.NewPostgresStore(pool)
	redisCache := cache.NewCache(redisClient)
	quorum := consensus.NewSimpleQuorum(pgStore)
	svc := ledger.NewService(pgStore, redisCache, quorum)
	srv := api.NewServer(svc)

	// Connect-go mux — serves gRPC, gRPC-Web, and Connect (HTTP/JSON) on the same port
	mux := http.NewServeMux()
	mux.Handle(truthmetalv1connect.NewTruthMetalServiceHandler(srv,
		connect.WithInterceptors(loggingInterceptor()),
	))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	httpSrv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      h2c.NewHandler(mux, &http2.Server{}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		slog.Info("TruthMetal listening", "addr", cfg.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return httpSrv.Shutdown(shutdownCtx)
}

func loggingInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			res, err := next(ctx, req)
			slog.Info("rpc",
				"procedure", req.Spec().Procedure,
				"duration_ms", time.Since(start).Milliseconds(),
				"err", err,
			)
			return res, err
		}
	}
}

type config struct {
	DatabaseURL    string
	RedisURL       string
	Addr           string
	MigrationsPath string
}

func loadConfig() config {
	return config{
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://truthmetal:truthmetal@localhost:5432/truthmetal?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379/0"),
		Addr:           getEnv("ADDR", ":8080"),
		MigrationsPath: getEnv("MIGRATIONS_PATH", "file://migrations"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func runMigrations(dbURL, migrationsPath string) error {
	m, err := migrate.New(migrationsPath, dbURL)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
