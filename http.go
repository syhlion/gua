package main

import (
	"net/http"

	"github.com/syhlion/restresp"
)

func Version(serverVersion string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		restresp.Write(w, serverVersion, http.StatusOK)
	}
}
