package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type logBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (lb *logBuffer) Write(p []byte) (n int, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Write(p)
}

func (lb *logBuffer) String() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.String()
}

func TestRequestID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Context().Value(requestIDKey).(string)
		w.Header().Set("X-Request-ID", id)
		w.WriteHeader(http.StatusOK)
	})
	wrapped := RequestID(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	resp := w.Result()
	headerID := resp.Header.Get("X-Request-ID")
	if headerID == "" {
		t.Fatal("X-Request-ID header not set")
	}
	if len(headerID) < 10 {
		t.Error("generated request ID too short")
	}
	if got := resp.Header.Get("X-Request-ID"); got != headerID {
		t.Errorf("header mismatch: want %q, got %q", headerID, got)
	}
}

func TestRequestID_PreservesExisting(t *testing.T) {
	existingID := "my-trace-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Context().Value(requestIDKey).(string)
		if id != existingID {
			t.Errorf("context ID = %q, want %q", id, existingID)
		}
		w.WriteHeader(http.StatusOK)
	})
	wrapped := RequestID(handler)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", existingID)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	if got := w.Header().Get("X-Request-ID"); got != existingID {
		t.Errorf("response header = %q, want %q", got, existingID)
	}
}

func TestLogger(t *testing.T) {
	logBuf := &logBuffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})
	wrapped := RequestID(Logger(logger)(handler))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	logOut := logBuf.String()
	if !strings.Contains(logOut, "http request") {
		t.Error("log line missing")
	}
	if !strings.Contains(logOut, "method=GET") {
		t.Error("method not logged")
	}
	if !strings.Contains(logOut, "path=/test") {
		t.Error("path not logged")
	}
	if !strings.Contains(logOut, "status=201") {
		t.Error("status not logged")
	}
	if !strings.Contains(logOut, "duration=") {
		t.Error("duration not logged")
	}
	if !strings.Contains(logOut, "request_id=") {
		t.Error("request_id not logged")
	}
}

func TestRecovery(t *testing.T) {
	logBuf := &logBuffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, nil))

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	wrapped := Recovery(logger)(panicHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "internal server error") {
		t.Error("response body missing expected text")
	}
	logOut := logBuf.String()
	if !strings.Contains(logOut, "panic recovered") {
		t.Error("panic not logged")
	}
	if !strings.Contains(logOut, "test panic") {
		t.Error("panic value not logged")
	}
}

func TestRateLimiter(t *testing.T) {
	rps := 10.0
	burst := 5
	limiterMiddleware := RateLimiter(rps, burst)

	var calls int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	})
	wrapped := limiterMiddleware(handler)

	for i := 0; i < burst; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status %d, want 200", i+1, w.Code)
		}
	}
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("after burst: status %d, want 429", w.Code)
	}
	retry := w.Header().Get("Retry-After")
	if retry != "1" {
		t.Errorf("Retry-After = %q, want \"1\"", retry)
	}
	body := w.Body.String()
	if !strings.Contains(body, "rate limit exceeded") {
		t.Error("error message missing")
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		remote  string
		want    string
	}{
		{
			name:    "X-Real-IP priority",
			headers: map[string]string{"X-Real-IP": "1.2.3.4", "X-Forwarded-For": "5.6.7.8"},
			remote:  "10.0.0.1:1234",
			want:    "1.2.3.4",
		},
		{
			name:    "X-Forwarded-For single",
			headers: map[string]string{"X-Forwarded-For": "192.168.1.1"},
			remote:  "10.0.0.1:1234",
			want:    "192.168.1.1",
		},
		{
			name:    "X-Forwarded-For multiple",
			headers: map[string]string{"X-Forwarded-For": "203.0.113.5, 10.0.0.2, 172.16.0.1"},
			remote:  "10.0.0.1:1234",
			want:    "203.0.113.5",
		},
		{
			name:    "fallback to RemoteAddr",
			headers: map[string]string{},
			remote:  "192.168.0.100:5678",
			want:    "192.168.0.100",
		},
		{
			name:    "invalid RemoteAddr",
			headers: map[string]string{},
			remote:  "invalid",
			want:    "invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tt.remote
			got := extractIP(req)
			if got != tt.want {
				t.Errorf("extractIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func getMetricValue(counterVec *prometheus.CounterVec, lvs ...string) int64 {
	counter, err := counterVec.GetMetricWithLabelValues(lvs...)
	if err != nil {
		return 0
	}
	var pb dto.Metric
	if err := counter.(prometheus.Metric).Write(&pb); err != nil {
		return 0
	}
	return int64(pb.Counter.GetValue())
}

func getMetricCount(histogramVec *prometheus.HistogramVec, lvs ...string) int64 {
	m, err := histogramVec.GetMetricWithLabelValues(lvs...)
	if err != nil {
		return 0
	}
	hist, ok := m.(prometheus.Histogram)
	if !ok {
		return 0
	}
	var pb dto.Metric
	if err := hist.Write(&pb); err != nil {
		return 0
	}
	return int64(pb.Histogram.GetSampleCount())
}

func TestMetrics(t *testing.T) {
	countBefore := getMetricValue(httpRequestsTotal, "GET", "/test", "200")
	durationCountBefore := getMetricCount(httpRequestDuration, "GET", "/test")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := Metrics(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	countAfter := getMetricValue(httpRequestsTotal, "GET", "/test", "200")
	durationCountAfter := getMetricCount(httpRequestDuration, "GET", "/test")

	if countAfter != countBefore+1 {
		t.Errorf("http_requests_total not incremented: before %d, after %d", countBefore, countAfter)
	}
	if durationCountAfter != durationCountBefore+1 {
		t.Errorf("http_request_duration_seconds count not incremented")
	}
}

func TestTimeout(t *testing.T) {
	timeout := 50 * time.Millisecond
	timeoutMw := Timeout(timeout)

	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(100 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		}
	})
	wrapped := timeoutMw(slowHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	start := time.Now()
	wrapped.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if elapsed > timeout+50*time.Millisecond {
		t.Errorf("request took too long: %v", elapsed)
	}
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "request timeout") {
		t.Error("response missing timeout message")
	}
}

func TestTimeout_NoTimeout(t *testing.T) {
	timeout := 50 * time.Millisecond
	timeoutMw := Timeout(timeout)

	fastHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := timeoutMw(fastHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestTimeout_PanicPropagation(t *testing.T) {
	timeout := 50 * time.Millisecond
	timeoutMw := Timeout(timeout)

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic inside timeout")
	})
	wrapped := timeoutMw(panicHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic, but got none")
		} else if r != "test panic inside timeout" {
			t.Errorf("unexpected panic value: %v", r)
		}
	}()
	wrapped.ServeHTTP(w, req)
}
