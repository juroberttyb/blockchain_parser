package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const UPDATE_BLOCK__SEONCD = 3
const RETRY_SECOND = 1

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
	println(" INFO parser block number:", p.currentBlock)
	return nil
}

func (p *EthereumParser) Subscribe(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	println("DEBUG subscribing to address: ", address)
	p.subscriptions[address] = true
	println(fmt.Sprintf(" INFO subscriptions after subscribe: %+v", p.subscriptions))
}

// FIXME: add implementation
func (p *EthereumParser) GetTransactions(address string) []Transaction {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.transactions[address]
}

func (p *EthereumParser) FetchTransactions() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	latestBlock, err := callEthereumRPC("eth_blockNumber", []interface{}{})
	if err != nil {
		return err
	} else if latestBlock["result"] == nil {
		return errors.New("ERROR getting latest block number")
	}
	latestBlockHex := latestBlock["result"].(string)

	var latestBlockNumber int
	fmt.Sscanf(latestBlockHex, "0x%x", &latestBlockNumber)
	// both latestBlockNumber and p.currentBlock are printed to ensure currentBlock is synced
	println(" INFO latest block number: ", latestBlockNumber)

	for p.currentBlock < latestBlockNumber {
		block, err := callEthereumRPC("eth_getBlockByNumber", []interface{}{fmt.Sprintf("0x%x", p.currentBlock+1), true})
		if err != nil {
			return err
		}

		result := block["result"]
		if result == nil {
			return errors.New("ERROR calling eth_getBlockByNumber: empty result in response")
		}
		transactions := result.(map[string]interface{})["transactions"].([]interface{})
		for _, tx := range transactions {
			txMap := tx.(map[string]interface{})
			from := txMap["from"].(string)

			// transaction to feild might be empty (contract deployment)
			var to string
			if txMap["to"] != nil {
				to = txMap["to"].(string)
			}
			value := txMap["value"].(string)
			hash := txMap["hash"].(string)

			transaction := Transaction{
				Hash:  hash,
				From:  from,
				To:    to,
				Value: value,
				Block: p.currentBlock + 1,
			}

			if p.subscriptions[from] {
				println("DEBUG transaction appended with matching from address: ", from)
				p.transactions[from] = append(p.transactions[from], transaction)
			}

			// from != to avoid appending duplicated transaction, for example sending token from caller to caller
			if p.subscriptions[to] && from != to {
				println("DEBUG transaction appended with matching to address: ", to)
				p.transactions[to] = append(p.transactions[to], transaction)
			}
		}
		p.currentBlock++
	}
	println(fmt.Sprintf("DEBUG transactions: %+v", p.transactions))

	p.currentBlock = latestBlockNumber
	return nil
}

func main() {
	parser := NewEthereumParser()
	if err := parser.UpdateCurrentBlock(); err != nil {
		println("ERROR initialze to latest block number: ", err.Error())
	}

	go func() {
		for {
			if err := parser.FetchTransactions(); err != nil {
				println("ERROR fetching new transactions: ", err.Error())
				time.Sleep(time.Second * RETRY_SECOND)
				continue
			}
			time.Sleep(time.Second * UPDATE_BLOCK__SEONCD)
		}
	}()

	http.HandleFunc("/subscribe", func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			Address string `json:"address"`
		}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			println("ERROR decoding subscribe request body: ", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		parser.Subscribe(data.Address)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/transactions", func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			Address string `json:"address"`
		}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			println("ERROR decoding subscribe request body: ", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		transactions := parser.GetTransactions(data.Address)
		if err := json.NewEncoder(w).Encode(transactions); err != nil {
			println("ERROR writing transactions response: ", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/currentBlock", func(w http.ResponseWriter, r *http.Request) {
		block := parser.GetCurrentBlock()
		if err := json.NewEncoder(w).Encode(map[string]int{"currentBlock": block}); err != nil {
			println("ERROR writing current block response: ", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
