package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecuteCreateBlockchain(t *testing.T) {
	dir := t.TempDir()

	// Temporarily override NODE_ID
	t.Setenv("NODE_ID", "testnode")

	// Since CLI hardcodes file creation in current dir, we change dir temporarily
	originalWd, err := os.Getwd()
	assert.NoError(t, err)
	err = os.Chdir(dir)
	assert.NoError(t, err)
	defer func() {
		err := os.Chdir(originalWd)
		assert.NoError(t, err)
	}()

	out := new(bytes.Buffer)

	// Wait, we need an address to create a blockchain.
	// We'll run createwallet first
	err = Execute([]string{"createwallet"}, out)
	assert.NoError(t, err)

	// Output format: "Your new address: <addr>\n"
	lines := strings.Split(strings.TrimSpace(out.String()), ": ")
	assert.Len(t, lines, 2)
	address := lines[1]

	out.Reset()
	err = Execute([]string{"createblockchain", "--address", address}, out)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Done!")

	// ReindexUTXO
	out.Reset()
	err = Execute([]string{"reindexutxo"}, out)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Done!")

	// GetBalance
	out.Reset()
	err = Execute([]string{"getbalance", "--address", address}, out)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Balance of")

	// PrintChain
	out.Reset()
	err = Execute([]string{"printchain"}, out)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "============ Block")
}
