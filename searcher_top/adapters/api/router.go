package api

import (
	"log/slog"
	"net/http"

	"github.com/malinavarvara/wb-search-top/searcher_top/adapters/api/middleware"
)

func NewRouter(h *Handler, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /top", h.GetTop)
	mux.HandleFunc("GET /stoplist", h.GetStopWords)
	mux.HandleFunc("POST /stoplist", h.AddStopWord)
	mux.HandleFunc("DELETE /stoplist/{word}", h.RemoveStopWord)
	mux.HandleFunc("GET /health", h.Health)

	return middleware.Recovery(log)(
		middleware.RequestID(
			middleware.Logger(log)(mux),
		),
	)
}
