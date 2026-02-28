package main

import (
	"log"

	"wallet-saas-v2/services/wallet-core/internal/bootstrap"
)

func main() {
	if err := bootstrap.Run(); err != nil {
		log.Fatal(err)
	}
}
