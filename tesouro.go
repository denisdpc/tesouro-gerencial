package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Contrato struct {
	Projeto string
	UGE     string
	Numero  string

	Empenhos map[string]*Empenho
}

type Empenho struct {
	Numero string

	Contrato   *Contrato
	Transacoes []*Transacao
}

type Transacao struct {
	Ano        int
	Observacao string
	Empenhado  float64 // DESPESAS EMPENHADAS (29)
	Liquidado  float64 // DESPESAS LIQUIDADAS (31)

	RP_inscrito   float64 // RP INSCRITO (40)
	RP_reinscrito float64 // RP REINSCRITO (41)
	RP_cancelado  float64 // RP CANCELADO (42)
	RP_liquidado  float64 // RP LIQUIDADO (44)

	//Empenho *Empenho
}

var ANO, EMP, LIQ, RP_INSC, RP_REINSCR, RP_CANCEL, RP_LIQ int // colunas
var UGE map[string]string                                     // inicio do empenho corresponente à UGE
var contratos map[string]*Contrato                            // mapa com os contratos

func setup() {

	// início do número de empenho de acordo com a UGE
	UGE = map[string]string{
		"GAL":    "12019500001",
		"GAP-SP": "12007100001",
		"CABE":   "12009100001",
		"CABW":   "1200900000"}

	// colunas da tabela para extração de dados de interesse
	ANO, EMP, LIQ = 0, 20, 22
	RP_INSC, RP_REINSCR, RP_CANCEL, RP_LIQ = 24, 45, 46, 48
}

// ler arquivo contratos.dat para obter empenhos de interesse
func getMapEmpenhos() map[string]*Empenho {
	file, err := os.Open("contratos.dat")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	empenhos := make(map[string]*Empenho)
	contratos = make(map[string]*Contrato)

	var cntNumero, ugeNumero string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		aux := strings.Split(scanner.Text(), ":")

		if len(aux) == 3 { // linhas com número do contrato
			cntNumero = aux[2]
			ugeNumero = UGE[aux[1]]
			contratos[cntNumero] = &Contrato{
				Projeto: aux[0],
				UGE:     aux[1],
				Numero:  cntNumero}

		} else if len(aux[0]) == 12 { // desconsidera linhas vazias
			contrato := contratos[cntNumero] // ponteiro
			empNumero := ugeNumero + aux[0]  // acrescenta numero UGE

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

// ler arquivo em CSV do Tesouro Gerencial para adicionar transaçoes no empenho
func adicionarTransacoes(empenhos map[string]*Empenho) {

	csvFile, _ := os.Open("tesouro.csv")
	reader := csv.NewReader(bufio.NewReader(csvFile))
	reader.Comma = ';'

	for {
		linha, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		empenho, temEmpenho := empenhos[linha[10]]
		if !temEmpenho {
			continue
		}

		ano, _ := strconv.Atoi(linha[ANO])
		obs := linha[14]
		emp := extrairValor(linha[20])        // DESPESAS EMPENHADAS (29)
		liq := extrairValor(linha[22])        // DESPESAS LIQUIDADAS (31)
		rp_inscr := extrairValor(linha[24])   // RP INSCRITO (40)
		rp_reinscr := extrairValor(linha[25]) // RP REINSCRITO (41)
		rp_cancel := extrairValor(linha[26])  // RP CANCELADO (42)
		rp_liq := extrairValor(linha[28])     // RP LIQUIDADO (44)

		transacao := Transacao{
			Ano:           ano,
			Observacao:    obs,
			Empenhado:     emp,
			Liquidado:     liq,
			RP_inscrito:   rp_inscr,
			RP_reinscrito: rp_reinscr,
			RP_cancelado:  rp_cancel,
			RP_liquidado:  rp_liq}

		empenho.Transacoes = append(empenho.Transacoes, &transacao)
	}

}

// obter saldos de empenhos indexidos a cada contrato
func processar() {
	for _, c := range contratos {
		c.saldos()
	}
}

func (cnt *Contrato) saldos() {
	fmt.Println(cnt.Numero)
	saldos := [7]float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}

	for _, emp := range cnt.Empenhos {
		for i, v := range emp.saldos() {
			saldos[i] += v
		}
	}
	fmt.Println("CNT=", saldos)
}

// emp, liq, rp_inscr,rp_liq_antigo,rp_cancel_antigo,rp_liq_atual,rp_cancel_atual
func (emp *Empenho) saldos() [7]float64 {
	ano_atual := time.Now().Local().Year()
	saldos := [7]float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}
	for _, v := range emp.Transacoes {
		saldos[0] += v.Empenhado
		saldos[1] += v.Liquidado
		saldos[2] += v.RP_inscrito
		if ano_atual > v.Ano { // execução de RP no ano anterior
			saldos[3] += v.RP_liquidado
			saldos[4] += v.RP_cancelado
		} else { // execução de RP no ano atual
			saldos[5] += v.RP_liquidado
			saldos[6] += v.RP_cancelado
		}
	}
	fmt.Println(saldos)
	return saldos
}

func main() {

	setup()
	mapEmpenhos := getMapEmpenhos() // string,*Empenho
	adicionarTransacoes(mapEmpenhos)

	for _, c := range contratos {
		c.saldos()
	}

}
