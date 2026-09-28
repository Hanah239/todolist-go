package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	// Open (or create) the database file called app.db
	db, err := sql.Open("sqlite", "app.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create the users table if it doesn't exist yet
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS users (
id            INTEGER PRIMARY KEY AUTOINCREMENT,
email         TEXT UNIQUE NOT NULL,
password_hash TEXT NOT NULL
)`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Database ready, users table created")
}
