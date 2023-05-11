package main

import "strings"

func ParseQuery(query string) map[string]string {
	expressions := strings.Split(query, "&")
	result := map[string]string{}
	for _, expression := range expressions {
		keyValue := strings.Split(expression, "=")
		if len(keyValue) != 2 {
			continue
		}
		key := keyValue[0]
		value := keyValue[1]

		result[key] = value
	}

	return result
}
