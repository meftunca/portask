package types

import (
	"crypto/rand"
	"encoding/base32"
	"sync"
	"time"
)

var (
	// ULID encoding uses Crockford's Base32 for better human-readability
	encoding = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

	// Pool for ULID generation to reduce allocations
	ulidPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 16)
		},
	}

	// Entropy source
	entropyPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 10)
		},
	}
)

// generateULID generates a ULID (Universally Unique Lexicographically Sortable Identifier)
// ULID is more efficient than UUID for our use case as it's lexicographically sortable
// and contains a timestamp component which is useful for message ordering
func generateULID() string {
	// Get buffer from pool
	ulid := ulidPool.Get().([]byte)
	defer ulidPool.Put(ulid)

	entropy := entropyPool.Get().([]byte)
	defer entropyPool.Put(entropy)

	// Get current timestamp in milliseconds
	now := time.Now().UnixMilli()

	// Encode timestamp (6 bytes)
	ulid[0] = byte(now >> 40)
	ulid[1] = byte(now >> 32)
	ulid[2] = byte(now >> 24)
	ulid[3] = byte(now >> 16)
	ulid[4] = byte(now >> 8)
	ulid[5] = byte(now)

	// Generate random entropy (10 bytes)
	if _, err := rand.Read(entropy); err != nil {
		// Fallback to pseudo-random if crypto/rand fails
		for i := range entropy {
			entropy[i] = byte(fastRand())
		}
	}

	// Copy entropy to ULID
	copy(ulid[6:], entropy)

	// Encode to base32
	return encoding.EncodeToString(ulid)
}

// fastRand provides a fast pseudo-random number generator
// This is a simple Linear Congruential Generator (LCG)
var randState uint64 = uint64(time.Now().UnixNano())

func fastRand() uint64 {
	randState = randState*1103515245 + 12345
	return randState
}

// generateShortID generates a shorter ID for internal use
func generateShortID() string {
	entropy := entropyPool.Get().([]byte)
	defer entropyPool.Put(entropy)

	// Use only 6 bytes for shorter ID
	if _, err := rand.Read(entropy[:6]); err != nil {
		for i := 0; i < 6; i++ {
			entropy[i] = byte(fastRand())
		}
	}

	return encoding.EncodeToString(entropy[:6])
}

// parseULIDTimestamp extracts the timestamp from a ULID
func parseULIDTimestamp(ulid string) (time.Time, error) {
	if len(ulid) != 26 {
		return time.Time{}, ErrInvalidULID
	}

	// Decode first 10 characters (timestamp part)
	timestampBytes, err := encoding.DecodeString(ulid[:10] + "======") // Add padding
	if err != nil {
		return time.Time{}, err
	}

	if len(timestampBytes) < 6 {
		return time.Time{}, ErrInvalidULID
	}

	// Extract timestamp
	timestamp := int64(timestampBytes[0])<<40 |
		int64(timestampBytes[1])<<32 |
		int64(timestampBytes[2])<<24 |
		int64(timestampBytes[3])<<16 |
		int64(timestampBytes[4])<<8 |
		int64(timestampBytes[5])

	return time.UnixMilli(timestamp), nil
}

// ErrInvalidULID is returned when ULID parsing fails
var ErrInvalidULID = NewPortaskError("INVALID_ULID", "invalid ULID format")

// IDGenerator provides an interface for ID generation strategies
type IDGenerator interface {
	Generate() string
	GenerateWithTimestamp(time.Time) string
}

// ULIDGenerator implements IDGenerator using ULID
type ULIDGenerator struct{}

// Generate creates a new ULID
func (g *ULIDGenerator) Generate() string {
	return generateULID()
}

// GenerateWithTimestamp creates a ULID with specific timestamp
func (g *ULIDGenerator) GenerateWithTimestamp(t time.Time) string {
	ulid := ulidPool.Get().([]byte)
	defer ulidPool.Put(ulid)

	entropy := entropyPool.Get().([]byte)
	defer entropyPool.Put(entropy)

	// Encode timestamp
	ms := t.UnixMilli()
	ulid[0] = byte(ms >> 40)
	ulid[1] = byte(ms >> 32)
	ulid[2] = byte(ms >> 24)
	ulid[3] = byte(ms >> 16)
	ulid[4] = byte(ms >> 8)
	ulid[5] = byte(ms)

	// Generate entropy
	if _, err := rand.Read(entropy); err != nil {
		for i := range entropy {
			entropy[i] = byte(fastRand())
		}
	}

	copy(ulid[6:], entropy)
	return encoding.EncodeToString(ulid)
}

// ShortIDGenerator implements IDGenerator using shorter IDs
type ShortIDGenerator struct{}

// Generate creates a new short ID
func (g *ShortIDGenerator) Generate() string {
	return generateShortID()
}

// GenerateWithTimestamp creates a short ID (ignores timestamp)
func (g *ShortIDGenerator) GenerateWithTimestamp(t time.Time) string {
	return generateShortID()
}

// NumericIDGenerator implements IDGenerator using numeric IDs
type NumericIDGenerator struct {
	counter uint64
	mutex   sync.Mutex
}

// Generate creates a new numeric ID
func (g *NumericIDGenerator) Generate() string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.counter++
	return itoa(g.counter)
}

// GenerateWithTimestamp creates a numeric ID (ignores timestamp)
func (g *NumericIDGenerator) GenerateWithTimestamp(t time.Time) string {
	return g.Generate()
}

// itoa converts uint64 to string (optimized version)
func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}

	// Buffer for maximum uint64 digits
	buf := make([]byte, 20)
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte(n%10) + '0'
		n /= 10
	}

	return string(buf[i:])
}
