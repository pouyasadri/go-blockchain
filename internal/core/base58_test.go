package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBase58EncodeDecode(t *testing.T) {
	data := []byte("hello world")
	encoded := Base58Encode(data)
	decoded := Base58Decode(encoded)

	assert.Equal(t, data, decoded)
}

func TestBase58EncodeDecodeWithZeroByte(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0x03}
	encoded := Base58Encode(data)
	decoded := Base58Decode(encoded)

	// Since decoding might drop leading zeros in the big integer representation,
	// the implementation manually restores the first 0x00 byte if present.
	// We want to ensure it roundtrips correctly.
	assert.Equal(t, data, decoded)
}
