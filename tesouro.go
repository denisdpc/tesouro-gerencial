package main

import (
	"fmt"
	"os"
	"log"
	"bufio"
	"strings"
)

type Contrato struct {
	Numero string
	Projeto string
	Empenhos map[string]*Empenho
}

type Empenho struct {
	Numero string
	Contrato *Contrato
	Transacoes []*Transacao
}

type Transacao struct {
	Ano int
	Observacao string
	Empenho *Empenho
}


func relacionarProjetoContrato() []Contrato {
  file, err := os.Open("contratos.txt")
  if err != nil {
    log.Fatal(err)
  }
  defer file.Close()
  
  var contratos []Contrato
  
  scanner := bufio.NewScanner(file)
  for scanner.Scan() {
    aux := strings.Split(scanner.Text(),":")
    contratos = append(contratos,Contrato{Numero:aux[0],Projeto:aux[1]})
  }
  
  if err := scanner.Err(); err != nil {
        log.Fatal(err)
  }
  return contratos
}

func main() {
  
  contratos := relacionarProjetoContrato()
  fmt.Println(contratos)
  
  
  
  // LEITURA DO ARQUIVO
  // CRIAR EMPENHO CONFORME LEITURA
  // VERIFICAR SE EMPENHO TEM CONTRATO ASSOCIADO
  // SE NAO TIVER, VERIFICAR SE NA OBSERVAÇÃO CONSTA ALGUM DOS CONTRATOS PARA ASSOCIAR
  // RELACIONAR TRANSACAO AO EMPENHO
  


  
  
  // TESTES
	
	e11 := Empenho{Numero:"NE800321"}		
	c1  := Contrato{Numero:"001/PAMASP/2011"}
	
	e11.Contrato = &c1

	c1.Empenhos = make(map[string]*Empenho)
	c1.Empenhos[e11.Numero] = &e11
	
	fmt.Println(c1.Numero, (*e11.Contrato).Numero)
	
	c1.Numero = "002/PAMASP"
	
	fmt.Println(c1.Numero, (*e11.Contrato).Numero)
	
	e11.Numero="NE12344"
	
	fmt.Println(c1.Empenhos["NE800321"].Numero)
	
}