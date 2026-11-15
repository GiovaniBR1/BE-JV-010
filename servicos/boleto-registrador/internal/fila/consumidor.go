package fila

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/registro"
	amqp "github.com/rabbitmq/amqp091-go"
)

type acao int

const (
	acaoAck acao = iota
	acaoRepublicar
	acaoDLQ
)

// Consumidor escuta boleto.emitido com ack manual.
type Consumidor struct {
	ch       *amqp.Channel
	gravador registro.Gravador
}

func NovoConsumidor(ch *amqp.Channel, gravador registro.Gravador) *Consumidor {
	return &Consumidor{ch: ch, gravador: gravador}
}

var _ registro.Consumidor = (*Consumidor)(nil)

func (c *Consumidor) Iniciar(ctx context.Context) error {
	if err := c.ch.Qos(Prefetch, 0, false); err != nil {
		return fmt.Errorf("prefetch: %w", err)
	}
	// autoAck falso: só confirmamos depois de gravar, ou de decidir a DLQ.
	entregas, err := c.ch.Consume(Fila, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consumir: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case entrega, ok := <-entregas:
			if !ok {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("canal da fila fechou")
			}
			c.tratar(ctx, entrega)
		}
	}
}

func (c *Consumidor) tratar(ctx context.Context, entrega amqp.Delivery) {
	msg, err := registro.Decodificar(entrega.Body)
	var novo bool
	if err == nil {
		novo, err = registro.Processar(ctx, c.gravador, msg)
	}
	if err == nil {
		err = c.publicarRegistrado(ctx, msg)
	}

	escolha := decidir(err, numeroDaTentativa(entrega))
	switch escolha {
	case acaoAck:
		if novo {
			slog.Info("boleto registrado", "id", msg.ID)
		} else {
			slog.Info("mensagem repetida, registro já existia", "id", msg.ID)
		}
		c.ack(entrega)
	case acaoRepublicar:
		tentativa := numeroDaTentativa(entrega)
		slog.Warn("falha ao registrar, nova tentativa", "id", msg.ID, "tentativa", tentativa, "erro", err)
		if pubErr := c.republicar(ctx, entrega, tentativa+1); pubErr != nil {
			slog.Error("não consegui republicar, enviando para a DLQ", "id", msg.ID, "erro", pubErr)
			c.nack(entrega)
			return
		}
		// O original sai da fila. A cópia, com o contador novo, é outra entrega.
		// Se as duas existirem por um instante, o id único segura a linha.
		c.ack(entrega)
	case acaoDLQ:
		if errors.Is(err, registro.ErrMensagemInvalida) {
			slog.Error("mensagem inválida, enviada para a DLQ", "erro", err)
		} else {
			slog.Error("tentativas esgotadas, enviada para a DLQ", "id", msg.ID, "erro", err)
		}
		c.nack(entrega)
	default:
		panic(fmt.Sprintf("ação de fila não tratada: %d", escolha))
	}
}

// decidir separa ack, nova tentativa e DLQ.
// Mensagem inválida não volta para a fila: o JSON não melhora sozinho.
func decidir(err error, tentativa int) acao {
	if err == nil {
		return acaoAck
	}
	if errors.Is(err, registro.ErrMensagemInvalida) || tentativa >= MaxTentativas {
		return acaoDLQ
	}
	return acaoRepublicar
}

func (c *Consumidor) publicarRegistrado(ctx context.Context, msg registro.Mensagem) error {
	corpo, err := registro.CodificarRegistrado(msg.ID, time.Now().UTC())
	if err != nil {
		return err
	}
	err = c.ch.PublishWithContext(ctx, Exchange, RoutingKeyRegistrado, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    msg.ID,
		Body:         corpo,
	})
	if err != nil {
		return fmt.Errorf("publicar boleto.registrado: %w", err)
	}
	return nil
}

func (c *Consumidor) republicar(ctx context.Context, entrega amqp.Delivery, proxima int) error {
	headers := amqp.Table{}
	for chave, valor := range entrega.Headers {
		headers[chave] = valor
	}
	headers["x-tentativas"] = int32(proxima)
	return c.ch.PublishWithContext(ctx, Exchange, RoutingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    entrega.MessageId,
		Headers:      headers,
		Body:         entrega.Body,
	})
}

func numeroDaTentativa(entrega amqp.Delivery) int {
	if entrega.Headers == nil {
		return 1
	}
	switch valor := entrega.Headers["x-tentativas"].(type) {
	case int:
		if valor >= 1 {
			return valor
		}
	case int32:
		if valor >= 1 {
			return int(valor)
		}
	case int64:
		if valor >= 1 {
			return int(valor)
		}
	}
	return 1
}

func (c *Consumidor) ack(entrega amqp.Delivery) {
	if err := entrega.Ack(false); err != nil {
		slog.Error("ack", "erro", err)
	}
}

// nack sem requeue. A fila manda a mensagem para a DLQ.
func (c *Consumidor) nack(entrega amqp.Delivery) {
	if err := entrega.Nack(false, false); err != nil {
		slog.Error("nack", "erro", err)
	}
}
