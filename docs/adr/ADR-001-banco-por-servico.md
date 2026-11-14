# ADR-001: Dois serviços, cada um com o seu banco

## Status

Aceito.

## Contexto

A central de boletos tem dois momentos diferentes. Alguém pede a emissão
(valor, vencimento, pagador). Depois o boleto precisa ser registrado no
banco. O critério do módulo pede dois serviços, cada um com o seu banco,
no mesmo espírito do emissor e do gravador do exemplo de comprovantes PIX.

## Decisão

São dois serviços:

- `boleto-emissor` recebe `POST /boletos`, valida e grava o boleto como
  `PENDENTE` no Postgres dele.
- `boleto-registrador` guarda, no Postgres dele, o boleto que ele
  registrar. Na Parte 1 ele só sobe, cria a tabela e responde `GET /health`.

Não há tabela compartilhada nem biblioteca de domínio compartilhada.
O id gerado pelo emissor é a chave primária do registrador, e é com ela
que a mensagem repetida deixa de virar uma segunda linha.

Como os dois vão conversar fica para o ADR-002. O cache de leitura fica
para o ADR-003.

## Alternativas consideradas

1. Um serviço só, com um banco só. Menos peças para subir, e o registro
   no banco aconteceria no mesmo processo que atende o cliente. Não
   atende o critério de dois serviços e dois bancos.
2. Dois serviços apontando para o mesmo Postgres. A implantação separa,
   mas o schema continua único: uma migração de um lado quebra o outro.
3. Dois serviços e dois bancos, com o registrador consultando o emissor
   por HTTP na hora de gravar. O dado do registrador passaria a depender
   do emissor estar no ar. A cópia local, alimentada pela mensagem,
   deixa o registrador atender a leitura com o que ele mesmo gravou.

A opção 3 é a escolhida. A comparação fila contra HTTP direto, incluindo
a garantia de "pelo menos uma vez", fica no ADR-002 para não misturar
duas decisões num arquivo só.

## Consequências

O mesmo boleto existe nos dois bancos, com campos parecidos e donos
diferentes. O emissor sabe que pediram a emissão. O registrador sabe
que registrou. Os dois podem divergir por um tempo, até a mensagem
ser consumida.

Subir o projeto pede dois Postgres. No perfil A isso fica no
`docker compose`.
