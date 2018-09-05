package main

import (
	"crypto/rand"
	"math"
	"math/big"

	plogger "github.com/heytribe/go-plogger"
)

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" // 52 possibilities
	letterIdxBits = 6                                                      // 6 bits to represent 64 possibilities / indexes
	letterIdxMask = 1<<letterIdxBits - 1                                   // All 1-bits, as many as letterIdxBits
)

func randString(length int) string {
	var err error

	result := make([]byte, length)
	bufferSize := int(float64(length) * 1.3)
	for i, j, randomBytes := 0, 0, []byte{}; i < length; j++ {
		if j%bufferSize == 0 {
			randomBytes, err = SecureRandomBytes(bufferSize)
			if err != nil {
				plogger.New().OnError(err, "could not generate random bytes with SecureRandomBytes")
				return ""
			}
		}
		if idx := int(randomBytes[j%length] & letterIdxMask); idx < len(letterBytes) {
			result[i] = letterBytes[idx]
			i++
		}
	}
	return string(result)
}

// SecureRandomBytes returns the requested number of bytes using crypto/rand
func SecureRandomBytes(length int) ([]byte, error) {
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	return randomBytes, err
}

func randInt64() int64 {
	max := big.NewInt(math.MaxInt64)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		plogger.New().OnError(err, "could not generate random int")
		return 0
	}
	return n.Int64()
}

func randUint64() uint64 {
	max := big.NewInt(math.MaxInt64)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		plogger.New().OnError(err, "could not generate random int")
		return 0
	}
	return n.Uint64()
}

func randUint32() uint32 {
	return uint32(randUint64())
}

func randUint16() uint16 {
	return uint16(randUint64())
}
