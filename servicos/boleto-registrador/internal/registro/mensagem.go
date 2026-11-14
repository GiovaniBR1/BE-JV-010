package registro

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrMensagemInvalida marca JSON que não dá para registrar.
// O consumidor manda esse caso para a DLQ, sem tentar de novo.
var ErrMensagemInvalida = errors.New("mensagem inválida")

// Mensagem é o JSON BoletoEmitido publicado pelo emissor.
type Mensagem struct {
	ID            string
	ValorCentavos int64
	Vencimento    time.Time
	Pagador       string
	EmitidoEm     time.Time
}

type mensagemJSON struct {
	ID            string `json:"id"`
	ValorCentavos int64  `json:"valor_centavos"`
	Vencimento    string `json:"vencimento"`
	Pagador       string `json:"pagador"`
	EmitidoEm     string `json:"emitido_em"`
}

// Decodificar lê o JSON. Não reaplica a regra de "vencimento no futuro":
// a mensagem pode chegar no dia seguinte e ainda assim ser válida.
func Decodificar(corpo []byte) (Mensagem, error) {
	var bruto mensagemJSON
	if err := json.Unmarshal(corpo, &bruto); err != nil {
		return Mensagem{}, fmt.Errorf("%w: %s", ErrMensagemInvalida, err.Error())
	}
	if _, err := uuid.Parse(bruto.ID); err != nil {
		return Mensagem{}, fmt.Errorf("%w: id", ErrMensagemInvalida)
	}
	if bruto.ValorCentavos <= 0 {
		return Mensagem{}, fmt.Errorf("%w: valor_centavos", ErrMensagemInvalida)
	}
	pagador := strings.TrimSpace(bruto.Pagador)
	if pagador == "" {
		return Mensagem{}, fmt.Errorf("%w: pagador", ErrMensagemInvalida)
	}
	vencimento, err := time.Parse("2006-01-02", bruto.Vencimento)
	if err != nil {
		return Mensagem{}, fmt.Errorf("%w: vencimento", ErrMensagemInvalida)
	}
	emitidoEm, err := time.Parse(time.RFC3339, bruto.EmitidoEm)
	if err != nil {
		return Mensagem{}, fmt.Errorf("%w: emitido_em", ErrMensagemInvalida)
	}
	return Mensagem{
		ID:            bruto.ID,
		ValorCentavos: bruto.ValorCentavos,
		Vencimento:    vencimento,
		Pagador:       pagador,
		EmitidoEm:     emitidoEm.UTC(),
	}, nil
}
