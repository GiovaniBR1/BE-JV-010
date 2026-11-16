// Pacote fila fala com o RabbitMQ deste serviço.
// A topologia é a mesma no emissor. Os dois declaram de novo:
// o RabbitMQ aceita se o nome e os argumentos forem iguais.
package fila

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	Exchange             = "boletos"
	RoutingKey           = "boleto.emitido"
	Fila                 = "boleto.emitido"
	RoutingKeyRegistrado = "boleto.registrado"
	FilaRegistrado       = "boleto.registrado"
	ExchangeDLQ          = "boletos.dlx"
	FilaDLQ              = "boleto.emitido.dlq"
	FilaDLQRegistrado    = "boleto.registrado.dlq"

	// Prefetch limita o que fica sem ack neste processo.
	Prefetch = 10

	// MaxTentativas conta a entrega atual. Na terceira falha de banco,
	// a mensagem vai para a DLQ em vez de girar para sempre.
	MaxTentativas = 3
)

// Conectar tenta de novo por uns 30s. O healthcheck do RabbitMQ
// às vezes passa um instante antes da porta AMQP aceitar conexão.
func Conectar(ctx context.Context, url string) (*amqp.Connection, *amqp.Channel, error) {
	var ultimo error
	for tentativa := 1; tentativa <= 15; tentativa++ {
		conn, err := amqp.Dial(url)
		if err == nil {
			ch, err := conn.Channel()
			if err == nil {
				return conn, ch, nil
			}
			conn.Close()
			ultimo = err
		} else {
			ultimo = err
		}
		slog.Warn("rabbitmq indisponível", "tentativa", tentativa, "erro", ultimo)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, nil, fmt.Errorf("conectar no rabbitmq: %w", ultimo)
}

// Declarar cria exchange, filas, DLQs e os binds.
// O emissor declara a mesma topologia. O RabbitMQ aceita de novo
// se os argumentos forem iguais.
func Declarar(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(Exchange, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("exchange: %w", err)
	}
	if err := ch.ExchangeDeclare(ExchangeDLQ, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("exchange da dlq: %w", err)
	}
	if err := declararFila(ch, FilaDLQ, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(FilaDLQ, RoutingKey, ExchangeDLQ, false, nil); err != nil {
		return fmt.Errorf("bind da dlq emitido: %w", err)
	}
	if err := declararFila(ch, FilaDLQRegistrado, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(FilaDLQRegistrado, RoutingKeyRegistrado, ExchangeDLQ, false, nil); err != nil {
		return fmt.Errorf("bind da dlq registrado: %w", err)
	}
	argsEmitido := amqp.Table{
		"x-dead-letter-exchange":    ExchangeDLQ,
		"x-dead-letter-routing-key": RoutingKey,
	}
	if err := declararFila(ch, Fila, argsEmitido); err != nil {
		return err
	}
	if err := ch.QueueBind(Fila, RoutingKey, Exchange, false, nil); err != nil {
		return fmt.Errorf("bind emitido: %w", err)
	}
	argsRegistrado := amqp.Table{
		"x-dead-letter-exchange":    ExchangeDLQ,
		"x-dead-letter-routing-key": RoutingKeyRegistrado,
	}
	if err := declararFila(ch, FilaRegistrado, argsRegistrado); err != nil {
		return err
	}
	if err := ch.QueueBind(FilaRegistrado, RoutingKeyRegistrado, Exchange, false, nil); err != nil {
		return fmt.Errorf("bind registrado: %w", err)
	}
	return nil
}

func declararFila(ch *amqp.Channel, nome string, args amqp.Table) error {
	if _, err := ch.QueueDeclare(nome, true, false, false, false, args); err != nil {
		return fmt.Errorf("fila %s: %w", nome, err)
	}
	return nil
}
