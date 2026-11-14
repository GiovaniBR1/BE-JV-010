package boleto

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	// Fuso embutido no binário: a imagem não traz America/Sao_Paulo.
	_ "time/tzdata"
)

const (
	msgValorObrigatorio      = "valor é obrigatório"
	msgValorInvalido         = "valor deve ser maior que zero e ter no máximo 2 casas decimais"
	msgVencimentoObrigatorio = "vencimento é obrigatório"
	msgVencimentoFormato     = "vencimento deve estar no formato AAAA-MM-DD"
	msgVencimentoPassado     = "vencimento não pode ser anterior a hoje"
	msgPagadorObrigatorio    = "pagador é obrigatório"
	msgPagadorLongo          = "pagador deve ter no máximo 255 caracteres"

	tamanhoMaximoPagador = 255
)

// Entrada é o JSON de POST /boletos, antes da validação.
// Valor fica cru de propósito: pode vir como texto ("150.50") ou número (150.50).
type Entrada struct {
	Valor      json.RawMessage `json:"valor"`
	Vencimento string          `json:"vencimento"`
	Pagador    string          `json:"pagador"`
}

// Dados é a entrada já aceita. Emitir confia neste valor.
type Dados struct {
	ValorCentavos int64
	Vencimento    time.Time
	Pagador       string
}

// fuso do calendário do boleto. O Brasil não usa horário de verão desde 2019,
// então America/Sao_Paulo é UTC-3 o ano inteiro.
var fusoSaoPaulo = carregarFusoSaoPaulo()

func carregarFusoSaoPaulo() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic("fuso America/Sao_Paulo: " + err.Error())
	}
	return loc
}

// Validar devolve os dados prontos para gravar, ou a lista de erros do 400.
// "Hoje" é o dia civil em São Paulo, não a data UTC do container.
func Validar(entrada Entrada, agora time.Time) (Dados, []string) {
	var dados Dados
	var erros []string

	centavos, msg := interpretarValor(entrada.Valor)
	if msg != "" {
		erros = append(erros, msg)
	} else {
		dados.ValorCentavos = centavos
	}

	vencimento, msg := interpretarVencimento(entrada.Vencimento, agora)
	if msg != "" {
		erros = append(erros, msg)
	} else {
		dados.Vencimento = vencimento
	}

	pagador, msg := interpretarPagador(entrada.Pagador)
	if msg != "" {
		erros = append(erros, msg)
	} else {
		dados.Pagador = pagador
	}

	return dados, erros
}

func interpretarValor(raw json.RawMessage) (int64, string) {
	texto := strings.TrimSpace(string(raw))
	if texto == "" || texto == "null" {
		return 0, msgValorObrigatorio
	}
	if strings.HasPrefix(texto, "\"") {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return 0, msgValorInvalido
		}
		texto = strings.TrimSpace(s)
		if texto == "" {
			return 0, msgValorObrigatorio
		}
	}
	centavos, err := centavosDeTexto(texto)
	if err != nil {
		return 0, msgValorInvalido
	}
	return centavos, ""
}

// centavosDeTexto lê um decimal com ponto e no máximo 2 casas.
// Não usa float: "150.50" vira 15050 direto pelo texto.
func centavosDeTexto(texto string) (int64, error) {
	reaisTxt := texto
	fracTxt := ""
	if ponto := strings.IndexByte(texto, '.'); ponto >= 0 {
		reaisTxt = texto[:ponto]
		fracTxt = texto[ponto+1:]
		if len(fracTxt) == 0 || len(fracTxt) > 2 {
			return 0, fmt.Errorf("casas decimais")
		}
	}
	if !soDigitos(reaisTxt) || (fracTxt != "" && !soDigitos(fracTxt)) {
		return 0, fmt.Errorf("formato")
	}
	// "0.50" pode. "01.50" não: zero à esquerda esconde typo.
	if len(reaisTxt) > 1 && reaisTxt[0] == '0' {
		return 0, fmt.Errorf("zero à esquerda")
	}
	reais, err := strconv.ParseInt(reaisTxt, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("reais: %w", err)
	}
	if reais > (math.MaxInt64-99)/100 {
		return 0, fmt.Errorf("valor grande demais")
	}
	centavos := reais * 100
	if fracTxt != "" {
		if len(fracTxt) == 1 {
			fracTxt += "0"
		}
		frac, err := strconv.ParseInt(fracTxt, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("centavos: %w", err)
		}
		centavos += frac
	}
	if centavos <= 0 {
		return 0, fmt.Errorf("valor não positivo")
	}
	return centavos, nil
}

func soDigitos(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func interpretarVencimento(texto string, agora time.Time) (time.Time, string) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return time.Time{}, msgVencimentoObrigatorio
	}
	// Parse exige o calendário real: 2026-02-31 não passa.
	vencimento, err := time.Parse("2006-01-02", texto)
	if err != nil {
		return time.Time{}, msgVencimentoFormato
	}
	// vencimento já é uma data civil (meia-noite UTC vinda do Parse).
	// Não converter esse instante para São Paulo, senão a meia-noite UTC
	// vira o dia anterior.
	if dataCivil(vencimento).Before(hojeEmSaoPaulo(agora)) {
		return time.Time{}, msgVencimentoPassado
	}
	return vencimento, ""
}

func hojeEmSaoPaulo(agora time.Time) time.Time {
	ano, mes, dia := agora.In(fusoSaoPaulo).Date()
	return time.Date(ano, mes, dia, 0, 0, 0, 0, time.UTC)
}

func dataCivil(dia time.Time) time.Time {
	ano, mes, d := dia.Date()
	return time.Date(ano, mes, d, 0, 0, 0, 0, time.UTC)
}

func interpretarPagador(texto string) (string, string) {
	nome := strings.TrimSpace(texto)
	if nome == "" {
		return "", msgPagadorObrigatorio
	}
	if utf8.RuneCountInString(nome) > tamanhoMaximoPagador {
		return "", msgPagadorLongo
	}
	return nome, ""
}

// FormatarValor escreve centavos como texto com 2 casas: 15050 -> "150.50".
func FormatarValor(centavos int64) string {
	sinal := ""
	if centavos < 0 {
		sinal = "-"
		centavos = -centavos
	}
	return fmt.Sprintf("%s%d.%02d", sinal, centavos/100, centavos%100)
}
