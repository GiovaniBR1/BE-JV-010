# ADR-002: Fila no RabbitMQ, com idempotência

## Status

Aceito.

## Contexto

O emissor grava o pedido da Maria. O registrador simula o registro e guarda a cópia dele. A secretaria não precisa esperar esse segundo passo. O critério do módulo pede uma fila entre os dois, e que a mesma mensagem, se chegar de novo, não grave outra linha. A entrega de uma fila costuma ser "pelo menos uma vez": o aviso pode repetir, e não se perde enquanto não for confirmado.

## Decisão

O emissor grava `BoletoEmitido` junto com o boleto. Um relay publica no exchange `boletos` (direct, durável), com a routing key `boleto.emitido`. O jeito de não perder essa publicação está no ADR-004. A mensagem é persistente e leva `id`, `valor_centavos`, `vencimento`, `pagador` e `emitido_em`. Para a Maria, `valor_centavos` é 15050 e o vencimento é `2026-12-31`.

O registrador consome a fila `boleto.emitido`, durável, com ack manual e prefetch 10. Grava com `INSERT ... ON CONFLICT (id) DO NOTHING`. O id do emissor é a chave primária. Se a linha já existe, o `INSERT` não muda nada, o serviço loga que a mensagem era repetida e dá ack mesmo assim.

Mensagem que não dá para ler vai para a DLQ com nack sem requeue. O exchange dessa fila morta é `boletos.dlx`, e a fila é `boleto.emitido.dlq`. A volta usa a irmã `boleto.registrado.dlq`, no mesmo exchange. Quando o banco do registrador falha, a tentativa é contada no cabeçalho `x-tentativas`. A primeira entrega não traz esse cabeçalho e vale 1. Cada republicação grava o número da próxima. `MaxTentativas` é 3: na terceira falha a mensagem deixa de voltar para a fila boa e cai na DLQ. Assim uma cobrança quebrada não atravanca as outras.

Antes de dar ack na mensagem original, o registrador publica `boleto.registrado` e espera o confirm do broker. Nack ou timeout são erro, então a original não é confirmada e volta para a fila. O `ON CONFLICT` impede a segunda linha.

## Alternativas consideradas

Chamar o registrador por HTTP faria a secretaria esperar o "banco". Se ele estivesse fora, o pedido da Maria falharia junto. Não haveria buffer nem uma entrega repetida por conta do correio.

Uma fila sem idempotência honraria o "pelo menos uma vez" gravando duas linhas quando o RabbitMQ entregasse de novo. A Maria apareceria com duas mensalidades. Por isso a chave primária e o `ON CONFLICT` fazem parte da decisão, não um detalhe.

Um tópico no Kafka no lugar do RabbitMQ é o extra que a aula sugere. O RabbitMQ já traz o painel em http://localhost:15672, que é onde se publica a mesma mensagem duas vezes na banca. Kafka ficaria de fora deste recorte.

## Consequências

A publicação da ida saiu do pedido HTTP e foi para a outbox, detalhada no ADR-004. A idempotência do registrador continua valendo quando a mesma mensagem sai duas vezes. Os dois serviços declaram a mesma topologia. O declare só é seguro quando os argumentos são iguais: um lado mudar a DLQ sozinho impede o outro de subir. O preço aceito é que o emissor e o registrador podem divergir por um tempo, até o consumo, e que a ordem entre boletos diferentes não é prometida. O que a Maria precisa é a ida e a volta do mesmo id.
