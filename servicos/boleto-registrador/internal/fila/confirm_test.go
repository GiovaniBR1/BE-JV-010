package fila

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/registro"
	amqp "github.com/rabbitmq/amqp091-go"
)

func TestEsperarConfirmacao(t *testing.T) {
	t.Run("ack", func(t *testing.T) {
		ch := make(chan amqp.Confirmation, 1)
		ch <- amqp.Confirmation{Ack: true}
		if err := esperarConfirmacao(context.Background(), ch, time.Second); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("nack", func(t *testing.T) {
		ch := make(chan amqp.Confirmation, 1)
		ch <- amqp.Confirmation{Ack: false}
		err := esperarConfirmacao(context.Background(), ch, time.Second)
		if err == nil || err.Error() != "broker não confirmou a mensagem" {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		ch := make(chan amqp.Confirmation)
		err := esperarConfirmacao(context.Background(), ch, 20*time.Millisecond)
		if err == nil || err.Error() != "confirmação demorou demais" {
			t.Fatalf("err = %v", err)
		}
	})
}

type gravadorFake struct {
	err  error
	novo bool
}

func (g gravadorFake) Registrar(context.Context, registro.Registro) (bool, error) {
	return g.novo, g.err
}

type ackFake struct {
	acks    int
	nacks   int
	requeue bool
}

func (a *ackFake) Ack(uint64, bool) error {
	a.acks++
	return nil
}

func (a *ackFake) Nack(_ uint64, _ bool, requeue bool) error {
	a.nacks++
	a.requeue = requeue
	return nil
}

func (a *ackFake) Reject(uint64, bool) error { return nil }

func entregaDeTeste(body []byte, tentativa int32) (amqp.Delivery, *ackFake) {
	ack := &ackFake{}
	d := amqp.Delivery{Body: body, Acknowledger: ack}
	if tentativa > 0 {
		d.Headers = amqp.Table{"x-tentativas": tentativa}
	}
	return d, ack
}

func TestTratarMandaJSONInvalidoParaDLQ(t *testing.T) {
	c := &Consumidor{gravador: gravadorFake{novo: true}}
	d, ack := entregaDeTeste([]byte(`{}`), 0)
	c.tratar(context.Background(), d)
	if ack.acks != 0 || ack.nacks != 1 || ack.requeue {
		t.Fatalf("acks=%d nacks=%d requeue=%v", ack.acks, ack.nacks, ack.requeue)
	}
}

func TestTratarNaTerceiraFalhaVaiParaDLQ(t *testing.T) {
	c := &Consumidor{gravador: gravadorFake{err: errors.New("banco fora")}}
	d, ack := entregaDeTeste([]byte(`{"id":"7c9e6679-7425-40de-944b-e07fc1f90ae7","valor_centavos":15050,"vencimento":"2026-12-31","pagador":"Maria Silva","emitido_em":"2026-11-14T18:00:00Z"}`), 3)
	c.tratar(context.Background(), d)
	if ack.acks != 0 || ack.nacks != 1 || ack.requeue {
		t.Fatalf("acks=%d nacks=%d requeue=%v", ack.acks, ack.nacks, ack.requeue)
	}
}

func TestTratarSemConfirmDoBrokerNaoDaAck(t *testing.T) {
	c := &Consumidor{
		gravador: gravadorFake{novo: true},
		publicar: func(context.Context, registro.Mensagem) error {
			return errors.New("broker não confirmou a mensagem")
		},
	}
	d, ack := entregaDeTeste([]byte(`{"id":"7c9e6679-7425-40de-944b-e07fc1f90ae7","valor_centavos":15050,"vencimento":"2026-12-31","pagador":"Maria Silva","emitido_em":"2026-11-14T18:00:00Z"}`), 3)
	c.tratar(context.Background(), d)
	if ack.acks != 0 || ack.nacks != 1 || ack.requeue {
		t.Fatalf("acks=%d nacks=%d requeue=%v", ack.acks, ack.nacks, ack.requeue)
	}
}
