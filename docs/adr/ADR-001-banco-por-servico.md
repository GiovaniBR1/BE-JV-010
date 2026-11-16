# ADR-001: Dois serviços, cada um com o seu banco

## Status

Aceito.

## Contexto

A Palavra Viva emite a mensalidade da Maria Silva, R$ 150,50 com vencimento em 31 de dezembro de 2026, em dois momentos diferentes. A secretaria pede a emissão. Depois, essa cobrança precisa ser registrada no banco. O critério do módulo pede dois serviços, cada um com o seu banco, no mesmo espírito do emissor e do gravador do exemplo de comprovantes PIX. Se os dois momentos morassem no mesmo processo, um atraso no registro travaria o atendimento.

## Decisão

São dois serviços, e cada um guarda a sua cópia. O `boleto-emissor` recebe `POST /boletos`, valida e grava o boleto da Maria como `PENDENTE` no Postgres dele. O `boleto-registrador` faz o papel do banco, mas só simula esse registro: não há convênio, CIP nem chamada externa. Ele grava a cópia no Postgres dele e, mais tarde, devolve o aviso `boleto.registrado`. Não há tabela compartilhada nem biblioteca de domínio compartilhada. O id gerado pelo emissor é a chave primária do registrador, então as duas linhas falam da mesma cobrança de 15050 centavos, cada uma no seu caderno.

Como os dois conversam fica no ADR-002. O cache fica no ADR-003. A outbox fica no ADR-004.

## Alternativas consideradas

Um serviço só, com um banco só, teria menos peças para subir, e o registro aconteceria no mesmo processo que atende a secretaria. Essa alternativa fica de fora porque o critério pede dois serviços e dois bancos, e porque o atraso do "banco" voltaria a segurar o pedido da Maria.

Dois serviços apontando para o mesmo Postgres separariam a implantação, mas o schema continuaria único. Uma migração de um lado quebraria o outro, então a separação seria só de processo, não de dado.

Dois serviços e dois bancos, com o registrador perguntando o dado ao emissor por HTTP na hora de gravar, ainda deixariam a cópia do registrador dependente do emissor estar no ar. A escolha foi cada um guardar o que precisa, ligados pelo id, e deixar a comparação entre fila e HTTP para o ADR-002.

## Consequências

O mesmo boleto existe nos dois bancos, com donos diferentes. O emissor sabe que pediram a emissão. O registrador sabe que simulou o registro. Os dois podem divergir por um tempo, até a mensagem ser consumida. Subir o projeto pede dois Postgres. No perfil A isso fica no `docker compose`. O preço aceito é que o "banco" desta banca não é um banco de verdade: a simulação basta para mostrar a fila, a idempotência e a volta do status.
