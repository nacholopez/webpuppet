package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("")
var logLevelEnv = os.Getenv("LOG_LEVEL")
var logLevel = loggo.INFO

func timeoutRequest(w http.ResponseWriter, r *http.Request) {
	var err error
	var seconds int
	vars := mux.Vars(r)
	seconds, err = strconv.Atoi(vars["seconds"])

	if err != nil {
		logger.Debugf("Failure reading timeout seconds")
		http.Error(w, "Invalid seconds value", http.StatusInternalServerError)
		return
	}

	time.Sleep(time.Duration(seconds) * time.Second)
	fmt.Fprintf(w, "Slept for %d seconds", seconds)
}

func healthRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func main() {
	var err error
	var valid bool
	var serverPortEnv = os.Getenv("PORT")
	var serverPort = 8080

	if len(logLevelEnv) > 0 {
		logLevel, valid = loggo.ParseLevel(logLevelEnv)
		if !valid {
			panic("Invalid log level")
		}
	}
	logger.SetLogLevel(logLevel)

	if len(serverPortEnv) > 0 {
		serverPort, err = strconv.Atoi(serverPortEnv)
		if err != nil {
			panic(err)
		}
	}

	logger.Debugf("Service starting ...")
	var r = mux.NewRouter()
	r.HandleFunc("/timeout/{seconds}", timeoutRequest).Methods("GET")
	r.HandleFunc("/health", healthRequest).Methods("GET")

	port := serverPort
	srv := &http.Server{
		Handler:     r,
		Addr:        ":" + strconv.Itoa(port),
		ReadTimeout: 10 * time.Second,
	}

	logger.Infof("Service ready. Listening on port %d", port)
	logger.Criticalf(srv.ListenAndServe().Error())
}
