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

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/banco"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/boleto"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/cache"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/config"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/fila"
	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/httpapi"
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

	repo := boleto.NovoRepositorio(pool)
	var memoria *cache.Redis
	if cfg.RedisURL != "" {
		criado, errRedis := cache.Novo(cfg.RedisURL, cfg.CacheTTL)
		if errRedis != nil {
			slog.Error("redis", "erro", errRedis)
			os.Exit(1)
		}
		memoria = criado
	}
	deps := fila.Dependencias{Caixa: repo, Atual: repo}
	if memoria != nil {
		deps.Cache = memoria
	}
	// O relay publica a outbox e aplica boleto.registrado. O POST não espera o broker.
	go fila.Rodar(ctx, cfg.RabbitMQURL, deps)

	servico := boleto.NovoServico(repo)
	handler := httpapi.NovoHandler(servico, pool.Ping)
	handler.Consulta = repo
	if memoria != nil {
		handler.Cache = memoria
	}

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

	slog.Info("boleto-emissor escutando", "porta", cfg.Porta)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http", "erro", err)
		os.Exit(1)
	}
}

func abrirBanco(ctx context.Context, url string) (*pgxpool.Pool, error) {
	prazo, cancelar := context.WithTimeout(ctx, 10*time.Second)
	defer cancelar()

	pool, err := banco.Conectar(prazo, url)
	if err != nil {
		return nil, err
	}
	if err := banco.Migrar(prazo, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
