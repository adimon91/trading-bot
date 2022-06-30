package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type configuration struct {
	apiKey    string
	apiSecret string
}

type handler struct {
	config configuration
	client alpaca.Client
}

func newConfig() (configuration, error) {
	log.Println("Executing newConfig()")
	key, ok := os.LookupEnv("ALPACA_API_KEY")
	if !ok {
		return configuration{}, errors.New("can not read env variable")
	}

	secret, ok := os.LookupEnv(("ALPACA_API_SECRET"))
	if !ok {
		return configuration{}, errors.New("can not read env variable")
	}

	return configuration{
		apiKey:    key,
		apiSecret: secret,
	}, nil
}

func (h *handler) newClient() alpaca.Client {
	log.Println("Executing newClient()")
	baseURL := "https://paper-api.alpaca.markets"

	client := alpaca.NewClient(alpaca.ClientOpts{
		ApiKey:    h.config.apiKey,
		ApiSecret: h.config.apiSecret,
		BaseURL:   baseURL,
	})

	return client
}

func (h *handler) trade(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Executing trade()")
	// retrieve alpaca account information
	acct, err := h.client.GetAccount()
	if err != nil {
		log.Fatalf("Failed to get the account %v", err)
	}

	fmt.Printf("%+v\n", *acct)
	body := fmt.Sprintf("Request successful with api gateway response. Account info: %+v\n", *acct)
	response := events.APIGatewayProxyResponse{Body: body, StatusCode: 200}
	return response, nil
}

func main() {
	log.Println("Executing main()")
	cfg, err := newConfig()

	if err != nil {
		log.Fatalf("Unable to create new config %v\n", err)
	}

	h := &handler{
		config: cfg,
	}

	client := h.newClient()
	h.client = client

	lambda.Start(h.trade)

}
