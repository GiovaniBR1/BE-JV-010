package fila

import (
	"context"
	"errors"
	"testing"
)

type atualFake struct {
	id    string
	achou bool
	err   error
}

func (a *atualFake) MarcarRegistrado(_ context.Context, id string) (bool, error) {
	a.id = id
	return a.achou, a.err
}

type cacheFake struct {
	apagados []string
	err      error
}

func (c *cacheFake) Apagar(_ context.Context, id string) error {
	c.apagados = append(c.apagados, id)
	return c.err
}

func TestAplicarRegistradoAtualizaEApagaCache(t *testing.T) {
	atual := &atualFake{achou: true}
	cache := &cacheFake{}
	err := AplicarRegistrado(context.Background(), atual, cache, []byte(`{"id":"7c9e6679-7425-40de-944b-e07fc1f90ae7","registrado_em":"2026-11-14T18:00:00Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	if atual.id != "7c9e6679-7425-40de-944b-e07fc1f90ae7" {
		t.Fatalf("id = %s", atual.id)
	}
	if len(cache.apagados) != 1 {
		t.Fatalf("apagados = %v", cache.apagados)
	}
}

func TestAplicarRegistradoRejeitaJSON(t *testing.T) {
	atual := &atualFake{}
	err := AplicarRegistrado(context.Background(), atual, nil, []byte(`{}`))
	if !errors.Is(err, ErrRegistradoInvalido) {
		t.Fatal(err)
	}
	if atual.id != "" {
		t.Fatal("não devia atualizar")
	}
}

func TestAplicarRegistradoNaoApagaCacheSeOBancoFalhar(t *testing.T) {
	atual := &atualFake{err: errors.New("banco fora")}
	cache := &cacheFake{}
	err := AplicarRegistrado(context.Background(), atual, cache, []byte(`{"id":"7c9e6679-7425-40de-944b-e07fc1f90ae7","registrado_em":"2026-11-14T18:00:00Z"}`))
	if err == nil {
		t.Fatal("esperava erro")
	}
	if len(cache.apagados) != 0 {
		t.Fatal("apagou o cache sem gravar o status")
	}
}
