package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/zongun/chirpy/internal/database"
)

const PORT = "8080"

func respondWithError(w http.ResponseWriter, code int, msg string) {
	respondWithJSON(w, code, struct {
		Error string `json:"error"`
	}{Error: msg})
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) {
	d, err := json.Marshal(payload)
	if err != nil {
		log.Println(err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(d)
}

func profanityFilter(text string) string {
	bannedWords := []string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Split(text, " ")

	for i, word := range words {
		for _, bword := range bannedWords {
			if strings.ToUpper(word) == strings.ToUpper(bword) {
				words[i] = "****"
			}
		}
	}

	return strings.Join(words, " ")
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(err)
	}
	dbQueries := database.New(db)
	api := NewApiConfig(dbQueries)

	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    ":" + (PORT),
		Handler: mux,
	}

	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", api.middlewareMetricsInc(fileServerHandler))

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /api/chirps", api.GetChirps)
	mux.HandleFunc("GET /api/chirps/{id}", api.GetChirpByID)
	mux.HandleFunc("POST /api/chirps", api.createChirp)
	mux.HandleFunc("POST /api/users", api.createUser)

	// Metrics
	mux.HandleFunc("GET /admin/metrics", api.showHits)
	mux.HandleFunc("POST /admin/reset", api.reset)

	log.Printf("Started listening on :%s\n", PORT)
	log.Fatal(srv.ListenAndServe())
}
