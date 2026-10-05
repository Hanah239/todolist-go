package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var db *sql.DB
var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

func register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	err := json.NewDecoder(r.Body).Decode(&in)
	if err != nil || in.Email == "" || len(in.Password) < 8 {
		http.Error(w, "email and a password of 8+ characters required", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	role := "user"
	if in.Email == os.Getenv("ADMIN_EMAIL") {
		role = "admin"
	}
	_, err = db.Exec("INSERT INTO users(email, password_hash, role) VALUES(?, ?, ?)", in.Email, string(hash), role)

	if err != nil {
		http.Error(w, "email already registered", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("registered\n"))
}

func main() {
	if len(jwtSecret) == 0 {
		log.Fatal("JWT_SECRET environment variable must be set")
	}

	var err error
	db, err = sql.Open("sqlite", "app.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			email         TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role          TEXT NOT NULL DEFAULT 'user'
		)`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title   TEXT NOT NULL,
			done    INTEGER NOT NULL DEFAULT 0,
			priority TEXT NOT NULL DEFAULT 'medium',
			due_date TEXT NOT NULL DEFAULT ''
		)`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS profiles (
			user_id  INTEGER PRIMARY KEY,
			name     TEXT NOT NULL DEFAULT '',
			bio      TEXT NOT NULL DEFAULT '',
			dp_url   TEXT NOT NULL DEFAULT ''
		)`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tags (
			id    INTEGER PRIMARY KEY AUTOINCREMENT,
			name  TEXT UNIQUE NOT NULL,
			color TEXT NOT NULL DEFAULT '#cccccc'
		)`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS todo_tags (
			todo_id INTEGER NOT NULL,
			tag_id  INTEGER NOT NULL,
			PRIMARY KEY (todo_id, tag_id)
		)`)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", register)
	mux.HandleFunc("POST /login", login)
	mux.HandleFunc("POST /logout", requireAuth(logout))
	mux.HandleFunc("GET /me", requireAuth(whoami))
	mux.HandleFunc("POST /todos", requireAuth(createTodo))
	mux.HandleFunc("GET /todos", requireAuth(listTodos))
	mux.HandleFunc("PUT /todos/{id}", requireAuth(updateTodo))
	mux.HandleFunc("DELETE /todos/{id}", requireAuth(deleteTodo))
	mux.HandleFunc("GET /profile", requireAuth(getProfile))
	mux.HandleFunc("PUT /profile", requireAuth(updateProfile))
	mux.HandleFunc("PUT /change-password", requireAuth(changePassword))
	mux.HandleFunc("GET /tags", requireAuth(listTags))
	mux.HandleFunc("POST /todos/{id}/tags", requireAuth(addTagToTodo))
	mux.HandleFunc("DELETE /todos/{id}/tags/{tagID}", requireAuth(removeTagFromTodo))

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
