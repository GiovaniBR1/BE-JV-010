package boleto

import "time"

const (
	// StatusPendente é o estado de um boleto recém-pedido.
	StatusPendente = "PENDENTE"
	// StatusRegistrado chega pelo aviso boleto.registrado.
	StatusRegistrado = "REGISTRADO"
)

// Boleto é a emissão pedida pelo cliente e guardada no Postgres do emissor.
type Boleto struct {
	ID            string
	ValorCentavos int64
	Vencimento    time.Time
	Pagador       string
	Status        string
	CriadoEm      time.Time
}
