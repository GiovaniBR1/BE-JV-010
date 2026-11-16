# A história da central de boletos

Este texto abre a banca e ajuda a revisar o projeto. O código vem depois. Os nomes da escola e da aluna são inventados, mas o valor, o vencimento e o caminho do boleto são os mesmos do README e dos arquivos em `contracts/`.

## 1. Contexto

A Palavra Viva é uma escola pequena de idiomas. Todo mês ela emite um boleto para cada aluno, com o valor da mensalidade e a data de vencimento. A secretaria não imprime o boleto na hora e não liga para o banco no meio do atendimento: ela pede a emissão pelo sistema e segue o dia.

Neste mês, a aluna Maria Silva deve R$ 150,50, com vencimento em 31 de dezembro de 2026. É esse boleto que a gente acompanha daqui pra frente, o mesmo exemplo que o projeto usa no README e no contrato das mensagens.

Um boleto, neste projeto, é o pedido de cobrança: valor, vencimento e o nome de quem vai pagar. Ainda não é o registro no banco. Registrar é o passo seguinte, em que o "banco" (aqui, o serviço `boleto-registrador`) aceita aquela cobrança e devolve a confirmação.

## 2. O problema

Se a secretaria dependesse do banco estar rápido na mesma hora do pedido, um dia ruim no registrador travaria a recepção inteira. A Maria já teria sido atendida, o valor já estaria certo, e mesmo assim o pedido falharia porque o outro lado estava lento ou fora do ar.

Há outro risco clássico de mensagem. O correio entre os serviços pode entregar o mesmo aviso duas vezes: a conexão caiu no meio, ou alguém republicou no painel. Sem uma trava, o boleto da Maria apareceria registrado em dobro, como se ela devesse duas mensalidades de R$ 150,50.

Dinheiro guardado em número de ponto flutuante, o chamado float, também apronta. Contas como 150,50, somadas muitas vezes, podem sair com um centavo a mais ou a menos. Num boleto, um centavo errado já é cobrança errada.

A consulta do boleto é o caminho mais apertado do dia. A secretaria e a própria Maria perguntam várias vezes se já registrou. Se cada pergunta for direto no banco de dados, a mesma linha é lida sem necessidade, e o banco sofre com o volume de GET repetido.

Por fim, os dois serviços não compartilham o código da mensagem. Se um lado renomear um campo ou trocar um número por texto, o outro quebra em silêncio, no meio do expediente, e o boleto da Maria fica parado no caminho.

## 3. A solução

A resposta não é um bloco só. Cada pedaço do desenho existe para um desses problemas, e cada escolha está escrita num ADR, que é um registro curto de decisão: o que foi escolhido, o que mais foi considerado e por quê.

**Dois serviços, cada um com o seu banco (ADR-001).** O `boleto-emissor` recebe o pedido da secretaria, valida e grava o boleto como `PENDENTE` no Postgres dele. O `boleto-registrador` faz o papel do banco: grava a cópia registrada no Postgres dele. Não há tabela compartilhada. O identificador criado pelo emissor é a chave primária do registrador, então as duas linhas falam do mesmo boleto da Maria, cada uma no seu caderno.

**Fila no RabbitMQ, e não uma chamada HTTP (ADR-002).** HTTP seria como a secretaria ficar no telefone até o banco atender. A fila é o correio: o emissor deixa o aviso `BoletoEmitido` e segue o atendimento. Se o registrador estiver lento ou fora, a mensagem espera na fila `boleto.emitido` e é entregue depois. O cliente não precisa esperar o segundo passo.

**Outbox e relay (ADR-004).** Publicar a mensagem depois de gravar o boleto era frágil: se o RabbitMQ recusasse, o boleto ficava `PENDENTE`, o cliente recebia erro e o aviso não saía. Agora, no mesmo instante em que o boleto é gravado, a "carta" já pronta entra numa tabela chamada `outbox`, na mesma transação. Transação, aqui, quer dizer: ou os dois entram, ou nenhum entra. O `POST /boletos` responde 201 mesmo com o RabbitMQ parado. Um relay, que é um carteiro rodando em segundo plano dentro do próprio emissor, lê as cartas ainda não enviadas, publica, espera o broker confirmar e só então marca `enviado_em`. Se a publicação falha, a carta fica e a próxima tentativa espera um pouco mais (2 segundos, 4 segundos, até no máximo 30).

**Pelo menos uma vez, e idempotência com `ON CONFLICT` (ADR-002).** A fila garante que a mensagem não se perde enquanto não for confirmada, mas ela pode chegar repetida. Isso se chama entrega "pelo menos uma vez". O registrador não se assusta: o `INSERT` usa `ON CONFLICT (id) DO NOTHING`. Se o id da Maria já existe, nada muda, o serviço anota no log que a mensagem era repetida e confirma o recebimento mesmo assim.

**DLQ, a fila de mensagens mortas.** Um JSON quebrado, que não dá para ler, não pode girar para sempre na fila boa. Ele vai para a DLQ (`boletos.dlx` / `boleto.emitido.dlq`), uma fila à parte, com um recado de recusa e sem recolocar a mensagem no lugar de origem. Se o banco do registrador falha, o serviço tenta de novo até três vezes. Esgotado o limite, essa mensagem também cai na DLQ, para não travar as outras.

**Dinheiro em centavos, como inteiro.** R$ 150,50 vira `15050`. O projeto não usa float: o texto `"150.50"` é lido casa por casa e vira inteiro. É a mesma unidade do banco e da mensagem. Um centavo a mais ou a menos não nasce de arredondamento escondido.

**Cache no Redis (ADR-003).** O `GET /boletos/{id}` do emissor usa cache-aside. Cache-aside quer dizer: primeiro pergunta no Redis; se não achar, busca no Postgres e guarda uma cópia. A chave é `boleto:{id}`, o valor é o JSON da resposta, e a cópia vale 5 minutos (o `CACHE_TTL`). Um boleto que ainda não existe (resposta 404) não é guardado, para a próxima leitura poder achar. Quando chega o aviso `boleto.registrado`, o emissor muda o status e apaga a chave. A leitura seguinte monta o JSON de novo, já com `REGISTRADO`. Se o Redis cair, a leitura segue no Postgres e o cabeçalho `X-Cache` vai `bypass`: o GET não falha por causa do cache. O GET do registrador não usa cache; ele mostra a cópia daquele serviço, escrita uma vez.

**Testes de contrato em `contracts/`.** Os dois serviços não compartilham o código da mensagem. O acordo entre eles é o JSON. Em `contracts/` ficam os exemplos `boleto.emitido` e `boleto.registrado`, e ao lado o tipo de cada campo (texto ou número). O teste do emissor monta a mensagem e exige o mesmo JSON do arquivo. O teste do registrador lê o arquivo e exige os mesmos valores. O outro par faz o inverso com `boleto.registrado`. Se um lado renomear um campo ou trocar número por texto, o teste quebra antes de chegar na secretaria.

## 4. Um boleto do começo ao fim

A secretaria da Palavra Viva pede o boleto da Maria: valor 150,50, vencimento 2026-12-31, pagador Maria Silva. O emissor valida, cria um identificador e grava a linha como `PENDENTE` no Postgres dele. Na mesma transação, o JSON do aviso já entra na outbox. A secretaria recebe a resposta 201 e pode seguir o atendimento, mesmo que o RabbitMQ esteja fora naquele instante.

Pouco depois, o relay lê a carta, publica `BoletoEmitido` no exchange `boletos`, com a routing key `boleto.emitido`, e só marca a outbox quando o broker confirma. O registrador consome o aviso, simula o registro, grava a cópia dele uma vez só e devolve `boleto.registrado`, com o mesmo id e o instante `registrado_em`.

O emissor recebe esse aviso, muda o status do boleto da Maria para `REGISTRADO` e apaga a cópia no Redis. Se a secretaria consultar de novo, a primeira leitura depois dessa mudança vai ao banco, já com o status novo, e a seguinte pode sair do cache. Do pedido até a confirmação, o boleto anda de `PENDENTE` até `REGISTRADO`, e os dois cadernos ficam falando da mesma cobrança de R$ 150,50.

## 5. Como contar na banca

Fale mais ou menos assim, em cerca de um minuto, apontando o desenho do README:

"Vou contar pelo boleto de uma aluna. A Palavra Viva é uma escola pequena. A Maria Silva deve R$ 150,50, vencendo em 31 de dezembro de 2026. A secretaria pede a emissão e não pode ficar parada esperando o banco.

Por isso são dois serviços, cada um com o seu Postgres. Isso está no ADR-001. No meio tem uma fila no RabbitMQ, e não uma chamada HTTP: se o registrador estiver lento, a mensagem espera, e o pedido da Maria já foi aceito. Isso é o ADR-002.

A carta não sai no escuro. O boleto e o aviso entram na mesma transação, numa outbox, e um relay publica quando o broker confirma. O POST responde 201 mesmo com a fila parada. Isso é o ADR-004.

A fila entrega pelo menos uma vez, então o aviso pode repetir. O registrador grava com ON CONFLICT no id, e a Maria não aparece duas vezes. Valor vai em centavos: 150,50 vira 15050, sem número quebrado. A leitura quente fica no Redis, ADR-003, e apaga a cópia quando o status vira REGISTRADO. O formato das mensagens está em contracts, e o teste dos dois lados quebra se um campo mudar.

Agora eu mostro esse caminho rodando."

## 6. O que fica em aberto

O projeto cobre o recado do módulo, e ainda assim tem limites honestos. Vale conhecê-los antes da banca, porque uma pergunta boa quase sempre cai em cima deles.

**O registrador só simula o banco.** Ele não fala com um banco de verdade, uma CIP ou um convênio. Ele recebe o aviso, grava a própria cópia como `REGISTRADO` e devolve a confirmação. Isso basta para ensinar a fila e a idempotência. Num sistema real, esse passo seria uma integração externa, com prazo, recusa e um estado de erro, não só o caminho feliz.

**Não há autenticação.** Qualquer um que alcance a porta pode pedir um boleto ou consultar um id. No recorte da banca isso simplifica a demonstração. Em produção, o `POST` e o `GET` precisariam de identidade e permissão, para a mensalidade da Maria não ser emitida ou lida por quem não deve.

**A DLQ guarda, mas ninguém reprocessa.** Mensagem inválida ou que esgotou as três tentativas sai da fila boa e cai em `boleto.emitido.dlq` (e há a irmã `boleto.registrado.dlq`). Isso evita o giro infinito. Não existe, neste repositório, um consumidor que releia a DLQ, corrija e devolva o aviso para a fila principal. Na prática, alguém olharia o painel, entenderia o erro e faria o reprocessamento com cuidado, para o boleto da Maria não ficar esquecido.

**Não há métricas de produto.** Os serviços registram logs e respondem em `/health`, mas não expõem contadores de publicação, de mensagem repetida, de queda no Redis ou de tamanho da outbox. Sem isso, um atraso no registro da Maria só aparece quando alguém pergunta. Um próximo passo natural seria contar esses eventos e alertar quando a outbox ou a DLQ crescer.

**O aviso `boleto.registrado` não tem outbox.** Só a ida (`BoletoEmitido`) entra na mesma transação do boleto. Se a volta falhar, o registrador não confirma a mensagem original, ela volta para a fila, e na próxima entrega o id já existe, então o aviso é publicado de novo. Funciona, mas o caminho de volta é mais frágil do que o de ida. A correção simétrica seria uma outbox também no registrador.

### Perguntas que a banca pode fazer

**E se o relay morrer no meio da publicação?** Se o processo cair depois que o RabbitMQ confirmou e antes de marcar `enviado_em`, a mesma carta pode sair de novo. Isso importa porque a garantia da fila é "pelo menos uma vez", não "exatamente uma vez". O registrador segura a ponta: o id é chave primária e o `ON CONFLICT` impede a segunda linha. Se o tema aparecer, é honesto dizer que a duplicata pode existir na fila, e que a idempotência é o que protege o cadastro da Maria.

**O cache pode mostrar `PENDENTE` depois do registro?** Pode, por uma janela curta. Uma leitura que já pegou `PENDENTE` no banco pode gravar essa cópia no Redis logo depois do apagar, se os dois se cruzam. Não há uma trava em volta disso. O TTL de 5 minutos limita o tempo dessa cópia velha. Uma evolução seria só gravar o cache com um número de versão, ou nem cachear enquanto o status ainda pode mudar tão rápido.

**As mensagens têm ordem garantida?** O projeto não promete uma ordem rígida entre boletos diferentes. A fila é compartilhada, o registrador pode ter até 10 mensagens reservadas por vez, e o que importa para cada id é a idempotência, não a posição na fila. Para a Maria, o que precisa acontecer em sequência é a ida e a volta do mesmo boleto, e isso já está amarrado pelo id. Se a ordem entre cobranças passasse a importar, aí sim caberia uma chave de partição ou uma fila por destino.

**E se o emissor subir em mais de um exemplar?** O relay é um loop dentro do mesmo binário do emissor, não um processo separado. A consulta da outbox não reserva a linha: não há `FOR UPDATE SKIP LOCKED`. Dois relays poderiam pegar a mesma carta e publicá-la duas vezes. De novo, o registrador não duplicaria o registro. Para crescer de verdade, a leitura da outbox precisaria travar ou pular as linhas já em uso, para cada carta ter um carteiro só.

**E se os dois serviços declararem a fila de forma diferente?** Os dois declaram a mesma topologia. O declare só é seguro quando os argumentos coincidem. Se um lado mudar sozinho o arranjo da DLQ, o outro nem sobe. Isso aparece no ADR-002 e no README. A defesa curta é: a topologia é parte do contrato, tanto quanto o JSON em `contracts/`.
