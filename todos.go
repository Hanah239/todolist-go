package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type Todo struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Priority string `json:"priority"`
	DueDate  string `json:"due_date"`
	Tags     []Tag  `json:"tags"`
}

func sendJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func tagsForTodo(todoID int64) []Tag {
	rows, err := db.Query(`
		SELECT t.id, t.name, t.color
		FROM tags t
		JOIN todo_tags tt ON tt.tag_id = t.id
		WHERE tt.todo_id = ?`, todoID)
	if err != nil {
		return []Tag{}
	}
	defer rows.Close()

	tags := []Tag{}
	for rows.Next() {
		var t Tag
		rows.Scan(&t.ID, &t.Name, &t.Color)
		tags = append(tags, t)
	}
	return tags
}

func validPriority(p string) bool {
	return p == "low" || p == "medium" || p == "high"
}

func createTodo(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title    string `json:"title"`
		Priority string `json:"priority"`
		DueDate  string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Title) == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	if !validPriority(in.Priority) {
		http.Error(w, "priority must be low, medium, or high", http.StatusBadRequest)
		return
	}

	res, err := db.Exec("INSERT INTO todos(user_id, title, priority, due_date) VALUES(?, ?, ?, ?)",
		currentUser(r).ID, in.Title, in.Priority, in.DueDate)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	sendJSON(w, http.StatusCreated, Todo{ID: id, Title: in.Title, Priority: in.Priority, DueDate: in.DueDate, Tags: []Tag{}})
}

func listTodos(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	search := r.URL.Query().Get("search")

	query := `SELECT DISTINCT t.id, t.title, t.done, t.priority, t.due_date
		FROM todos t`
	args := []any{}

	if tag != "" {
		query += ` JOIN todo_tags tt ON tt.todo_id = t.id
			JOIN tags tg ON tg.id = tt.tag_id`
	}

	query += ` WHERE t.user_id = ?`
	args = append(args, currentUser(r).ID)

	if tag != "" {
		query += ` AND tg.name = ?`
		args = append(args, tag)
	}
	if search != "" {
		query += ` AND t.title LIKE ?`
		args = append(args, "%"+search+"%")
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	todos := []Todo{}
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Done, &t.Priority, &t.DueDate); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		t.Tags = tagsForTodo(t.ID)
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
		Title    string `json:"title"`
		Done     bool   `json:"done"`
		Priority string `json:"priority"`
		DueDate  string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Title) == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	if !validPriority(in.Priority) {
		http.Error(w, "priority must be low, medium, or high", http.StatusBadRequest)
		return
	}

	res, err := db.Exec("UPDATE todos SET title = ?, done = ?, priority = ?, due_date = ? WHERE id = ? AND user_id = ?",
		in.Title, in.Done, in.Priority, in.DueDate, id, currentUser(r).ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	sendJSON(w, http.StatusOK, Todo{ID: id, Title: in.Title, Done: in.Done, Priority: in.Priority, DueDate: in.DueDate, Tags: tagsForTodo(id)})
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

func adminListTodos(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, user_id, title, done, priority, due_date FROM todos")
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
		if err := rows.Scan(&x.ID, &x.UserID, &x.Title, &x.Done, &x.Priority, &x.DueDate); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		x.Tags = tagsForTodo(x.ID)
		out = append(out, x)
	}
	sendJSON(w, http.StatusOK, out)
}
