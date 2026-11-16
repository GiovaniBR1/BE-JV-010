package boleto

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type repoFake struct {
	boleto   Boleto
	evento   []byte
	err      error
	chamadas int
}

func (f *repoFake) Salvar(_ context.Context, b Boleto, evento []byte) error {
	f.chamadas++
	f.boleto = b
	f.evento = append([]byte(nil), evento...)
	return f.err
}

func dadosDeTeste() Dados {
	return Dados{
		ValorCentavos: 15050,
		Vencimento:    time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		Pagador:       "Maria Silva",
	}
}

func TestEmitirGravaPendenteEOutbox(t *testing.T) {
	repo := &repoFake{}
	svc := NovoServico(repo)

	boleto, err := svc.Emitir(context.Background(), dadosDeTeste())
	if err != nil {
		t.Fatalf("emitir: %v", err)
	}
	if boleto.Status != StatusPendente || boleto.ID == "" {
		t.Fatalf("boleto = %+v", boleto)
	}
	if repo.chamadas != 1 || repo.boleto.ID != boleto.ID {
		t.Fatal("não gravou o boleto")
	}

	var msg Mensagem
	if err := json.Unmarshal(repo.evento, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.ID != boleto.ID || msg.ValorCentavos != 15050 || msg.Pagador != "Maria Silva" || msg.Vencimento != "2026-12-31" {
		t.Fatalf("evento = %+v", msg)
	}
}

func TestEmitirDevolveErroDoBanco(t *testing.T) {
	repo := &repoFake{err: errors.New("banco fora")}
	svc := NovoServico(repo)

	_, err := svc.Emitir(context.Background(), dadosDeTeste())
	if err == nil {
		t.Fatal("esperava erro")
	}
	if repo.chamadas != 1 {
		t.Fatalf("chamadas = %d", repo.chamadas)
	}
}
