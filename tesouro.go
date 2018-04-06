package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Saldo struct {
	RP          float64
	Atual       float64
	Empenhado   float64
	EmpenhadoRP float64
	Liquidado   float64
	Anulado     float64
}

type Contrato struct {
	Projeto string
	UGE     string
	Numero  string
	Saldo

	Empenhos map[string]*Empenho
}

type Empenho struct {
	Numero string
	ND     string
	Saldo

	Contrato   *Contrato
	Transacoes []*Transacao
}

type Transacao struct {
	Ano       int
	Empenhado float64 // DESPESAS EMPENHADAS (29)
	Anulado   float64
	Liquidado float64 // DESPESAS LIQUIDADAS (31)

	RpInscrito   float64 // RP INSCRITO (40)
	RpReinscrito float64 // RP REINSCRITO (41)
	RpCancelado  float64 // RP CANCELADO (42)
	RpLiquidado  float64 // RP LIQUIDADO (44)
}

var colAno, colNumEmp, colEmp, colLiq, colNd int       // colunas
var colRpInsc, colRpReinscr, colRpCancel, colRpLiq int // colunas
var uge map[string]string                              // inicio do empenho corresponente à UGE
var contratos map[string]*Contrato                     // mapa com os contratos

func setup() {

	// início do número de empenho de acordo com a UGE
	uge = map[string]string{
		"GAL":    "12019500001",
		"GAP-SP": "12063300001",
		"CABE":   "12009100001",
		"CABW":   "12009000001",
		"CELOG":  "12007100001"}
}

// ler arquivo empenhos.txt para obter empenhos de interesse
func popularEmpenhos() map[string]*Empenho {
	file, err := os.Open("empenhos.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	empenhos := make(map[string]*Empenho)
	contratos = make(map[string]*Contrato)

	var cntNumero, ugeNumero, chave string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		aux := strings.Split(scanner.Text(), ":")

		if len(aux) == 3 { // linhas com número do contrato
			chave = aux[1] + " " + aux[0] + " " + aux[2]

			cntNumero = aux[2]
			ugeNumero = uge[aux[1]]

			contratos[chave] = &Contrato{
				Projeto: aux[0],
				UGE:     aux[1],
				Numero:  cntNumero}

		} else {
			aux[0] = strings.Trim(aux[0], " ")
			if len(aux[0]) == 12 { // desconsidera linhas vazias
				contrato := contratos[chave]    // ponteiro
				empNumero := ugeNumero + aux[0] // acrescenta numero UGE

				empenho := Empenho{
					Numero:   empNumero,
					Contrato: contrato} // ponteiro

				if contrato.Empenhos == nil {
					contrato.Empenhos = make(map[string]*Empenho)
				}
				contrato.Empenhos[empNumero] = &empenho

				empenhos[empNumero] = &empenho
			}
		}

	}

	return empenhos
}

func extrairValor(v string) float64 {
	if len(v) == 0 {
		return 0.0
	}
	x := strings.Replace(v, ".", "", -1)
	x = strings.Replace(x, ",", ".", 1)
	if strings.Contains(x, "(") {
		x = strings.Replace(x, ")", "", 1)
		x = strings.Replace(x, "(", "", 1)
		x = "-" + x
	}
	y, _ := strconv.ParseFloat(x, 64)

	return y
}

func setarCampos(linha []string) {
	cont := 0
	for _, l := range linha {

		switch col := l; col {
		case "DESPESAS EMPENHADAS (CONTROLE EMPENHO)":
			colEmp = cont
		case "Natureza Despesa":
			colNd = cont
		case "Nota Empenho CCor":
			colNumEmp = cont
		case "DESPESAS LIQUIDADAS (CONTROLE EMPENHO)":
			colLiq = cont
		case "RESTOS A PAGAR NAO PROCESSADOS INSCRITOS":
			colRpInsc = cont
		case "RESTOS A PAGAR NAO PROCESSADOS REINSCRITOS":
			colRpReinscr = cont
		case "RESTOS A PAGAR NAO PROCESSADOS CANCELADOS":
			colRpCancel = cont
		case "RESTOS A PAGAR NAO PROCESSADOS LIQUIDADOS":
			colRpLiq = cont
		}
		cont++
	}
	colAno = 0
}

// ler arquivo em CSV do Tesouro Gerencial para adicionar transaçoes no empenho
func adicionarTransacoes(empenhos map[string]*Empenho) {
	csvFile, _ := os.Open("tesouro.csv")
	reader := csv.NewReader(bufio.NewReader(csvFile))
	reader.Comma = ';'

	primeiraLinha := true
	anoAtual := time.Now().Local().Year()

	for {
		linha, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		if primeiraLinha {
			primeiraLinha = false
			setarCampos(linha)
		}

		empenho, temEmpenho := empenhos[linha[colNumEmp]]
		if !temEmpenho {
			continue
		}

		ano, _ := strconv.Atoi(linha[colAno]) // ANO DA TRANSAÇÃO (0)

		aux := extrairValor(linha[colEmp]) // DESPESAS EMPENHADAS (29)
		var emp, anul float64
		if aux >= 0 {
			emp = aux
		} else {
			anul = aux
		}

		liq := extrairValor(linha[colLiq]) // DESPESAS LIQUIDADAS (31)

		var rpInscr, rpReinscr float64
		if ano == anoAtual { // desconsidera RP gerados em anos anteriores ao atual
			rpInscr = extrairValor(linha[colRpInsc])      // RP INSCRITO (40)
			rpReinscr = extrairValor(linha[colRpReinscr]) // RP REINSCRITO (41)
		}

		rpCancel := extrairValor(linha[colRpCancel]) // RP CANCELADO (42)
		rpLiq := extrairValor(linha[colRpLiq])       // RP LIQUIDADO (44)

		nd := linha[colNd] // NATUREZA DE DESPESA

		transacao := Transacao{
			Ano:          ano,
			Empenhado:    emp,
			Anulado:      anul,
			Liquidado:    liq,
			RpInscrito:   rpInscr,
			RpReinscrito: rpReinscr,
			RpCancelado:  rpCancel,
			RpLiquidado:  rpLiq}

		empenho.Transacoes = append(empenho.Transacoes, &transacao)
		empenho.ND = nd
	}

}

func valorToText(valor float64) string {
	aux := strconv.FormatFloat(valor, 'f', -1, 64)
	return strings.Replace(aux, ".", ",", -1)
}

// retorna um array de strings formatado [RP,Atual]
func (s Saldo) toTextArray() [6]string {
	var saldos [6]string

	saldos[0] = valorToText(s.RP)
	saldos[1] = valorToText(s.Atual)
	saldos[2] = valorToText(s.Empenhado)
	if s.Liquidado < 0 {
		saldos[3] = valorToText(-s.Liquidado)
		saldos[4] = "0"
	} else {
		saldos[3] = "0"
		saldos[4] = valorToText(s.Liquidado)
	}
	saldos[5] = valorToText(s.Anulado)

	return saldos
}

func (cnt *Contrato) setSaldos() {
	saldoRP := 0.0
	saldoATUAL := 0.0
	empenhado := 0.0
	liquidado := 0.0
	anulado := 0.0

	for _, emp := range cnt.Empenhos {
		emp.setSaldos()
		saldoRP += emp.Saldo.RP
		saldoATUAL += emp.Saldo.Atual
		empenhado += emp.Empenhado
		liquidado += emp.Liquidado
		anulado += emp.Anulado
	}

	cnt.Saldo.RP = saldoRP
	cnt.Saldo.Atual = saldoATUAL
	cnt.Saldo.Empenhado = empenhado
	cnt.Saldo.Liquidado = liquidado
	cnt.Saldo.Anulado = anulado

	fmt.Println()
}

// (0) emp, (1) liq, (2) rp_inscr, (3) rp_reinscr,
// (4) rp_liq_exerc_anterior, (5) rp_cancel_exerc_anterior,
// (6) rp_liq_exerc_atual, (7) rp_cancel_exerc_atual
func (emp *Empenho) setSaldos() {
	anoAtual := time.Now().Local().Year()
	saldos := [9]float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}
	for _, v := range emp.Transacoes {
		saldos[0] += v.Empenhado
		saldos[1] -= v.Anulado

		saldos[2] += v.Liquidado
		saldos[3] += v.RpInscrito
		saldos[4] += v.RpReinscrito
		if anoAtual > v.Ano { // execução de RP no ano anterior
			saldos[5] += v.RpLiquidado
			saldos[6] += v.RpCancelado
		} else { // execução de RP no ano atual
			saldos[7] += v.RpLiquidado
			saldos[8] += v.RpCancelado
		}
	}

	saldoRP := 0.0
	saldoATUAL := 0.0

	rpInscrito := saldos[3]
	rpReinscrito := saldos[4]

	if rpReinscrito > 0 || rpInscrito > 0 { // cálculo de saldo
		if rpReinscrito > 0 {
			saldoRP = rpReinscrito
		} else {
			saldoRP = rpInscrito
		}
		rpLiqExercAtual := saldos[7]
		rpCancelExercAtual := saldos[8]
		saldoRP -= rpLiqExercAtual + rpCancelExercAtual
	} else {
		empenhado := saldos[0] + saldos[1]
		liquidado := saldos[2]
		saldoATUAL = empenhado - liquidado
	}

	emp.Saldo.RP = saldoRP
	emp.Saldo.Atual = saldoATUAL

	emp.Saldo.Empenhado = saldos[0] // valor integral empenhado
	emp.Saldo.Anulado = saldos[1]   // valor anulado no empenhp
	emp.Saldo.Liquidado = saldos[0] - saldos[1] - saldoRP - saldoATUAL

	rp := strconv.FormatFloat(emp.Saldo.RP, 'f', 2, 32)
	rp = strings.Repeat(" ", 15-len(rp)) + rp

	fmt.Println(emp.Numero, "\t",
		rp, "\t\t\t",
		strconv.FormatFloat(emp.Saldo.Atual, 'f', 2, 32))
}

func gravarCabecalho(writer *csv.Writer) {
	writer.Write([]string{"UGE", "PRJ", "Numero", "ND", "Saldo RP", "Saldo Exerc Atual", "",
		"Empenhado", "RP reinsc atual", "Liquidado", "Anulado"})
}

func gravarResumido(chaves []string, writer *csv.Writer) {
	for _, k := range chaves {
		fmt.Println(k)
		c := contratos[k]
		c.setSaldos()
		saldos := c.Saldo.toTextArray()

		registro := []string{
			c.UGE,
			c.Projeto,
			c.Numero,
			"",
			saldos[0],
			saldos[1],
			"",
			saldos[2],
			saldos[3],
			saldos[4],
			saldos[5]}

		writer.Write(registro)
	}
}

func gravarDetalhado(chaves []string, writer *csv.Writer) {
	for _, kc := range chaves {
		fmt.Println(kc)
		c := contratos[kc]
		c.setSaldos()

		registro := []string{
			c.UGE,
			c.Projeto,
			c.Numero}

		gravarCabecalho(writer)
		writer.Write(registro)

		for _, ke := range c.Empenhos {
			saldos := ke.Saldo.toTextArray()

			registro = []string{
				"",
				"",
				ke.Numero,
				ke.ND,
				saldos[0],
				saldos[1],
				"",
				saldos[2],
				saldos[3],
				saldos[4],
				saldos[5]}

			writer.Write(registro)
		}
		writer.Write([]string{}) // pula linha
	}
}

func gravarSaldos() {
	t := time.Now().Local()
	arq := "db/saldos " + t.Format("2006-01-02") + ".csv"

	file, _ := os.Create(arq)
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	chaves := make([]string, 0, len(contratos)) // ordenação
	for k := range contratos {
		chaves = append(chaves, k) // UGE, PROJ, CNT.NUMERO
	}
	sort.Strings(chaves)

	gravarCabecalho(writer)
	gravarResumido(chaves, writer)
	writer.Write([]string{}) // pula linha
	//gravarCabecalho(writer)
	gravarDetalhado(chaves, writer)
}

func pressionarTecla() { // utilizar para testes
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\n\n Pressione uma tecla")
	text, _ := reader.ReadString('\n')
	fmt.Println(text)
}

func main() {
	setup()
	mapEmpenhos := popularEmpenhos() // string,*Empenho
	adicionarTransacoes(mapEmpenhos)
	gravarSaldos()
}
