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
	ano_base := time.Now().Local().Year()
	for k, v := range contratos {
		fmt.Println(k, v)
	}
	fmt.Println(ano_base)
}

func main() {

	setup()
	mapEmpenhos := getMapEmpenhos() // string,*Empenho
	adicionarTransacoes(mapEmpenhos)

	processar()

	// LEITURA DAS LINHAS

	/*
		p_contratos := relacionarProjetoContrato()
		fmt.Println(p_contratos)

		linha := []string{"NE80032", "xxxx 031/GAL-PAMASP/2016 DSFF"}
		//obs := "xxxx 046/GAL-PAMASP/2017 DSFF"
		j := identificarContrato(p_contratos, linha[1])
		fmt.Println(j) // se j == -1 , contrato nao esta presente na observacao
	*/
	// TESTE COMMIT 21"
	/*
		for i, _ := range contratos {
			cnt := contratos[i]
			fmt.Println(i, cnt)
		}

		aux := [2]string{"asas", "dfdf"}
		identificarContrato(contratos, aux)

		for i, _ := range contratos {
			cnt := contratos[i]
			fmt.Println(i, cnt)
		}
	*/

	// LEITURA DO ARQUIVOs
	// CRIAR EMPENHO CONFORME LEITURA
	// VERIFICAR SE EMPENHO TEM CONTRATO ASSOCIADO
	// SE NAO TIVER, VERIFICAR SE NA OBSERVAcao CONSTA ALGUM DOS CONTRATOS PARA ASSOCIAR
	// RELACIONAR TRANSACAO AO EMPENHO

	// TESTES
	/*
		e11 := Empenho{Numero: "NE800321"}
		c1 := Contrato{Numero: "001/PAMASP/2011"}

		e11.Contrato = &c1

		c1.Empenhos = make(map[string]*Empenho)
		c1.Empenhos[e11.Numero] = &e11

		fmt.Println(c1.Numero, (*e11.Contrato).Numero)

		c1.Numero = "002/PAMASP"

		fmt.Println(c1.Numero, (*e11.Contrato).Numero)

		e11.Numero = "NE12344"

		fmt.Println(c1.Empenhos["NE800321"].Numero)
	*/
}

/*
func relacionarProjetoContrato() []*Contrato {
	file, err := os.Open("contratos.dat")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var p_contratos []*Contrato

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		aux := strings.Split(scanner.Text(), ":")
		p_contratos = append(p_contratos, &Contrato{Projeto: aux[0], Numero: aux[1]})
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return p_contratos
}
*/

/*
func identificarContrato(contratos []*Contrato, obs string) int {
	for i, cnt := range contratos {
		if strings.Contains(obs, cnt.Numero) {
			fmt.Println(cnt.Numero)
			return i
		}
	}
	return -1
}
*/
