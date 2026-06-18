package network

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/pouyasadri/go-blockchain/internal/core"
	"github.com/pouyasadri/go-blockchain/internal/storage/bolt"
	"github.com/stretchr/testify/assert"
)

func TestServerStartAndCommands(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test.db")

	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, db.Close())
	}()

	wallet, _ := core.NewWallet()
	walletAddr := string(wallet.GetAddress())

	bc, err := core.CreateBlockchain(walletAddr, db)
	assert.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewServer("5000", walletAddr, bc, logger)
	assert.NotNil(t, server)

	// Since Start blocks, we run it in a goroutine
	go server.Start()

	// Wait a moment for the server to start listening
	time.Sleep(100 * time.Millisecond)

	// Connect and send a dummy payload to trigger code paths
	conn, err := net.Dial("tcp", "localhost:5000")
	assert.NoError(t, err)

	// Send version command
	bestHeight, _ := bc.GetBestHeight()
	payload := gobEncode(verzion{Version: 1, BestHeight: bestHeight, AddrFrom: "localhost:5001"})
	request := append(commandToBytes("version"), payload...)

	_, err = io.Copy(conn, bytes.NewReader(request))
	assert.NoError(t, err)
	assert.NoError(t, conn.Close())

	// Another command
	conn2, err := net.Dial("tcp", "localhost:5000")
	assert.NoError(t, err)
	getBlocksPayload := gobEncode(getblocks{AddrFrom: "localhost:5001"})
	req2 := append(commandToBytes("getblocks"), getBlocksPayload...)
	_, err = io.Copy(conn2, bytes.NewReader(req2))
	assert.NoError(t, err)
	assert.NoError(t, conn2.Close())

	// Addr command
	conn3, err := net.Dial("tcp", "localhost:5000")
	assert.NoError(t, err)
	addrPayload := gobEncode(addr{AddrList: []string{"localhost:5002"}})
	req3 := append(commandToBytes("addr"), addrPayload...)
	_, err = io.Copy(conn3, bytes.NewReader(req3))
	assert.NoError(t, err)
	assert.NoError(t, conn3.Close())

	// GetData block command
	conn4, err := net.Dial("tcp", "localhost:5000")
	assert.NoError(t, err)
	hashes := bc.GetBlockHashes()
	if len(hashes) > 0 {
		getDataPayload := gobEncode(getdata{AddrFrom: "localhost:5001", Type: "block", ID: hashes[0]})
		req4 := append(commandToBytes("getdata"), getDataPayload...)
		_, err = io.Copy(conn4, bytes.NewReader(req4))
		assert.NoError(t, err)
	}
	assert.NoError(t, conn4.Close())

	// Inv command
	conn5, err := net.Dial("tcp", "localhost:5000")
	assert.NoError(t, err)
	invPayload := gobEncode(inv{AddrFrom: "localhost:5001", Type: "block", Items: [][]byte{[]byte("dummyhash")}})
	req5 := append(commandToBytes("inv"), invPayload...)
	_, err = io.Copy(conn5, bytes.NewReader(req5))
	assert.NoError(t, err)
	assert.NoError(t, conn5.Close())

	// Tx command
	conn6, err := net.Dial("tcp", "localhost:5000")
	assert.NoError(t, err)
	txin := core.TXInput{
		Txid:      []byte{},
		Vout:      -1,
		Signature: nil,
		PubKey:    wallet.PublicKey,
	}
	txout := core.NewTXOutput(10, walletAddr)
	txn := &core.Transaction{
		ID:   nil,
		Vin:  []core.TXInput{txin},
		Vout: []core.TXOutput{*txout},
	}
	txn.ID = txn.Hash()
	txPayload := gobEncode(tx{AddFrom: "localhost:5001", Transaction: txn.Serialize()})
	req6 := append(commandToBytes("tx"), txPayload...)
	_, err = io.Copy(conn6, bytes.NewReader(req6))
	assert.NoError(t, err)
	assert.NoError(t, conn6.Close())

	// Block command
	conn7, err := net.Dial("tcp", "localhost:5000")
	assert.NoError(t, err)
	newBlock := core.NewBlock([]*core.Transaction{txn}, []byte("prevhash"), 1)
	blockPayload := gobEncode(block{AddrFrom: "localhost:5001", Block: newBlock.Serialize()})
	req7 := append(commandToBytes("block"), blockPayload...)
	_, err = io.Copy(conn7, bytes.NewReader(req7))
	assert.NoError(t, err)
	assert.NoError(t, conn7.Close())

	// Wait for handling
	time.Sleep(200 * time.Millisecond)

	// Since server runs infinitely, we just let the test finish.
	// The listener will close when the test exits.
}
