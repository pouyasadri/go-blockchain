package cli

import (
	"bytes"
	"net"
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

	// Create second wallet
	out.Reset()
	err = Execute([]string{"createwallet"}, out)
	assert.NoError(t, err)
	lines = strings.Split(strings.TrimSpace(out.String()), ": ")
	assert.Len(t, lines, 2)
	address2 := lines[1]

	// Send coins with --mine
	out.Reset()
	err = Execute([]string{"send", "--from", address, "--to", address2, "--amount", "2", "--mine"}, out)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Success!")

	// Send coins without --mine
	out.Reset()
	err = Execute([]string{"send", "--from", address, "--to", address2, "--amount", "1"}, out)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Success!")

	// ListAddresses
	out.Reset()
	err = Execute([]string{"listaddresses"}, out)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), address)
	assert.Contains(t, out.String(), address2)

	// Send coins with insufficient funds
	out.Reset()
	err = Execute([]string{"send", "--from", address2, "--to", address, "--amount", "100", "--mine"}, out)
	assert.Error(t, err)

	// Test startnode with port in use
	// 1. Create a dummy listener on port 9005
	t.Setenv("NODE_ID", "9005")
	ln, err := net.Listen("tcp", "localhost:9005")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, ln.Close())
	}()

	// 2. We need a blockchain DB for nodeID "9005" first
	out.Reset()
	err = Execute([]string{"createblockchain", "--address", address}, out)
	assert.NoError(t, err)

	// 3. Now run startnode
	out.Reset()
	err = Execute([]string{"startnode"}, out)
	assert.NoError(t, err)
}

func TestExecuteCliValidationErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NODE_ID", "testnode")

	originalWd, err := os.Getwd()
	assert.NoError(t, err)
	err = os.Chdir(dir)
	assert.NoError(t, err)
	defer func() {
		err := os.Chdir(originalWd)
		assert.NoError(t, err)
	}()

	out := new(bytes.Buffer)

	// 1. createblockchain missing address
	err = Execute([]string{"createblockchain"}, out)
	assert.NoError(t, err) // Cobra returns nil when displaying help
	assert.Contains(t, out.String(), "Usage:")

	// 2. getbalance missing address
	out.Reset()
	err = Execute([]string{"getbalance"}, out)
	assert.NoError(t, err) // Cobra returns nil when displaying help
	assert.Contains(t, out.String(), "Usage:")

	// 3. getbalance invalid address
	out.Reset()
	err = Execute([]string{"getbalance", "--address", "invalid_address"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Address is not valid")

	// 4. send missing parameters
	out.Reset()
	err = Execute([]string{"send"}, out)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Usage:")

	// 5. send invalid sender address
	out.Reset()
	err = Execute([]string{"send", "--from", "invalid_addr", "--to", "valid_addr", "--amount", "10"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Sender address is not valid")

	// 6. send invalid recipient address
	// Let's create a valid address first to use as sender
	out.Reset()
	err = Execute([]string{"createwallet"}, out)
	assert.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(out.String()), ": ")
	validAddr := lines[1]

	out.Reset()
	err = Execute([]string{"send", "--from", validAddr, "--to", "invalid_addr", "--amount", "10"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Recipient address is not valid")

	// 7. startnode with invalid miner address
	out.Reset()
	err = Execute([]string{"startnode", "--miner", "invalid_miner"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Miner address is not valid")

	// 8. createwallet SaveToFile error
	t.Setenv("NODE_ID", "nonexistent_dir_1234/testnode")
	out.Reset()
	err = Execute([]string{"createwallet"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save wallets")

	// 9. reindexutxo on empty DB failing in NewBlockchain
	t.Setenv("NODE_ID", "empty_node")
	out.Reset()
	err = Execute([]string{"reindexutxo"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open blockchain")

	// 10. unknown subcommand
	out.Reset()
	err = Execute([]string{"unknowncmd"}, out)
	assert.Error(t, err)

	// 11. NODE_ID empty error
	t.Setenv("NODE_ID", "")
	out.Reset()
	err = Execute([]string{"createwallet"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NODE_ID env. var is not set")

	// Restore NODE_ID for other tests
	t.Setenv("NODE_ID", "testnode")

	// 12. createblockchain already exists error
	out.Reset()
	err = Execute([]string{"createwallet"}, out)
	assert.NoError(t, err)
	lines = strings.Split(strings.TrimSpace(out.String()), ": ")
	addr := lines[1]

	out.Reset()
	err = Execute([]string{"createblockchain", "--address", addr}, out)
	assert.NoError(t, err)

	// Call createblockchain again
	out.Reset()
	err = Execute([]string{"createblockchain", "--address", addr}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blockchain already exists")

	// 13. createblockchain bolt.Open fails (invalid path in NODE_ID)
	t.Setenv("NODE_ID", "nonexistent_dir_1234/testnode")
	out.Reset()
	err = Execute([]string{"createblockchain", "--address", addr}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open storage")

	// 14. getbalance bolt.Open fails
	out.Reset()
	err = Execute([]string{"getbalance", "--address", addr}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open storage")

	// 15. getbalance NewBlockchain fails
	t.Setenv("NODE_ID", "empty_node")
	out.Reset()
	err = Execute([]string{"getbalance", "--address", addr}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open blockchain")

	// 16. listaddresses NewWallets fails
	t.Setenv("NODE_ID", "corrupted")
	err = os.WriteFile("wallet_corrupted.dat", []byte("corrupted wallet data"), 0644)
	assert.NoError(t, err)
	out.Reset()
	err = Execute([]string{"listaddresses"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load wallets")

	// 17. printchain bolt.Open fails
	t.Setenv("NODE_ID", "nonexistent_dir_1234/testnode")
	out.Reset()
	err = Execute([]string{"printchain"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open storage")

	// 18. printchain NewBlockchain fails
	t.Setenv("NODE_ID", "empty_node")
	out.Reset()
	err = Execute([]string{"printchain"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open blockchain")

	// 19. reindexutxo bolt.Open fails
	t.Setenv("NODE_ID", "nonexistent_dir_1234/testnode")
	out.Reset()
	err = Execute([]string{"reindexutxo"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open storage")

	// 20. send bolt.Open fails
	out.Reset()
	err = Execute([]string{"send", "--from", addr, "--to", addr, "--amount", "10"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open storage")

	// 21. send NewBlockchain fails
	t.Setenv("NODE_ID", "empty_node")
	out.Reset()
	err = Execute([]string{"send", "--from", addr, "--to", addr, "--amount", "10"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open blockchain")

	// 22. startnode bolt.Open fails
	t.Setenv("NODE_ID", "nonexistent_dir_1234/testnode")
	out.Reset()
	err = Execute([]string{"startnode"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open storage")

	// 23. startnode NewBlockchain fails
	t.Setenv("NODE_ID", "empty_node")
	out.Reset()
	err = Execute([]string{"startnode"}, out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open blockchain")
}
