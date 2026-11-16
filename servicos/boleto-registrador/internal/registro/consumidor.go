package registro

import "context"

// Consumidor recebe BoletoEmitido.
// A implementação de verdade está em fila.Consumidor: ela escuta o
// RabbitMQ, grava uma vez só e só confirma a mensagem depois do broker.
type Consumidor interface {
	Iniciar(ctx context.Context) error
}
