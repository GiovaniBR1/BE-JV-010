//go:build integration

package integra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/banco"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/boleto"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/cache"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/fila"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/httpapi"
)

func TestOutboxRegistroECache(t *testing.T) {
	_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("emissor"),
		postgres.WithUsername("emissor"),
		postgres.WithPassword("emissor"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	rabbit, err := rabbitmq.Run(ctx, "rabbitmq:3-management-alpine")
	if err != nil {
		t.Fatalf("rabbitmq: %v", err)
	}
	t.Cleanup(func() { _ = rabbit.Terminate(context.Background()) })

	rd, err := redis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	t.Cleanup(func() { _ = rd.Terminate(context.Background()) })

	urlPG, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	urlRabbit, err := rabbit.AmqpURL(ctx)
	if err != nil {
		t.Fatal(err)
	}
	urlRedis, err := rd.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	pool, err := banco.Conectar(ctx, urlPG)
	if err != nil {
		t.Fatal(err)
	}
	if err := banco.Migrar(ctx, pool); err != nil {
		t.Fatal(err)
	}
	repo := boleto.NovoRepositorio(pool)
	memoria, err := cache.Novo(urlRedis, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	trab, parar := context.WithCancel(context.Background())
	t.Cleanup(func() {
		parar()
		time.Sleep(300 * time.Millisecond)
		pool.Close()
	})
	go fila.Rodar(trab, urlRabbit, fila.Dependencias{Caixa: repo, Atual: repo, Cache: memoria})

	criado, err := boleto.NovoServico(repo).Emitir(ctx, boleto.Dados{
		ValorCentavos: 15050,
		Vencimento:    time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC),
		Pagador:       "Maria Silva",
	})
	if err != nil {
		t.Fatal(err)
	}

	esperar(t, 40*time.Second, func() bool {
		var n int
		err := pool.QueryRow(ctx, `select count(*) from outbox where boleto_id = $1 and enviado_em is not null`, criado.ID).Scan(&n)
		return err == nil && n == 1
	})

	publicarRegistrado(t, urlRabbit, criado.ID)
	esperar(t, 20*time.Second, func() bool {
		b, ok, err := repo.BuscarPorID(ctx, criado.ID)
		return err == nil && ok && b.Status == boleto.StatusRegistrado
	})

	h := httpapi.NovoHandler(boleto.NovoServico(repo), func(context.Context) error { return nil })
	h.Consulta = repo
	h.Cache = memoria
	primeira := buscar(t, h, criado.ID)
	segunda := buscar(t, h, criado.ID)
	if primeira.Header().Get("X-Cache") != "miss" || segunda.Header().Get("X-Cache") != "hit" {
		t.Fatalf("cache = %s depois %s", primeira.Header().Get("X-Cache"), segunda.Header().Get("X-Cache"))
	}
}

func esperar(t *testing.T, prazo time.Duration, ok func() bool) {
	t.Helper()
	limite := time.Now().Add(prazo)
	for time.Now().Before(limite) {
		if ok() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("prazo esgotado")
}

func buscar(t *testing.T, h *httpapi.Handler, id string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/boletos/"+id, nil)
	rec := httptest.NewRecorder()
	h.Rotas().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d corpo = %s", rec.Code, rec.Body.String())
	}
	return rec
}

func publicarRegistrado(t *testing.T, url, id string) {
	t.Helper()
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	corpo := []byte(`{"id":"` + id + `","registrado_em":"2026-11-14T18:00:00Z"}`)
	err = ch.PublishWithContext(context.Background(), fila.Exchange, fila.RoutingKeyRegistrado, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         corpo,
	})
	if err != nil {
		t.Fatal(err)
	}
}
