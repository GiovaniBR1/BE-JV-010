package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/boleto"
)

func relogioFixo() time.Time {
	return time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
}

func pingOK(context.Context) error { return nil }

type emissorFake struct {
	chamado bool
	dados   boleto.Dados
	boleto  boleto.Boleto
	err     error
}

func (f *emissorFake) Emitir(_ context.Context, dados boleto.Dados) (boleto.Boleto, error) {
	f.chamado = true
	f.dados = dados
	if f.err != nil {
		return boleto.Boleto{}, f.err
	}
	return f.boleto, nil
}

func postar(h *Handler, corpo string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/boletos", strings.NewReader(corpo))
	rec := httptest.NewRecorder()
	h.Rotas().ServeHTTP(rec, req)
	return rec
}

func handlerCom(fake *emissorFake) *Handler {
	h := NovoHandler(fake, pingOK)
	h.Agora = relogioFixo
	return h
}

func TestCriarBoletoValido(t *testing.T) {
	criadoEm := time.Date(2026, 11, 14, 18, 0, 0, 0, time.UTC)
	fake := &emissorFake{boleto: boleto.Boleto{
		ID:            "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		ValorCentavos: 15050,
		Vencimento:    time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		Pagador:       "Maria Silva",
		Status:        boleto.StatusPendente,
		CriadoEm:      criadoEm,
	}}
	rec := postar(handlerCom(fake), `{"valor":"150.50","vencimento":"2026-12-31","pagador":" Maria Silva "}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	if !fake.chamado {
		t.Fatal("não chamou o serviço")
	}
	if fake.dados.ValorCentavos != 15050 || fake.dados.Pagador != "Maria Silva" {
		t.Fatalf("dados = %+v", fake.dados)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type = %s", rec.Header().Get("Content-Type"))
	}

	var bruto map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &bruto); err != nil {
		t.Fatal(err)
	}
	if len(bruto["valor"]) == 0 || bruto["valor"][0] != '"' {
		t.Fatalf("valor na resposta deve ser texto, veio %s", bruto["valor"])
	}

	var resp boletoJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Valor != "150.50" || resp.Status != boleto.StatusPendente || resp.Pagador != "Maria Silva" {
		t.Fatalf("resposta = %+v", resp)
	}
	if resp.Vencimento != "2026-12-31" || resp.ID != fake.boleto.ID || resp.CriadoEm != "2026-11-14T18:00:00Z" {
		t.Fatalf("resposta = %+v", resp)
	}
}

func TestCriarBoletoComNumeroJSON(t *testing.T) {
	fake := &emissorFake{boleto: boleto.Boleto{
		ID:            "abc",
		ValorCentavos: 1050,
		Vencimento:    time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		Pagador:       "João",
		Status:        boleto.StatusPendente,
		CriadoEm:      relogioFixo(),
	}}
	rec := postar(handlerCom(fake), `{"valor":10.5,"vencimento":"2026-12-31","pagador":"João"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	if fake.dados.ValorCentavos != 1050 {
		t.Fatalf("centavos = %d", fake.dados.ValorCentavos)
	}
}

func TestCriarBoletoInvalidoNaoChamaServico(t *testing.T) {
	fake := &emissorFake{}
	rec := postar(handlerCom(fake), `{"valor":"10.00","vencimento":"2026-11-13","pagador":"Ana"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if fake.chamado {
		t.Fatal("chamou o serviço com dados inválidos")
	}
	var corpo corpoErro
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
		t.Fatal(err)
	}
	if len(corpo.Erros) != 1 || corpo.Erros[0] != "vencimento não pode ser anterior a hoje" {
		t.Fatalf("erros = %#v", corpo.Erros)
	}
}

func TestCriarBoletoListaTodosOsErros(t *testing.T) {
	fake := &emissorFake{}
	rec := postar(handlerCom(fake), `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	var corpo corpoErro
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
		t.Fatal(err)
	}
	if len(corpo.Erros) != 3 {
		t.Fatalf("erros = %#v", corpo.Erros)
	}
}

func TestCorpoJSONInvalido(t *testing.T) {
	fake := &emissorFake{}
	rec := postar(handlerCom(fake), `{`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if fake.chamado {
		t.Fatal("chamou o serviço")
	}
	var corpo corpoErro
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
		t.Fatal(err)
	}
	if len(corpo.Erros) != 1 || corpo.Erros[0] != "corpo JSON inválido" {
		t.Fatalf("erros = %#v", corpo.Erros)
	}
}

func TestErroInternoNaoVazaDetalhe(t *testing.T) {
	fake := &emissorFake{err: errors.New("senha do banco")}
	rec := postar(handlerCom(fake), `{"valor":"10.00","vencimento":"2026-12-31","pagador":"Ana"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "senha do banco") {
		t.Fatalf("vazou o erro interno: %s", rec.Body.String())
	}
}

func TestHealth(t *testing.T) {
	h := NovoHandler(&emissorFake{}, pingOK)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.Rotas().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	h = NovoHandler(&emissorFake{}, func(context.Context) error { return errors.New("sem banco") })
	rec = httptest.NewRecorder()
	h.Rotas().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}
