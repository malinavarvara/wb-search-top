package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/malinavarvara/wb-search-top/searcher_top/adapters/api/middleware"
)

type RouterConfig struct {
	HandlerTimeout time.Duration
	RateLimitRPS   float64
	RateLimitBurst int
}

func NewRouter(h *Handler, log *slog.Logger, cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /top", h.GetTop)
	mux.HandleFunc("GET /stoplist", h.GetStopWords)
	mux.HandleFunc("POST /stoplist", h.AddStopWord)
	mux.HandleFunc("DELETE /stoplist/{word}", h.RemoveStopWord)
	mux.HandleFunc("GET /health", h.Health)
	mux.Handle("GET /metrics", promhttp.Handler())

	var handler http.Handler = mux
	handler = middleware.Timeout(cfg.HandlerTimeout)(handler)
	handler = middleware.Metrics(handler)
	handler = middleware.Logger(log)(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.RateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)(handler)
	handler = middleware.Recovery(log)(handler)

	return handler
}
