package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// HTTPError represents a standardized HTTP error response
type HTTPError struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// JSONError sends a JSON error response with the specified status code
func JSONError(w http.ResponseWriter, error string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(HTTPError{
		Error: error,
	})
}

// JSONErrorWithMessage sends a JSON error response with additional message
func JSONErrorWithMessage(w http.ResponseWriter, error string, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(HTTPError{
		Error:   error,
		Message: message,
	})
}

// JSONSuccess sends a JSON success response
func JSONSuccess(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// RequireMethod validates that the request uses the specified HTTP method
func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		JSONError(w, fmt.Sprintf("Method %s not allowed", r.Method), http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// RequireJSONBody validates that the request has a valid JSON body
func RequireJSONBody(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		JSONError(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return false
	}
	return true
}

// RequireHeader validates that the request has the specified header
func RequireHeader(w http.ResponseWriter, r *http.Request, headerName string) string {
	value := r.Header.Get(headerName)
	if value == "" {
		JSONError(w, fmt.Sprintf("%s header is required", headerName), http.StatusBadRequest)
		return ""
	}
	return value
}