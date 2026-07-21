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
	testCases := [][]byte{
		{0x00, 0x01, 0x02, 0x03},
		{0x00, 0x00, 0x01, 0x02, 0x03},
		{0x00, 0x00, 0x00, 0x15, 0xab},
		{0x00, 0x00, 0x00},
	}

	for _, data := range testCases {
		encoded := Base58Encode(data)
		decoded := Base58Decode(encoded)
		assert.Equal(t, data, decoded)
	}
}
