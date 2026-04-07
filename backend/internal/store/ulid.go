package store

import (
	"math/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	entropyPool = sync.Pool{
		New: func() any {
			return ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0) //nolint:gosec
		},
	}
)

// NewULID generates a new ULID string.
func NewULID() string {
	entropy := entropyPool.Get().(*ulid.MonotonicEntropy)
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	entropyPool.Put(entropy)
	return id.String()
}
