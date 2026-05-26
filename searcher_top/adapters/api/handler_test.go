package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"

	"github.com/malinavarvara/wb-search-top/searcher_top/adapters/api"
	"github.com/malinavarvara/wb-search-top/searcher_top/core"
)

type fakeService struct {
	topItems  []core.TopItem
	topErr    error
	stopWords []string
	addErr    error
	removeErr error
}

func (f *fakeService) ProcessEvent(_ context.Context, _ core.SearchEvent) error { return nil }

func (f *fakeService) GetTop(_ context.Context, _ int) ([]core.TopItem, error) {
	return f.topItems, f.topErr
}

func (f *fakeService) AddStopWord(_ context.Context, _ string) error {
	return f.addErr
}

func (f *fakeService) RemoveStopWord(_ context.Context, _ string) error {
	return f.removeErr
}

func (f *fakeService) GetStopWords(_ context.Context) ([]string, error) {
	return f.stopWords, nil
}

var silentLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func newHandler(svc *fakeService) *api.Handler {
	return api.NewHandler(svc, silentLog)
}

func TestGetTop_OK(t *testing.T) {
	svc := &fakeService{
		topItems: []core.TopItem{
			{Query: "кроссовки", Count: 42},
			{Query: "платье", Count: 17},
		},
	}
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/top?n=10", nil)
	w := httptest.NewRecorder()
	h.GetTop(w, req)

	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Items []core.TopItem `json:"items"`
		Total int            `json:"total"`
	}
	mustDecodeJSON(t, w, &resp)

	if resp.Total != 2 {
		t.Errorf("want total=2, got %d", resp.Total)
	}
	if resp.Items[0].Query != "кроссовки" {
		t.Errorf("want first item 'кроссовки', got %q", resp.Items[0].Query)
	}
}

func TestGetTop_InvalidN(t *testing.T) {
	h := newHandler(&fakeService{})

	req := httptest.NewRequest(http.MethodGet, "/top?n=abc", nil)
	w := httptest.NewRecorder()
	h.GetTop(w, req)

	assertStatus(t, w, http.StatusBadRequest)
	assertErrorField(t, w)
}

func TestGetTop_NegativeN(t *testing.T) {
	h := newHandler(&fakeService{})

	req := httptest.NewRequest(http.MethodGet, "/top?n=-5", nil)
	w := httptest.NewRecorder()
	h.GetTop(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestGetTop_ServiceError(t *testing.T) {
	svc := &fakeService{topErr: core.ErrInvalidLimit}
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/top?n=99999", nil)
	w := httptest.NewRecorder()
	h.GetTop(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestGetTop_NoN_UsesDefault(t *testing.T) {
	svc := &fakeService{topItems: []core.TopItem{{Query: "найк", Count: 5}}}
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/top", nil)
	w := httptest.NewRecorder()
	h.GetTop(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestAddStopWord_OK(t *testing.T) {
	h := newHandler(&fakeService{})

	body, _ := json.Marshal(map[string]string{"word": "спам"})
	req := httptest.NewRequest(http.MethodPost, "/stoplist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.AddStopWord(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestAddStopWord_InvalidJSON(t *testing.T) {
	h := newHandler(&fakeService{})

	req := httptest.NewRequest(http.MethodPost, "/stoplist", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.AddStopWord(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestAddStopWord_EmptyWord(t *testing.T) {
	svc := &fakeService{addErr: core.ErrEmptyWord}
	h := newHandler(svc)

	body, _ := json.Marshal(map[string]string{"word": "  "})
	req := httptest.NewRequest(http.MethodPost, "/stoplist", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.AddStopWord(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestRemoveStopWord_OK(t *testing.T) {
	h := newHandler(&fakeService{})

	req := httptest.NewRequest(http.MethodDelete, "/stoplist/спам", nil)
	req.SetPathValue("word", "спам")
	w := httptest.NewRecorder()
	h.RemoveStopWord(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestGetStopWords_ReturnsArray(t *testing.T) {
	svc := &fakeService{stopWords: []string{"спам", "накрутка"}}
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/stoplist", nil)
	w := httptest.NewRecorder()
	h.GetStopWords(w, req)

	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Words []string `json:"words"`
		Total int      `json:"total"`
	}
	mustDecodeJSON(t, w, &resp)

	if resp.Total != 2 {
		t.Errorf("want total=2, got %d", resp.Total)
	}
}

func TestGetStopWords_EmptyIsArray(t *testing.T) {
	h := newHandler(&fakeService{stopWords: nil})

	req := httptest.NewRequest(http.MethodGet, "/stoplist", nil)
	w := httptest.NewRecorder()
	h.GetStopWords(w, req)

	assertStatus(t, w, http.StatusOK)

	body := w.Body.String()
	if !bytes.Contains([]byte(body), []byte(`"words":[]`)) {
		t.Errorf("expected words:[] (not null), got: %s", body)
	}
}

func TestHealth_OK(t *testing.T) {
	h := newHandler(&fakeService{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.Health(w, req)

	assertStatus(t, w, http.StatusOK)
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status: want %d, got %d (body: %s)", want, w.Code, w.Body.String())
	}
}

func assertErrorField(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if _, ok := resp["error"]; !ok {
		t.Error("expected 'error' field in response body")
	}
}

func mustDecodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}
