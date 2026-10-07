package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type Tag struct {
	ID    int64  `json:"tag_id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// List every tag that exists (not just ones on your to-dos)
func listTags(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, name, color FROM tags")
	if err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tags := []Tag{}
	for rows.Next() {
		var t Tag
		rows.Scan(&t.ID, &t.Name, &t.Color)
		tags = append(tags, t)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}

// Attach a tag to one of YOUR to-dos. Creates the tag if it doesn't exist yet.
func addTagToTodo(w http.ResponseWriter, r *http.Request) {
	todoID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpError(w, "bad id", http.StatusBadRequest)
		return
	}

	// ownership check: is this todo actually yours?
	var owner int64
	err = db.QueryRow("SELECT user_id FROM todos WHERE id = ?", todoID).Scan(&owner)
	if err != nil || owner != currentUser(r).ID {
		httpError(w, "not found", http.StatusNotFound)
		return
	}

	var in struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Name) == "" {
		httpError(w, "tag name required", http.StatusBadRequest)
		return
	}
	if in.Color == "" {
		in.Color = "#cccccc"
	}

	// create the tag if it doesn't exist, do nothing if it does
	db.Exec("INSERT OR IGNORE INTO tags(name, color) VALUES(?, ?)", in.Name, in.Color)

	var tagID int64
	if err := db.QueryRow("SELECT id FROM tags WHERE name = ?", in.Name).Scan(&tagID); err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}

	// link them; INSERT OR IGNORE so re-adding the same tag doesn't error
	db.Exec("INSERT OR IGNORE INTO todo_tags(todo_id, tag_id) VALUES(?, ?)", todoID, tagID)

	w.WriteHeader(http.StatusNoContent)
}

// Remove a tag from one of YOUR to-dos
func removeTagFromTodo(w http.ResponseWriter, r *http.Request) {
	todoID, err1 := strconv.ParseInt(r.PathValue("id"), 10, 64)
	tagID, err2 := strconv.ParseInt(r.PathValue("tagID"), 10, 64)
	if err1 != nil || err2 != nil {
		httpError(w, "bad id", http.StatusBadRequest)
		return
	}

	var owner int64
	err := db.QueryRow("SELECT user_id FROM todos WHERE id = ?", todoID).Scan(&owner)
	if err != nil || owner != currentUser(r).ID {
		httpError(w, "not found", http.StatusNotFound)
		return
	}

	db.Exec("DELETE FROM todo_tags WHERE todo_id = ? AND tag_id = ?", todoID, tagID)
	w.WriteHeader(http.StatusNoContent)
}
