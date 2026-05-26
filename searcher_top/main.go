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

	"github.com/malinavarvara/wb-search-top/searcher_top/adapters/api"
	"github.com/malinavarvara/wb-search-top/searcher_top/adapters/broker"
	"github.com/malinavarvara/wb-search-top/searcher_top/adapters/db"
	"github.com/malinavarvara/wb-search-top/searcher_top/config"
	"github.com/malinavarvara/wb-search-top/searcher_top/core"
)

func main() {
	cfg := config.MustLoad("searcher_top/config.yaml")
	log := setupLogger(cfg.Env)
	log.Info("starting searcher_top",
		slog.String("env", cfg.Env),
		slog.String("port", cfg.HTTP.Port),
	)

	topRepo := db.NewTopRepository(
		cfg.App.WindowDuration,
		cfg.App.BucketSize,
	)
	stopRepo := db.NewStopListRepository()
	service := core.NewService(topRepo, stopRepo, log, cfg.App.MaxTopLimit)

	handler := api.NewHandler(service, log)
	router := api.NewRouter(handler, log, api.RouterConfig{
		HandlerTimeout: cfg.HTTP.HandlerTimeout,
		RateLimitRPS:   float64(cfg.RateLimit.RPS),
		RateLimitBurst: cfg.RateLimit.Burst,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.HTTP.Port,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.Timeout,
		WriteTimeout: cfg.HTTP.Timeout,
		IdleTimeout:  cfg.HTTP.Timeout * 3,
	}

	consumerCfg := broker.DefaultConfig(
		cfg.Kafka.Brokers,
		cfg.Kafka.Topic,
		cfg.Kafka.ConsumerGroup,
	)
	consumer := broker.NewConsumer(consumerCfg, service, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := consumer.Run(ctx); err != nil {
			log.Error("kafka consumer stopped with error", slog.Any("err", err))
		}
	}()

	go func() {
		log.Info("http server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", slog.Any("err", err))
			stop() // инициируем shutdown если сервер упал
		}
	}()

	<-ctx.Done()
	log.Info("shutdown signal received, shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http server shutdown failed", slog.Any("err", err))
	}

	log.Info("shutdown complete")
}

func setupLogger(env string) *slog.Logger {
	var handler slog.Handler

	switch env {
	case "local":
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	default:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	return slog.New(handler)
}
