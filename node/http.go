package main

import (
	"net/http"
)

func GetNodeId(node *Node) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(200)
		w.Write([]byte(node.id))
	}
}
