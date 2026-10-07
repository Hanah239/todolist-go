package main

import (
	"encoding/json"
	"net/http"
)

type Profile struct {
	Name  string `json:"name"`
	Bio   string `json:"bio"`
	DPURL string `json:"dp_url"`
}

func getProfile(w http.ResponseWriter, r *http.Request) {
	var p Profile
	err := db.QueryRow("SELECT name, bio, dp_url FROM profiles WHERE user_id = ?", currentUser(r).ID).
		Scan(&p.Name, &p.Bio, &p.DPURL)
	if err != nil {
		// no row yet -> just return an empty profile, not an error
		p = Profile{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func updateProfile(w http.ResponseWriter, r *http.Request) {
	var in Profile
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpError(w, "bad request", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(`
		INSERT INTO profiles(user_id, name, bio, dp_url) VALUES(?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET name=excluded.name, bio=excluded.bio, dp_url=excluded.dp_url`,
		currentUser(r).ID, in.Name, in.Bio, in.DPURL)
	if err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(in)
}
