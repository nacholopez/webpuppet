package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
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

func initRandom() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

const (
	maxSleepSeconds  = 3600
	maxSleepMs       = maxSleepSeconds * 1000
	maxMessageLength = 1 << 20
	maxBodyBytes     = 1 << 20
	shutdownTimeout  = 60 * time.Second
)

// sleep is a variable so tests can avoid real delays.
var sleep = time.Sleep

// RandStringRunes RandStringRunes
func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// RandInteger returns a random integer in the inclusive range [lo, hi].
// It fails if lo is negative or greater than hi.
func RandInteger(lo int, hi int) (int, error) {
	if lo < 0 || lo > hi {
		return -1, errors.New("invalid range")
	}
	return rand.Intn(hi-lo+1) + lo, nil
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := ioutil.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		logger.Errorf("error reading body: %v", err)
		http.Error(w, "error reading body", http.StatusBadRequest)
		return nil, false
	}
	return body, true
}

func sleepMsMessageLength(w http.ResponseWriter, r *http.Request) {
	var err error
	var maxMs, minMs, maxML, minML int
	var randomSleepMs, randomMessageLength int
	var randomMessage string

	vars := mux.Vars(r)

	maxMs, errMax := strconv.Atoi(vars["maxms"])
	minMs, errMin := strconv.Atoi(vars["minms"])
	randomSleepMs, err = RandInteger(minMs, maxMs)
	if errMax != nil || errMin != nil || err != nil || maxMs > maxSleepMs {
		logger.Debugf("Invalid timeout ms values")
		http.Error(w, "Invalid sleepms input", http.StatusBadRequest)
		return
	}

	maxML, errMax = strconv.Atoi(vars["maxML"])
	minML, errMin = strconv.Atoi(vars["minML"])
	randomMessageLength, err = RandInteger(minML, maxML)
	if errMax != nil || errMin != nil || err != nil || maxML > maxMessageLength {
		logger.Debugf("Invalid message length values")
		http.Error(w, "Invalid messagelength input", http.StatusBadRequest)
		return
	}

	randomMessage = RandStringRunes(randomMessageLength)

	logger.Debugf("Sleeping for %d ms", randomSleepMs)
	sleep(time.Duration(randomSleepMs) * time.Millisecond)
	w.Header().Add("Content-Type", "application/json")
	if randomMessageLength > 0 {
		logger.Debugf("Replying with a random message length of %d", randomMessageLength)
		fmt.Fprintf(w, "{ \"msg\": \"%s\" }\n", randomMessage)
	} else {
		fmt.Fprintf(w, "{ \"msg\": \"Slept for %d milliseconds\" }\n", randomSleepMs)
	}
	logger.Debugf("Slept for %d ms", randomSleepMs)
}

func sleepRequest(w http.ResponseWriter, r *http.Request) {
	var err error
	var seconds int
	var traceID string
	vars := mux.Vars(r)
	seconds, err = strconv.Atoi(vars["seconds"])
	traceID = r.Header.Get("TraceID")

	if err != nil || seconds < 0 || seconds > maxSleepSeconds {
		logger.Debugf("Invalid timeout seconds")
		http.Error(w, "Invalid seconds value", http.StatusBadRequest)
		return
	}

	logger.Debugf("[%s] Sleeping for %d", traceID, seconds)
	sleep(time.Duration(seconds) * time.Second)
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, "{ \"msg\": \"Slept for %d seconds\" }\n", seconds)
	logger.Debugf("[%s] Slept for %d", traceID, seconds)
}

func healthRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func printToStderr(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	os.Stderr.WriteString(string(body) + "\n")
}

func printToStdout(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	os.Stdout.WriteString(string(body) + "\n")
}

func mirror(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	// Reflected input must never be sniffed as HTML.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	fmt.Fprintf(w, "HEADERS\n")
	fmt.Fprintf(w, "=======\n")
	for k, v := range r.Header {
		fmt.Fprintf(w, "%s: %s\n", k, strings.Join(v, ", "))
	}
	if len(body) > 0 {
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "BODY\n")
		fmt.Fprintf(w, "====\n")
		fmt.Fprint(w, string(body)+"\n")
	}
}

func httpResponseCode(w http.ResponseWriter, r *http.Request) {
	var err error
	var code int
	vars := mux.Vars(r)
	code, err = strconv.Atoi(vars["code"])

	w.Header().Set("Content-Type", "application/json")

	if err != nil || code < 100 || code > 599 {
		logger.Debugf("Invalid code")
		// http.Error would override the Content-Type with text/plain.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "{ \"error\": \"invalid code\" }\n")
		return
	}
	w.WriteHeader(code)
	fmt.Fprintf(w, "{ \"code\": \"%d\" }\n", code)
}

func newRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/sleep/{seconds}", sleepRequest).Methods("GET", "POST")
	r.HandleFunc("/health", healthRequest).Methods("GET")
	r.HandleFunc("/print/stderr", printToStderr).Methods("POST")
	r.HandleFunc("/print/stdout", printToStdout).Methods("POST")
	r.HandleFunc("/mirror", mirror).Methods("GET", "POST")
	r.HandleFunc("/httpResponseCode/{code}", httpResponseCode).Methods("GET", "POST")
	r.HandleFunc("/sleepms/max/{maxms}/min/{minms}/messagelength/max/{maxML}/min/{minML}", sleepMsMessageLength).Methods("GET", "POST")
	return r
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

	initRandom()

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

	r := newRouter()

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

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Warningf("Stopping service failed: %+v", err)
	}
	logger.Infof("Service stopped.")
}
