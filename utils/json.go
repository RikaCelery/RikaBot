// Package utils common utils
package utils

import "encoding/json"

// ToJSON marshal struct to json string
func ToJSON(s any) string {
	marshal, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(marshal)
}

// FromJSON parse json string to struct
func FromJSON[T any](s string) *T {
	var marshal T
	err := json.Unmarshal([]byte(s), &marshal)
	if err != nil {
		panic(err)
	}
	return &marshal
}
