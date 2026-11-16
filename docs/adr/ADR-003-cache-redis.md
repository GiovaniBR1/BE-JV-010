# ADR-003: Cache do GET no Redis

## Status

Aceito.

## Contexto

A secretaria da Palavra Viva e a própria Maria perguntam mais de uma vez se o boleto de R$ 150,50 já foi registrado. A leitura volta no emissor, que é quem sabe se o pedido ainda está `PENDENTE` ou se o registrador já avisou `REGISTRADO`. Se cada pergunta fosse direto ao Postgres, a mesma linha seria lida de novo só porque a consulta se repetiu.

## Decisão

`GET /boletos/{id}` no emissor usa cache-aside no Redis. A chave é `boleto:{id}` e o valor é o JSON da resposta, o mesmo corpo que o cliente vê, com `valor` em texto (`"150.50"`) e `status`. O TTL vem de `CACHE_TTL` e o padrão é 5 minutos.

A primeira leitura de um id que está no banco responde com o cabeçalho `X-Cache: miss` e guarda a cópia. A seguinte, dentro do TTL, responde `X-Cache: hit` e não volta ao Postgres. Um boleto que não existe responde 404 e não é guardado, para a próxima leitura ainda poder achá-lo. Quando chega `boleto.registrado`, o emissor muda o status para `REGISTRADO` e apaga a chave. A leitura seguinte monta o JSON de novo, já com o status novo.

Se o Redis não responde, a leitura segue no Postgres, o cabeçalho vai `X-Cache: bypass` e o GET continua 200. O cache não é caminho obrigatório. O `GET /boletos/{id}` do registrador não usa cache: ele mostra a cópia daquele serviço, escrita uma vez. O cache que muda de estado é o do emissor.

## Alternativas consideradas

Sem cache, a leitura seria simples e, neste volume, barata. O critério do módulo pede um cache com validade e uma regra de quando apagar, então essa alternativa não responde o pedido.

Cachear também o 404 deixaria uma consulta feita cedo demais como "não existe" até o TTL, mesmo depois de o boleto da Maria aparecer. Por isso o não encontrado fica de fora do Redis.

Só o TTL, sem apagar na mudança, faria a secretaria ver `PENDENTE` por até 5 minutos depois do registro. O apagar existe porque o dado muda. O TTL continua como limite para a corrida em que uma leitura antiga grava a cópia logo depois do apagar.

## Consequências

Uma leitura que já leu `PENDENTE` no banco pode gravar essa cópia depois do apagar, se os dois se cruzam. Não há trava em volta disso. O TTL de 5 minutos limita essa janela. O registrador continua lendo o próprio banco direto, e o preço aceito é essa janela curta em troca de não segurar o GET quando o Redis cai.
