# Roteiro da banca (10 minutos)

O relógio já inclui as perguntas. O projeto está no ar
(`docker compose up --build -d`). Fale curto e aponte a tela.

## 0:00 — O que é (40s)

"Central de boletos, perfil A, tudo no Docker. O cliente pede o boleto
num serviço. Outro registra. Cada um tem o seu Postgres. No meio tem
fila. A leitura quente fica no Redis."

Aponte o desenho do README.

## 0:40 — Dois bancos, ADR-001 (1min20)

Faça o POST do README. "150,50 vira 15050, sem float. Vencimento de
ontem volta 400. Hoje é o dia de São Paulo."

Mostre os dois `select`. "Bancos diferentes. O id do emissor é a chave
do registrador."

## 2:00 — Fila, ADR-002 (1min30)

"Não é HTTP: se o registrador estiver lento, o cliente não espera, e a
mensagem não se perde. A garantia é pelo menos uma vez, então ela pode
chegar repetida. O INSERT usa `ON CONFLICT (id) DO NOTHING`."

No painel http://localhost:15672, publique o JSON de
`contracts/boleto.emitido.json` duas vezes, routing key `boleto.emitido`.
Mostre uma linha só.

## 3:30 — Outbox, ADR-004 (1min10)

"O boleto e o JSON entram na mesma transação. O POST responde 201
mesmo com o RabbitMQ parado. Um relay publica, espera o confirm e só
então marca a outbox. Se falhar, tenta de novo com espera."

O enunciado pede três ADRs. Este é o quarto: a outbox é outra decisão,
não um detalhe da fila.

## 4:40 — Cache, ADR-003 (1min10)

"O GET do emissor guarda `boleto:{id}` por 5 minutos. Não guarda 404.
Quando chega `boleto.registrado`, apago a chave."

Duas leituras do mesmo id: `X-Cache: miss`, depois `hit`. "Se o Redis
cair, o header vai `bypass` e a resposta sai do Postgres."

## 5:50 — Contrato (40s)

"Os dois serviços não compartilham código de mensagem. O JSON fica em
`contracts/`. O teste do emissor prova o que ele publica. O do
registrador prova o que ele consome. Se um lado mudar campo ou tipo,
o teste quebra. Roda no `go test` normal, sem Docker."

## 6:30 — Limites e IA (40s)

"Três limites: a outbox pode publicar duas vezes se o processo morrer
no meio, e o registrador segura; o cache pode regravar `PENDENTE` por
uma corrida, e o TTL corta; o aviso de registrado não tem outbox, a
mensagem original volta para a fila."

"Usei IA no Cursor. Eu revisei e explico cada arquivo."

## 7:10 — Perguntas (até 10:00)

Deixe o painel e os dois bancos abertos.

### Por que fila, e não HTTP?

O cliente não espera o registro no banco. Se o registrador cair, a
mensagem fica na fila e é entregue depois. HTTP direto falharia o
pedido junto. A entrega é pelo menos uma vez: pode repetir, não se perde.

### A mensagem repetida não duplica?

O id do emissor é a chave primária do registrador. O INSERT usa
`ON CONFLICT (id) DO NOTHING`. A segunda vez acha a linha, loga que já
existia e dá ack mesmo assim.

### Por que a outbox?

Publicar depois do INSERT perdia a mensagem se o RabbitMQ recusasse,
e o cliente recebia erro. Agora os dois inserts são a mesma transação.
O POST responde 201, e o relay publica quando o broker confirma.

### O que o cache guarda, por quanto tempo, e quando apaga?

Guarda o JSON do `GET /boletos/{id}` do emissor, na chave `boleto:{id}`,
por 5 minutos (`CACHE_TTL`). Não guarda 404. Apaga a chave quando o
status vira `REGISTRADO`.

### Por que quatro ADRs, se o enunciado pede três?

Os três pedidos são serviço/banco, fila e cache. A outbox é uma quarta
decisão, com outras alternativas. Ela não substitui o ADR da fila.

### E se o Redis cair?

O GET segue no Postgres e responde 200. O header `X-Cache` vai
`bypass`. O cache não é caminho obrigatório da leitura.

### Por que Go, e não Spring?

O desafio não prende a linguagem. Go 1.22 já roteia HTTP na biblioteca
padrão, e cada serviço vira um binário no Docker. A fila e o cache
entram por interfaces pequenas.

### Você usou IA?

Sim, no Cursor, para código e texto. Eu li os ADRs, rodei os testes e
o `./scripts/e2e.sh`, e consigo explicar cada escolha.
