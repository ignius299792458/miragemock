package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/ignius299792458/miragemock"
)

// this the env = "prod"
func main() {

	router := http.NewServeMux()

	// 1. Health check endpoint
	router.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy"}`))
	})

	// 2. Echo endpoint to test dynamic route parameters
	router.HandleFunc("GET /v1/echo/{message}", func(w http.ResponseWriter, r *http.Request) {
		message := r.PathValue("message")
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Echo: " + message))
	})

	// 3. Post data endpoint to test request body and custom header handling
	router.HandleFunc("POST /v1/submit", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Echo back the submitted body along with a correlation flag
		w.Header().Set("X-MirageMock-Validated", "true")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write(body)
	})

	// 4. Heavy payload simulation endpoint
	router.HandleFunc("GET /v1/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items": ["item1", "item2", "item3"], "count": 3}`))
	})

	mirageMockerConfig := miragemock.Config{
		MaxWorkers:           20,
		QueueCap:             100,
		TargetClient:         "http://localhost:8081",
		ReWriter:             nil,
		KeysValueToReWritten: []string{"Authorization", "X-Forward-Value"},
	}
	mirageMockerProxy := miragemock.NewProxy(mirageMockerConfig)
	wrapMirageMockerToRouter := mirageMockerProxy.AsMiddleware(router)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      LoggingMiddleware(wrapMirageMockerToRouter), // Wrap the router with your middleware stack
		ReadTimeout:  5 * time.Second,                             // Prevents slowloris denial-of-service attacks
		WriteTimeout: 10 * time.Second,                            // Drops connection if client cannot ingest fast enough
		IdleTimeout:  120 * time.Second,                           // Recycles keep-alive connections aggressively
	}

	log.Printf("Starting prod server src: %s", srv.Addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		log.Printf("%s %s took %s", r.Method, r.URL.Path, time.Since(start))

	})
}
