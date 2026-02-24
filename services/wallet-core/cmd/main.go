package main

import (
	"log"

	"wallet-saas-v2/services/wallet-core/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
