package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/zongun/chirpy/internal/database"
)

const PORT = "8080"

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type apiConfig struct {
	queries        *database.Queries
	fileServerHits atomic.Int32
}

func NewApiConfig(q *database.Queries) *apiConfig {
	return &apiConfig{queries: q}
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

func (a *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	a.fileServerHits.Store(0)

	result, err := a.queries.ResetUsers(r.Context())
	if err != nil {
		log.Println(err)
		return
	}

	count, err := result.RowsAffected()
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("Deleted %d rows\n", count)
}

func (a *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	type userCreate struct {
		Email string `json:"email"`
	}

	data := &userCreate{}
	raw := json.NewDecoder(r.Body)
	if err := raw.Decode(data); err != nil {
		respondWithError(w, http.StatusBadRequest, "Request was not structured properly")
		return
	}

	user, err := a.queries.CreateUser(r.Context(), data.Email)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to create user")
		return
	}

	respondWithJSON(w, http.StatusCreated, User(user))
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
	mux.HandleFunc("POST /api/validate_chirp", validateChirp)

	mux.HandleFunc("POST /api/users", api.createUser)

	// Metrics
	mux.HandleFunc("GET /admin/metrics", api.showHits)
	mux.HandleFunc("POST /admin/reset", api.reset)

	log.Printf("Started listening on :%s\n", PORT)
	log.Fatal(srv.ListenAndServe())
}
