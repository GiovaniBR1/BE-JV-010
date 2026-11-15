package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/registro"
)

// Consulta lê a cópia registrada neste serviço.
type Consulta interface {
	BuscarPorID(ctx context.Context, id string) (registro.Registro, bool, error)
}

type Handler struct {
	Ping     func(context.Context) error
	Consulta Consulta
}

func NovoHandler(ping func(context.Context) error) *Handler {
	return &Handler{Ping: ping}
}

func (h *Handler) Rotas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /boletos/{id}", h.Buscar)
	return mux
}

func (h *Handler) Buscar(w http.ResponseWriter, r *http.Request) {
	if h.Consulta == nil {
		escreverJSON(w, http.StatusInternalServerError, map[string]string{"erro": "consulta indisponível"})
		return
	}
	encontrado, ok, err := h.Consulta.BuscarPorID(r.Context(), r.PathValue("id"))
	if err != nil {
		slog.Error("buscar registro", "erro", err)
		escreverJSON(w, http.StatusInternalServerError, map[string]string{"erro": "não foi possível buscar o boleto"})
		return
	}
	if !ok {
		escreverJSON(w, http.StatusNotFound, map[string]string{"erro": "boleto não encontrado"})
		return
	}
	escreverJSON(w, http.StatusOK, map[string]string{
		"id":            encontrado.ID,
		"valor":         formatarValor(encontrado.ValorCentavos),
		"vencimento":    encontrado.Vencimento.Format("2006-01-02"),
		"pagador":       encontrado.Pagador,
		"status":        encontrado.Status,
		"registrado_em": encontrado.RegistradoEm.UTC().Format(time.RFC3339),
	})
}

func formatarValor(centavos int64) string {
	return fmt.Sprintf("%d.%02d", centavos/100, centavos%100)
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
