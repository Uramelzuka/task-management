package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	"github.com/example/task-management/backend/internal/cache"
	"github.com/example/task-management/backend/internal/handler"
	"github.com/example/task-management/backend/internal/repository"
	"github.com/example/task-management/backend/internal/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	port := env("PORT", "8080")
	// clientFoundRows=true: UPDATE reports matched rows, not changed rows.
	dsn := env("DB_DSN", "task:task@tcp(localhost:3307)/taskdb?parseTime=true&clientFoundRows=true&loc=UTC")
	redisAddr := env("REDIS_ADDR", "localhost:6379")
	ttl, err := time.ParseDuration(env("CACHE_TTL", "60s"))
	if err != nil {
		return err
	}
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := pingWithRetry(func(ctx context.Context) error { return db.PingContext(ctx) }); err != nil {
		return err
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()
	// Redis being down is not fatal: the service falls back to MySQL.
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Warn("redis unavailable at startup; serving without cache", "err", err)
	}

	svc := service.NewTaskService(
		repository.NewMySQLTaskRepository(db),
		cache.NewRedisTaskCache(rdb, ttl),
		slog.Default(),
	)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler.NewRouter(handler.NewTaskHandler(svc)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			stop()
		}
	}()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func pingWithRetry(ping func(context.Context) error) error {
	var err error
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = ping(ctx)
		cancel()
		if err == nil {
			return nil
		}
		slog.Info("waiting for mysql", "attempt", i+1, "err", err)
		time.Sleep(time.Second)
	}
	return err
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
