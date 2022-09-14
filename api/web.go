package api

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

func configWebRoutes(r *mux.Router) {
	r.HandleFunc("/abcd", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "ABCD") })
}
