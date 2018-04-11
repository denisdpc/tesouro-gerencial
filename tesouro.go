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
	Resumido
}

type Resumido struct {
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
	Ano int

	RpInscrito   float64 // RP INSCRITO (40)
	RpReinscrito float64 // RP REINSCRITO (41)
	RpCancelado  float64 // RP CANCELADO (42)
	RpLiquidado  float64 // RP LIQUIDADO (44)

	Resumido
}

type Projeto struct {
	PI    string
	sigla string // sigla do projet
}

var colAno, colUGE, colPI, colNumEmp, colEmp, colLiq, colNd int    // colunas
var colCredito, colRpInsc, colRpReinscr, colRpCancel, colRpLiq int // colunas
var uge map[string]string                                          // inicio do empenho corresponente à UGE
var contratos map[string]*Contrato                                 // mapa com os contratos
var projetos map[string]*Projeto                                   // pi --> projeto
var creditos map[[3]string]float64                                 // pi,uge,nd -->credito acumulado

func setup() {
	uge = map[string]string{ // início do número de empenho de acordo com a UGE
		"GAL":    "12019500001",
		"GAP-SP": "12063300001",
		"CABE":   "12009100001",
		"CABW":   "12009000001",
		"CELOG":  "12007100001"}
}

func lerArq(arq string) *os.File {
	file, err := os.Open(arq)
	if err != nil {
		log.Fatal(err)
	}
	return file
}

// ler arquivo PI.txt
// retorna map(projeto) -> PI
func lerArqPI() map[string]*Projeto {
	file := lerArq("PI.txt")
	defer file.Close()

	projetos = make(map[string]*Projeto)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		aux := strings.Split(scanner.Text(), ":")
		if len(aux) == 2 {
			proj := Projeto{PI: aux[0], sigla: strings.Trim(aux[1], " ")}
			projetos[aux[0]] = &proj
		}
	}
	return projetos
}

// ler arquivo empenhos.txt
// retorna map(numEmpenho) -> empenho
func lerArqEmpenhos() map[string]*Empenho {
	file := lerArq("empenhos.txt")
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
		case "UG Executora":
			colUGE = cont
		case "DESPESAS EMPENHADAS (CONTROLE EMPENHO)":
			colEmp = cont
		case "PI":
			colPI = cont
		case "Natureza Despesa":
			colNd = cont
		case "Nota Empenho CCor":
			colNumEmp = cont
		case "CREDITO DISPONIVEL":
			colCredito = cont
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

// ler arquivo em CSV do Tesouro Gerencial
// para adicionar transaçoes no empenho e
// crédito nos projetos
func lerArqTesouro(empenhos map[string]*Empenho,
	projetos map[string]*Projeto) {

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

		ano, _ := strconv.Atoi(linha[colAno]) // ANO DA TRANSAÇÃO (0)

		empNumero := linha[colNumEmp]
		empenho, temEmpenho := empenhos[empNumero]
		if !temEmpenho {
			if empNumero == "-9" { // contabilizar credito
				if ano == anoAtual {
					pi := linha[colPI]
					if _, temPI := projetos[pi]; temPI {
						valor := extrairValor(linha[colCredito])
						if valor == 0 {
							continue
						}
						uge := linha[colUGE]
						nd := linha[colNd]

						if creditos == nil {
							creditos = make(map[[3]string]float64)
						}
						chave := [3]string{pi, uge, nd}
						creditos[chave] += valor
					}
				}
			} else { // contabilizar outros empenhos com PI de interesse
				pi := linha[colPI]
				if _, temPI := projetos[pi]; temPI {
					fmt.Println(projetos[pi].sigla, empNumero)
				}
			}
			continue
		}

		aux := extrairValor(linha[colEmp]) // DESPESAS EMPENHADAS (29)
		var emp, empRP, anul float64
		if aux >= 0 {
			if ano == anoAtual {
				emp = aux
			} else {
				empRP = aux
			}
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
			Ano: ano,
			Resumido: Resumido{
				Empenhado:   emp,
				EmpenhadoRP: empRP,
				Anulado:     anul,
				Liquidado:   liq},
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
func (s Saldo) toTextArray() [7]string {
	var saldos [7]string

	saldos[0] = valorToText(s.RP)
	saldos[1] = valorToText(s.Atual)
	saldos[2] = valorToText(s.Empenhado)
	saldos[3] = valorToText(s.EmpenhadoRP)
	if s.Resumido.Liquidado < 0 {
		saldos[4] = valorToText(-s.Liquidado)
		saldos[5] = "0"
	} else {
		saldos[4] = "0"
		saldos[5] = valorToText(s.Liquidado)
	}
	saldos[6] = valorToText(s.Anulado)

	return saldos
}

func (cnt *Contrato) setSaldos() {
	var saldoRP, saldoATUAL, empenhado, empenhadoRP, liquidado, anulado float64

	for _, emp := range cnt.Empenhos {
		emp.setSaldos()
		saldoRP += emp.Saldo.RP
		saldoATUAL += emp.Saldo.Atual
		empenhado += emp.Empenhado
		empenhadoRP += emp.EmpenhadoRP
		liquidado += emp.Liquidado
		anulado += emp.Anulado
	}

	cnt.RP = saldoRP
	cnt.Atual = saldoATUAL
	cnt.Empenhado = empenhado
	cnt.EmpenhadoRP = empenhadoRP
	cnt.Liquidado = liquidado
	cnt.Anulado = anulado
}

// (0) emp, (1) liq, (2) rp_inscr, (3) rp_reinscr,
// (4) rp_liq_exerc_anterior, (5) rp_cancel_exerc_anterior,
// (6) rp_liq_exerc_atual, (7) rp_cancel_exerc_atual
func (emp *Empenho) setSaldos() {
	anoAtual := time.Now().Local().Year()
	saldos := [10]float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}
	for _, transacao := range emp.Transacoes {
		if anoAtual > transacao.Ano { // execução ano anterior
			saldos[1] += transacao.EmpenhadoRP
			saldos[6] += transacao.RpLiquidado
			saldos[7] += transacao.RpCancelado
		} else { // execução ano atual
			saldos[0] += transacao.Empenhado
			saldos[8] += transacao.RpLiquidado
			saldos[9] += transacao.RpCancelado
		}
		saldos[2] -= transacao.Anulado
		saldos[3] += transacao.Liquidado
		saldos[4] += transacao.RpInscrito
		saldos[5] += transacao.RpReinscrito
	}

	saldoRP := 0.0
	saldoATUAL := 0.0

	rpInscrito := saldos[4]
	rpReinscrito := saldos[5]

	if rpReinscrito > 0 || rpInscrito > 0 { // cálculo de saldo
		if rpReinscrito > 0 {
			saldoRP = rpReinscrito
		} else {
			saldoRP = rpInscrito
		}
		rpLiqExercAtual := saldos[8]
		rpCancelExercAtual := saldos[9]
		saldoRP -= rpLiqExercAtual + rpCancelExercAtual
	} else {
		empenhado := saldos[0] + saldos[1] + saldos[2]
		liquidado := saldos[3]
		saldoATUAL = empenhado - liquidado
	}

	emp.RP = saldoRP
	emp.Atual = saldoATUAL

	emp.Empenhado = saldos[0]   // valor empenhado atual
	emp.EmpenhadoRP = saldos[1] // valor empenhado RP
	emp.Anulado = saldos[2]     // valor anulado no empenho
	emp.Liquidado = saldos[0] + saldos[1] - saldos[2] - saldoRP - saldoATUAL

	rp := strconv.FormatFloat(emp.Saldo.RP, 'f', 2, 32)
	rp = strings.Repeat(" ", 15-len(rp)) + rp

	/*
		fmt.Println(emp.Numero, "\t",
			rp, "\t\t\t",
			strconv.FormatFloat(emp.Saldo.Atual, 'f', 2, 32))
	*/
}

func gravarContratosCabecalho(writer *csv.Writer) {
	writer.Write([]string{"UGE", "PRJ", "Numero", "ND", "Saldo RP", "Saldo Exerc Atual", "",
		"Empenhado", "Empenhado RP", "RP reinsc atual", "Liquidado", "Anulado"})
}

func gravarContratosResumido(chaves []string, writer *csv.Writer) {
	for _, k := range chaves {
		//fmt.Println(k)
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
			saldos[5],
			saldos[6]}

		writer.Write(registro)
	}
}

func gravarContratosDetalhado(chaves []string, writer *csv.Writer) {
	for _, kc := range chaves {
		c := contratos[kc]
		c.setSaldos()

		registro := []string{
			c.UGE,
			c.Projeto,
			c.Numero}

		gravarContratosCabecalho(writer)
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
				saldos[5],
				saldos[6]}

			writer.Write(registro)
		}
		writer.Write([]string{}) // pula linha
	}
}

func gravarCreditosNaoEmpenhados(writer *csv.Writer) {
	writer.Write([]string{"UGE", "PRJ", "PI", "ND", "Credito"})

	chaves := make([]string, 0, len(creditos))
	for k := range creditos { // ordenação
		chaves = append(chaves, k[0]+":"+k[1]+":"+k[2]) // PI, UGE, ND
	}
	sort.Strings(chaves)

	for _, chave := range chaves {
		aux := strings.Split(chave, ":")
		c := [3]string{aux[0], aux[1], aux[2]}
		credito := creditos[c]
		if credito == 0 {
			continue
		}
		projeto := projetos[c[0]]
		registro := []string{c[1], // uge
			projeto.sigla,
			c[0], // PI
			c[2], // nd
			valorToText(credito)}
		writer.Write(registro)

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

	gravarContratosCabecalho(writer)
	gravarContratosResumido(chaves, writer)
	writer.Write([]string{}) // pula linha
	gravarCreditosNaoEmpenhados(writer)
	writer.Write([]string{}) // pula linha
	writer.Write([]string{}) // pula linha
	gravarContratosDetalhado(chaves, writer)
}

func pressionarTecla() { // utilizar para testes
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\n\n Pressione uma tecla")
	text, _ := reader.ReadString('\n')
	fmt.Println(text)
}

func main() {
	setup()
	mapProjetos := lerArqPI()       // string(PI),Projeto
	mapEmpenhos := lerArqEmpenhos() // string(numEmpenho),*Empenho
	lerArqTesouro(mapEmpenhos, mapProjetos)

	gravarSaldos()
}
