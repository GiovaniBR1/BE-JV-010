//go:build integration

package integra

import (
	"context"
	"os"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/banco"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/fila"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/registro"
)

func TestMensagemRepetidaGravaUmaLinha(t *testing.T) {
	_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("registrador"),
		postgres.WithUsername("registrador"),
		postgres.WithPassword("registrador"),
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

	urlPG, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	urlRabbit, err := rabbit.AmqpURL(ctx)
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
	repo := registro.NovoRepositorio(pool)

	trab, parar := context.WithCancel(context.Background())
	t.Cleanup(func() {
		parar()
		time.Sleep(300 * time.Millisecond)
		pool.Close()
	})
	go fila.Rodar(trab, urlRabbit, repo)

	const id = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
	corpo := []byte(`{"id":"` + id + `","valor_centavos":15050,"vencimento":"2027-01-15","pagador":"Maria Silva","emitido_em":"2026-11-14T18:00:00Z"}`)
	// Republica até a fila existir. Mensagem que chega antes do declare se perde.
	esperar(t, 30*time.Second, func() bool {
		publicar(t, urlRabbit, corpo)
		_, ok, err := repo.BuscarPorID(ctx, id)
		return err == nil && ok
	})

	publicar(t, urlRabbit, corpo)
	time.Sleep(2 * time.Second)

	var linhas int
	if err := pool.QueryRow(ctx, `select count(*) from boletos_registrados where id = $1`, id).Scan(&linhas); err != nil {
		t.Fatal(err)
	}
	if linhas != 1 {
		t.Fatalf("linhas = %d", linhas)
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

func publicar(t *testing.T, url string, corpo []byte) {
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
	err = ch.PublishWithContext(context.Background(), fila.Exchange, fila.RoutingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         corpo,
	})
	if err != nil {
		t.Fatal(err)
	}
}
