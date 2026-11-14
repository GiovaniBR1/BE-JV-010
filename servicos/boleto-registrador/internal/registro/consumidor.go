package registro

import (
	"context"
	"fmt"
)

// Consumidor recebe BoletoEmitido.
// A Parte 2 troca o Noop por quem escuta o RabbitMQ, confere o id
// e grava com StatusRegistrado só na primeira vez.
type Consumidor interface {
	Iniciar(ctx context.Context) error
}

// ConsumidorNoop segura o repositório e não escuta fila.
type ConsumidorNoop struct {
	Repo *Repositorio
}

func (c ConsumidorNoop) Iniciar(context.Context) error {
	if c.Repo == nil {
		return fmt.Errorf("repositório não configurado")
	}
	return nil
}

// Garante, na compilação, que o noop continua implementando Consumidor.
var _ Consumidor = ConsumidorNoop{}
