package boleto

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCodificarMensagem(t *testing.T) {
	criado := time.Date(2026, 11, 14, 18, 0, 0, 0, time.UTC)
	corpo, err := Codificar(Boleto{
		ID:            "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		ValorCentavos: 15050,
		Vencimento:    time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		Pagador:       "Maria Silva",
		Status:        StatusPendente,
		CriadoEm:      criado,
	})
	if err != nil {
		t.Fatal(err)
	}

	var bruto map[string]json.RawMessage
	if err := json.Unmarshal(corpo, &bruto); err != nil {
		t.Fatal(err)
	}
	if len(bruto["valor_centavos"]) == 0 || bruto["valor_centavos"][0] == '"' {
		t.Fatalf("valor_centavos deve ser número, veio %s", bruto["valor_centavos"])
	}

	var msg Mensagem
	if err := json.Unmarshal(corpo, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.ID != "7c9e6679-7425-40de-944b-e07fc1f90ae7" || msg.ValorCentavos != 15050 {
		t.Fatalf("mensagem = %+v", msg)
	}
	if msg.Vencimento != "2026-12-31" || msg.Pagador != "Maria Silva" || msg.EmitidoEm != "2026-11-14T18:00:00Z" {
		t.Fatalf("mensagem = %+v", msg)
	}
}
