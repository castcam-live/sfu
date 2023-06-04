package main

import "encoding/json"

type TypeDataJSON struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func Decode[V any](b []byte) (V, error) {
	var result V
	err := json.Unmarshal(b, &result)
	return result, err
}
