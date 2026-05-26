package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env       string          `yaml:"env" env-default:"local"`
	HTTP      HTTPConfig      `yaml:"http"`
	Kafka     KafkaConfig     `yaml:"kafka"`
	App       AppConfig       `yaml:"app"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

type HTTPConfig struct {
	Port           string        `yaml:"port" env-default:"8080"`
	Timeout        time.Duration `yaml:"timeout" env-default:"4s"`
	HandlerTimeout time.Duration `yaml:"handler_timeout" env-default:"2s"`
}

type KafkaConfig struct {
	Brokers       []string `yaml:"brokers" env:"KAFKA_BROKERS"`
	Topic         string   `yaml:"topic" env-default:"search-logs"`
	ConsumerGroup string   `yaml:"consumer_group" env-default:"searcher-top-group"`
}

type AppConfig struct {
	WindowDuration time.Duration `yaml:"window_duration" env-default:"5m"`
	BucketSize     time.Duration `yaml:"bucket_size" env-default:"10s"`
	MaxTopLimit    int           `yaml:"max_top_limit" env-default:"100"`
}

type RateLimitConfig struct {
	RPS   int `yaml:"rps" env-default:"100"`
	Burst int `yaml:"burst" env-default:"20"`
}

func MustLoad(configPath string) *Config {
	var cfg Config

	if configPath != "" {
		if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
			slog.Error("failed to read config file", "path", configPath, "error", err)
			os.Exit(1)
		}
		slog.Info("config loaded from file", "path", configPath)
	} else {
		slog.Info("loading config from environment variables")
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			slog.Error("failed to read config from env", "error", err)
			os.Exit(1)
		}
		slog.Info("config loaded from env")
	}
	return &cfg
}
