// Handler definitions for the net/http stdlib router fixture. The
// integration test asserts source_file rebinds here from router.go and
// that the original registration line is preserved as a property.
package nethttpfixture

import (
	"net/http"
)

func legacyHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("legacy"))
}

func itemsHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`{"items":[]}`))
}

func getUser(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`{"id":1}`))
}

func createUser(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
}
