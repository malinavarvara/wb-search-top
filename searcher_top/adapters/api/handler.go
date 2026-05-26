package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/malinavarvara/wb-search-top/searcher_top/core"
)

type Handler struct {
	service core.SearchService
	log     *slog.Logger
}

func NewHandler(service core.SearchService, log *slog.Logger) *Handler {
	return &Handler{service: service, log: log}
}

func (h *Handler) GetTop(w http.ResponseWriter, r *http.Request) {
	n := 0 // дефолт из конфига
	if raw := r.URL.Query().Get("n"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "n must be a positive integer")
			return
		}
		n = parsed
	}

	items, err := h.service.GetTop(r.Context(), n)
	if err != nil {
		if errors.Is(err, core.ErrInvalidLimit) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.log.Error("GetTop failed", slog.Any("err", err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

func (h *Handler) AddStopWord(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Word string `json:"word"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := h.service.AddStopWord(r.Context(), req.Word); err != nil {
		if errors.Is(err, core.ErrEmptyWord) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.log.Error("AddStopWord failed", slog.Any("err", err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"word": strings.TrimSpace(req.Word)})
}

func (h *Handler) RemoveStopWord(w http.ResponseWriter, r *http.Request) {
	word := r.PathValue("word")
	if word == "" {
		word = strings.TrimPrefix(r.URL.Path, "/stoplist/")
	}

	if err := h.service.RemoveStopWord(r.Context(), word); err != nil {
		if errors.Is(err, core.ErrEmptyWord) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.log.Error("RemoveStopWord failed", slog.Any("err", err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"word": word})
}

func (h *Handler) GetStopWords(w http.ResponseWriter, r *http.Request) {
	words, err := h.service.GetStopWords(r.Context())
	if err != nil {
		h.log.Error("GetStopWords failed", slog.Any("err", err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if words == nil {
		words = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"words": words,
		"total": len(words),
	})
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Default().Error("writeJSON encode failed", slog.Any("err", err))
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
