package core

import (
	"bytes"
	"encoding/binary"
)

// IntToHex converts an int64 to a byte array
func IntToHex(num int64) []byte {
	buff := new(bytes.Buffer)
	// binary.Write to bytes.Buffer with valid type (int64) will never fail
	_ = binary.Write(buff, binary.BigEndian, num)
	return buff.Bytes()
}

// ReverseBytes reverses a byte array
func ReverseBytes(data []byte) {
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
}
