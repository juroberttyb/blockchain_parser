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

func (p *EthereumParser) FetchTransactions() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	latestBlock, err := callEthereumRPC("eth_blockNumber", []interface{}{})
	if err != nil {
		return err
	} else if latestBlock["result"] == nil {
		return errors.New("error getting latest block number")
	}
	latestBlockHex := latestBlock["result"].(string)

	var latestBlockNumber int
	fmt.Sscanf(latestBlockHex, "0x%x", &latestBlockNumber)
	println("latestBlockNumber: ", latestBlockNumber)

	for p.currentBlock < latestBlockNumber {
		println("p.currentBlock", p.currentBlock)
		block, err := callEthereumRPC("eth_getBlockByNumber", []interface{}{fmt.Sprintf("0x%x", p.currentBlock+1), true})
		if err != nil {
			return err
		}

		result := block["result"]
		transactions := result.(map[string]interface{})["transactions"].([]interface{})
		println("processing transactions with length:", len(transactions))
		for _, tx := range transactions {
			txMap := tx.(map[string]interface{})
			from := txMap["from"].(string)
			to := txMap["to"].(string)
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
				println("transaction appended with matching from address: ", from)
				p.transactions[from] = append(p.transactions[from], transaction)
			}

			if p.subscriptions[to] {
				println("transaction appended with matching to address: ", to)
				p.transactions[to] = append(p.transactions[to], transaction)
			}
		}
		p.currentBlock++
	}
	println(fmt.Sprintf("transactions has len %d with value %+v", len(p.transactions), p.transactions))

	p.currentBlock = latestBlockNumber
	println("current block number is updated to:", p.currentBlock)
	return nil
}

func main() {
	parser := NewEthereumParser()
	if err := parser.UpdateCurrentBlock(); err != nil {
		println("error initialze to latest block number: ", err.Error())
	}

	go func() {
		for {
			if err := parser.FetchTransactions(); err != nil {
				println("error fetching new transactions: ", err.Error())
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
			println("error decoding subscribe request body: ", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		println("subscribe address:", data.Address)
		parser.Subscribe(data.Address)
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
		transactions := parser.GetTransactions(data.Address)
		if err := json.NewEncoder(w).Encode(transactions); err != nil {
			println("error writing transactions response: ", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
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
