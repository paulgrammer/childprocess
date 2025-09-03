package httpapi

import (
	"encoding/json"
	"net/http"
)

// respondWithJSON writes the given payload as JSON with the provided status code.
// If encoding fails, it falls back to http.Error.
func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "failed to encode json", http.StatusInternalServerError)
	}
}

// respondWithError writes a standardized JSON error payload.
func respondWithError(w http.ResponseWriter, status int, message string) {
	respondWithJSON(w, status, map[string]string{"error": message})
}


