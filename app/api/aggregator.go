package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// entry point
func main() {
	go func() {
		http.HandleFunc("/", handleQuoteRequest)
		log.Println(http.ListenAndServe(":8080", nil))
	}()

	go func() {
		http.HandleFunc("/ytd", ytdService)
		log.Println(http.ListenAndServe(":8081", nil))
	}()

	go func() {
		http.HandleFunc("/fin", finService)
		log.Println(http.ListenAndServe(":8082", nil))
	}()

	go func() {
		http.HandleFunc("/news", newsService)
		log.Fatal(http.ListenAndServe(":8083", nil))
	}()

	go func() {
		http.HandleFunc("/desc", descriptionService)
		log.Println(http.ListenAndServe(":8084", nil))
	}()

	// Keep the main goroutine running to prevent the program from exiting
	select {}
}

// handles the ticker request
func handleQuoteRequest(w http.ResponseWriter, r *http.Request) {

	//to represent the aggregate of all financial information
	type QueriedInfoAggregate struct {
		Ticker   string
		YtdInfo  string
		FinInfo  string
		NewsInfo string
		DescInfo string
	}

	//to represent the aggregate of all prompt inferences
	type PromptInference struct {
		StockPerformance string
		FinancialHealth  string
		NewsSummary      string
		CompanyDesc      string
	}

	// to represent data posted to the data ingestor
	type PostDataInfo struct {
		Ticker           string
		StockPerformance string
		FinancialHealth  string
		NewsSummary      string
		CompanyDesc      string
	}

	// aggregate of all event sequences
	type AGGLOG struct {
		Timestamp       time.Time
		ExecutionTimeMs float32
		RequestIP       string
		EventSequence   []string
	}

	//event aggregation object
	var aggLog AGGLOG

	//Create a new instance of event logging
	var eventSequenceArray []string

	// Create a new instance of PromptInference
	var promptInference PromptInference

	// Create a new instance of QueriedInfoAggregate
	var queriedInfoAggregate QueriedInfoAggregate

	// Process query string parameters from the request URL
	startTime := time.Now()
	queryParams := r.URL.Query()
	ticker := queryParams.Get("ticker")
	if len(ticker) == 0 {
		log.Println("Missing required parameter 'ticker' in the query string")
		w.Write([]byte(http.StatusText(http.StatusBadRequest)))
		w.Write([]byte("Error: Bad Request(400), Missing required parameter 'ticker' in the query string."))
		eventSequenceArray = append(eventSequenceArray, "missing ticker \n")
		return
	}
	fmt.Println(ticker)

	// Get the financial information from the services
	// and return it as the response

	err := godotenv.Load("../../.env") // load the .env file
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		fmt.Fprintf(w, "Error parsing IP address: %v", err)
		return
	}
	eventSequenceArray = append(eventSequenceArray, "collected request ip \n")
	aggLog.RequestIP = ip

	YTD_SERVICE_URL := os.Getenv("YTD_SERVICE_URL")
	FIN_SERVICE_URL := os.Getenv("FIN_SERVICE_URL")
	NEWS_SERVICE_URL := os.Getenv("NEWS_SERVICE_URL")
	DESC_SERVICE_URL := os.Getenv("DESC_SERVICE_URL")
	YTD_TEMPLATE := os.Getenv("YTD_TEMPLATE")
	FIN_TEMPLATE := os.Getenv("FIN_TEMPLATE")
	NEWS_TEMPLATE := os.Getenv("NEWS_TEMPLATE")
	DESC_TEMPLATE := os.Getenv("DESC_TEMPLATE")
	PASS_KEY := os.Getenv("PASS_KEY")

	// connnect to mongodb
	MONGO_DB_LOGGER_PASSWORD := os.Getenv("MONGO_DB_LOGGER_PASSWORD")
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	mongoURI := "mongodb+srv://kobenaidun:" + MONGO_DB_LOGGER_PASSWORD + "@cluster0.z9znpv9.mongodb.net/?retryWrites=true&w=majority"
	opts := options.Client().ApplyURI(mongoURI).SetServerAPIOptions(serverAPI)
	// Create a new client and connect to the server
	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			eventSequenceArray = append(eventSequenceArray, "could not connect to database \n")
			panic(err)
		}
		eventSequenceArray = append(eventSequenceArray, "connected to the database \n")
	}()

	// create a 256 sha hash of pass key from env file
	hash := sha256.New()
	hash.Write([]byte(PASS_KEY))
	getPassHash := hash.Sum(nil)
	passHash := hex.EncodeToString(getPassHash)

	fmt.Print("passHash: " + passHash + "\n")

	queriedInfoAggregate.Ticker = ticker
	eventSequenceArray = append(eventSequenceArray, "queried ticker \n")

	ytd_info := getFinancialInfo(ticker, "/ytd", YTD_SERVICE_URL, passHash, eventSequenceArray)
	queriedInfoAggregate.YtdInfo = ytd_info
	eventSequenceArray = append(eventSequenceArray, "queried ytd info \n")

	fin_info := getFinancialInfo(ticker, "/fin", FIN_SERVICE_URL, passHash, eventSequenceArray)
	queriedInfoAggregate.FinInfo = fin_info
	eventSequenceArray = append(eventSequenceArray, "queried fin info \n")

	news_info := getFinancialInfo(ticker, "/news", NEWS_SERVICE_URL, passHash, eventSequenceArray)
	queriedInfoAggregate.NewsInfo = news_info
	eventSequenceArray = append(eventSequenceArray, "queried news info \n")

	desc_info := getFinancialInfo(ticker, "/desc", DESC_SERVICE_URL, passHash, eventSequenceArray)
	queriedInfoAggregate.DescInfo = desc_info
	eventSequenceArray = append(eventSequenceArray, "queried desc info \n")

	// stock perfomance
	ytdTemplate := YTD_TEMPLATE
	ytdInference := getPromptInference(string(queriedInfoAggregate.YtdInfo), ytdTemplate, "/llm", "http://127.0.0.1:5000", eventSequenceArray, passHash)
	ytdInference = strings.Trim(ytdInference, "{}")
	promptInference.StockPerformance = ytdInference
	eventSequenceArray = append(eventSequenceArray, "collected ytd prompt inference \n")

	// financial health
	finTemplate := FIN_TEMPLATE
	finInference := getPromptInference(string(queriedInfoAggregate.FinInfo), finTemplate, "/llm", "http://127.0.0.1:5000", eventSequenceArray, passHash)
	finInference = strings.Trim(finInference, "{}")
	promptInference.FinancialHealth = finInference
	eventSequenceArray = append(eventSequenceArray, "collected fin prompt inference \n")

	// news summary
	newsTemplate := NEWS_TEMPLATE
	newsInference := getPromptInference(string(queriedInfoAggregate.NewsInfo), newsTemplate, "/llm", "http://127.0.0.1:5000", eventSequenceArray, passHash)
	newsInference = strings.Trim(newsInference, "{}")
	promptInference.NewsSummary = newsInference
	eventSequenceArray = append(eventSequenceArray, "collected news prompt inference \n")

	// company description
	descTemplate := DESC_TEMPLATE
	descInference := getPromptInference(string(queriedInfoAggregate.DescInfo), descTemplate, "/llm", "http://127.0.0.1:5000", eventSequenceArray, passHash)
	descInference = strings.Trim(descInference, "{}")
	promptInference.CompanyDesc = descInference
	eventSequenceArray = append(eventSequenceArray, "collected desc prompt inference \n")

	//constructs valid json string
	stockperformace := strings.Replace(promptInference.StockPerformance, "{", "|", -1)
	stockperformace = strings.Replace(stockperformace, "}", "|", -1)
	financialhealth := strings.Replace(promptInference.FinancialHealth, "{", "|", -1)
	financialhealth = strings.Replace(financialhealth, "}", "|", -1)
	newssummary := strings.Replace(promptInference.NewsSummary, "{", "|", -1)
	newssummary = strings.Replace(newssummary, "}", "|", -1)
	companydesc := strings.Replace(promptInference.CompanyDesc, "{", "|", -1)
	companydesc = strings.Replace(companydesc, "}", "|", -1)

	postDataInfo := PostDataInfo{
		Ticker:           ticker,
		StockPerformance: stockperformace,
		FinancialHealth:  financialhealth,
		NewsSummary:      newssummary,
		CompanyDesc:      companydesc,
	}

	// Marshal the struct into JSON
	postJsonData, err := json.Marshal(postDataInfo)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	// Posts the whole financial data blob to the data ingestor
	resPostFinancialData := postFinancialData(string(postJsonData), eventSequenceArray, passHash)
	if resPostFinancialData != "200 Status OK" {
		panic(err)
	}
	if err != nil {
		panic(err)
	}

	// Return the PromptInference json object as the response
	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)
	aggLog.ExecutionTimeMs = float32(elapsedTime.Milliseconds())
	w.Header().Set("Content-Type", "application/json")
	promptInferenceJson, err := json.Marshal(promptInference)
	if err != nil {
		log.Println(err)
	}
	w.Write([]byte(promptInferenceJson))
	aggLog.Timestamp = time.Now()
	eventSequenceArray = append(eventSequenceArray, "sent prompt inference response \n")
	aggLog.EventSequence = eventSequenceArray
	db := client.Database("MicroserviceLogs")
	collection := db.Collection("AggregatorServiceLogs")
	_, err = collection.InsertOne(context.TODO(), aggLog)
	if err != nil {
		log.Fatal(err)
	}

}

// gets the financial information from the polygon.io services
func getFinancialInfo(ticker string, handlerID string, handlerURL string, passHash string, eventSequenceArray []string) string {

	// Create a new HTTP client
	client := &http.Client{}

	// Construct the URL with query parameters
	base_url := handlerURL + handlerID
	url := base_url + "?" + "ticker=" + ticker

	// Create a GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	// Set the Authorization header with the Bearer token
	req.Header.Set("Authorization", "Bearer "+passHash)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Read the response body as a string
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return string(responseBody)

}

// gets the prompt inference from the LLM service
func getPromptInference(prompt string, template string, handlerID string, handlerURL string, eventSequenceArray []string, passHash string) string {

	//Create an HTTP client
	client := &http.Client{}

	baseUrl := handlerURL + handlerID

	url := baseUrl + "?" + "prompt=" + urlConverter(template+prompt)

	// Create a GET request

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	// Set the Authorization header with the Bearer token
	req.Header.Set("Authorization", "Bearer "+passHash)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Read the response body as a string
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return string(responseBody)
}

// Posts financial data to data ingestor service
func postFinancialData(dataValue string, eventSequenceArray []string, passHash string) string {

	url := "http://127.0.0.1:6001/ingestor"
	bearerToken := passHash
	infoData := dataValue

	// Create payload as bytes
	payload := []byte(fmt.Sprintf("info=%s", infoData))

	// Create HTTP client
	client := &http.Client{}

	// Create POST request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		fmt.Println("Error creating request:", err)

	}

	// Set Authorization header
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)

	}

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)

	}
	defer resp.Body.Close()

	fmt.Println("Response:", string(respBody))
	return string(respBody)
}

// converts prompt to a URL compatible format
func urlConverter(_url string) string {

	// Construct the URL with query parameters
	encodedPrompt := url.QueryEscape(_url)
	encodedPrompt = strings.ReplaceAll(encodedPrompt, "+", "%20")
	encodedPrompt = strings.ReplaceAll(encodedPrompt, "%2F", "/")
	encodedPrompt = strings.ReplaceAll(encodedPrompt, "%3A", ":")

	return encodedPrompt
}
