package network

import (
	"bytes"
	"encoding/hex"
	"io"
	"log/slog"
	"net"
	"os"
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

	// Send version command (where foreignerBestHeight > myBestHeight)
	connV1, err := net.Dial("tcp", "localhost:5000")
	assert.NoError(t, err)
	payloadV1 := gobEncode(verzion{Version: 1, BestHeight: bestHeight + 1, AddrFrom: "localhost:5001"})
	reqV1 := append(commandToBytes("version"), payloadV1...)
	_, err = io.Copy(connV1, bytes.NewReader(reqV1))
	assert.NoError(t, err)
	assert.NoError(t, connV1.Close())

	// Send version command (where foreignerBestHeight < myBestHeight)
	connV2, err := net.Dial("tcp", "localhost:5000")
	assert.NoError(t, err)
	payloadV2 := gobEncode(verzion{Version: 1, BestHeight: bestHeight - 1, AddrFrom: "localhost:5001"})
	reqV2 := append(commandToBytes("version"), payloadV2...)
	_, err = io.Copy(connV2, bytes.NewReader(reqV2))
	assert.NoError(t, err)
	assert.NoError(t, connV2.Close())

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

	// Inv command with Type: tx
	connInvTx, err := net.Dial("tcp", "localhost:5000")
	assert.NoError(t, err)
	invTxPayload := gobEncode(inv{AddrFrom: "localhost:5001", Type: "tx", Items: [][]byte{[]byte("dummytxhash")}})
	reqInvTx := append(commandToBytes("inv"), invTxPayload...)
	_, err = io.Copy(connInvTx, bytes.NewReader(reqInvTx))
	assert.NoError(t, err)
	assert.NoError(t, connInvTx.Close())

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

	// Manually populate mempool to avoid async race condition for getdata
	server.mempool[hex.EncodeToString(txn.ID)] = *txn

	// GetData tx command
	connX, err := net.Dial("tcp", "localhost:5000")
	assert.NoError(t, err)
	getDataTxPayload := gobEncode(getdata{AddrFrom: "localhost:5001", Type: "tx", ID: txn.ID})
	reqX := append(commandToBytes("getdata"), getDataTxPayload...)
	_, err = io.Copy(connX, bytes.NewReader(reqX))
	assert.NoError(t, err)
	assert.NoError(t, connX.Close())

	// Block command
	server.blocksInTransit = [][]byte{[]byte("dummy_hash")}
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

func TestSendTxToKnownNodes(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test2.db")

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
	server := NewServer("6000", walletAddr, bc, logger)
	assert.NotNil(t, server)

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

	// 1. empty knownNodes (should be no-op/return)
	server.SendTxToKnownNodes(txn)

	// 2. nodeAddress == knownNodes[0] (should sendInv to all other nodes)
	server.knownNodes = []string{server.nodeAddress, "localhost:5555"}
	server.SendTxToKnownNodes(txn)

	// 3. nodeAddress != knownNodes[0] (should sendTx to knownNodes[0])
	server.knownNodes = []string{"localhost:5555"}
	server.SendTxToKnownNodes(txn)
}

func TestServerDecodingErrors(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test3.db")

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
	server := NewServer("7000", walletAddr, bc, logger)
	assert.NotNil(t, server)

	go server.Start()
	time.Sleep(100 * time.Millisecond)

	commands := []string{"version", "block", "inv", "getblocks", "getdata", "tx"}
	for _, cmd := range commands {
		conn, err := net.Dial("tcp", "localhost:7000")
		assert.NoError(t, err)

		// Send header with corrupted/empty payload
		request := append(commandToBytes(cmd), []byte("corrupted payload")...)
		_, err = io.Copy(conn, bytes.NewReader(request))
		assert.NoError(t, err)
		assert.NoError(t, conn.Close())
	}
	time.Sleep(100 * time.Millisecond)
}

func TestServerHandleTxMining(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test4.db")

	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, db.Close())
	}()

	wallet, _ := core.NewWallet()
	walletAddr := string(wallet.GetAddress())

	bc, err := core.CreateBlockchain(walletAddr, db)
	assert.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	server := NewServer("8000", walletAddr, bc, logger)
	assert.NotNil(t, server)

	// Set mining address and known nodes
	server.miningAddress = walletAddr
	server.knownNodes = []string{"localhost:8001", "localhost:8000"}

	// Send two transaction commands to trigger mining block
	txn1, err := core.NewCoinbaseTX(walletAddr, "coinbase1")
	assert.NoError(t, err)
	txn2, err := core.NewCoinbaseTX(walletAddr, "coinbase2")
	assert.NoError(t, err)

	for _, txn := range []*core.Transaction{txn1, txn2} {
		payload := gobEncode(tx{AddFrom: "localhost:8001", Transaction: txn.Serialize()})
		request := append(commandToBytes("tx"), payload...)
		server.handleTx(request)
	}
}

func TestServerHelpers(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test5.db")

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
	server := NewServer("9000", walletAddr, bc, logger)
	assert.NotNil(t, server)

	// 1. nodeIsKnown true test
	server.knownNodes = []string{"localhost:9000"}
	assert.True(t, server.nodeIsKnown("localhost:9000"))
	assert.False(t, server.nodeIsKnown("localhost:9001"))

	// Start server
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	// 2. Unknown command test
	conn, err := net.Dial("tcp", "localhost:9000")
	assert.NoError(t, err)
	req := append(commandToBytes("unknown"), []byte{}...)
	_, err = io.Copy(conn, bytes.NewReader(req))
	assert.NoError(t, err)
	assert.NoError(t, conn.Close())
	time.Sleep(100 * time.Millisecond)

	// 3. Listen failing when port is in use
	server2 := NewServer("9000", walletAddr, bc, logger)
	server2.Start() // should log error and return immediately
}

func TestGobEncodeError(t *testing.T) {
	// gob cannot encode channels, so this should trigger gobEncode error path
	data := gobEncode(make(chan int))
	assert.Nil(t, data)
}

func TestServerExtraCoverage(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "extra.db")
	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)

	wallet, _ := core.NewWallet()
	walletAddr := string(wallet.GetAddress())
	bc, err := core.CreateBlockchain(walletAddr, db)
	assert.NoError(t, err)

	// 1. NewServer with nil logger
	server := NewServer("8500", walletAddr, bc, nil)
	assert.NotNil(t, server)

	// 2. handleInv with multiple items to trigger the blockHash exclusion loop
	invPayload := gobEncode(inv{AddrFrom: "localhost:8501", Type: "block", Items: [][]byte{[]byte("hash1"), []byte("hash2")}})
	req := append(commandToBytes("inv"), invPayload...)
	server.handleInv(req)

	// 3. handleGetData with nonexistent block
	getDataPayload := gobEncode(getdata{AddrFrom: "localhost:8501", Type: "block", ID: []byte("nonexistent_block")})
	req2 := append(commandToBytes("getdata"), getDataPayload...)
	server.handleGetData(req2)

	// 4. handleBlock with invalid block data (failed to deserialize block)
	blockPayload := gobEncode(block{AddrFrom: "localhost:8501", Block: []byte("corrupted_block_data")})
	req3 := append(commandToBytes("block"), blockPayload...)
	server.handleBlock(req3)

	// 5. Close database to trigger GetBestHeight / AddBlock / GetBlock errors
	assert.NoError(t, db.Close())

	// 6. sendVersion error path
	server.sendVersion("localhost:8501")

	// 7. handleVersion error path (GetBestHeight fails)
	versionPayload := gobEncode(verzion{Version: 1, BestHeight: 10, AddrFrom: "localhost:8501"})
	req4 := append(commandToBytes("version"), versionPayload...)
	server.handleVersion(req4)

	// 8. handleBlock AddBlock fails
	cbTx, err := core.NewCoinbaseTX(walletAddr, "test")
	assert.NoError(t, err)
	validBlock := core.NewBlock([]*core.Transaction{cbTx}, []byte("prevhash"), 1)
	blockPayload2 := gobEncode(block{AddrFrom: "localhost:8501", Block: validBlock.Serialize()})
	req5 := append(commandToBytes("block"), blockPayload2...)
	server.handleBlock(req5)
}
