package core

import (
	"crypto/sha256"
	"errors"
)

// MerkleTree represent a Merkle tree
type MerkleTree struct {
	RootNode *MerkleNode
}

// MerkleNode represent a Merkle tree node
type MerkleNode struct {
	Left  *MerkleNode
	Right *MerkleNode
	Data  []byte
}

// NewMerkleTree creates a new Merkle tree from a sequence of data.
// It returns nil for empty input. It applies the CVE-2012-2459 second
// pre-image protection: when the leaf count is odd, the last leaf is hashed
// again (double SHA-256) rather than simply duplicated, preventing two
// different transaction sets from producing the same Merkle root.
func NewMerkleTree(data [][]byte) *MerkleTree {
	if len(data) == 0 {
		return nil
	}

	var nodes []MerkleNode
	for _, datum := range data {
		node := NewMerkleNode(nil, nil, datum)
		nodes = append(nodes, *node)
	}

	for len(nodes) > 1 {
		// If the number of nodes at this level is odd, protect against
		// the second pre-image attack by hashing the last node's data again
		// instead of duplicating the raw node pointer.
		if len(nodes)%2 != 0 {
			lastNode := nodes[len(nodes)-1]
			// Double-hash the leaf data to produce a distinct value
			rehashed := sha256.Sum256(lastNode.Data)
			extra := NewMerkleNode(nil, nil, rehashed[:])
			// Manually set the Data so it doesn't re-hash via sha256(rehashed)
			extra.Data = rehashed[:]
			nodes = append(nodes, *extra)
		}

		var newLevel []MerkleNode
		for j := 0; j < len(nodes); j += 2 {
			node := NewMerkleNode(&nodes[j], &nodes[j+1], nil)
			newLevel = append(newLevel, *node)
		}
		nodes = newLevel
	}

	return &MerkleTree{RootNode: &nodes[0]}
}

// NewMerkleNode creates a new Merkle tree node
func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	mNode := MerkleNode{}

	if left == nil && right == nil {
		hash := sha256.Sum256(data)
		mNode.Data = hash[:]
	} else {
		prevHashes := append(left.Data, right.Data...)
		hash := sha256.Sum256(prevHashes)
		mNode.Data = hash[:]
	}

	mNode.Left = left
	mNode.Right = right

	return &mNode
}

// ErrEmptyMerkleTree is returned when building a tree from empty data.
var ErrEmptyMerkleTree = errors.New("cannot build Merkle tree from empty data")
