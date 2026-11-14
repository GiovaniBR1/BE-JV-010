package fila

import (
	"errors"
	"testing"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/registro"
	amqp "github.com/rabbitmq/amqp091-go"
)

func TestDecidir(t *testing.T) {
	if decidir(nil, 1) != acaoAck {
		t.Fatal("sucesso deve dar ack")
	}
	if decidir(registro.ErrMensagemInvalida, 1) != acaoDLQ {
		t.Fatal("mensagem inválida deve ir para a DLQ na hora")
	}
	if decidir(errors.New("banco fora"), 1) != acaoRepublicar {
		t.Fatal("falha de banco na primeira vez deve tentar de novo")
	}
	if decidir(errors.New("banco fora"), MaxTentativas) != acaoDLQ {
		t.Fatal("na última tentativa a falha deve ir para a DLQ")
	}
}

func TestNumeroDaTentativa(t *testing.T) {
	if numeroDaTentativa(amqp.Delivery{}) != 1 {
		t.Fatal("sem header, a tentativa é 1")
	}
	entrega := amqp.Delivery{Headers: amqp.Table{"x-tentativas": int32(2)}}
	if numeroDaTentativa(entrega) != 2 {
		t.Fatal("header int32")
	}
}
