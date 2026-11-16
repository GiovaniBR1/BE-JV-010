# ADR-004: Outbox no emissor

## Status

Aceito.

## Contexto

O ADR-002 escolhe a fila. A primeira versão publicava `BoletoEmitido` depois do `INSERT`. Se o RabbitMQ recusasse, o boleto da Maria ficava `PENDENTE`, a secretaria recebia erro e o aviso não saía. O pedido HTTP não devia depender do broker estar no ar, porque emitir a mensalidade de R$ 150,50 e registrar no banco são esperas diferentes.

## Decisão

O `POST /boletos` grava, na mesma transação, a linha do boleto e uma linha na tabela `outbox` com o JSON já pronto (`valor_centavos` 15050, vencimento `2026-12-31`, pagador Maria Silva). Ou os dois entram, ou nenhum entra. A resposta é `201` mesmo com o RabbitMQ fora.

Um relay em segundo plano, dentro do mesmo binário do emissor, lê as linhas com `enviado_em` nulo, publica com publisher confirm e só então marca `enviado_em`. Se a publicação falha, a linha fica e `proxima_tentativa_em` anda em backoff: 2 segundos, 4 segundos, até 30. Se a conexão cai, o relay espera e abre outra, declarando a topologia de novo.

A volta, `boleto.registrado`, não tem outbox. O registrador publica esse aviso no mesmo canal em confirm mode e só dá ack na mensagem `boleto.emitido` depois que o broker confirma. Nack ou timeout são erro, então a original não é confirmada e volta para a fila. Na próxima entrega o id já existe, o `ON CONFLICT` não cria outra linha, e o aviso é publicado de novo.

## Alternativas consideradas

Publicar no pedido HTTP teria menos peças, mas a secretaria ficaria sem boleto quando só a fila estivesse fora. O pedido da Maria falharia por um problema que não é o da recepção.

Publicar antes de gravar poderia fazer o aviso chegar no registrador sem o emissor ter a linha. A transação única evita esse buraco.

Uma outbox em outro processo, fora deste binário, teria o mesmo efeito com mais um serviço para subir na banca. O relay ficou dentro do emissor.

Uma segunda outbox, agora no registrador, deixaria a volta tão protegida quanto a ida. Ela foi descartada porque o ack só sai depois do confirm, e a mensagem original volta para a fila quando esse confirm falta. O `ON CONFLICT` já impede a linha duplicada. Mais uma tabela repetiria o resultado com mais código para operar.

## Consequências

A mesma carta de ida pode ser publicada duas vezes se o processo cair depois do confirm e antes de marcar `enviado_em`. O registrador grava uma vez só, pela chave primária. O relay não reserva a linha da outbox com `FOR UPDATE SKIP LOCKED`, então dois exemplares do emissor poderiam pegar a mesma carta. De novo, o registrador não duplicaria o cadastro. O preço aceito da volta sem outbox é um caminho mais simples, que repete o aviso em vez de perdê-lo, e que não cria uma segunda mensalidade da Maria.
