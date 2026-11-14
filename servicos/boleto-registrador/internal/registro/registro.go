package registro

import "time"

// StatusRegistrado é o estado que a Parte 2 grava depois de simular o banco.
const StatusRegistrado = "REGISTRADO"

// Registro é a cópia do boleto neste serviço.
// O ID é o do emissor, e também a chave primária da tabela.
type Registro struct {
	ID            string
	ValorCentavos int64
	Vencimento    time.Time
	Pagador       string
	Status        string
	RegistradoEm  time.Time
}
