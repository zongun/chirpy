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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.fileServerHits.Add(1)
		log.Println("Hit:", a.fileServerHits.Load())

		next.ServeHTTP(w, r)
	})
}

func (a *apiConfig) showHits(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	res := fmt.Sprintf("Hits: %d", a.fileServerHits.Load())
	w.Write([]byte(res))
}

func (a *apiConfig) resetHits(w http.ResponseWriter, r *http.Request) {
	log.Println("Reset hits to 0")
	a.fileServerHits.Store(0)
}

func main() {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    ":" + (PORT),
		Handler: mux,
	}
	api := NewApiConfig()

	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", api.middlewareMetricsInc(fileServerHandler))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Metrics
	mux.HandleFunc("GET /metrics", api.showHits)
	mux.HandleFunc("GET /reset", api.resetHits)

	log.Printf("Started listening on :%s\n", PORT)
	log.Fatal(srv.ListenAndServe())
}
