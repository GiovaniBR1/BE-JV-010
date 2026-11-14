package boleto

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Servico grava o boleto PENDENTE e avisa o publicador.
type Servico struct {
	repo Repositorio
	pub  Publicador
}

func NovoServico(repo Repositorio, pub Publicador) *Servico {
	return &Servico{repo: repo, pub: pub}
}

// Emitir espera dados que já passaram por Validar.
func (s *Servico) Emitir(ctx context.Context, dados Dados) (Boleto, error) {
	boleto := Boleto{
		ID:            uuid.NewString(),
		ValorCentavos: dados.ValorCentavos,
		Vencimento:    dados.Vencimento,
		Pagador:       dados.Pagador,
		Status:        StatusPendente,
		CriadoEm:      time.Now().UTC(),
	}
	if err := s.repo.Salvar(ctx, boleto); err != nil {
		return Boleto{}, err
	}
	// O boleto já está PENDENTE. Se a fila recusar, o cliente recebe erro
	// e a mensagem não sai. Não há outbox: ver ADR-002.
	if err := s.pub.PublicarBoletoEmitido(ctx, boleto); err != nil {
		return Boleto{}, err
	}
	return boleto, nil
}
