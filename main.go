package main

import (
	"encoding/json"
	"net/http"
)

type Transaction struct {
	Hash  string
	From  string
	To    string
	Value string
	Block int
}

type Parser interface {
	GetCurrentBlock() int
	Subscribe(address string) bool
	GetTransactions(address string) []Transaction
}

type EthereumParser struct {
	currentBlock  int
	subscriptions map[string]bool
	transactions  map[string][]Transaction
}

func NewEthereumParser() *EthereumParser {
	return &EthereumParser{
		subscriptions: make(map[string]bool),
		transactions:  make(map[string][]Transaction),
	}
}

func (p *EthereumParser) GetCurrentBlock() int {
	return p.currentBlock
}

func callEthereumRPC(method string, params []interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (p *EthereumParser) UpdateCurrentBlock() error {
	_, err := callEthereumRPC("eth_blockNumber", []interface{}{})
	if err != nil {
		return err
	}

	return nil
}

func main() {
	parser := NewEthereumParser()
	if err := parser.UpdateCurrentBlock(); err != nil {
		println("error initialze to latest block number: ", err.Error())
	}

	http.HandleFunc("/subscribe", func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			Address string `json:"address"`
		}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			println("error decoding subscribe request body: ", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/transactions", func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			Address string `json:"address"`
		}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			println("error decoding subscribe request body: ", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/currentBlock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
