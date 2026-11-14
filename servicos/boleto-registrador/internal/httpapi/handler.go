package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type Handler struct {
	Ping func(context.Context) error
}

func NovoHandler(ping func(context.Context) error) *Handler {
	return &Handler{Ping: ping}
}

func (h *Handler) Rotas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	return mux
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if h.Ping == nil {
		escreverJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "indisponível"})
		return
	}
	if err := h.Ping(ctx); err != nil {
		slog.Warn("health", "erro", err)
		escreverJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "indisponível"})
		return
	}
	escreverJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func escreverJSON(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(corpo); err != nil {
		slog.Error("escrever json", "erro", err)
	}
}
