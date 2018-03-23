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
	RP    float64
	Atual float64
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
	Saldo

	Contrato   *Contrato
	Transacoes []*Transacao
}

type Transacao struct {
	Ano       int
	Empenhado float64 // DESPESAS EMPENHADAS (29)
	Liquidado float64 // DESPESAS LIQUIDADAS (31)

	RP_inscrito   float64 // RP INSCRITO (40)
	RP_reinscrito float64 // RP REINSCRITO (41)
	RP_cancelado  float64 // RP CANCELADO (42)
	RP_liquidado  float64 // RP LIQUIDADO (44)
}

var ANO, NUM_EMP, EMP, LIQ, RP_INSC, RP_REINSCR, RP_CANCEL, RP_LIQ int // colunas
var UGE map[string]string                                              // inicio do empenho corresponente à UGE
var contratos map[string]*Contrato                                     // mapa com os contratos

func setup() {

	// início do número de empenho de acordo com a UGE
	UGE = map[string]string{
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
			ugeNumero = UGE[aux[1]]

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
			EMP = cont
		case "Nota Empenho CCor":
			NUM_EMP = cont
		case "DESPESAS LIQUIDADAS (CONTROLE EMPENHO)":
			LIQ = cont
		case "RESTOS A PAGAR NAO PROCESSADOS INSCRITOS":
			RP_INSC = cont
		case "RESTOS A PAGAR NAO PROCESSADOS REINSCRITOS":
			RP_REINSCR = cont
		case "RESTOS A PAGAR NAO PROCESSADOS CANCELADOS":
			RP_CANCEL = cont
		case "RESTOS A PAGAR NAO PROCESSADOS LIQUIDADOS":
			RP_LIQ = cont
		}
		cont++
	}
	ANO = 0
}

// ler arquivo em CSV do Tesouro Gerencial para adicionar transaçoes no empenho
func adicionarTransacoes(empenhos map[string]*Empenho) {
	csvFile, _ := os.Open("tesouro.csv")
	reader := csv.NewReader(bufio.NewReader(csvFile))
	reader.Comma = ';'

	primeiraLinha := true

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

		empenho, temEmpenho := empenhos[linha[NUM_EMP]]
		if !temEmpenho {
			continue
		}

		ano, _ := strconv.Atoi(linha[ANO])            // ANO DA TRANSAÇÃO (0)
		emp := extrairValor(linha[EMP])               // DESPESAS EMPENHADAS (29)
		liq := extrairValor(linha[LIQ])               // DESPESAS LIQUIDADAS (31)
		rp_inscr := extrairValor(linha[RP_INSC])      // RP INSCRITO (40)
		rp_reinscr := extrairValor(linha[RP_REINSCR]) // RP REINSCRITO (41)
		rp_cancel := extrairValor(linha[RP_CANCEL])   // RP CANCELADO (42)
		rp_liq := extrairValor(linha[RP_LIQ])         // RP LIQUIDADO (44)

		transacao := Transacao{
			Ano:           ano,
			Empenhado:     emp,
			Liquidado:     liq,
			RP_inscrito:   rp_inscr,
			RP_reinscrito: rp_reinscr,
			RP_cancelado:  rp_cancel,
			RP_liquidado:  rp_liq}

		empenho.Transacoes = append(empenho.Transacoes, &transacao)
	}

}

// retorna um array de strings formatado [RP,Atual]
func (s Saldo) toTextArray() [2]string {
	var saldos [2]string

	saldos[0] = strconv.FormatFloat(s.RP, 'f', -1, 64)
	saldos[0] = strings.Replace(saldos[0], ".", ",", -1)

	saldos[1] = strconv.FormatFloat(s.Atual, 'f', -1, 64)
	saldos[1] = strings.Replace(saldos[1], ".", ",", -1)

	return saldos
}

func (cnt *Contrato) setSaldos() {
	saldoRP := 0.0
	saldoATUAL := 0.0

	for _, emp := range cnt.Empenhos {
		emp.setSaldos()
		saldoRP += emp.Saldo.RP
		saldoATUAL += emp.Saldo.Atual
	}

	cnt.Saldo.RP = saldoRP
	cnt.Saldo.Atual = saldoATUAL

	fmt.Println("\n")
}

// (0) emp, (1) liq, (2) rp_inscr, (3) rp_reinscr,
// (4) rp_liq_exerc_anterior, (5) rp_cancel_exerc_anterior,
// (6) rp_liq_exerc_atual, (7) rp_cancel_exerc_atual
func (emp *Empenho) setSaldos() {
	ano_atual := time.Now().Local().Year()
	saldos := [8]float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}
	for _, v := range emp.Transacoes {
		saldos[0] += v.Empenhado
		saldos[1] += v.Liquidado
		saldos[2] += v.RP_inscrito
		saldos[3] += v.RP_reinscrito
		if ano_atual > v.Ano { // execução de RP no ano anterior
			saldos[4] += v.RP_liquidado
			saldos[5] += v.RP_cancelado
		} else { // execução de RP no ano atual
			saldos[6] += v.RP_liquidado
			saldos[7] += v.RP_cancelado
		}
	}

	saldoRP := 0.0
	saldoATUAL := 0.0

	rp_inscrito := saldos[2]
	rp_reinscrito := saldos[3]

	if rp_reinscrito > 0 || rp_inscrito > 0 { // cálculo de saldo
		if rp_reinscrito > 0 {
			saldoRP = rp_reinscrito
		} else {
			saldoRP = rp_inscrito
		}
		rp_liq_exerc_atual := saldos[6]
		rp_cancel_exerc_atual := saldos[7]
		saldoRP -= rp_liq_exerc_atual + rp_cancel_exerc_atual
	} else {
		empenhado := saldos[0]
		liquidado := saldos[1]
		saldoATUAL = empenhado - liquidado
		fmt.Println("SALDO:", saldoATUAL, "    ", empenhado, "   ", liquidado, emp.Numero)
	}

	emp.Saldo.RP = saldoRP
	emp.Saldo.Atual = saldoATUAL

	rp := strconv.FormatFloat(emp.Saldo.RP, 'f', 2, 32)
	rp = strings.Repeat(" ", 15-len(rp)) + rp

	fmt.Println(emp.Numero, "\t",
		rp, "\t\t\t",
		strconv.FormatFloat(emp.Saldo.Atual, 'f', 2, 32))
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
			saldos[0],
			saldos[1]}

		writer.Write(registro)
	}
}

func gravarDetalhado(chaves []string, writer *csv.Writer) {
	for _, kc := range chaves {
		fmt.Println(kc)
		c := contratos[kc]
		c.setSaldos()
		saldos := c.Saldo.toTextArray()

		registro := []string{
			c.UGE,
			c.Projeto,
			c.Numero,
			saldos[0],
			saldos[1]}

		writer.Write(registro)

		for _, ke := range c.Empenhos {
			saldos := ke.Saldo.toTextArray()

			registro = []string{
				"",
				"",
				ke.Numero,
				saldos[0],
				saldos[1]}

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

	registro := []string{
		"UGE",
		"PRJ",
		"Numero",
		"Saldo RP",
		"Saldo Exerc Atual"}

	writer.Write(registro)

	chaves := make([]string, 0, len(contratos)) // ordenação
	for k, _ := range contratos {
		chaves = append(chaves, k) // UGE, PROJ, CNT.NUMERO
	}
	sort.Strings(chaves)

	gravarResumido(chaves, writer)

	writer.Write([]string{}) // pula linha

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
