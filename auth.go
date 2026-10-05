package main

import (
	"context"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"strings"
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
		tokenStr := tokenFrom(r)

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			httpError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			httpError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		userIDFloat, ok1 := claims["user_id"].(float64)
		role, ok2 := claims["role"].(string)
		if !ok1 || !ok2 {
			httpError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		u := User{ID: int64(userIDFloat), Role: role}
		ctx := context.WithValue(r.Context(), userKey, u)
		next(w, r.WithContext(ctx))
	}
}

func currentUser(r *http.Request) User {
	return r.Context().Value(userKey).(User)
}

func whoami(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": u.ID, "role": u.Role})
}

// With JWT there's no server-side session to delete — the token stays
// valid until it naturally expires (12 hours). Logging out here just
// confirms the request was authenticated; the client should discard
// the token on their end.
func logout(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, http.StatusOK, map[string]string{"message": "logged out (token remains valid until it expires)"})

}
