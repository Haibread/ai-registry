package middleware

import "testing"

func SetMaxBuckets(t *testing.T, n int) {
	prev := maxBuckets
	maxBuckets = n
	t.Cleanup(func() { maxBuckets = prev })
}
