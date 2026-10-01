package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

	var id int64
	var hash, role string
	err := db.QueryRow("SELECT id, password_hash, role FROM users WHERE email = ?", in.Email).Scan(&id, &hash, &role)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	// Build the JWT: user id, role, and a 12-hour expiry, all inside the token itself.
	claims := jwt.MapClaims{
		"user_id": id,
		"role":    role,
		"exp":     time.Now().Add(12 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": signed})
}
