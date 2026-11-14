package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/registro"
)

func TestHealth(t *testing.T) {
	h := NovoHandler(func(context.Context) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.Rotas().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}

	h = NovoHandler(func(context.Context) error { return errors.New("sem banco") })
	rec = httptest.NewRecorder()
	h.Rotas().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

type consultaFake struct {
	reg registro.Registro
	ok  bool
}

func (c consultaFake) BuscarPorID(context.Context, string) (registro.Registro, bool, error) {
	return c.reg, c.ok, nil
}

func TestBuscar(t *testing.T) {
	h := NovoHandler(func(context.Context) error { return nil })
	h.Consulta = consultaFake{ok: false}
	req := httptest.NewRequest(http.MethodGet, "/boletos/ausente", nil)
	rec := httptest.NewRecorder()
	h.Rotas().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}

	h.Consulta = consultaFake{ok: true, reg: registro.Registro{
		ID:            "abc",
		ValorCentavos: 15050,
		Vencimento:    time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		Pagador:       "Maria Silva",
		Status:        registro.StatusRegistrado,
		RegistradoEm:  time.Date(2026, 11, 14, 18, 0, 0, 0, time.UTC),
	}}
	rec = httptest.NewRecorder()
	h.Rotas().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"valor":"150.50"`) {
		t.Fatalf("status = %d corpo = %s", rec.Code, rec.Body.String())
	}
}
