package fila

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/boleto"
)

type caixaFake struct {
	pendentes []boleto.EventoPendente
	enviados  []string
	adiados   []string
}

func (c *caixaFake) Pendentes(context.Context, int) ([]boleto.EventoPendente, error) {
	return c.pendentes, nil
}

func (c *caixaFake) MarcarEnviado(_ context.Context, id string) error {
	c.enviados = append(c.enviados, id)
	return nil
}

func (c *caixaFake) Adiar(_ context.Context, id string, _ time.Time) error {
	c.adiados = append(c.adiados, id)
	return nil
}

type pubFake struct {
	err error
	n   int
}

func (p *pubFake) Publicar(context.Context, []byte) error {
	p.n++
	return p.err
}

func TestEspera(t *testing.T) {
	if Espera(1) != 2*time.Second || Espera(2) != 4*time.Second {
		t.Fatalf("backoff = %s %s", Espera(1), Espera(2))
	}
	if Espera(10) != 30*time.Second {
		t.Fatalf("teto = %s", Espera(10))
	}
}

func TestEntregarPendentesMarcaEnviado(t *testing.T) {
	caixa := &caixaFake{pendentes: []boleto.EventoPendente{{ID: "a", Payload: []byte(`{}`)}}}
	pub := &pubFake{}
	if err := EntregarPendentes(context.Background(), caixa, pub); err != nil {
		t.Fatal(err)
	}
	if pub.n != 1 || len(caixa.enviados) != 1 || caixa.enviados[0] != "a" {
		t.Fatalf("enviados = %v publicações = %d", caixa.enviados, pub.n)
	}
	if len(caixa.adiados) != 0 {
		t.Fatal("não devia adiar")
	}
}

func TestEntregarPendentesAdiaSeOBrokerFalhar(t *testing.T) {
	caixa := &caixaFake{pendentes: []boleto.EventoPendente{{ID: "a", Payload: []byte(`{}`), Tentativas: 1}}}
	pub := &pubFake{err: errors.New("broker fora")}
	if err := EntregarPendentes(context.Background(), caixa, pub); err == nil {
		t.Fatal("esperava erro")
	}
	if len(caixa.enviados) != 0 || len(caixa.adiados) != 1 {
		t.Fatalf("enviados=%v adiados=%v", caixa.enviados, caixa.adiados)
	}
}
