package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
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
	mu            sync.Mutex
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
	requestBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	})

	resp, err := http.Post("https://cloudflare-eth.com", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	return result, nil
}

func (p *EthereumParser) UpdateCurrentBlock() error {
	result, err := callEthereumRPC("eth_blockNumber", []interface{}{})
	if err != nil {
		return err
	}

	blockNumber := result["result"].(string)
	fmt.Sscanf(blockNumber, "0x%x", &p.currentBlock)
	println("block number updated to: ", p.currentBlock)
	return nil
}

func (p *EthereumParser) Subscribe(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	println("subscribing to address: ", address)
	p.subscriptions[address] = true
	println(fmt.Sprintf("subscriptions after subscribe: %+v", p.subscriptions))
}

// FIXME: add implementation
func (p *EthereumParser) GetTransactions(address string) []Transaction {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.transactions[address]
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
		block := parser.GetCurrentBlock()
		if err := json.NewEncoder(w).Encode(map[string]int{"currentBlock": block}); err != nil {
			println("error writing current block response: ", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
