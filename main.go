package main

import (
	"encoding/json"
	"net/http"
)

func main() {
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
