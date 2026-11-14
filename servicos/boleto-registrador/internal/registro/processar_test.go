package registro

import (
	"context"
	"testing"
	"time"
)

// memoria repete a regra do INSERT ... ON CONFLICT (id) DO NOTHING.
type memoria struct {
	porID map[string]Registro
}

func (m *memoria) Registrar(_ context.Context, reg Registro) (bool, error) {
	if m.porID == nil {
		m.porID = map[string]Registro{}
	}
	if _, existe := m.porID[reg.ID]; existe {
		return false, nil
	}
	m.porID[reg.ID] = reg
	return true, nil
}

func mensagemDeTeste() Mensagem {
	return Mensagem{
		ID:            "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		ValorCentavos: 15050,
		Vencimento:    time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		Pagador:       "Maria Silva",
		EmitidoEm:     time.Date(2026, 11, 14, 18, 0, 0, 0, time.UTC),
	}
}

func TestMensagemRepetidaGravaUmaLinha(t *testing.T) {
	repo := &memoria{}
	msg := mensagemDeTeste()

	novo, err := Processar(context.Background(), repo, msg)
	if err != nil {
		t.Fatal(err)
	}
	if !novo {
		t.Fatal("a primeira entrega devia gravar")
	}

	repetida := msg
	repetida.Pagador = "Outro Nome"
	novo, err = Processar(context.Background(), repo, repetida)
	if err != nil {
		t.Fatal(err)
	}
	if novo {
		t.Fatal("a segunda entrega não devia gravar de novo")
	}
	if len(repo.porID) != 1 {
		t.Fatalf("linhas = %d", len(repo.porID))
	}
	gravado := repo.porID[msg.ID]
	if gravado.Status != StatusRegistrado || gravado.Pagador != "Maria Silva" || gravado.ValorCentavos != 15050 {
		t.Fatalf("gravado = %+v", gravado)
	}
}
