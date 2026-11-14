package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/boleto"
)

// Emissor é o que o HTTP precisa do serviço. O teste usa um dublê.
type Emissor interface {
	Emitir(ctx context.Context, dados boleto.Dados) (boleto.Boleto, error)
}

type Handler struct {
	Emissor Emissor
	Agora   func() time.Time
	Ping    func(context.Context) error
}

func NovoHandler(emissor Emissor, ping func(context.Context) error) *Handler {
	return &Handler{Emissor: emissor, Ping: ping}
}

func (h *Handler) Rotas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /boletos", h.CriarBoleto)
	return mux
}

func (h *Handler) agora() time.Time {
	if h.Agora != nil {
		return h.Agora()
	}
	return time.Now()
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

func (h *Handler) CriarBoleto(w http.ResponseWriter, r *http.Request) {
	// Três campos não precisam de mais que 1 MiB.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var entrada boleto.Entrada
	if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
		escreverErros(w, http.StatusBadRequest, []string{"corpo JSON inválido"})
		return
	}

	dados, erros := boleto.Validar(entrada, h.agora())
	if len(erros) > 0 {
		escreverErros(w, http.StatusBadRequest, erros)
		return
	}

	criado, err := h.Emissor.Emitir(r.Context(), dados)
	if err != nil {
		slog.Error("emitir boleto", "erro", err)
		escreverErros(w, http.StatusInternalServerError, []string{"não foi possível emitir o boleto"})
		return
	}
	escreverJSON(w, http.StatusCreated, paraJSON(criado))
}

type boletoJSON struct {
	ID         string `json:"id"`
	Valor      string `json:"valor"`
	Vencimento string `json:"vencimento"`
	Pagador    string `json:"pagador"`
	Status     string `json:"status"`
	CriadoEm   string `json:"criado_em"`
}

func paraJSON(b boleto.Boleto) boletoJSON {
	return boletoJSON{
		ID:         b.ID,
		Valor:      boleto.FormatarValor(b.ValorCentavos),
		Vencimento: b.Vencimento.Format("2006-01-02"),
		Pagador:    b.Pagador,
		Status:     b.Status,
		CriadoEm:   b.CriadoEm.UTC().Format(time.RFC3339),
	}
}

type corpoErro struct {
	Erros []string `json:"erros"`
}

func escreverErros(w http.ResponseWriter, status int, erros []string) {
	escreverJSON(w, status, corpoErro{Erros: erros})
}

func escreverJSON(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(corpo); err != nil {
		slog.Error("escrever json", "erro", err)
	}
}
