package boleto

import "time"

// StatusPendente é o único estado que o emissor grava.
// Quem marca o boleto como registrado é o outro serviço.
const StatusPendente = "PENDENTE"

// Boleto é a emissão pedida pelo cliente e guardada no Postgres do emissor.
type Boleto struct {
	ID            string
	ValorCentavos int64
	Vencimento    time.Time
	Pagador       string
	Status        string
	CriadoEm      time.Time
}
