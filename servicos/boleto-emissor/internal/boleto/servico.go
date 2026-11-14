package boleto

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Servico grava o boleto PENDENTE e a mensagem na outbox.
// Quem publica é o relay, fora deste pedido.
type Servico struct {
	repo Repositorio
}

func NovoServico(repo Repositorio) *Servico {
	return &Servico{repo: repo}
}

// Emitir espera dados que já passaram por Validar.
// O POST responde mesmo se o RabbitMQ estiver fora: a outbox segura a mensagem.
func (s *Servico) Emitir(ctx context.Context, dados Dados) (Boleto, error) {
	boleto := Boleto{
		ID:            uuid.NewString(),
		ValorCentavos: dados.ValorCentavos,
		Vencimento:    dados.Vencimento,
		Pagador:       dados.Pagador,
		Status:        StatusPendente,
		CriadoEm:      time.Now().UTC(),
	}
	evento, err := Codificar(boleto)
	if err != nil {
		return Boleto{}, err
	}
	if err := s.repo.Salvar(ctx, boleto, evento); err != nil {
		return Boleto{}, err
	}
	return boleto, nil
}
