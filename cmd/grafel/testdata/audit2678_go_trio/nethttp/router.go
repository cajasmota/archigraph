// net/http stdlib router fixture for issue #2685. Exercises three
// registration shapes: package-level http.HandleFunc (ANY verb), local
// http.ServeMux HandleFunc (ANY verb), and Go 1.22+ method-prefix
// patterns where the verb is the first token of the pattern string.
package nethttpfixture

import (
	"net/http"
)

func setup() *http.ServeMux {
	http.HandleFunc("/legacy", legacyHandler)

	mux := http.NewServeMux()
	mux.HandleFunc("/items", itemsHandler)
	mux.HandleFunc("GET /users/{id}", getUser)
	mux.HandleFunc("POST /users", createUser)
	return mux
}
