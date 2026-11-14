package registro

import (
	"context"
	"time"
)

// Gravador persiste o registro. Registrar devolve novo=false quando o id
// já existe: é o ON CONFLICT (id) DO NOTHING do Postgres.
type Gravador interface {
	Registrar(ctx context.Context, reg Registro) (novo bool, err error)
}

// Processar simula o registro no banco gravando status REGISTRADO.
// registrado_em é o instante desta gravação, não o emitido_em da mensagem.
// A segunda chamada com o mesmo id não cria outra linha.
func Processar(ctx context.Context, g Gravador, msg Mensagem) (bool, error) {
	return g.Registrar(ctx, Registro{
		ID:            msg.ID,
		ValorCentavos: msg.ValorCentavos,
		Vencimento:    msg.Vencimento,
		Pagador:       msg.Pagador,
		Status:        StatusRegistrado,
		RegistradoEm:  time.Now().UTC(),
	})
}
