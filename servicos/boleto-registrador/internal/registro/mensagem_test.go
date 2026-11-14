package registro

import (
	"errors"
	"testing"
	"time"
)

const exemplo = `{
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "valor_centavos": 15050,
  "vencimento": "2026-12-31",
  "pagador": "Maria Silva",
  "emitido_em": "2026-11-14T18:00:00Z"
}`

func TestDecodificar(t *testing.T) {
	msg, err := Decodificar([]byte(exemplo))
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID != "7c9e6679-7425-40de-944b-e07fc1f90ae7" || msg.ValorCentavos != 15050 {
		t.Fatalf("mensagem = %+v", msg)
	}
	if msg.Vencimento.Format("2006-01-02") != "2026-12-31" || msg.Pagador != "Maria Silva" {
		t.Fatalf("mensagem = %+v", msg)
	}
	if !msg.EmitidoEm.Equal(time.Date(2026, 11, 14, 18, 0, 0, 0, time.UTC)) {
		t.Fatalf("emitido_em = %s", msg.EmitidoEm)
	}
}

func TestDecodificarRejeitaLixo(t *testing.T) {
	casos := []string{
		`{`,
		`{}`,
		`{"id":"nao-e-uuid","valor_centavos":10,"vencimento":"2026-12-31","pagador":"Ana","emitido_em":"2026-11-14T18:00:00Z"}`,
		`{"id":"7c9e6679-7425-40de-944b-e07fc1f90ae7","valor_centavos":0,"vencimento":"2026-12-31","pagador":"Ana","emitido_em":"2026-11-14T18:00:00Z"}`,
		`{"id":"7c9e6679-7425-40de-944b-e07fc1f90ae7","valor_centavos":10,"vencimento":"31/12/2026","pagador":"Ana","emitido_em":"2026-11-14T18:00:00Z"}`,
		`{"id":"7c9e6679-7425-40de-944b-e07fc1f90ae7","valor_centavos":10,"vencimento":"2026-12-31","pagador":"  ","emitido_em":"2026-11-14T18:00:00Z"}`,
	}
	for _, corpo := range casos {
		_, err := Decodificar([]byte(corpo))
		if !errors.Is(err, ErrMensagemInvalida) {
			t.Fatalf("corpo %s: err = %v", corpo, err)
		}
	}
}
