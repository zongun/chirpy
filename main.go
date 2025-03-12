package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	res := fmt.Sprintf(`<html>
	  <body>
	    <h1>Welcome, Chirpy Admin</h1>
	    <p>Chirpy has been visited %d times!</p>
	  </body>
	</html>`, a.fileServerHits.Load())

	w.Write([]byte(res))
}

func (a *apiConfig) resetHits(w http.ResponseWriter, r *http.Request) {
	log.Println("Reset hits to 0")
	a.fileServerHits.Store(0)
}

func validateChirp(w http.ResponseWriter, r *http.Request) {
	type params struct {
		Body string `json:"body"`
	}

	data := &params{}
	raw := json.NewDecoder(r.Body)
	raw.Decode(data)

	if len(data.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirping too much, max is 140 characters")
		return
	}

	result := profanityFilter(data.Body)

	respondWithJSON(w, http.StatusOK, struct {
		Clean string `json:"cleaned_body"`
	}{Clean: result})
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	d, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{Error: msg})

	w.Write(d)
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) {
	d, err := json.Marshal(payload)
	if err != nil {
		log.Printf("%v", err)
		return
	}

	w.Header().Add("Content-Type", "text/json")
	w.WriteHeader(code)
	w.Write(d)
}

func profanityFilter(text string) string {
	bannedWords := []string{"kerfuffle", "sharbert", "fornax"}
	allWords := strings.Split(text, " ")

	for i, word := range allWords {
		for _, bword := range bannedWords {
			if strings.ToUpper(word) == strings.ToUpper(bword) {
				allWords[i] = "****"
			}
		}
	}

	return strings.Join(allWords, " ")
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

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("POST /api/validate_chirp", validateChirp)

	// Metrics
	mux.HandleFunc("GET /admin/metrics", api.showHits)
	mux.HandleFunc("POST /admin/reset", api.resetHits)

	log.Printf("Started listening on :%s\n", PORT)
	log.Fatal(srv.ListenAndServe())
}
