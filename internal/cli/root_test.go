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
	os.Setenv("NODE_ID", "testnode")
	defer os.Unsetenv("NODE_ID")

	// Since CLI hardcodes file creation in current dir, we change dir temporarily
	originalWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalWd)

	out := new(bytes.Buffer)

	// Wait, we need an address to create a blockchain.
	// We'll run createwallet first
	err := Execute([]string{"createwallet"}, out)
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
