package loadbalancer

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	_ "net/http/httputil"
	_ "net/url"
	"sync"
	"sync/atomic"
)

type LoadBalancer struct {
	replicas []string
	counter  uint64
}

func New(replicas []string) *LoadBalancer {
	return &LoadBalancer{replicas: replicas}
}

// nextBackend selects the next replica using round-robin.
func (lb *LoadBalancer) nextBackend() string {
	idx := atomic.AddUint64(&lb.counter, 1)
	selected := lb.replicas[idx%uint64(len(lb.replicas))]
	slog.Debug("Selected backend", "index", idx, "address", selected)
	return selected
}

// PutKvKey handles PUT /kv/{key} by broadcasting the update to all replicas.
func (lb *LoadBalancer) PutKvKey(w http.ResponseWriter, r *http.Request, key string) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Failed to read PUT request body", "error", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	slog.Info("Broadcasting PUT request", "key", key, "replica_count", len(lb.replicas))

	var wg sync.WaitGroup
	wg.Add(len(lb.replicas))

	for _, backend := range lb.replicas {
		go func(backend string) {
			defer wg.Done()

			targetURL := "http://" + backend + "/kv/" + key
			req, _ := http.NewRequest("PUT", targetURL, bytes.NewReader(bodyBytes))
			req.Header = r.Header.Clone()

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				slog.Warn("PUT request failed", "backend", backend, "error", err)
				return
			}
			slog.Debug("PUT request succeeded", "backend", backend, "status", resp.StatusCode)
			resp.Body.Close()
		}(backend)
	}

	wg.Wait()
	slog.Info("PUT request broadcast complete", "key", key)
	w.WriteHeader(http.StatusOK)
}

// GetKvKey handles GET /kv/{key} by trying each replica until one returns 200 OK.
func (lb *LoadBalancer) GetKvKey(w http.ResponseWriter, r *http.Request, key string) {
	for i := 0; i < len(lb.replicas); i++ {
		backend := lb.nextBackend()
		target := "http://" + backend + "/kv/" + key

		slog.Info("Sending GET request", "key", key, "backend", backend)

		resp, err := http.Get(target)
		if err != nil {
			slog.Warn("GET request failed", "backend", backend, "error", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			slog.Info("GET request succeeded", "key", key, "backend", backend)

			for k, vs := range resp.Header {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			return
		}

		slog.Debug("GET response not OK", "key", key, "backend", backend, "status", resp.StatusCode)
	}

	slog.Error("GET failed on all replicas", "key", key)
	http.Error(w, "key not found on any replica", http.StatusNotFound)
}
