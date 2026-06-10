package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
)

// CreateUser is a representative net/http handler with an env-gate
// (os.Getenv), the dominant `if err != nil { ... return }` guard writing an
// http.Error status, a validation guard returning a 409 via http.Error, and a
// panic on an invariant violation.
func CreateUser(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("SIGNUP_ENABLED") == "" {
		http.Error(w, "signup disabled", 503)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read body", http.StatusBadRequest)
		return
	}

	var dto userDTO
	if err := json.Unmarshal(body, &dto); err != nil {
		http.Error(w, "invalid json", http.StatusUnprocessableEntity)
		return
	}

	if dto.Email == "" {
		http.Error(w, "email required", http.StatusConflict)
		return
	}

	if w == nil {
		panic("nil response writer")
	}

	w.WriteHeader(http.StatusCreated)
}
