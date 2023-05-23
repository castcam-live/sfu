package main

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
