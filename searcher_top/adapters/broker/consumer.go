package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/malinavarvara/wb-search-top/searcher_top/core"
)

type Config struct {
	Brokers       []string
	Topic         string
	ConsumerGroup string
	MinBytes      int
	MaxBytes      int
	MaxWait       time.Duration
}

func DefaultConfig(brokers []string, topic, group string) Config {
	return Config{
		Brokers:       brokers,
		Topic:         topic,
		ConsumerGroup: group,
		MinBytes:      10_000,     // 10 KB
		MaxBytes:      10_000_000, // 10 MB
		MaxWait:       500 * time.Millisecond,
	}
}

type Consumer struct {
	reader  *kafka.Reader
	service core.SearchService
	log     *slog.Logger
}

func NewConsumer(cfg Config, service core.SearchService, log *slog.Logger) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.ConsumerGroup,
		MinBytes:       cfg.MinBytes,
		MaxBytes:       cfg.MaxBytes,
		MaxWait:        cfg.MaxWait,
		CommitInterval: 0, // ручной коммит
		StartOffset:    kafka.LastOffset,
		ErrorLogger: kafka.LoggerFunc(func(msg string, args ...interface{}) {
			log.Error("kafka reader error", slog.String("msg", fmt.Sprintf(msg, args...)))
		}),
	})

	return &Consumer{
		reader:  reader,
		service: service,
		log:     log,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("kafka consumer started",
		slog.String("topic", c.reader.Config().Topic),
		slog.String("group", c.reader.Config().GroupID),
		slog.Any("brokers", c.reader.Config().Brokers),
	)

	defer func() {
		if err := c.reader.Close(); err != nil {
			c.log.Error("failed to close kafka reader", slog.Any("err", err))
		}
		c.log.Info("kafka consumer stopped")
	}()

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				c.log.Warn("kafka connection lost, retrying...", slog.Any("err", err))
				continue
			}
			c.log.Error("failed to fetch message", slog.Any("err", err))
			continue
		}
		c.processMessage(ctx, msg)
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message) {
	var event core.SearchEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		c.log.Warn("failed to parse message, skipping",
			slog.Any("err", err),
			slog.Int64("offset", msg.Offset),
			slog.String("value_preview", truncate(string(msg.Value), 100)),
		)
		c.commit(ctx, msg)
		return
	}

	if err := c.service.ProcessEvent(ctx, event); err != nil {
		if !errors.Is(err, core.ErrEmptyQuery) {
			c.log.Warn("failed to process event",
				slog.Any("err", err),
				slog.String("query", event.Query),
			)
		}
	}

	c.commit(ctx, msg)
}

func (c *Consumer) commit(ctx context.Context, msg kafka.Message) {
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		if !errors.Is(err, context.Canceled) {
			c.log.Error("failed to commit message",
				slog.Any("err", err),
				slog.Int64("offset", msg.Offset),
			)
		}
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
