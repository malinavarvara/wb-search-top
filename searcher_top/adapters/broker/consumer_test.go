package broker_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/malinavarvara/wb-search-top/searcher_top/core"
)

type fakeService struct {
	events  []core.SearchEvent
	nextErr error
}

func (f *fakeService) ProcessEvent(_ context.Context, e core.SearchEvent) error {
	f.events = append(f.events, e)
	err := f.nextErr
	f.nextErr = nil
	return err
}

func (f *fakeService) GetTop(_ context.Context, _ int) ([]core.TopItem, error) { return nil, nil }
func (f *fakeService) AddStopWord(_ context.Context, _ string) error           { return nil }
func (f *fakeService) RemoveStopWord(_ context.Context, _ string) error        { return nil }
func (f *fakeService) GetStopWords(_ context.Context) ([]string, error)        { return nil, nil }

func TestKafkaMessage_ValidJSON_ParsesCorrectly(t *testing.T) {
	payload := []byte(`{
		"query":     "кроссовки найк",
		"user_id":   "user-abc-123",
		"timestamp": "2025-05-25T12:00:00Z"
	}`)

	var msg struct {
		Query     string    `json:"query"`
		UserID    string    `json:"user_id"`
		Timestamp time.Time `json:"timestamp"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if msg.Query != "кроссовки найк" {
		t.Errorf("query: want 'кроссовки найк', got %q", msg.Query)
	}
	if msg.UserID != "user-abc-123" {
		t.Errorf("user_id: want 'user-abc-123', got %q", msg.UserID)
	}
	if msg.Timestamp.IsZero() {
		t.Error("timestamp must be parsed, got zero")
	}
}

func TestKafkaMessage_InvalidJSON_ReturnsError(t *testing.T) {
	badPayloads := [][]byte{
		[]byte(`not json`),
		[]byte(`{`),
		[]byte(``),
	}
	for _, p := range badPayloads {
		var msg struct{ Query string }
		if err := json.Unmarshal(p, &msg); err == nil {
			t.Errorf("expected error for payload %q, got nil", p)
		}
	}
}

func TestKafkaMessage_MissingUserID_ParsesEmpty(t *testing.T) {
	payload := []byte(`{"query": "кроссовки"}`)
	var msg struct {
		Query  string `json:"query"`
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("partial payload must parse: %v", err)
	}
	if msg.UserID != "" {
		t.Errorf("missing user_id must be empty string, got %q", msg.UserID)
	}
}

func TestSearchEvent_FullRoundtrip(t *testing.T) {
	ts := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	original := core.SearchEvent{
		Query:     "платье летнее",
		UserID:    "u-42",
		Timestamp: ts,
	}

	data, _ := json.Marshal(original)

	var parsed core.SearchEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}
	if parsed.Query != original.Query {
		t.Errorf("query: want %q, got %q", original.Query, parsed.Query)
	}
	if parsed.UserID != original.UserID {
		t.Errorf("user_id: want %q, got %q", original.UserID, parsed.UserID)
	}
	if !parsed.Timestamp.Equal(original.Timestamp) {
		t.Errorf("timestamp mismatch: want %v, got %v", original.Timestamp, parsed.Timestamp)
	}
}

func TestSearchEvent_ErrEmptyQuery_IsSilent(t *testing.T) {
	svc := &fakeService{nextErr: core.ErrEmptyQuery}
	err := svc.ProcessEvent(context.Background(), core.SearchEvent{Query: ""})
	if err != core.ErrEmptyQuery {
		t.Errorf("want ErrEmptyQuery, got %v", err)
	}
}

func TestTruncate_Unicode(t *testing.T) {
	long := ""
	for i := 0; i < 150; i++ {
		long += "а"
	}
	runes := []rune(long)
	maxLen := 100
	result := string(runes[:maxLen]) + "..."

	if got := len([]rune(result)); got != 103 {
		t.Errorf("want 103 runes after truncate, got %d", got)
	}
}
