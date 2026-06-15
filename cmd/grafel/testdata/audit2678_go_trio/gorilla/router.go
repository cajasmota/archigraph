// gorilla/mux router — registers routes that point at handlers defined
// in handlers.go. The #2684 endpoint synthesis emits one
// http_endpoint_definition per (verb, path) tuple; the shared resolver
// then rebinds source_file/source_line to the handler def.
package gorillafixture

import (
	"net/http"

	"github.com/gorilla/mux"
)

func setupRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/users", listUsers).Methods("GET")
	r.HandleFunc("/users/{id}", getUser).Methods("GET", "HEAD")
	r.HandleFunc("/items", createItem).Methods(http.MethodPost)

	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/health", healthCheck).Methods("GET")
	return r
}
