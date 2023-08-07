package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/models"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// handles the desc request
func descriptionService(w http.ResponseWriter, r *http.Request) {

	type DESCLOG struct {
		Timestamp       time.Time
		ExecutionTimeMs float32
		RequestIP       string
		EventSequence   []string
	}

	var descLog DESCLOG
	var eventSequenceArray []string

	//load information structures
	startTime := time.Now()
	queryParams := r.URL.Query()
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
	descLog.RequestIP = ip

	// secure service with pass key hash
	PASS_KEY := os.Getenv("PASS_KEY")
	hash := sha256.New()
	hash.Write([]byte(PASS_KEY))
	getPassHash := hash.Sum(nil)
	passHash := hex.EncodeToString(getPassHash)
	passHashFromRequest := queryParams.Get("passhash")
	if passHash != passHashFromRequest {
		log.Println("Incorrect passhash: unathorized request")
		w.Write([]byte(http.StatusText(http.StatusUnauthorized)))
		w.Write([]byte("Error: Unauthorized(401), Incorrect passhash."))
		eventSequenceArray = append(eventSequenceArray, "passhash unauthorized \n")
		return
	}
	eventSequenceArray = append(eventSequenceArray, "passhash checked \n")

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
		eventSequenceArray = append(eventSequenceArray, "connected to database \n")

	}()

	// polygon API connection
	API_KEY := os.Getenv("API_KEY")
	c := polygon.New(API_KEY)

	// ticker input checking
	ticker := queryParams.Get("ticker")
	if len(ticker) == 0 {
		log.Println("Missing required parameter 'ticker' in the query string")
		w.Write([]byte(http.StatusText(http.StatusBadRequest)))
		w.Write([]byte("Error: Bad Request(400), Missing required parameter 'ticker' in the query string."))
		eventSequenceArray = append(eventSequenceArray, "missing ticker \n")
		return
	}

	//log ticker
	eventSequenceArray = append(eventSequenceArray, "ticker collected \n")

	// set params
	params := models.GetTickerDetailsParams{
		Ticker: ticker,
	}.WithDate(models.Date(time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.UTC)))

	// make request
	res, err := c.GetTickerDetails(context.Background(), params)
	if err != nil {
		log.Println(err)
	}
	fmt.Println(res)
	eventSequenceArray = append(eventSequenceArray, "Collected response from polygon: \n")

	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)
	descLog.ExecutionTimeMs = float32(elapsedTime.Milliseconds())
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprint(res)))
	descLog.Timestamp = time.Now()

	// insert the log into the database
	eventSequenceArray = append(eventSequenceArray, "successfully served ytd data \n")
	descLog.EventSequence = eventSequenceArray
	db := client.Database("MicroserviceLogs")
	collection := db.Collection("DescriptionServiceLogs")
	_, err = collection.InsertOne(context.TODO(), descLog)
	if err != nil {
		log.Fatal(err)
	}

}
