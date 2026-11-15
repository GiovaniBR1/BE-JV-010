package fila

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConsumoBateComContratoRegistrado(t *testing.T) {
	corpo := lerArquivoContrato(t, "boleto.registrado.json")
	var fixture map[string]any
	if err := json.Unmarshal(corpo, &fixture); err != nil {
		t.Fatal(err)
	}
	var tipos map[string]string
	if err := json.Unmarshal(lerArquivoContrato(t, "boleto.registrado.tipos.json"), &tipos); err != nil {
		t.Fatal(err)
	}
	if len(fixture) != len(tipos) {
		t.Fatalf("campos %#v, contrato %#v", fixture, tipos)
	}

	atual := &atualFake{achou: true}
	err := AplicarRegistrado(context.Background(), atual, &cacheFake{}, corpo)
	if err != nil {
		t.Fatal(err)
	}
	if atual.id != fixture["id"].(string) {
		t.Fatalf("id aplicado = %s, contrato = %s", atual.id, fixture["id"])
	}
}

func lerArquivoContrato(t *testing.T, nome string) []byte {
	t.Helper()
	_, arquivo, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("não achei o arquivo de teste")
	}
	dir := filepath.Dir(arquivo)
	for {
		candidato := filepath.Join(dir, "contracts", nome)
		corpo, err := os.ReadFile(candidato)
		if err == nil {
			return corpo
		}
		pai := filepath.Dir(dir)
		if pai == dir {
			t.Fatalf("contracts/%s não encontrado", nome)
		}
		dir = pai
	}
}
