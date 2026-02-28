package main

import (
	"log"

	"wallet-saas-v2/services/api-gateway/internal/bootstrap"
)

func main() {
	if err := bootstrap.Run(); err != nil {
		log.Fatal(err)
	}
}
