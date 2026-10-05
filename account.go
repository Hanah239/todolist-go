package main

import (
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func changePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.CurrentPassword == "" || in.NewPassword == "" {
		http.Error(w, "current_password and new_password required", http.StatusBadRequest)
		return
	}

	// Get the stored hash for the logged-in user (ID comes from the JWT)
	var hash string
	err := db.QueryRow("SELECT password_hash FROM users WHERE id = ?", currentUser(r).ID).Scan(&hash)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// 1. Prove they know the current password
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.CurrentPassword)) != nil {
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	// 2. Validate the new password (same 8+ rule as register; bcrypt rejects over 72 bytes)
	if len(in.NewPassword) < 8 || len(in.NewPassword) > 72 {
		http.Error(w, "new password must be 8 to 72 characters", http.StatusBadRequest)
		return
	}

	// 3. Must actually be different from the old one
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.NewPassword)) == nil {
		http.Error(w, "new password cannot be same as your old one", http.StatusBadRequest)
		return
	}

	// 4. Hash and save
	newHash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if _, err := db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(newHash), currentUser(r).ID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "password changed successfully"})
}
