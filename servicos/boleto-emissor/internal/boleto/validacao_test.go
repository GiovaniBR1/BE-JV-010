package boleto

import (
	"encoding/json"
	"slices"
	"testing"
	"time"
)

// 14/11/2026 às 15:00 UTC ainda é 14/11 em São Paulo (meio-dia).
func hojeDeTeste() time.Time {
	return time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
}

func TestValidar(t *testing.T) {
	casos := []struct {
		nome       string
		corpo      string
		erros      []string
		centavos   int64
		pagador    string
		vencimento string
		agora      time.Time
	}{
		{
			nome:       "aceita valor em texto e corta espaço do pagador",
			corpo:      `{"valor":"150.50","vencimento":"2026-11-14","pagador":" Maria Silva "}`,
			centavos:   15050,
			pagador:    "Maria Silva",
			vencimento: "2026-11-14",
		},
		{
			nome:       "aceita número JSON com uma casa",
			corpo:      `{"valor":10.5,"vencimento":"2026-12-31","pagador":"João"}`,
			centavos:   1050,
			pagador:    "João",
			vencimento: "2026-12-31",
		},
		{
			nome:       "aceita inteiro JSON",
			corpo:      `{"valor":150,"vencimento":"2026-12-31","pagador":"Ana"}`,
			centavos:   15000,
			pagador:    "Ana",
			vencimento: "2026-12-31",
		},
		{
			nome:       "aceita um centavo",
			corpo:      `{"valor":"0.01","vencimento":"2026-11-15","pagador":"Ana"}`,
			centavos:   1,
			pagador:    "Ana",
			vencimento: "2026-11-15",
		},
		{
			nome:  "rejeita valor ausente, data ruim e pagador vazio juntos",
			corpo: `{}`,
			erros: []string{msgValorObrigatorio, msgVencimentoObrigatorio, msgPagadorObrigatorio},
		},
		{
			nome:  "rejeita zero",
			corpo: `{"valor":"0.00","vencimento":"2026-12-31","pagador":"Ana"}`,
			erros: []string{msgValorInvalido},
		},
		{
			nome:  "rejeita negativo",
			corpo: `{"valor":-1,"vencimento":"2026-12-31","pagador":"Ana"}`,
			erros: []string{msgValorInvalido},
		},
		{
			nome:  "rejeita mais de duas casas",
			corpo: `{"valor":"10.555","vencimento":"2026-12-31","pagador":"Ana"}`,
			erros: []string{msgValorInvalido},
		},
		{
			nome:  "rejeita notação científica",
			corpo: `{"valor":1e2,"vencimento":"2026-12-31","pagador":"Ana"}`,
			erros: []string{msgValorInvalido},
		},
		{
			nome:  "rejeita zero à esquerda",
			corpo: `{"valor":"01.50","vencimento":"2026-12-31","pagador":"Ana"}`,
			erros: []string{msgValorInvalido},
		},
		{
			nome:  "rejeita data impossível",
			corpo: `{"valor":"10.00","vencimento":"2026-02-31","pagador":"Ana"}`,
			erros: []string{msgVencimentoFormato},
		},
		{
			nome:  "rejeita data com barra",
			corpo: `{"valor":"10.00","vencimento":"14/11/2026","pagador":"Ana"}`,
			erros: []string{msgVencimentoFormato},
		},
		{
			nome:  "rejeita vencimento de ontem",
			corpo: `{"valor":"10.00","vencimento":"2026-11-13","pagador":"Ana"}`,
			erros: []string{msgVencimentoPassado},
		},
		{
			nome:  "rejeita pagador só com espaço",
			corpo: `{"valor":"10.00","vencimento":"2026-12-31","pagador":"   "}`,
			erros: []string{msgPagadorObrigatorio},
		},
		{
			nome:  "rejeita pagador longo demais",
			corpo: `{"valor":"10.00","vencimento":"2026-12-31","pagador":"` + repetir("a", 256) + `"}`,
			erros: []string{msgPagadorLongo},
		},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			agora := caso.agora
			if agora.IsZero() {
				agora = hojeDeTeste()
			}
			dados, erros := Validar(decodificar(t, caso.corpo), agora)
			if !slices.Equal(erros, caso.erros) {
				t.Fatalf("erros = %#v, quero %#v", erros, caso.erros)
			}
			if len(caso.erros) > 0 {
				return
			}
			if dados.ValorCentavos != caso.centavos {
				t.Fatalf("centavos = %d, quero %d", dados.ValorCentavos, caso.centavos)
			}
			if dados.Pagador != caso.pagador {
				t.Fatalf("pagador = %q, quero %q", dados.Pagador, caso.pagador)
			}
			if dados.Vencimento.Format("2006-01-02") != caso.vencimento {
				t.Fatalf("vencimento = %s, quero %s", dados.Vencimento.Format("2006-01-02"), caso.vencimento)
			}
		})
	}
}

func TestVencimentoUsaDiaDeSaoPaulo(t *testing.T) {
	// 14/11 02:30 UTC ainda é 13/11 às 23:30 em São Paulo.
	agora := time.Date(2026, 11, 14, 2, 30, 0, 0, time.UTC)

	_, erros := Validar(decodificar(t, `{"valor":"10.00","vencimento":"2026-11-13","pagador":"Ana"}`), agora)
	if len(erros) != 0 {
		t.Fatalf("13/11 ainda é hoje em São Paulo, erros = %#v", erros)
	}

	_, erros = Validar(decodificar(t, `{"valor":"10.00","vencimento":"2026-11-12","pagador":"Ana"}`), agora)
	if !slices.Equal(erros, []string{msgVencimentoPassado}) {
		t.Fatalf("erros = %#v, quero vencimento no passado", erros)
	}
}

func TestFormatarValor(t *testing.T) {
	casos := []struct {
		centavos int64
		texto    string
	}{
		{15050, "150.50"},
		{1000, "10.00"},
		{5, "0.05"},
		{1, "0.01"},
	}
	for _, caso := range casos {
		if got := FormatarValor(caso.centavos); got != caso.texto {
			t.Fatalf("FormatarValor(%d) = %s, quero %s", caso.centavos, got, caso.texto)
		}
	}
}

func decodificar(t *testing.T, corpo string) Entrada {
	t.Helper()
	var entrada Entrada
	if err := json.Unmarshal([]byte(corpo), &entrada); err != nil {
		t.Fatalf("json de teste: %v", err)
	}
	return entrada
}

func repetir(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
