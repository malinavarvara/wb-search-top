package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/malinavarvara/wb-search-top/searcher_top/config"
)

func TestMustLoad_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
env: "test_env"

http:
  port: "9090"
  timeout: "5s"
  handler_timeout: "3s"

kafka:
  brokers: ["kafka1:9092", "kafka2:9092"]
  topic: "test-topic"
  consumer_group: "test-group"

app:
  window_duration: "10m"
  bucket_size: "20s"
  max_top_limit: 200

rate_limit:
  rps: 50
  burst: 10
`

	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg := config.MustLoad(cfgPath)

	if cfg.Env != "test_env" {
		t.Errorf("Env = %q, want %q", cfg.Env, "test_env")
	}

	if cfg.HTTP.Port != "9090" {
		t.Errorf("HTTP.Port = %q, want %q", cfg.HTTP.Port, "9090")
	}
	if cfg.HTTP.Timeout != 5*time.Second {
		t.Errorf("HTTP.Timeout = %v, want 5s", cfg.HTTP.Timeout)
	}
	if cfg.HTTP.HandlerTimeout != 3*time.Second {
		t.Errorf("HTTP.HandlerTimeout = %v, want 3s", cfg.HTTP.HandlerTimeout)
	}

	if len(cfg.Kafka.Brokers) != 2 {
		t.Fatalf("Kafka.Brokers len = %d, want 2", len(cfg.Kafka.Brokers))
	}
	if cfg.Kafka.Brokers[0] != "kafka1:9092" || cfg.Kafka.Brokers[1] != "kafka2:9092" {
		t.Errorf("Kafka.Brokers = %v, want [kafka1:9092 kafka2:9092]", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.Topic != "test-topic" {
		t.Errorf("Kafka.Topic = %q, want %q", cfg.Kafka.Topic, "test-topic")
	}
	if cfg.Kafka.ConsumerGroup != "test-group" {
		t.Errorf("Kafka.ConsumerGroup = %q, want %q", cfg.Kafka.ConsumerGroup, "test-group")
	}

	if cfg.App.WindowDuration != 10*time.Minute {
		t.Errorf("App.WindowDuration = %v, want 10m", cfg.App.WindowDuration)
	}
	if cfg.App.BucketSize != 20*time.Second {
		t.Errorf("App.BucketSize = %v, want 20s", cfg.App.BucketSize)
	}
	if cfg.App.MaxTopLimit != 200 {
		t.Errorf("App.MaxTopLimit = %d, want 200", cfg.App.MaxTopLimit)
	}

	if cfg.RateLimit.RPS != 50 {
		t.Errorf("RateLimit.RPS = %d, want 50", cfg.RateLimit.RPS)
	}
	if cfg.RateLimit.Burst != 10 {
		t.Errorf("RateLimit.Burst = %d, want 10", cfg.RateLimit.Burst)
	}
}

func TestMustLoad_FromEnv(t *testing.T) {
	oldEnv := map[string]string{
		"KAFKA_BROKERS": os.Getenv("KAFKA_BROKERS"),
		"ENV":           os.Getenv("ENV"),
	}
	defer func() {
		for k, v := range oldEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	os.Setenv("KAFKA_BROKERS", "env-kafka:9093,backup:9093")
	cfg := config.MustLoad("") // только env

	if len(cfg.Kafka.Brokers) != 2 {
		t.Fatalf("Kafka.Brokers len = %d, want 2", len(cfg.Kafka.Brokers))
	}
	expectedBrokers := []string{"env-kafka:9093", "backup:9093"}
	for i, b := range cfg.Kafka.Brokers {
		if b != expectedBrokers[i] {
			t.Errorf("Kafka.Brokers[%d] = %q, want %q", i, b, expectedBrokers[i])
		}
	}

	if cfg.Env != "local" {
		t.Errorf("Env = %q, want default 'local'", cfg.Env)
	}
	if cfg.HTTP.Port != "8080" {
		t.Errorf("HTTP.Port = %q, want default 8080", cfg.HTTP.Port)
	}

	if cfg.Kafka.Topic != "search-logs" {
		t.Errorf("Kafka.Topic = %q, want default 'search-logs'", cfg.Kafka.Topic)
	}
}
