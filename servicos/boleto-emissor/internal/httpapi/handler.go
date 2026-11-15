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

// Consulta lê o boleto no Postgres do emissor.
type Consulta interface {
	BuscarPorID(ctx context.Context, id string) (boleto.Boleto, bool, error)
}

// Memoria é o cache do GET. Erro significa Redis fora: o HTTP segue no banco.
type Memoria interface {
	Buscar(ctx context.Context, id string) (corpo []byte, ok bool, err error)
	Guardar(ctx context.Context, id string, corpo []byte) error
}

type Handler struct {
	Emissor  Emissor
	Consulta Consulta
	Cache    Memoria
	Agora    func() time.Time
	Ping     func(context.Context) error
}

func NovoHandler(emissor Emissor, ping func(context.Context) error) *Handler {
	return &Handler{Emissor: emissor, Ping: ping}
}

func (h *Handler) Rotas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /boletos", h.CriarBoleto)
	mux.HandleFunc("GET /boletos/{id}", h.BuscarBoleto)
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

func (h *Handler) BuscarBoleto(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.Cache == nil {
		h.buscarNoBanco(w, r, id, "bypass")
		return
	}
	corpo, ok, err := h.Cache.Buscar(r.Context(), id)
	if err != nil {
		slog.Warn("cache", "erro", err)
		h.buscarNoBanco(w, r, id, "bypass")
		return
	}
	if ok {
		w.Header().Set("X-Cache", "hit")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(corpo)
		return
	}
	h.buscarNoBanco(w, r, id, "miss")
}

func (h *Handler) buscarNoBanco(w http.ResponseWriter, r *http.Request, id, origem string) {
	if h.Consulta == nil {
		escreverErros(w, http.StatusInternalServerError, []string{"consulta indisponível"})
		return
	}
	encontrado, ok, err := h.Consulta.BuscarPorID(r.Context(), id)
	if err != nil {
		slog.Error("buscar boleto", "erro", err)
		escreverErros(w, http.StatusInternalServerError, []string{"não foi possível buscar o boleto"})
		return
	}
	// 404 não entra no cache: a próxima leitura precisa poder achar.
	if !ok {
		escreverErros(w, http.StatusNotFound, []string{"boleto não encontrado"})
		return
	}
	corpo, err := json.Marshal(paraJSON(encontrado))
	if err != nil {
		escreverErros(w, http.StatusInternalServerError, []string{"não foi possível buscar o boleto"})
		return
	}
	corpo = append(corpo, '\n')
	if origem == "miss" && h.Cache != nil {
		if err := h.Cache.Guardar(r.Context(), id, corpo); err != nil {
			slog.Warn("guardar cache", "erro", err)
			origem = "bypass"
		}
	}
	w.Header().Set("X-Cache", origem)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(corpo)
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
