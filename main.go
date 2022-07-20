package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"

	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/buger/jsonparser"
	"github.com/shopspring/decimal"
)

const (
	// TODO: calculate quantity of stock purchased dynamically, only want to spend a certain percentage of my equity on each buy
	QUANTITY = 1
)

type configuration struct {
	alpacaApiKey    string
	alpacaApiSecret string
	yahooApiKey     string
}

type handler struct {
	config       configuration
	alpacaClient alpaca.Client
	httpClient   *http.Client
}

type Trade struct {
	Ticker   string `json:"ticker"`
	Interval string `json:"interval"`
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

func (h *handler) newAlpacaClient() alpaca.Client {
	log.Println("Executing newAlpacaClient()")
	baseURL := "https://paper-api.alpaca.markets"

	alpacaClient := alpaca.NewClient(alpaca.ClientOpts{
		ApiKey:    h.config.alpacaApiKey,
		ApiSecret: h.config.alpacaApiSecret,
		BaseURL:   baseURL,
	})

	return alpacaClient
}

func (h *handler) getHistoricalData(ticker string, interval string, length int) []float64 {
	log.Printf("Getting historical data with length %d, ticker %s, and interval %s", length, ticker, interval)
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

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Fatalf("Error sending request to yahoo finance %v\n", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Fatalf("Error reading body %v\n", err)
	}

	// TODO: handle ticker "not found" response (gives a 200 ok for some reason)
	// TODO: handle if there is less than 40 and/or 14 data points (increase range to 5d in yahoo call?)
	log.Println(string(body), resp.Status)

	// parse JSON
	data := []byte(body)

	values := make([]float64, 0)

	jsonparser.ArrayEach(data, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		val, err := jsonparser.GetFloat(value)
		if err != nil {
			log.Fatalf("Failed to parse JSON %v\n", err)
		}
		values = append(values, val)

	}, ticker, "close")

	return values
}

func (h *handler) calculateRSI(ticker string, interval string, length int) float64 {
	log.Printf("Calculating RSI with length %d, ticker %s, and interval %s", length, ticker, interval)

	// get stock price
	values := h.getHistoricalData(ticker, interval, length)
	log.Println(values)

	m := len(values) - length

	// set lastRed to first val in case there are no red candles
	// lastRed := values[0]
	posSum := 0.00
	negSum := 0.00

	for i := m; i < len(values)-1; i++ {
		change := values[i+1] - values[i]
		if change >= 0 {
			posSum += change
		} else {
			negSum += math.Abs(change)
		}
	}

	posSum /= float64(length)
	negSum /= float64(length)

	rs := posSum / negSum

	// RSI = 100 - 100/(1 + rs)
	rsi := 100 - 100/(1+rs)

	fmt.Printf("RSI length %v was %v\n", length, rsi)

	// TODO: verify RSI is correct
	return rsi
}

func (h *handler) calculateLastRed(ticker string, interval string) float64 {
	// this returns close of last red candle
	log.Printf("Getting value of last red close with ticker %s, and interval %s", ticker, interval)

	url := fmt.Sprintf("https://yfapi.net/v8/finance/chart/%s?", ticker)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatalf("Error creating http request %v\n", err)
	}

	// appending to existing query args
	q := req.URL.Query()
	q.Add("interval", interval)
	q.Add("range", "1d")

	// assign encoded query string to http request
	req.URL.RawQuery = q.Encode()

	// add headers
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-API-KEY", h.config.yahooApiKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Fatalf("Error sending request to yahoo finance %v\n", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Fatalf("Error reading body %v\n", err)
	}

	// TODO: handle ticker "not found" response (gives a 200 ok for some reason)
	log.Println(string(body), resp.Status)

	// parse JSON
	data := []byte(body)

	openArr := make([]float64, 0)
	closeArr := make([]float64, 0)

	jsonparser.ArrayEach(data, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {

		jsonparser.ArrayEach(value, func(val []byte, dataType jsonparser.ValueType, offset int, err error) {
			jsonparser.ArrayEach(val, func(v []byte, dataType jsonparser.ValueType, offset int, err error) {
				open, err := jsonparser.GetFloat(v)
				if err != nil {
					fmt.Println(err)
				}
				openArr = append(openArr, open)

			}, "open")

			jsonparser.ArrayEach(val, func(v []byte, dataType jsonparser.ValueType, offset int, err error) {
				close, err := jsonparser.GetFloat(v)
				if err != nil {
					fmt.Println(err)
				}
				closeArr = append(closeArr, close)

			}, "close")

		}, "indicators", "quote")

	}, "chart", "result")

	// fmt.Println(openArr)
	// fmt.Println(closeArr)

	lastRed := closeArr[0]

	for i := 0; i < len(openArr); i++ {
		if closeArr[i] < openArr[i] {
			// candle is red
			lastRed = closeArr[i]
		}
	}

	return lastRed

}

func (h *handler) trade(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Executing trade()")
	log.Printf("Request body was: %+v\n", req.Body)

	// retrieve alpaca account information
	acct, err := h.alpacaClient.GetAccount()
	if err != nil {
		log.Fatalf("Failed to get the account %v", err)
	}

	if req.HTTPMethod == "GET" {
		body := fmt.Sprintf("Request successful with api gateway response. Account info: %+v\n", *acct)
		response := events.APIGatewayProxyResponse{Body: body, StatusCode: 200}
		return response, nil
	}

	b := []byte(req.Body)
	var resp Trade
	err = json.Unmarshal(b, &resp)

	if err != nil {
		log.Fatalf("Error unmarshalling json %v\n", err)
	}

	rsi14 := h.calculateRSI(resp.Ticker, resp.Interval, 14)
	rsi40 := h.calculateRSI(resp.Ticker, resp.Interval, 40)

	// make a trade if rsi 40 is below 50 and rsi 14 is below 37.5
	// TODO: would be good to wait for 1-2 green candles before making a trade
	// (possible way to do this is to subscribe to alpaca data market alerts?) or,
	// it might be an issue to gather data from 2 different sources (yahoo and alpaca)
	// could to it manually by sending request for close on the 5 min mark
	// TODO: is there a way to check if rsi is declining?
	// TODO: uncomment below if statement when I find more stocks that are good buy options
	// if rsi40 < 50 && rsi14 < 37.5 {
	// make trade
	adjSide := alpaca.Side("buy")
	decimalQty := decimal.NewFromInt(int64(QUANTITY))
	order, err := h.alpacaClient.PlaceOrder(alpaca.PlaceOrderRequest{
		AccountID:   acct.ID,
		AssetKey:    &resp.Ticker,
		Qty:         &decimalQty,
		Side:        adjSide,
		Type:        "market",
		TimeInForce: "day",
	})

	if err == nil {
		log.Printf("Market order of | %d %s buy %s| sent.\n", QUANTITY, resp.Ticker, order.ID)
		// TODO: if there was no error on buy, need to set a stop loss and limit sell
		// stop loss should be set just below last red candle
		lastRed := h.calculateLastRed(resp.Ticker, resp.Interval)
		log.Printf("Last red candle close was: %f\n", lastRed)

		// TODO: set stop loss at lastRed
	}

	// }

	body := fmt.Sprintf("Request body was: %+v\n, rsi 14 is %v\n, rsi 40 is %v\n", req.Body, rsi14, rsi40)
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

	alpacaClient := h.newAlpacaClient()
	h.alpacaClient = alpacaClient

	httpClient := &http.Client{}
	h.httpClient = httpClient

	lambda.Start(h.trade)

}
