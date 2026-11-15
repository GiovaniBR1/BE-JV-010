# ADR-002: Fila no RabbitMQ, com idempotência

## Status

Aceito.

## Contexto

O emissor grava o pedido. O registrador simula o registro no banco e
guarda a cópia dele. O cliente não precisa esperar esse segundo passo.
O critério do módulo pede uma fila entre os dois, e que a mesma mensagem,
se chegar de novo, não grave outra linha.

## Decisão

O emissor grava `BoletoEmitido` junto com o boleto. Um relay publica
no exchange `boletos` (direct, durável), com a routing key `boleto.emitido`.
O jeito de não perder essa publicação está no ADR-004.
A mensagem é persistente e leva `id`, `valor_centavos`, `vencimento`,
`pagador` e `emitido_em`.

O registrador consome a fila `boleto.emitido`, durável, com ack manual
e prefetch 10. Grava com:

```sql
INSERT ... ON CONFLICT (id) DO NOTHING
```

O `id` do emissor é a chave primária. Se a linha já existe, o `INSERT`
não muda nada, o serviço loga que era repetida e dá ack mesmo assim.
A garantia da fila é "pelo menos uma vez": a mensagem pode chegar
repetida, e não se perde enquanto não for confirmada.

Mensagem que não dá para ler vai para a DLQ (`boletos.dlx` /
`boleto.emitido.dlq`) com nack sem requeue. Falha de banco tenta de
novo até 3 vezes e, esgotado o limite, também vai para a DLQ.

## Alternativas consideradas

1. O emissor chama o registrador por HTTP. O cliente espera o "banco",
   e se o registrador estiver fora o pedido falha junto. Não há buffer
   nem como repetir a entrega sozinho.
2. Fila sem idempotência. "Pelo menos uma vez" gravaria duas linhas
   quando o RabbitMQ entregasse de novo.
3. Outbox na mesma transação do boleto. O `INSERT` e a intenção de
   publicar sairiam juntos, e um processo à parte publicaria. É o
   passo seguinte se a publicação não puder se perder. Nesta parte
   fica de fora: são mais peças para explicar na banca, e a
   idempotência do registrador continua valendo quando isso entrar.

## Consequências

A publicação em si saiu do pedido HTTP e foi para a outbox. O motivo
e o formato estão no ADR-004. A idempotência do registrador continua
valendo quando a mesma mensagem sai duas vezes.

Os dois serviços declaram a mesma topologia. O declare é idempotente
só quando os argumentos são iguais; um lado mudar a DLQ sozinho quebra
a subida do outro.

A mensagem repetida não duplica o registro. O preço é aceitar que o
emissor e o registrador fiquem diferentes por um tempo, até o consumo.
