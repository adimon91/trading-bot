package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
)

type Secret struct {
	SecretKey string
}

func main() {
	var secret map[string]string
	var apiK string
	var apiS string

	log.Print("Retreiving secrets...")
	apiKey, err := getSecret()
	if err != nil {
		log.Fatalf("Failed to get secrets %v", err)
	}

	// unmarshal api key json into secret var
	err = json.Unmarshal([]byte(apiKey), &secret)

	if err != nil {
		log.Fatalf("could not unmarshal json: %v\n", err)
	}

	// get key and secret
	for k, v := range secret {
		apiK = k
		apiS = v
	}

	// create alpaca client
	client := alpaca.NewClient(alpaca.ClientOpts{
		ApiKey:    apiK,
		ApiSecret: apiS,
		BaseURL:   "https://paper-api.alpaca.markets",
	})

	// retrieve alpaca account information
	acct, err := client.GetAccount()
	if err != nil {
		log.Fatalf("Failed to get the account %v", err)
	}

	fmt.Printf("%+v\n", *acct)
}
