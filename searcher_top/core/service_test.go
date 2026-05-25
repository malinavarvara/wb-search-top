package core_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/malinavarvara/wb-search-top/searcher_top/core"
)

type fakeTopRepo struct {
	calls []struct {
		query  string
		userID string
	}
	result []core.TopItem
}

func (f *fakeTopRepo) Increment(query, userID string, _ time.Time) {
	f.calls = append(f.calls, struct {
		query  string
		userID string
	}{query, userID})
}

func (f *fakeTopRepo) GetTop(_ int) []core.TopItem {
	return f.result
}

type fakeStopRepo struct {
	words map[string]struct{}
}

func newFakeStop() *fakeStopRepo {
	return &fakeStopRepo{words: make(map[string]struct{})}
}

func (f *fakeStopRepo) Add(w string)           { f.words[w] = struct{}{} }
func (f *fakeStopRepo) Remove(w string)        { delete(f.words, w) }
func (f *fakeStopRepo) Contains(w string) bool { _, ok := f.words[w]; return ok }
func (f *fakeStopRepo) GetAll() []string {
	out := make([]string, 0, len(f.words))
	for w := range f.words {
		out = append(out, w)
	}
	return out
}

var silentLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func newService(top *fakeTopRepo, stop *fakeStopRepo) *core.Service {
	return core.NewService(top, stop, silentLog, 100)
}

func TestService_ProcessEvent_NormalizesQuery(t *testing.T) {
	top := &fakeTopRepo{}
	svc := newService(top, newFakeStop())

	err := svc.ProcessEvent(context.Background(), core.SearchEvent{
		Query:     "  Найк КРОССОВКИ  ",
		UserID:    "u1",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(top.calls) != 1 {
		t.Fatalf("want 1 call to Increment, got %d", len(top.calls))
	}
	if top.calls[0].query != "найк кроссовки" {
		t.Errorf("want normalized query %q, got %q", "найк кроссовки", top.calls[0].query)
	}
}

func TestService_ProcessEvent_EmptyQueryReturnsError(t *testing.T) {
	top := &fakeTopRepo{}
	svc := newService(top, newFakeStop())

	err := svc.ProcessEvent(context.Background(), core.SearchEvent{
		Query:  "   ",
		UserID: "u1",
	})
	if err != core.ErrEmptyQuery {
		t.Errorf("want ErrEmptyQuery, got %v", err)
	}
	if len(top.calls) != 0 {
		t.Error("Increment must not be called for empty query")
	}
}

func TestService_ProcessEvent_StopListFilters(t *testing.T) {
	top := &fakeTopRepo{}
	stop := newFakeStop()
	stop.Add("спам")
	svc := newService(top, stop)

	err := svc.ProcessEvent(context.Background(), core.SearchEvent{
		Query:  "спам",
		UserID: "u1",
	})
	if err != nil {
		t.Errorf("stop list hit should not return error, got %v", err)
	}
	if len(top.calls) != 0 {
		t.Error("Increment must not be called for stop list query")
	}
}

func TestService_ProcessEvent_StopListCaseInsensitive(t *testing.T) {
	top := &fakeTopRepo{}
	stop := newFakeStop()
	stop.Add("спам") // добавили в нижнем регистре
	svc := newService(top, stop)

	err := svc.ProcessEvent(context.Background(), core.SearchEvent{
		Query:  "СПАМ", // пришло в верхнем
		UserID: "u1",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(top.calls) != 0 {
		t.Error("Increment must not be called: СПАМ should match стоп-слово спам")
	}
}

func TestService_ProcessEvent_FallbackUserID(t *testing.T) {
	top := &fakeTopRepo{}
	svc := newService(top, newFakeStop())

	err := svc.ProcessEvent(context.Background(), core.SearchEvent{
		Query:     "кроссовки",
		UserID:    "",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(top.calls) == 0 {
		t.Fatal("expected Increment to be called")
	}

	if len(top.calls[0].userID) < 5 || top.calls[0].userID[:5] != "anon-" {
		t.Errorf("expected fallback userID to start with 'anon-', got %q", top.calls[0].userID)
	}
}

func TestService_GetTop_ReturnsResult(t *testing.T) {
	top := &fakeTopRepo{
		result: []core.TopItem{
			{Query: "кроссовки", Count: 42},
		},
	}
	svc := newService(top, newFakeStop())

	items, err := svc.GetTop(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Query != "кроссовки" {
		t.Errorf("unexpected result: %v", items)
	}
}

func TestService_GetTop_ZeroLimitUsesMax(t *testing.T) {
	top := &fakeTopRepo{}
	svc := newService(top, newFakeStop())

	_, err := svc.GetTop(context.Background(), 0)
	if err != nil {
		t.Fatalf("n=0 should use maxTopLimit, got error: %v", err)
	}
}

func TestService_GetTop_ExceedsMaxReturnsError(t *testing.T) {
	top := &fakeTopRepo{}
	svc := newService(top, newFakeStop())

	_, err := svc.GetTop(context.Background(), 99999)
	if err != core.ErrInvalidLimit {
		t.Errorf("want ErrInvalidLimit, got %v", err)
	}
}

func TestService_AddStopWord_EmptyReturnsError(t *testing.T) {
	svc := newService(&fakeTopRepo{}, newFakeStop())
	err := svc.AddStopWord(context.Background(), "  ")
	if err != core.ErrEmptyWord {
		t.Errorf("want ErrEmptyWord, got %v", err)
	}
}

func TestService_RemoveStopWord_EmptyReturnsError(t *testing.T) {
	svc := newService(&fakeTopRepo{}, newFakeStop())
	err := svc.RemoveStopWord(context.Background(), "")
	if err != core.ErrEmptyWord {
		t.Errorf("want ErrEmptyWord, got %v", err)
	}
}

func TestService_GetStopWords(t *testing.T) {
	stop := newFakeStop()
	stop.Add("слово1")
	stop.Add("слово2")
	svc := newService(&fakeTopRepo{}, stop)

	words, err := svc.GetStopWords(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(words) != 2 {
		t.Errorf("want 2 stop words, got %d", len(words))
	}
}

func TestService_NormalizeQuery_CollapseSpaces(t *testing.T) {
	top := &fakeTopRepo{}
	svc := newService(top, newFakeStop())

	_ = svc.ProcessEvent(context.Background(), core.SearchEvent{
		Query:  "найк   кроссовки   мужские",
		UserID: "u1",
	})

	if top.calls[0].query != "найк кроссовки мужские" {
		t.Errorf("spaces not collapsed: %q", top.calls[0].query)
	}
}
