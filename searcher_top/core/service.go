package core

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode"
)

type topRepo interface {
	Increment(query, userID string, ts time.Time)
	GetTop(n int) []TopItem
}

type stopRepo interface {
	Add(word string)
	Remove(word string)
	Contains(word string) bool
	GetAll() []string
}

type Service struct {
	top         topRepo
	stopList    stopRepo
	log         *slog.Logger
	maxTopLimit int
}

func NewService(
	top topRepo,
	stopList stopRepo,
	log *slog.Logger,
	maxTopLimit int,
) *Service {
	return &Service{
		top:         top,
		stopList:    stopList,
		log:         log,
		maxTopLimit: maxTopLimit,
	}
}

func (s *Service) ProcessEvent(_ context.Context, event SearchEvent) error {
	query := normalizeQuery(event.Query)
	if query == "" {
		s.log.Debug("skipping empty query", slog.String("raw", event.Query))
		return ErrEmptyQuery
	}

	if s.stopList.Contains(query) {
		s.log.Debug("query in stop list, skipping",
			slog.String("query", query),
		)
		return nil
	}

	userID := event.UserID
	if userID == "" {
		userID = "anon-" + event.Timestamp.Format("20060102150405.000")
	}

	ts := event.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	s.top.Increment(query, userID, ts)

	s.log.Debug("event processed",
		slog.String("query", query),
		slog.String("user_id", userID),
	)
	return nil
}

func (s *Service) GetTop(_ context.Context, n int) ([]TopItem, error) {
	if n <= 0 {
		n = s.maxTopLimit
	}
	if n > s.maxTopLimit {
		return nil, ErrInvalidLimit
	}

	raw := s.top.GetTop(n)

	result := make([]TopItem, 0, len(raw))
	for _, item := range raw {
		if !s.stopList.Contains(item.Query) {
			result = append(result, item)
		}
	}

	if n < len(result) {
		return result[:n], nil
	}
	return result, nil
}

func (s *Service) AddStopWord(_ context.Context, word string) error {
	if strings.TrimSpace(word) == "" {
		return ErrEmptyWord
	}
	s.stopList.Add(word)
	s.log.Info("stop word added", slog.String("word", word))
	return nil
}

func (s *Service) RemoveStopWord(_ context.Context, word string) error {
	if strings.TrimSpace(word) == "" {
		return ErrEmptyWord
	}
	s.stopList.Remove(word)
	s.log.Info("stop word removed", slog.String("word", word))
	return nil
}

func (s *Service) GetStopWords(_ context.Context) ([]string, error) {
	return s.stopList.GetAll(), nil
}

func normalizeQuery(s string) string {
	s = strings.ToLower(s)

	fields := strings.FieldsFunc(s, unicode.IsSpace)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}
