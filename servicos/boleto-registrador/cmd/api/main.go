package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/banco"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/config"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/fila"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/httpapi"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/registro"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	cfg, err := config.Carregar()
	if err != nil {
		slog.Error("configuração", "erro", err)
		os.Exit(1)
	}

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	pool, err := abrirBanco(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("banco", "erro", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := registro.NovoRepositorio(pool)
	go fila.Rodar(ctx, cfg.RabbitMQURL, repo)

	handler := httpapi.NovoHandler(pool.Ping)
	handler.Consulta = repo

	srv := &http.Server{
		Addr:              ":" + cfg.Porta,
		Handler:           handler.Rotas(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		prazo, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelar()
		if err := srv.Shutdown(prazo); err != nil {
			slog.Error("desligar http", "erro", err)
		}
	}()

	slog.Info("boleto-registrador escutando", "porta", cfg.Porta, "fila", fila.Fila)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http", "erro", err)
		os.Exit(1)
	}
}

func abrirBanco(ctx context.Context, url string) (*pgxpool.Pool, error) {
	var ultimo error
	for tentativa := 1; tentativa <= 15; tentativa++ {
		prazo, cancelar := context.WithTimeout(ctx, 5*time.Second)
		pool, err := banco.Conectar(prazo, url)
		if err == nil {
			err = banco.Migrar(prazo, pool)
		}
		cancelar()
		if err == nil {
			return pool, nil
		}
		if pool != nil {
			pool.Close()
		}
		ultimo = err
		slog.Warn("banco indisponível", "tentativa", tentativa, "erro", ultimo)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, ultimo
}
