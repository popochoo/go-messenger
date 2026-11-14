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

	"github.com/popochoo/go-messenger/internal/config"
	"github.com/popochoo/go-messenger/internal/repository/postgres"
	"github.com/popochoo/go-messenger/internal/repository/redis"
	"github.com/popochoo/go-messenger/internal/service"
	transport_http "github.com/popochoo/go-messenger/internal/transport/http"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.MustLoad()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pgPool, err := postgres.NewPool(ctx, cfg.PostgresDSN())
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pgPool.Close()
	slog.Info("connected to PostgreSQL")

	rdb, err := redis.NewClient(ctx, cfg.RedisAddr(), cfg.RedisPassword)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()
	slog.Info("connected to Redis")

	userRepository := postgres.NewUserRepository(pgPool)
	authService := service.NewAuthService(userRepository, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	authHandler := transport_http.NewAuthHandler(authService)

	chatRepository := postgres.NewChatRepository(pgPool)
	chatService := service.NewChatService(chatRepository, userRepository)
	chatHandler := transport_http.NewChatHandler(chatService)

	router := transport_http.NewRouter(transport_http.Handlers{
		Auth:        authHandler,
		Chat:        chatHandler,
		TokenParser: authService,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.HttpPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("starting HTTP server", "port", cfg.HttpPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server exited properly")
}
