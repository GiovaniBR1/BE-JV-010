package fila

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ErrRegistradoInvalido é JSON que não dá para aplicar. Vai para a DLQ.
var ErrRegistradoInvalido = errors.New("boleto.registrado inválido")

// Atualizador muda o status do boleto no Postgres do emissor.
type Atualizador interface {
	MarcarRegistrado(ctx context.Context, id string) (bool, error)
}

// Apagador remove a cópia do GET no Redis.
type Apagador interface {
	Apagar(ctx context.Context, id string) error
}

// AplicarRegistrado marca REGISTRADO e apaga o cache.
// O apagar falhar não desfaz o status: o TTL ainda expira a cópia velha.
func AplicarRegistrado(ctx context.Context, atual Atualizador, cache Apagador, corpo []byte) error {
	var msg struct {
		ID           string `json:"id"`
		RegistradoEm string `json:"registrado_em"`
	}
	if err := json.Unmarshal(corpo, &msg); err != nil || msg.ID == "" {
		return ErrRegistradoInvalido
	}
	if _, err := time.Parse(time.RFC3339, msg.RegistradoEm); err != nil {
		return ErrRegistradoInvalido
	}
	achou, err := atual.MarcarRegistrado(ctx, msg.ID)
	if err != nil {
		return err
	}
	if achou {
		slog.Info("boleto marcado como registrado", "id", msg.ID)
	} else {
		slog.Info("boleto.registrado sem linha no emissor", "id", msg.ID)
	}
	if cache != nil {
		if err := cache.Apagar(ctx, msg.ID); err != nil {
			slog.Warn("apagar cache", "id", msg.ID, "erro", err)
		}
	}
	return nil
}

func tratarRegistrado(ctx context.Context, atual Atualizador, cache Apagador, entrega amqp.Delivery) error {
	err := AplicarRegistrado(ctx, atual, cache, entrega.Body)
	if errors.Is(err, ErrRegistradoInvalido) {
		slog.Error("boleto.registrado inválido, enviado para a DLQ", "erro", err)
		if nackErr := entrega.Nack(false, false); nackErr != nil {
			return fmt.Errorf("nack: %w", nackErr)
		}
		return nil
	}
	if err != nil {
		// Sem ack. Ao fechar a conexão o RabbitMQ devolve a mensagem.
		return err
	}
	if err := entrega.Ack(false); err != nil {
		return fmt.Errorf("ack: %w", err)
	}
	return nil
}
