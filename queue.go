package miragemock

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

type Worker struct {
	rewriter     ReWriter
	targetClient string
	httpClient   *http.Client
}

func NewWorker(rewriter ReWriter, target string, client *http.Client) *Worker {
	return &Worker{
		rewriter:     rewriter,
		targetClient: target,
		httpClient:   client,
	}
}

func (w *Worker) processJob(job InFlightRequest) {
	var bodyReader io.Reader
	if len(job.Body) > 0 {
		mutatedBody := w.rewriter.RewriteBody(job.Body)
		bodyReader = bytes.NewReader(mutatedBody)
	}

	targetReq, err := http.NewRequest(job.Method, w.targetClient+job.UrlPath, bodyReader)
	if err != nil {
		return
	}

	if len(job.Headers) > 0 {
		targetReq.Header = w.rewriter.RewriteHeader(job.Headers)
	} else {
		targetReq.Header = make(http.Header)
	}

	resp, err := w.httpClient.Do(targetReq)
	if err != nil {
		return
	}

	// Drain and close body to allow TCP connection reuse
	if resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func (p *Proxy) startWorkers() {
	// Configure shared transport for socket pooling
	sharedClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        p.config.MaxWorkers,
			MaxIdleConnsPerHost: p.config.MaxWorkers,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	for i := 0; i < p.config.MaxWorkers; i++ {
		worker := NewWorker(p.config.ReWriter, p.config.TargetClient, sharedClient)

		go func() {
			for job := range p.queue {
				worker.processJob(job)
			}
		}()
	}
}
