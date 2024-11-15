package utils

import "math/rand"

func Probability(p float64) bool {
	return rand.Float64() < p
}
