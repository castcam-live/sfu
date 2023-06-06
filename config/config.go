package config

import (
	"os"
	"strconv"
)

var portNumber = 8080

func init() {
	port := os.Getenv("PORT")
	num, err := strconv.Atoi(port)

	if err == nil {
		portNumber = num
	}
}

func PortNumber() int {
	return portNumber
}
