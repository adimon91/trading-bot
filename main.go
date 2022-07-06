package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type configuration struct {
	alpacaApiKey    string
	alpacaApiSecret string
	yahooApiKey     string
}

type handler struct {
	config configuration
	client alpaca.Client
}

func newConfig() (configuration, error) {
	log.Println("Executing newConfig()")
	alpacaKey, ok := os.LookupEnv("ALPACA_API_KEY")
	if !ok {
		return configuration{}, errors.New("can not read env variable")
	}

	alpacaSecret, ok := os.LookupEnv(("ALPACA_API_SECRET"))
	if !ok {
		return configuration{}, errors.New("can not read env variable")
	}

	yahooKey, ok := os.LookupEnv("YAHOO_API_KEY")
	if !ok {
		return configuration{}, errors.New("can not read env variable")
	}

	return configuration{
		alpacaApiKey:    alpacaKey,
		alpacaApiSecret: alpacaSecret,
		yahooApiKey:     yahooKey,
	}, nil
}

func (h *handler) newClient() alpaca.Client {
	log.Println("Executing newClient()")
	baseURL := "https://paper-api.alpaca.markets"

	client := alpaca.NewClient(alpaca.ClientOpts{
		ApiKey:    h.config.alpacaApiKey,
		ApiSecret: h.config.alpacaApiSecret,
		BaseURL:   baseURL,
	})

	return client
}

func (h *handler) getHistoricalData(ticker string, interval string, length int32) {
	log.Printf("Getting historical data with length %d, ticker %s, and interval %s", length, ticker, interval)
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, "https://yfapi.net/v8/finance/spark?", nil)
	if err != nil {
		log.Fatalf("Error creating http request %v\n", err)
	}

	// appending to existing query args
	q := req.URL.Query()
	q.Add("interval", interval)
	q.Add("range", "1d")
	q.Add("symbols", ticker)

	// assign encoded query string to http request
	req.URL.RawQuery = q.Encode()

	// add headers
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-API-KEY", h.config.yahooApiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending request to yahoo finance %v\n", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Fatalf("Error reading body %v\n", err)
	}

	log.Println(string(body))

	// TODO: parse and return array with `length` points of historical data
}

func (h *handler) calculateRSI(ticker string, interval string, length int32) {
	log.Printf("Calculating RSI with length %d, ticker %s, and interval %s", length, ticker, interval)

	// TODO: accept return of array of data points
	h.getHistoricalData(ticker, interval, length)

	// TODO: calculate RSI with data points
}

func (h *handler) trade(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Executing trade()")
	log.Printf("Request body was: %+v\n", req.Body)

	// retrieve alpaca account information
	acct, err := h.client.GetAccount()
	if err != nil {
		log.Fatalf("Failed to get the account %v", err)
	}

	if req.HTTPMethod == "GET" {
		body := fmt.Sprintf("Request successful with api gateway response. Account info: %+v\n", *acct)
		response := events.APIGatewayProxyResponse{Body: body, StatusCode: 200}
		return response, nil
	}

	type Trade struct {
		Ticker   string `json:"ticker"`
		Interval string `json:"interval"`
	}

	b := []byte(req.Body)
	var resp Trade
	err = json.Unmarshal(b, &resp)

	if err != nil {
		log.Fatalf("Error unmarshalling json %v\n", err)
	}

	h.calculateRSI(resp.Ticker, resp.Interval, 14)

	body := fmt.Sprintf("Request body was: %+v\n", req.Body)
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
