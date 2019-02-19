package main

import (
	"net/http"
	"strconv"
)

func GetNodeId(node *Node) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(200)
		nodeId := strconv.FormatInt(node.id, 10)
		w.Write([]byte(nodeId))
	}
}
