package main

import (
	"log"

	"wallet-saas-v2/services/sign-service/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
