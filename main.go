package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

const PORT = "8080"

type apiConfig struct {
	fileServerHits atomic.Int32
}

func NewApiConfig() *apiConfig {
	return &apiConfig{}
}

func (a *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	a.fileServerHits.Add(1)
	return next
}

func main() {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    ":" + (PORT),
		Handler: mux,
	}
	apiCfg := NewApiConfig()

	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))

	mux.Handle("GET /app/", apiCfg.middlewareMetricsInc(fileServerHandler))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		res := fmt.Sprintf("Hits: %d", apiCfg.fileServerHits.Load())
		w.Write([]byte(res))
	})

	mux.HandleFunc("GET /reset", func(w http.ResponseWriter, r *http.Request) {
		apiCfg.fileServerHits.CompareAndSwap(0, 0)
	})

	log.Printf("Started listening on :%s\n", PORT)
	log.Fatal(srv.ListenAndServe())
}
