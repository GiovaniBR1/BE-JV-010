package boleto

import (
	"context"
	"errors"
	"testing"
	"time"
)

type repoFake struct {
	boleto   Boleto
	err      error
	chamadas int
}

func (f *repoFake) Salvar(_ context.Context, b Boleto) error {
	f.chamadas++
	f.boleto = b
	return f.err
}

type pubFake struct {
	boleto   Boleto
	err      error
	chamadas int
}

func (f *pubFake) PublicarBoletoEmitido(_ context.Context, b Boleto) error {
	f.chamadas++
	f.boleto = b
	return f.err
}

func dadosDeTeste() Dados {
	return Dados{
		ValorCentavos: 15050,
		Vencimento:    time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		Pagador:       "Maria Silva",
	}
}

func TestEmitirGravaPendenteEPublica(t *testing.T) {
	repo := &repoFake{}
	pub := &pubFake{}
	svc := NovoServico(repo, pub)

	boleto, err := svc.Emitir(context.Background(), dadosDeTeste())
	if err != nil {
		t.Fatalf("emitir: %v", err)
	}
	if boleto.Status != StatusPendente {
		t.Fatalf("status = %s", boleto.Status)
	}
	if boleto.ID == "" {
		t.Fatal("id vazio")
	}
	if boleto.ValorCentavos != 15050 || boleto.Pagador != "Maria Silva" {
		t.Fatalf("boleto gravado diferente: %+v", boleto)
	}
	if repo.chamadas != 1 || repo.boleto.ID != boleto.ID {
		t.Fatalf("repositório não recebeu o boleto emitido")
	}
	if pub.chamadas != 1 || pub.boleto.ID != boleto.ID {
		t.Fatalf("publicador não recebeu o mesmo boleto")
	}
}

func TestEmitirNaoPublicaSeOBancoFalhar(t *testing.T) {
	repo := &repoFake{err: errors.New("banco fora")}
	pub := &pubFake{}
	svc := NovoServico(repo, pub)

	_, err := svc.Emitir(context.Background(), dadosDeTeste())
	if err == nil {
		t.Fatal("esperava erro")
	}
	if pub.chamadas != 0 {
		t.Fatal("publicou mesmo com falha ao gravar")
	}
}

func TestEmitirDevolveErroDoPublicador(t *testing.T) {
	repo := &repoFake{}
	pub := &pubFake{err: errors.New("fila fora")}
	svc := NovoServico(repo, pub)

	_, err := svc.Emitir(context.Background(), dadosDeTeste())
	if err == nil {
		t.Fatal("esperava erro")
	}
	if repo.chamadas != 1 {
		t.Fatal("devia ter gravado antes de publicar")
	}
}

func TestPublicadorNoop(t *testing.T) {
	var pub PublicadorNoop
	if err := pub.PublicarBoletoEmitido(context.Background(), Boleto{ID: "x"}); err != nil {
		t.Fatal(err)
	}
}
