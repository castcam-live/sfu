package main

import "net/http"

func main() {
	router := CreateHandlers()

	// TODO: soft code this
	http.ListenAndServe(":8080", router)
}
