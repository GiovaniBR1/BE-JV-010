package boleto

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestPublicacaoBateComContratoEmitido(t *testing.T) {
	fixture := lerContrato(t, "boleto.emitido.json")
	tipos := lerTipos(t, "boleto.emitido.tipos.json")
	exigirTipos(t, fixture, tipos)

	vencimento, err := time.Parse("2006-01-02", fixture["vencimento"].(string))
	if err != nil {
		t.Fatal(err)
	}
	emitidoEm, err := time.Parse(time.RFC3339, fixture["emitido_em"].(string))
	if err != nil {
		t.Fatal(err)
	}
	corpo, err := Codificar(Boleto{
		ID:            fixture["id"].(string),
		ValorCentavos: int64(fixture["valor_centavos"].(float64)),
		Vencimento:    vencimento,
		Pagador:       fixture["pagador"].(string),
		CriadoEm:      emitidoEm,
	})
	if err != nil {
		t.Fatal(err)
	}
	var publicado map[string]any
	if err := json.Unmarshal(corpo, &publicado); err != nil {
		t.Fatal(err)
	}
	exigirTipos(t, publicado, tipos)
	if !reflect.DeepEqual(publicado, fixture) {
		t.Fatalf("publicado = %#v\ncontrato = %#v", publicado, fixture)
	}
}

func lerContrato(t *testing.T, nome string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(lerArquivoContrato(t, nome), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func lerTipos(t *testing.T, nome string) map[string]string {
	t.Helper()
	var tipos map[string]string
	if err := json.Unmarshal(lerArquivoContrato(t, nome), &tipos); err != nil {
		t.Fatal(err)
	}
	return tipos
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

func exigirTipos(t *testing.T, doc map[string]any, tipos map[string]string) {
	t.Helper()
	if len(doc) != len(tipos) {
		t.Fatalf("campos %#v, contrato %#v", doc, tipos)
	}
	for campo, tipo := range tipos {
		valor, ok := doc[campo]
		if !ok {
			t.Fatalf("falta o campo %s", campo)
		}
		if got := tipoDoValor(valor); got != tipo {
			t.Fatalf("%s é %s, o contrato pede %s", campo, got, tipo)
		}
	}
}

func tipoDoValor(valor any) string {
	switch valor.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	default:
		return "outro"
	}
}
