package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type Todo struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

func sendJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func createTodo(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Title) == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	res, err := db.Exec("INSERT INTO todos(user_id, title) VALUES(?, ?)", currentUser(r).ID, in.Title)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	sendJSON(w, http.StatusCreated, Todo{ID: id, Title: in.Title})
}

func listTodos(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, title, done FROM todos WHERE user_id = ?", currentUser(r).ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	todos := []Todo{}
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Done); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		todos = append(todos, t)
	}
	sendJSON(w, http.StatusOK, todos)
}

func updateTodo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var in struct {
		Title string `json:"title"`
		Done  bool   `json:"done"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Title) == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	// "AND user_id = ?" is the authorization: you can only change YOUR rows.
	res, err := db.Exec("UPDATE todos SET title = ?, done = ? WHERE id = ? AND user_id = ?",
		in.Title, in.Done, id, currentUser(r).ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	sendJSON(w, http.StatusOK, Todo{ID: id, Title: in.Title, Done: in.Done})
}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	res, err := db.Exec("DELETE FROM todos WHERE id = ? AND user_id = ?", id, currentUser(r).ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Admin only: everyone's todos, with the owner's id.
func adminListTodos(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, user_id, title, done FROM todos")
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type row struct {
		Todo
		UserID int64 `json:"user_id"`
	}
	out := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ID, &x.UserID, &x.Title, &x.Done); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		out = append(out, x)
	}
	sendJSON(w, http.StatusOK, out)
}
