package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMerkleTree(t *testing.T) {
	data := [][]byte{
		[]byte("node1"),
		[]byte("node2"),
		[]byte("node3"),
	}

	tree := NewMerkleTree(data)
	assert.NotNil(t, tree)
	assert.NotNil(t, tree.RootNode)
	assert.NotEmpty(t, tree.RootNode.Data)
}

func TestNewMerkleNode(t *testing.T) {
	node := NewMerkleNode(nil, nil, []byte("data"))
	assert.NotNil(t, node)
	assert.NotEmpty(t, node.Data)

	node2 := NewMerkleNode(nil, nil, []byte("data2"))

	parent := NewMerkleNode(node, node2, nil)
	assert.NotNil(t, parent)
	assert.NotEmpty(t, parent.Data)
	assert.Equal(t, node, parent.Left)
	assert.Equal(t, node2, parent.Right)
}
