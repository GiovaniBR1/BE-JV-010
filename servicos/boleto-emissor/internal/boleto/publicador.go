package boleto

import "context"

// Publicador avisa que um boleto foi emitido.
// A Parte 2 troca o Noop por uma implementação que publica BoletoEmitido
// no RabbitMQ. A mensagem leva id, valor, vencimento, pagador e emitido_em.
// O id daqui é a chave primária no registrador.
type Publicador interface {
	PublicarBoletoEmitido(ctx context.Context, boleto Boleto) error
}

// PublicadorNoop não envia nada. A Parte 1 termina no banco do emissor.
type PublicadorNoop struct{}

func (PublicadorNoop) PublicarBoletoEmitido(context.Context, Boleto) error {
	return nil
}
