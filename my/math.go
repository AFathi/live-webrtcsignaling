package my

import "math/rand"

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func RandIntBetween(min int, max int) int {
	return min + rand.Intn(max-min)
}
