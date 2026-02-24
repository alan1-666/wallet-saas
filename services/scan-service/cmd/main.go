package main

import (
	"log"

	"wallet-saas-v2/services/scan-service/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
