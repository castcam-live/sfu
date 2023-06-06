package main

import (
	"fmt"
	"net/http"

	"github.com/castcam-live/simple-forwarding-unit/config"
)

func main() {
	router := CreateHandlers()

	fmt.Println("Listening on port", config.PortNumber())
	panic(http.ListenAndServe(fmt.Sprintf(":%d", config.PortNumber()), router))
}
