package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func baseURL() string {
	if v := os.Getenv("BASE_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

// With SMTP_HOST set it sends through that server (Mailtrap in dev).
// Without it, it just prints, so the app and your curl tests still work.
func sendEmail(to, subject, body string) error {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		log.Printf("[DEV EMAIL] to=%s subject=%q\n%s", to, subject, body)
		return nil
	}
	if strings.ContainsAny(to+subject, "\r\n") {
		return errors.New("invalid header value")
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "2525"
	}
	from := os.Getenv("MAIL_FROM")
	if from == "" {
		from = "noreply@todolist.local"
	}

	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" + body + "\r\n"

	auth := smtp.PlainAuth("", os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"), host)
	return smtp.SendMail(host+":"+port, auth, from, []string{to}, []byte(msg))
}

// For notices sent AFTER a change has already happened: a mail failure
// shouldn't turn a successful change into an error, so just log it.
func notify(to, subject, body string) {
	go func() {
		if err := sendEmail(to, subject, body); err != nil {
			log.Printf("notification to %s failed: %v", to, err)
		}
	}()
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func changeEmail(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewEmail        string `json:"new_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.CurrentPassword == "" || in.NewEmail == "" {
		httpError(w, "current_password and new_email required", http.StatusBadRequest)
		return
	}

	// Must look like a plain address (rejects things like "Name <a@b.com>")
	addr, err := mail.ParseAddress(in.NewEmail)
	if err != nil || addr.Address != in.NewEmail {
		httpError(w, "invalid email address", http.StatusBadRequest)
		return
	}

	// 1. Prove they know the current password
	var hash, currentEmail string
	err = db.QueryRow("SELECT password_hash, email FROM users WHERE id = ?", currentUser(r).ID).Scan(&hash, &currentEmail)
	if err != nil {
		httpError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.CurrentPassword)) != nil {
		httpError(w, "current password is incorrect", http.StatusUnauthorized)
		return
	}

	// 2. Must be a new address, and not taken by someone else
	if in.NewEmail == currentEmail {
		httpError(w, "new email must be different", http.StatusBadRequest)
		return
	}
	var taken int
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", in.NewEmail).Scan(&taken); err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}
	if taken > 0 {
		httpError(w, "email already in use", http.StatusConflict)
		return
	}

	// 3. Make the one-time token; store only its hash, valid for 1 hour
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(b)
	expires := time.Now().Add(time.Hour).Unix()

	// One pending change per user: a new request replaces the old one
	_, err = db.Exec(`
		INSERT INTO email_changes(user_id, new_email, token_hash, expires_at) VALUES(?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET new_email=excluded.new_email, token_hash=excluded.token_hash, expires_at=excluded.expires_at`,
		currentUser(r).ID, in.NewEmail, hashToken(token), expires)
	if err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}

	// 4. "Send" the link
	link := baseURL() + "/verify-email?token=" + token
	if err := sendEmail(in.NewEmail, "Confirm your new email", "Click to confirm: "+link); err != nil {
		httpError(w, "could not send email", http.StatusInternalServerError)
		return
	}

	sendJSON(w, http.StatusOK, map[string]string{"message": "verification link sent to the new email"})
}
func verifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httpError(w, "invalid or expired link", http.StatusBadRequest)
		return
	}

	// One transaction: either everything below happens, or nothing does
	tx, err := db.Begin()
	if err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback() // does nothing after a successful Commit

	// 1. Find the pending change by the HASH of the token in the link
	var userID, expires int64
	var newEmail string
	err = tx.QueryRow("SELECT user_id, new_email, expires_at FROM email_changes WHERE token_hash = ?",
		hashToken(token)).Scan(&userID, &newEmail, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		httpError(w, "invalid or expired link", http.StatusBadRequest)
		return
	}
	if err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}

	// 2. Expired? Remove it, and give the same message as "not found"
	if time.Now().Unix() > expires {
		tx.Exec("DELETE FROM email_changes WHERE user_id = ?", userID)
		tx.Commit()
		httpError(w, "invalid or expired link", http.StatusBadRequest)
		return
	}

	// 3. Re-check the address is still free (someone may have registered it since)
	var taken int
	if err := tx.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", newEmail).Scan(&taken); err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}
	if taken > 0 {
		tx.Exec("DELETE FROM email_changes WHERE user_id = ?", userID)
		tx.Commit()
		httpError(w, "email already in use", http.StatusConflict)
		return
	}

	// NEW (1): read the old address BEFORE it gets replaced
	var oldEmail string
	if err := tx.QueryRow("SELECT email FROM users WHERE id = ?", userID).Scan(&oldEmail); err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}

	// 4. Activate the new email, then delete the row so the link can't be reused
	if _, err := tx.Exec("UPDATE users SET email = ? WHERE id = ?", newEmail, userID); err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec("DELETE FROM email_changes WHERE user_id = ?", userID); err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		httpError(w, "server error", http.StatusInternalServerError)
		return
	}
	notify(oldEmail, "Your email address was changed", "The email on your account was changed to "+newEmail+".\n\nIf this wasn't you, contact support immediately.")

	sendJSON(w, http.StatusOK, map[string]string{"message": "email verified"})
}
