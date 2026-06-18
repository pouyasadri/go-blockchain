package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntToHex(t *testing.T) {
	result := IntToHex(int64(256))
	// 256 in int64 big endian is 00 00 00 00 00 00 01 00
	expected := []byte{0, 0, 0, 0, 0, 0, 1, 0}
	assert.Equal(t, expected, result)
}

func TestReverseBytes(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	ReverseBytes(data)
	assert.Equal(t, []byte{5, 4, 3, 2, 1}, data)
}
