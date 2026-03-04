package main

import (
	"log"

	"wallet-saas-v2/services/scan-account-service/internal/bootstrap"
)

func main() {
	if err := bootstrap.RunAccount(); err != nil {
		log.Fatal(err)
	}
}
