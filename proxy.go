package miragemock

import (
	"bytes"
	"io"
	"log"
	"net/http"
)

const (
	DefaultQueueCap   = 1000
	DefaultMaxWorkers = 10
)

// InFlightRequest encapsulates the minimal data required to re-route
// and rewrite a mirrored production request inside background workers.
type InFlightRequest struct {
	Method  string      // "GET", "POST", etc.
	UrlPath string      // e.g., "/api/v1/user?id=123"
	Headers http.Header // Cloned routing headers
	Body    []byte      // Will be nil for GET requests, populated for POST/PUT
}

type Proxy struct {
	config Config
	queue  chan InFlightRequest
}

// Config houses immutable parameters for the MirageMock lifecycle.
type Config struct {
	MaxWorkers           int
	QueueCap             int
	TargetClient         string
	ReWriter             ReWriter
	KeysValueToReWritten map[SanitizingKeyNameType][]string
}

// NewProxy instantiates a new Proxy with sanitized and defaulted configuration parameters.
func NewProxy(cfg Config) *Proxy {
	// Sanitize numeric limits using idiomatic guard clauses
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = DefaultMaxWorkers
	}
	if cfg.QueueCap <= 0 {
		cfg.QueueCap = DefaultQueueCap
	}

	// Keep KeysValueToReWritten as nil if unused, or slice it safely
	if cfg.KeysValueToReWritten == nil {
		cfg.KeysValueToReWritten = make(map[SanitizingKeyNameType][]string, 0)
	}

	// Use the provided keys for the default rewriter if available
	if cfg.ReWriter == nil {
		rewriterKeys := cfg.KeysValueToReWritten
		cfg.ReWriter = NewDefaultReWriter(rewriterKeys)
	}

	p := &Proxy{
		config: cfg,
		queue:  make(chan InFlightRequest, cfg.QueueCap),
	}

	// Start the internal background workers
	p.startWorkers()

	return p
}

// Handler acts as standard HTTP middleware, intercepting and cloning
// traffic transparently without interrupting the core handler chain.
func (p *Proxy) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.dispatchAsync(r)
		next.ServeHTTP(w, r)
	})
}

func (p *Proxy) dispatchAsync(r *http.Request) {

	reqCarrier := InFlightRequest{
		Method:  r.Method,
		UrlPath: r.URL.RequestURI(),
		Headers: cloneHeaders(r.Header),
	}

	// Safely capture the body ONLY if it exists (POST, PUT, PATCH)
	if r.Body != nil && r.Body != http.NoBody {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			// Restore the stream for the production path
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			// Allocate the isolated buffer slice for background processing
			reqCarrier.Body = make([]byte, len(bodyBytes))
			copy(reqCarrier.Body, bodyBytes)
		}
	}

	// Blast the carrier into our bounded safety queue
	select {
	case p.queue <- reqCarrier:
		log.Printf("Dispatched to : %s data", reqCarrier.UrlPath)
	default:
		// Queue full protection circuit breaker triggered; drop safely
		log.Printf("Queue Channel Full")
	}
}

// Helper to deeply clone headers to avoid concurrent map read/write panics
func cloneHeaders(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, v := range h {
		v2 := make([]string, len(v))
		copy(v2, v)
		h2[k] = v2
	}
	return h2
}
