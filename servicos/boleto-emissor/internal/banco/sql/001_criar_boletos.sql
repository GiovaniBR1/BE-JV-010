-- Boletos pedidos a este serviço.
-- valor_centavos usa a mesma unidade do Go: 15050 = R$ 150,50.

CREATE TABLE IF NOT EXISTS boletos (
    id UUID PRIMARY KEY,
    valor_centavos BIGINT NOT NULL CHECK (valor_centavos > 0),
    vencimento DATE NOT NULL,
    pagador TEXT NOT NULL CHECK (pagador <> ''),
    status TEXT NOT NULL DEFAULT 'PENDENTE',
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);
