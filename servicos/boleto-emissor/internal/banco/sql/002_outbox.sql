-- Intenção de publicar, gravada na mesma transação do boleto.
-- O relay manda as linhas com enviado_em nulo e só então marca a data.

CREATE TABLE IF NOT EXISTS outbox (
    id UUID PRIMARY KEY,
    boleto_id UUID NOT NULL,
    payload TEXT NOT NULL CHECK (payload <> ''),
    tentativas INT NOT NULL DEFAULT 0,
    proxima_tentativa_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    enviado_em TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS outbox_pendentes
    ON outbox (proxima_tentativa_em)
    WHERE enviado_em IS NULL;
