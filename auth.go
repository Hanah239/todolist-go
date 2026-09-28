package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type User struct {
	ID   int64
	Role string
}

type ctxKey string

const userKey ctxKey = "user"

func tokenFrom(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}

// The gatekeeper: runs BEFORE a handler and rejects requests without a valid token.
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var u User
		err := db.QueryRow(`
			SELECT u.id, u.role
			FROM sessions s JOIN users u ON u.id = s.user_id
			WHERE s.token = ? AND s.expires_at > ?`,
			tokenFrom(r), time.Now().Unix()).Scan(&u.ID, &u.Role)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userKey, u)
		next(w, r.WithContext(ctx))
	}
}

func currentUser(r *http.Request) User {
	return r.Context().Value(userKey).(User)
}

// A second gatekeeper on top of the first: also requires the admin role.
func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r).Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

func whoami(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": u.ID, "role": u.Role})
}

func logout(w http.ResponseWriter, r *http.Request) {
	db.Exec("DELETE FROM sessions WHERE token = ?", tokenFrom(r))
	w.Write([]byte("logged out\n"))
}

func adminPing(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello, admin\n"))
}
