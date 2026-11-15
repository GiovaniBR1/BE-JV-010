# Roteiro da banca (10 minutos)

Fale de pé, com o projeto já no ar (`docker compose up --build -d`).
O relógio abaixo já inclui uma folga para pergunta curta no fim.

## 0:00 — O que é

"É uma central de boletos, perfil A, tudo no Docker.
O cliente pede o boleto num serviço. Outro serviço registra.
Cada um tem o seu Postgres. No meio tem fila. A leitura quente fica no Redis."

Aponte o desenho do README.

## 1:00 — Os dois bancos (ADR-001)

Mostre o `POST`:

```bash
curl -s -X POST http://localhost:8080/boletos \
  -H 'Content-Type: application/json' \
  -d '{"valor":"150.50","vencimento":"2026-12-31","pagador":"Maria Silva"}'
```

"O valor não é float. 150 reais e 50 centavos viram 15050, no Go e na coluna.
O vencimento de ontem volta 400. O dia de hoje é o de São Paulo, não o UTC do container."

Abra os dois `select` do README. "São bancos diferentes. O emissor sabe que pediram.
O registrador sabe que registrou. O id de um é a chave do outro."

## 3:00 — A fila (ADR-002)

"Não chamei o registrador por HTTP. Se ele estiver lento, o cliente não espera,
e a mensagem não se perde: a garantia é pelo menos uma vez.
Por isso a mesma mensagem pode chegar duas vezes. O INSERT tem
`ON CONFLICT (id) DO NOTHING`. A segunda vez só dá ack e escreve no log
que já existia."

No painel http://localhost:15672, publique o JSON do README duas vezes
na routing key `boleto.emitido`. Mostre uma linha só.

"JSON quebrado não volta para a fila. Vai para a DLQ, senão fica girando."

## 5:30 — A outbox (ADR-004)

"Publicar dentro do POST era frágil: o boleto gravava e a fila recusava.
Agora o boleto e a mensagem entram na mesma transação.
O POST responde 201 mesmo com o RabbitMQ parado.
Um relay publica, espera o confirm, e só então marca a outbox como enviada.
Se cair, tenta de novo com espera de 2s, 4s, até 30s.
Se a conexão cair, o processo abre outra e declara a fila de novo."

Se sobrar tempo: `docker compose stop rabbitmq`, faça um POST, suba o broker
de novo e mostre `enviado_em` preenchido.

## 7:30 — O cache (ADR-003)

"O GET do emissor é cache-aside. A chave é `boleto:{id}`, o TTL é 5 minutos.
Não guardo o 404. Quando o registrador avisa `boleto.registrado`,
eu mudo o status e apago a chave. A leitura seguinte vê `REGISTRADO`."

```bash
curl -sD - http://localhost:8080/boletos/ID | grep -i x-cache
curl -sD - http://localhost:8080/boletos/ID | grep -i x-cache
```

"A primeira é miss, a segunda é hit. Se o Redis cair, o header vai bypass
e a resposta sai do Postgres. O GET do registrador não tem cache:
ele só mostra a cópia que não muda."

## 9:00 — Limites e IA

"Três limites que eu sei de cor.
A outbox pode publicar duas vezes se o processo morrer entre o confirm e o update;
o registrador segura isso.
Uma leitura pode regravar `PENDENTE` no cache logo depois do apagar; o TTL corta isso.
O aviso de registrado não tem outbox própria: se falhar, a mensagem original volta para a fila."

"Usei IA no Cursor para escrever código e texto. Eu revisei as decisões
e consigo abrir cada arquivo e explicar."

## 9:40 — Perguntas

Deixe o painel do RabbitMQ e os dois bancos abertos.
Se pedirem o teste: `go test` na unidade, e `./scripts/e2e.sh` no fluxo inteiro.
