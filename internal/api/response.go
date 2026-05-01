package api

import (
	"encoding/json"
	"net/http"
)

// errorBody is the standard error envelope returned on all non-2xx responses.
type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// writeJSON encodes v as JSON and writes it with the given HTTP status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// writeError writes a structured JSON error response.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorBody{Error: msg, Code: code})
}

func errBadRequest(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusBadRequest, "BAD_REQUEST", msg)
}

func errNotFound(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusNotFound, "NOT_FOUND", msg)
}

func errConflict(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusConflict, "CONFLICT", msg)
}

func errInternal(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", msg)
}

func errUnavailable(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", msg)
}
