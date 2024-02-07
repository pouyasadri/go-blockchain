package main

import (
	"bytes"
	"testing"
)

func TestNewBlock(t *testing.T) {
	data := "Test Block"
	prevBlockHash := []byte{}
	block := NewBlock(data, prevBlockHash)

	if string(block.Data) != data {
		t.Errorf("Data in block is incorrect, got: %s, want: %s.", block.Data, data)
	}

	if !bytes.Equal(block.PrevBlockHash, prevBlockHash) {
		t.Errorf("PrevBlockHash in block is incorrect, got: %x, want: %x.", block.PrevBlockHash, prevBlockHash)
	}
}

func TestAddBlock(t *testing.T) {
	bc := NewBlockchain()
	data := "Test Block"
	bc.AddBlock(data)

	if len(bc.blocks) != 2 {
		t.Errorf("Number of blocks in blockchain is incorrect, got: %d, want: %d.", len(bc.blocks), 2)
	}

	if string(bc.blocks[1].Data) != data {
		t.Errorf("Data in last block is incorrect, got: %s, want: %s.", bc.blocks[1].Data, data)
	}
}
