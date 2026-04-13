package api

import (
	"encoding/json"
	"net/http"

	"defi-pnl/internal/pnl"
	"defi-pnl/internal/storage"
)

func GetPnL(w http.ResponseWriter, r *http.Request) {

	address := r.URL.Query().Get("address")

	txs, err := storage.GetTransactions(address)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	result := pnl.Calculate(txs, 1800)

	json.NewEncoder(w).Encode(result)
}
