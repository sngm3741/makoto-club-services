package common

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON serializes payload to JSON with status and logs on failure.
func WriteJSON(logger *log.Logger, w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil && logger != nil {
		logger.Printf("JSON エンコードに失敗: %v", err)
	}
}
