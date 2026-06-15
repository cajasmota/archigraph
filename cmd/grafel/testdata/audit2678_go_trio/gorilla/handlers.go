// Handler definitions for the gorilla/mux router fixture. After the
// resolver rebind, each http_endpoint_definition's source_file should
// land in THIS file (not router.go).
package gorillafixture

import (
	"net/http"
)

func listUsers(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`{"users":[]}`))
}

func getUser(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`{"id":1}`))
}

func createItem(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
