package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("")
var logLevelEnv = os.Getenv("LOG_LEVEL")
var logLevel = loggo.INFO

func sleepRequest(w http.ResponseWriter, r *http.Request) {
	var err error
	var seconds int
	var traceID string
	vars := mux.Vars(r)
	seconds, err = strconv.Atoi(vars["seconds"])
	traceID = r.Header.Get("TraceID")

	if err != nil {
		logger.Debugf("Failure reading timeout seconds")
		http.Error(w, "Invalid seconds value", http.StatusInternalServerError)
		return
	}

	logger.Debugf("[%s] Sleeping for %d", traceID, seconds)
	time.Sleep(time.Duration(seconds) * time.Second)
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, "{ \"msg\": \"Slept for %d seconds\" }\n", seconds)
	logger.Debugf("[%s] Slept for %d", traceID, seconds)
}

func healthRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func printToStderr(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("error parsing body")
		http.Error(w, "error parsing body", http.StatusInternalServerError)
		return
	}
	os.Stderr.WriteString(string(body) + "\n")
}

func printToStdout(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("error parsing body")
		http.Error(w, "error parsing body", http.StatusInternalServerError)
		return
	}
	os.Stdout.WriteString(string(body) + "\n")
}

func mirror(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("error parsing body")
		http.Error(w, "error parsing body", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "HEADERS\n")
	fmt.Fprintf(w, "=======\n")
	for k, v := range r.Header {
		fmt.Fprintf(w, "%s: %s\n", k, strings.Join(v, ", "))
	}
	if len(string(body)) > 0 {
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "BODY\n")
		fmt.Fprintf(w, "====\n")
		fmt.Fprintf(w, string(body)+"\n")
	}
}

func httpResponseCode(w http.ResponseWriter, r *http.Request) {
	var err error
	var code int
	vars := mux.Vars(r)
	code, err = strconv.Atoi(vars["code"])

	w.Header().Add("Content-Type", "application/json")

	if err != nil || code < 100 || code > 599 {
		logger.Debugf("Invalid code")
		http.Error(w, "{ \"error\": \"invalid code\" }", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(code)
	fmt.Fprintf(w, "{ \"code\": \"%d\" }\n", code)
}

func main() {
	var err error
	var valid bool
	var serverPortEnv = os.Getenv("PORT")
	var bootWaitEnv = os.Getenv("BOOT_WAIT_SECS")
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

	if len(bootWaitEnv) > 0 {
		bootWait, err := strconv.Atoi(bootWaitEnv)
		if err != nil {
			panic(err)
		}
		logger.Infof("Boot waiting for %d secs", bootWait)
		time.Sleep(time.Duration(bootWait) * time.Second)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	var r = mux.NewRouter()
	r.HandleFunc("/sleep/{seconds}", sleepRequest).Methods("GET")
	r.HandleFunc("/health", healthRequest).Methods("GET")
	r.HandleFunc("/print/stderr", printToStderr).Methods("POST")
	r.HandleFunc("/print/stdout", printToStdout).Methods("POST")
	r.HandleFunc("/mirror", mirror).Methods("GET", "POST")
	r.HandleFunc("/httpResponseCode/{code}", httpResponseCode).Methods("GET", "POST")

	port := serverPort
	srv := &http.Server{
		Handler:     r,
		Addr:        ":" + strconv.Itoa(port),
		ReadTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
	logger.Infof("Service ready. Listening on port %d", port)

	<-done
	logger.Infof("Stopping service ...")

	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)

	if err := srv.Shutdown(ctx); err != nil {
		logger.Warningf("Stopping service failed: %+v", err)
	}
	logger.Infof("Service stopped.")
}
