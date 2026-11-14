package boleto

import "context"

// Publicador avisa que um boleto foi emitido.
// Em produção, fila.Publicador manda o JSON BoletoEmitido ao RabbitMQ.
// O teste usa o Noop, ou um dublê, para não abrir conexão.
type Publicador interface {
	PublicarBoletoEmitido(ctx context.Context, boleto Boleto) error
}

// PublicadorNoop não envia nada. Serve para teste e para subir sem fila.
type PublicadorNoop struct{}

func (PublicadorNoop) PublicarBoletoEmitido(context.Context, Boleto) error {
	return nil
}
