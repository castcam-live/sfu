package main

import "encoding/json"

type TypeData[T any] struct {
	Type string `json:"type"`
	Data T      `json:"data"`
}

func NewTypeData[T any](t string, v T) TypeData[T] {
	return TypeData[T]{
		Type: t,
		Data: v,
	}
}

func Decode[V any](b []byte) (TypeData[V], error) {
	var result TypeData[V]
	err := json.Unmarshal(b, &result)
	return result, err
}
