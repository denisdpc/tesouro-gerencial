package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Saldo ...
type Saldo struct {
	Atual float64
	RP    float64
	Resumido
}

// Resumido ...
type Resumido struct {
	Empenhado   float64
	EmpenhadoRP float64
	Liquidado   float64
	Anulado     float64
}

// Contrato ...
type Contrato struct {
	Projeto string
	UGE     string
	Numero  string
	Saldo

	Empenhos map[string]*Empenho
}

// Empenho ...
type Empenho struct {
	Numero string
	ND     string
	Saldo

	Contrato   *Contrato
	Transacoes []*Transacao
}

// Transacao ...
type Transacao struct {
	Ano int

	RpInscrito   float64 // RP INSCRITO (40)
	RpReinscrito float64 // RP REINSCRITO (41)
	RpCancelado  float64 // RP CANCELADO (42)
	RpLiquidado  float64 // RP LIQUIDADO (44)

	Resumido
}

// Projeto ...
type Projeto struct {
	PI    string
	sigla string // sigla do projet
}

var colAnoTrans, colUGE, colPI, colNumEmp, colEmp, colLiq, colNd int // colunas
var colCredito, colRpInsc, colRpReinscr, colRpCancel, colRpLiq int   // colunas
var uge map[string]string                                            // inicio do empenho corresponente à UGE
var contratos map[string]*Contrato                                   // uge_completo, prj, cnt_num --> contratos
var projetos map[string]*Projeto                                     // pi --> projeto
var creditos map[[3]string]float64                                   // pi,uge,nd -->credito acumulado
var tabelaTG [][]string

func setup() {
	uge = map[string]string{ // início do número de empenho de acordo com a UGE
		"CAE":    "12019500001",
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
func lerArqPI() {
	//func lerArqPI() map[string]*Projeto {
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
	//return projetos
}

// ler arquivo empenhos.txt
// retorna map(numEmpenho) -> empenho
// relaciona empenhos a contrato
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
	colAnoTrans = 0
}

// upload do arquivo do tesouro gerencial
func upload(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		f, _, err := req.FormFile("usrfile")
		if err != nil {
			log.Println(err)
			http.Error(w, "Error uploading file", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		arq, err := ioutil.ReadAll(f)
		if err != nil {
			log.Println(err)
			http.Error(w, "Error reading file", http.StatusInternalServerError)
			return
		}
		popularTabelaTG(&arq)
		lerArqPI()                      // string(PI),Projeto
		mapEmpenhos := lerArqEmpenhos() // string(numEmpenho),*Empenho

		processarTG(mapEmpenhos, projetos)

		tabResumido, tabExtras, tabCredito, tabDetalhado,
			tabCabecalhoEmpenhado, tabCabecalhoCredito := gerarTabelas()

		linha := &[][]string{{}}
		content := bytes.NewReader(
			tabelasToByteArray(
				tabCabecalhoEmpenhado, tabResumido, tabExtras,
				linha, tabCabecalhoCredito, tabCredito,
				linha, tabCabecalhoEmpenhado, tabDetalhado))
		modtime := time.Now()

		w.Header().Add("Content-Disposition", "Attachment;filename=resultado.csv")
		w.Header().Add("filename", "text/csv")
		http.ServeContent(w, req, "", modtime, content)

		arq = nil
		tabelaTG = nil
		projetos = nil
		creditos = nil
		contratos = nil
		content = nil

		tabResumido, tabExtras, tabCredito, tabDetalhado,
			tabCabecalhoEmpenhado, tabCabecalhoCredito = nil, nil, nil, nil, nil, nil

		runtime.GC()
	}

	w.Header().Set("CONTENT-TYPE", "text/html; charset=UTF-8")
	fmt.Fprintf(w, `<form action="/upload" method="post" enctype="multipart/form-data">
		            <input type="file" name="usrfile" value="arquivo">
					<input type="submit"></form><br><br>`)

}

func tabelasToByteArray(tabelas ...*[][]string) []byte {
	var b []byte
	for _, tabela := range tabelas {
		for _, linha := range *tabela {
			for _, col := range linha {
				cel := []byte(col)
				for _, ch := range cel {
					b = append(b, ch)
				}
				b = append(b, 59) // adicionar ";"
			}
			b = append(b, 10) // adicionar LF
		}
	}
	return b
}

func gerarTabelaCredito() *[][]string {
	var tabCredito [][]string

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

		tabCredito = append(tabCredito, registro)
	}
	return &tabCredito
}

// retorna tabela resumido e extra respectivamente
func gerarTabelaResumidoExtra(chaves []string) (*[][]string, *[][]string) {
	var tabResumido, tabExtras [][]string

	for _, k := range chaves {
		c := contratos[k]
		c.setSaldos()
		saldos := c.Saldo.toTextArray()

		registro := []string{
			c.UGE,
			c.Projeto,
			c.Numero,
			"",
			saldos[0], // saldo Exerc Atual
			saldos[1], // saldo RP
			"",
			saldos[2],
			saldos[3],
			saldos[4],
			saldos[5],
			saldos[6]}

		if c.Numero == "EXTRA" {
			tabExtras = append(tabExtras, registro)
		} else {
			tabResumido = append(tabResumido, registro)
		}
	}

	return &tabResumido, &tabExtras
}

func gerarTabelaDetalhado(chaves []string) *[][]string {
	var tabDetalhado [][]string

	for _, kc := range chaves {
		c := contratos[kc]
		c.setSaldos()

		for _, ke := range c.Empenhos {
			saldos := ke.Saldo.toTextArray()

			if ke.Saldo.Atual+ke.Saldo.RP < 0.001 {
				continue
			}

			registro := []string{
				c.UGE,
				c.Projeto,
				c.Numero + " : " + ke.Numero,
				ke.ND,
				saldos[0],
				saldos[1],
				"",
				saldos[2],
				saldos[3],
				saldos[4],
				saldos[5],
				saldos[6]}

			tabDetalhado = append(tabDetalhado, registro)
		}
	}
	return &tabDetalhado
}

// popular tabelaTG
func popularTabelaTG(arq *[]byte) {
	if arq == nil { // processament local
		csvFile, _ := os.Open("tesouro.csv")
		reader := csv.NewReader(bufio.NewReader(csvFile))
		reader.Comma = ';'
		tabelaTG, _ = reader.ReadAll()
	} else { // processamento do servidor
		var linha []string
		for _, b := range *arq {
			if b == 10 { // salto de linha (LF)
				var aux1 string
				var aux2 []string

				aux1 = strings.Join(linha, "")
				aux2 = strings.Split(aux1, ";")

				tabelaTG = append(tabelaTG, aux2)
				linha = []string{}

				continue
			}
			if b == 34 { // desconsiderar aspas "
				continue
			}
			linha = append(linha, string(b))
		}
	}
	setarCampos(tabelaTG[0])
}

func processarTG(empenhos map[string]*Empenho,
	projetos map[string]*Projeto) {

	anoAtual := time.Now().Local().Year()

	for _, linha := range tabelaTG {
		anoTrans, _ := strconv.Atoi(linha[colAnoTrans]) // ANO DA TRANSAÇÃO (0)

		empNumero := linha[colNumEmp]
		empenho, empenhoListado := empenhos[empNumero]
		if !empenhoListado {
			if empNumero == "-9" { // contabilizar credito
				if anoTrans == anoAtual {
					pi := linha[colPI]
					if _, temPI := projetos[pi]; temPI {
						valor := extrairValor(linha[colCredito])
						if valor == 0 {
							continue
						}
						uge := linha[colUGE]

						// modifica de GAL para CAE
						if uge == "GAL" {
							uge = "CAE"
						}

						nd := linha[colNd]

						if creditos == nil {
							creditos = make(map[[3]string]float64)
						}
						chave := [3]string{pi, uge, nd}
						creditos[chave] += valor
					}
				}
				continue
			} else { // contabilizar empenho não lista em empenhos.txt
				pi := linha[colPI]
				if _, piListado := projetos[pi]; piListado { // PI listado em PI.txt
					ugeNumero := linha[colUGE]
					prjSigla := projetos[pi].sigla

					// modifica de GAL para CAE
					if ugeNumero == "GAL" {
						ugeNumero = "CAE"
					}

					chave := ugeNumero + " " + prjSigla + " EXTRA"
					if contratos[chave] == nil {
						contratos[chave] = &Contrato{
							Projeto: prjSigla,
							UGE:     ugeNumero,
							Numero:  "EXTRA"}

						cnt := contratos[chave]
						cnt.Empenhos = make(map[string]*Empenho)
					}

					cnt := contratos[chave]

					if cnt.Empenhos[empNumero] == nil {
						empenho = &Empenho{
							Numero: empNumero}

						cnt.Empenhos[empNumero] = empenho
						empenho.Contrato = cnt
					} else {
						empenho = cnt.Empenhos[empNumero]
					}
				} else { // empenho e PI não listado
					continue
				}
			}
		}

		valorEmp := extrairValor(linha[colEmp])              // DESPESAS EMPENHADAS (29)
		anoEmp, _ := strconv.Atoi((linha[colNumEmp])[11:15]) // ANO EM QUE FOI GERADO O EMPENHO

		var emp, empRP, anulado float64
		if valorEmp >= 0 {
			if anoEmp == anoAtual {
				emp = valorEmp
			} else {
				empRP = valorEmp
			}
		} else {
			anulado = valorEmp
		}

		liq := extrairValor(linha[colLiq]) // DESPESAS LIQUIDADAS (31)

		var rpInscr, rpReinscr float64
		if anoTrans == anoAtual { // desconsidera RP gerados em anos anteriores ao atual
			rpInscr = extrairValor(linha[colRpInsc])      // RP INSCRITO (40)
			rpReinscr = extrairValor(linha[colRpReinscr]) // RP REINSCRITO (41)
		}

		rpCancel := extrairValor(linha[colRpCancel]) // RP CANCELADO (42)
		rpLiq := extrairValor(linha[colRpLiq])       // RP LIQUIDADO (44)

		nd := linha[colNd] // NATUREZA DE DESPESA

		transacao := Transacao{
			Ano: anoTrans,
			Resumido: Resumido{
				Empenhado:   emp,
				EmpenhadoRP: empRP,
				Anulado:     anulado,
				Liquidado:   liq},
			RpInscrito:   rpInscr,
			RpReinscrito: rpReinscr,
			RpCancelado:  rpCancel,
			RpLiquidado:  rpLiq} // RESTOS A PAGAR NAO PROCESSADOS LIQUIDADOS

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

	saldos[0] = valorToText(s.Atual)
	saldos[1] = valorToText(s.RP)
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
			saldos[0] += transacao.Empenhado   // empenhado no ano atual
			saldos[8] += transacao.RpLiquidado // RP liquidado no ano atual
			saldos[9] += transacao.RpCancelado // RP cancelado no ano atual
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

		if saldos[0] > 0 { // houve empenho no ano atual ?
			empenhado := saldos[0] + saldos[1] - saldos[2]
			liquidado := saldos[3]
			saldoATUAL = empenhado - liquidado
		} else { // se não houve
			saldoATUAL = 0
		}

	}

	emp.RP = saldoRP
	emp.Atual = saldoATUAL

	emp.Empenhado = saldos[0]   // valor empenhado atual
	emp.EmpenhadoRP = saldos[1] // valor empenhado RP
	emp.Anulado = saldos[2]     // valor anulado no empenho
	emp.Liquidado = saldos[0] + saldos[1] - saldos[2] - saldoRP - saldoATUAL

	rp := strconv.FormatFloat(emp.Saldo.RP, 'f', 2, 32)
	rp = strings.Repeat(" ", 15-len(rp)) + rp
}

func getCabecalhoEmpenhado() *[][]string {
	return &[][]string{{"UGE", "PRJ", "Numero", "ND", "Saldo ATUAL", "Saldo RP", "",
		"Empenhado ATUAL", "Empenhado RP", "RP reinsc atual", "Liquidado", "Anulado"}}
}

func getCabecalhoCredito() *[][]string {
	return &[][]string{{"UGE", "PRJ", "PI", "ND", "Credito"}}
}

func gravarTabela(writer *csv.Writer, tabelas ...*[][]string) {
	for _, tabela := range tabelas {
		writer.WriteAll(*tabela)
	}
}

func gerarTabelas() (*[][]string, *[][]string, *[][]string, *[][]string, *[][]string, *[][]string) {
	chaves := getChavesOrdenacao()

	tabResumido, tabExtra := gerarTabelaResumidoExtra(chaves)
	tabCredito := gerarTabelaCredito()
	tabDetalhado := gerarTabelaDetalhado(chaves)

	tabCabecalhoEmpenhado := getCabecalhoEmpenhado()
	tabCabecalhoCredito := getCabecalhoCredito()

	return tabResumido, tabExtra, tabCredito, tabDetalhado, tabCabecalhoEmpenhado, tabCabecalhoCredito
}

func gravarSaldos() {

	tabResumido, tabExtra, tabCredito, tabDetalhado,
		tabCabecalhoEmpenhado, tabCabecalhoCredito := gerarTabelas()

	t := time.Now().Local()
	arq := "db/saldos " + t.Format("2006-01-02") + ".csv"

	file, _ := os.Create(arq)
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	gravarTabela(writer, tabCabecalhoEmpenhado, tabResumido, tabExtra)
	writer.Write([]string{}) // pula linha

	gravarTabela(writer, tabCabecalhoCredito, tabCredito)
	writer.Write([]string{}) // pula linha

	gravarTabela(writer, getCabecalhoEmpenhado(), tabDetalhado)
}

func pressionarTecla() { // utilizar para testes
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\n\n Pressione uma tecla")
	text, _ := reader.ReadString('\n')
	fmt.Println(text)
}

func getChavesOrdenacao() []string {
	chaves := make([]string, 0, len(contratos)) // ordenação
	for k := range contratos {
		chaves = append(chaves, k) // UGE, PROJ, CNT.NUMERO
	}
	sort.Strings(chaves)
	return chaves
}

func main() {
	setup()

	modoServer := flag.Bool("server", false, "modo servidor")
	flag.Parse()

	if *modoServer {
		http.HandleFunc("/", upload)
		http.ListenAndServe(":8080", nil)
	} else {
		popularTabelaTG(nil)
		lerArqPI()                      // projetos : string(PI),Projeto
		mapEmpenhos := lerArqEmpenhos() // string(numEmpenho),*Empenho
		processarTG(mapEmpenhos, projetos)
		gravarSaldos()
	}
}
