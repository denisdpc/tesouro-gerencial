package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

type Contrato struct {
	Projeto  string
	Numero   string
	Empenhos map[string]*Empenho
}

type Empenho struct {
	Numero     string
	Contrato   *Contrato
	Transacoes []*Transacao
}

type Transacao struct {
	Ano        int
	Observacao string
	Empenho    *Empenho
}

func relacionarProjetoContrato() []*Contrato {
	file, err := os.Open("contratos.txt")
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

func identificarContrato(contratos []*Contrato, obs string) int {
	for i, cnt := range contratos {
		if strings.Contains(obs, cnt.Numero) {
			fmt.Println(cnt.Numero)
			return i
		}
	}
	return -1
}

func main() {

	p_contratos := relacionarProjetoContrato()
	fmt.Println(p_contratos)

	linha := []string{"NE80032", "xxxx 031/GAL-PAMASP/2016 DSFF"}
	//obs := "xxxx 046/GAL-PAMASP/2017 DSFF"
	j := identificarContrato(p_contratos, linha[1])
	fmt.Println(j) // se j == -1 , contrato nao esta presente na observacao

	// TESTE COMMIT 21
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
