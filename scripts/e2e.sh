#!/usr/bin/env bash
# Sobe o fluxo inteiro contra o compose já no ar.
# Uso, na raiz, depois de `docker compose up --build -d`: ./scripts/e2e.sh
set -euo pipefail

raiz="$(cd "$(dirname "$0")/.." && pwd)"
cd "$raiz"

emissor="http://localhost:8080"
registrador="http://localhost:8081"

esperar_saude() {
	local url="$1"
	local i
	for i in $(seq 1 60); do
		if curl -sf "$url" >/dev/null; then
			return 0
		fi
		sleep 2
	done
	echo "sem resposta em $url" >&2
	exit 1
}

esperar_saude "$emissor/health"
esperar_saude "$registrador/health"

vencimento="$(date -d '+40 days' +%F)"
resposta="$(curl -sf -X POST "$emissor/boletos" \
	-H 'Content-Type: application/json' \
	-d "{\"valor\":\"150.50\",\"vencimento\":\"$vencimento\",\"pagador\":\"Maria Silva\"}")"
echo "POST $resposta"
id="$(printf '%s' "$resposta" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"

echo "esperando o registrador"
ok=""
for _ in $(seq 1 40); do
	codigo="$(curl -s -o /tmp/boleto-reg.json -w '%{http_code}' "$registrador/boletos/$id")"
	if [ "$codigo" = "200" ]; then
		ok=1
		break
	fi
	sleep 0.5
done
if [ -z "$ok" ]; then
	echo "registrador não recebeu $id" >&2
	cat /tmp/boleto-reg.json >&2 || true
	exit 1
fi
echo "registrador $(cat /tmp/boleto-reg.json)"

echo "esperando REGISTRADO e um miss seguido de hit"
visto=""
for _ in $(seq 1 40); do
	cabecalho="$(curl -s -D - -o /tmp/boleto-get.json "$emissor/boletos/$id" | tr -d '\r')"
	cache="$(printf '%s\n' "$cabecalho" | awk 'tolower($1)=="x-cache:" {print $2}')"
	if grep -q REGISTRADO /tmp/boleto-get.json && [ "$cache" = "miss" ]; then
		visto=1
		break
	fi
	sleep 0.4
done
if [ -z "$visto" ]; then
	echo "não vi miss de um boleto REGISTRADO" >&2
	echo "ultimo cache=$cache" >&2
	cat /tmp/boleto-get.json >&2
	exit 1
fi

segunda="$(curl -s -D - -o /tmp/boleto-get2.json "$emissor/boletos/$id" | tr -d '\r' | awk 'tolower($1)=="x-cache:" {print $2}')"
echo "cache miss depois $segunda"
if [ "$segunda" != "hit" ]; then
	echo "a segunda leitura não veio do cache" >&2
	exit 1
fi

payload="$(ID="$id" VENCIMENTO="$vencimento" python3 - <<'PY'
import json, os
print(json.dumps({
    "properties": {"delivery_mode": 2, "content_type": "application/json"},
    "routing_key": "boleto.emitido",
    "payload_encoding": "string",
    "payload": json.dumps({
        "id": os.environ["ID"],
        "valor_centavos": 15050,
        "vencimento": os.environ["VENCIMENTO"],
        "pagador": "Maria Silva",
        "emitido_em": "2026-11-14T18:00:00Z",
    }),
}))
PY
)"
curl -sf -u guest:guest -H 'content-type: application/json' \
	-X POST 'http://localhost:15672/api/exchanges/%2F/boletos/publish' \
	-d "$payload" >/dev/null
sleep 2

linhas="$(docker compose exec -T postgres-registrador \
	psql -U registrador -d registrador -tAc "select count(*) from boletos_registrados where id = '$id'")"
linhas="$(printf '%s' "$linhas" | tr -d '[:space:]')"
echo "linhas=$linhas"
if [ "$linhas" != "1" ]; then
	echo "a mensagem repetida criou outra linha" >&2
	exit 1
fi

echo "e2e ok id=$id"
