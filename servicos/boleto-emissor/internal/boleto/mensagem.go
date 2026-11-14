package boleto

import (
	"encoding/json"
	"time"
)

// Mensagem é o JSON BoletoEmitido. O registrador tem a mesma forma,
// sem pacote compartilhado: o contrato é este JSON.
type Mensagem struct {
	ID            string `json:"id"`
	ValorCentavos int64  `json:"valor_centavos"`
	Vencimento    string `json:"vencimento"`
	Pagador       string `json:"pagador"`
	EmitidoEm     string `json:"emitido_em"`
}

// Codificar monta a mensagem a partir do boleto já gravado.
// valor_centavos é inteiro, a mesma unidade do banco.
func Codificar(b Boleto) ([]byte, error) {
	return json.Marshal(Mensagem{
		ID:            b.ID,
		ValorCentavos: b.ValorCentavos,
		Vencimento:    b.Vencimento.Format("2006-01-02"),
		Pagador:       b.Pagador,
		EmitidoEm:     b.CriadoEm.UTC().Format(time.RFC3339),
	})
}
