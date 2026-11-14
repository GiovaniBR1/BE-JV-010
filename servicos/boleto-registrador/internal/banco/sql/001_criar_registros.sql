-- Cópia do boleto que este serviço registrar.
-- O id é o mesmo gerado pelo emissor. Na Parte 2, se a mensagem
-- chegar de novo, a chave primária impede a segunda linha.

CREATE TABLE IF NOT EXISTS boletos_registrados (
    id UUID PRIMARY KEY,
    valor_centavos BIGINT NOT NULL CHECK (valor_centavos > 0),
    vencimento DATE NOT NULL,
    pagador TEXT NOT NULL CHECK (pagador <> ''),
    status TEXT NOT NULL CHECK (status <> ''),
    registrado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);
