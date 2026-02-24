package main

import (
	"log"

	"wallet-saas-v2/services/api-gateway/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
