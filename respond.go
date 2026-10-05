package main

import "net/http"

// Same signature as http.Error, but sends {"error": "..."} as JSON.
func httpError(w http.ResponseWriter, msg string, code int) {
	sendJSON(w, code, map[string]string{"error": msg})
}
