package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Find the user and compare the typed password to the stored hash
	var id int64
	var hash string
	err := db.QueryRow("SELECT id, password_hash FROM users WHERE email = ?", in.Email).Scan(&id, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	// Make a random token and save it with an expiry time (24 hours)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(b)
	expires := time.Now().Add(24 * time.Hour).Unix()

	_, err = db.Exec("INSERT INTO sessions(token, user_id, expires_at) VALUES(?, ?, ?)", token, id, expires)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}
