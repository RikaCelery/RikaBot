package utils

import "encoding/json"

func ToJson(s any) string {
	marshal, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(marshal)
}

func FromJson[T any](s string) *T {
	var marshal T
	err := json.Unmarshal([]byte(s), &marshal)
	if err != nil {
		panic(err)
	}
	return &marshal
}
