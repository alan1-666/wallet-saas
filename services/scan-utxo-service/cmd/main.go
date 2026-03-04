package main

import (
	"log"

	"wallet-saas-v2/services/scan-utxo-service/internal/bootstrap"
)

func main() {
	if err := bootstrap.RunUTXO(); err != nil {
		log.Fatal(err)
	}
}
