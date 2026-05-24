package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/malinavarvara/wb-search-top/searcher_top/config"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()
	cfg := config.MustLoad(configPath)

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(log)

	slog.Info("service starting", slog.String("env", cfg.Env))
}
