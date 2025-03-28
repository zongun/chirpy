package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/zongun/chirpy/internal/auth"
	"github.com/zongun/chirpy/internal/database"
)

const (
	minExpireSec = 600
	maxExpireSec = 3600
)

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	Token     string    `json:"token,omitempty"`
}

type apiConfig struct {
	queries        *database.Queries
	fileServerHits atomic.Int32
	tokenSecret    string
}

func NewApiConfig(q *database.Queries, tokenSecret string) *apiConfig {
	return &apiConfig{
		queries:     q,
		tokenSecret: tokenSecret,
	}
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
	var (
		data = &database.CreateUserParams{}
		raw  = json.NewDecoder(r.Body)
	)

	if err := raw.Decode(data); err != nil {
		respondWithError(w, http.StatusBadRequest, "Request was not structured properly")
		return
	}

	hash, err := auth.HashPassword(data.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to create user")
		return
	}
	data.Password = hash

	user, err := a.queries.CreateUser(r.Context(), *data)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to create user")
		return
	}

	respondWithJSON(w, http.StatusCreated, UserResponse{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	})
}

func (a *apiConfig) Login(w http.ResponseWriter, r *http.Request) {
	type LoginRequest struct {
		Email             string `json:"email"`
		Password          string `json:"password"`
		ExpiresInSecondes int    `json:"expires_in_seconds,omitempty"`
	}

	var (
		expire time.Duration
		req    = &LoginRequest{}
	)

	raw := json.NewDecoder(r.Body)
	if err := raw.Decode(req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Request not correctly formatted")
		return
	}

	user, err := a.queries.GetUserAuth(r.Context(), req.Email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := auth.VerifyPassword(req.Password, user.Password); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if req.ExpiresInSecondes < minExpireSec || req.ExpiresInSecondes > maxExpireSec {
		expire = time.Second * maxExpireSec
	} else {
		expire = time.Second * time.Duration(req.ExpiresInSecondes)
	}

	token, err := auth.CreateJWT(user.ID, a.tokenSecret, expire)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	respondWithJSON(w, http.StatusOK, UserResponse{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
		Token:     token,
	})
}

func (a *apiConfig) CreateChirp(w http.ResponseWriter, r *http.Request) {
	var (
		data = &database.CreateChirpParams{}
		raw  = json.NewDecoder(r.Body)
	)

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	userID, err := auth.ValidateJWT(token, a.tokenSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	raw.Decode(data)
	if len(data.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirping too much, max is 140 characters")
		return
	}

	data.Body = profanityFilter(data.Body)
	data.UserID = userID

	result, err := a.queries.CreateChirp(r.Context(), *data)
	if err != nil {
		log.Printf("Failed to create chirp: %v\n", err)
		respondWithError(w, http.StatusBadRequest, "Failed to create chirp")
		return
	}

	respondWithJSON(w, http.StatusCreated, result)
}

func (a *apiConfig) GetChirps(w http.ResponseWriter, r *http.Request) {
	results, err := a.queries.GetChirps(r.Context())
	if err != nil {
		log.Printf("Failed to retrieve chirps: %v\n", err)
		respondWithError(w, http.StatusNotFound, "Failed to retrieve any chirps")
		return
	}

	respondWithJSON(w, http.StatusOK, results)
}

func (a *apiConfig) GetChirpByID(w http.ResponseWriter, r *http.Request) {
	pathID := r.PathValue("id")

	id, err := uuid.Parse(pathID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Not valid id")
		return
	}

	result, err := a.queries.GetChirp(r.Context(), id)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Not found")
		return
	}

	respondWithJSON(w, http.StatusOK, result)
}
