# ADR-003: Cache do GET no Redis

## Status

Aceito.

## Contexto

O cliente consulta o boleto pelo id. A leitura volta no emissor, que é
quem sabe se o pedido ainda está `PENDENTE` ou se o registrador já
avisou `REGISTRADO`. A mesma leitura, em sequência, não precisa ir ao
Postgres toda vez.

## Decisão

`GET /boletos/{id}` no emissor usa cache-aside no Redis.

- A chave é `boleto:{id}`.
- O valor é o JSON da resposta.
- O TTL vem de `CACHE_TTL`, padrão 5 minutos.
- "Não encontrei" não é guardado. A próxima leitura precisa poder achar.
- Quando chega `boleto.registrado`, o emissor muda o status e apaga a
  chave. A leitura seguinte monta o JSON de novo, já com `REGISTRADO`.
- Se o Redis não responde, a leitura segue no Postgres e o header
  `X-Cache` vai `bypass`. O GET não falha por causa do cache.

O `GET /boletos/{id}` do registrador não usa cache. Ele mostra a cópia
daquele serviço, escrita uma vez. O cache que a banca pede é o da
leitura que muda de estado.

## Alternativas consideradas

1. Sem cache. A leitura é barata neste volume, e o critério do módulo
   pede um cache com TTL e uma regra de apagar.
2. Cachear também o 404. Uma consulta feita cedo demais ficaria "não
   existe" até o TTL, mesmo depois do boleto aparecer.
3. Só o TTL, sem apagar na mudança. O cliente veria `PENDENTE` por até
   5 minutos depois do registro. O apagar existe porque o dado muda.

## Consequências

Uma leitura que já leu `PENDENTE` no banco pode gravar essa cópia
depois do apagar, se os dois se cruzam. O TTL limita essa janela a 5
minutos. Não há trava em volta disso.

O registrador continua lendo o próprio banco direto.
