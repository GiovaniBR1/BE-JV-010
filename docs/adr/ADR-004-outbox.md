# ADR-004: Outbox no emissor

## Status

Aceito.

## Contexto

O ADR-002 escolhe a fila. A primeira versão publicava depois do
`INSERT`. Se o RabbitMQ recusasse, o boleto ficava `PENDENTE`, o
cliente recebia erro, e a mensagem não saía. O pedido HTTP não devia
depender do broker estar no ar.

## Decisão

O `POST /boletos` grava, na mesma transação, a linha do boleto e uma
linha na tabela `outbox` com o JSON já pronto. A resposta é `201`
mesmo com o RabbitMQ fora.

Um relay em segundo plano lê as linhas com `enviado_em` nulo, publica
com publisher confirm e só então marca `enviado_em`. Se a publicação
falha, a linha fica e `proxima_tentativa_em` anda em backoff (2s, 4s,
até 30s). Se a conexão cai, o relay espera e abre outra, declarando
a topologia de novo.

## Alternativas consideradas

1. Publicar no pedido HTTP. Menos peças, e o cliente fica sem boleto
   quando só a fila está fora.
2. Publicar antes de gravar. A mensagem pode chegar no registrador
   sem o emissor ter o boleto.
3. Outbox com um processo separado, fora deste binário. O mesmo
   efeito, com mais um serviço para subir na banca.

## Consequências

A mesma mensagem pode ser publicada duas vezes se o processo cair
depois do confirm e antes de marcar `enviado_em`. O registrador já
grava uma vez só, pela chave primária.

O relay é um loop dentro do emissor. Não há um segundo processo.

O aviso `boleto.registrado` não tem outbox. Se a publicação falha, a
mensagem `boleto.emitido` não é confirmada e volta para a fila. Na
próxima entrega o registrador acha o id e publica o aviso de novo.
