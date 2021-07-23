package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
)

const Port = 5000

func returnBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(flyetcd.Bootstrapped())
}

func handleRequests() {
	http.HandleFunc("/bootstrapped", returnBootstrapStatus)
	log.Printf("Listening on port %d", Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", Port), nil))
}

func main() {
	handleRequests()
}
