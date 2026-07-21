package network

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/pouyasadri/go-blockchain/api/proto"
	"github.com/pouyasadri/go-blockchain/internal/core"
	"github.com/pouyasadri/go-blockchain/internal/storage/bolt"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestGRPCServer(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "test.db")

	db, err := bolt.Open(dbFile)
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	// 1. Setup blockchain and P2P Server
	authorityWallet, _ := core.NewWallet()
	authorityAddr := string(authorityWallet.GetAddress())

	bc, err := core.CreateBlockchain(authorityAddr, db)
	assert.NoError(t, err)

	utxoSet := core.UTXOSet{Blockchain: bc}
	err = utxoSet.Reindex()
	assert.NoError(t, err)

	p2pServer := NewServer("7000", authorityAddr, bc, nil)

	// Create authority key for PoA minting
	curve := elliptic.P256()
	authKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	assert.NoError(t, err)

	xBytes := make([]byte, 32)
	yBytes := make([]byte, 32)
	authKey.X.FillBytes(xBytes)
	authKey.Y.FillBytes(yBytes)
	pubKeyBytes := append(xBytes, yBytes...)

	core.SetAuthorizedKeys([][]byte{pubKeyBytes})

	// 2. Start gRPC Server in background
	grpcSrv := NewGRPCServer(p2pServer, authKey)

	// Find free port dynamically
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping test due to network bind restriction in sandbox: %v", err)
		return
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	go func() {
		_ = grpcSrv.Start(addr)
	}()
	defer grpcSrv.Stop()

	// Wait briefly for server to boot
	time.Sleep(100 * time.Millisecond)

	// 3. Establish gRPC client connection
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer func() {
		_ = conn.Close()
	}()

	client := proto.NewNodeServiceClient(conn)

	// 4. Test MintStableBalance
	recipientWallet, _ := core.NewWallet()
	recipientAddr := string(recipientWallet.GetAddress())

	mintRes, err := client.MintStableBalance(context.Background(), &proto.MintRequest{
		Address: recipientAddr,
		Amount:  50,
	})
	assert.NoError(t, err)
	assert.True(t, mintRes.Success)
	assert.NotEmpty(t, mintRes.TxId)

	// Verify the minted balance using core UTXOSet
	recipientPubKeyHash := core.HashPubKey(recipientWallet.PublicKey)
	utxos, err := utxoSet.FindUTXO(recipientPubKeyHash)
	assert.NoError(t, err)
	assert.Len(t, utxos, 1)
	assert.Equal(t, int64(50), utxos[0].Value)

	// 5. Test StreamNewBlocks
	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()

	stream, err := client.StreamNewBlocks(streamCtx, &proto.StreamBlocksRequest{
		StartHeight: 1, // Start streaming from first block
	})
	assert.NoError(t, err)

	// In the background, mine another block so we get an event
	go func() {
		time.Sleep(100 * time.Millisecond)
		cb, _ := core.NewCoinbaseTX(authorityAddr, "New event block")
		blk, _ := bc.MineBlock(context.Background(), []*core.Transaction{cb})
		_ = bc.AddBlock(blk)
		p2pServer.notifyBlock(blk)
	}()

	// Wait and read block events from stream
	event, err := stream.Recv()
	assert.NoError(t, err)
	assert.NotNil(t, event)
	assert.Equal(t, int32(1), event.Block.Height) // First block streamed was block height 1 (historical)

	event2, err := stream.Recv()
	assert.NoError(t, err)
	assert.NotNil(t, event2)
	assert.Equal(t, int32(2), event2.Block.Height) // Second block was the newly mined block

	// 6. Test SubmitTransaction
	// Create another receiver
	receiverWallet, _ := core.NewWallet()
	receiverAddr := string(receiverWallet.GetAddress())

	// Create and sign a transaction spending the minted outputs from recipientWallet
	// We need to re-index recipientWallet UTXO first
	tx, err := core.NewUTXOTransaction(recipientWallet, receiverAddr, 10, &utxoSet)
	assert.NoError(t, err)

	protoTx := CoreTxToProto(tx)
	submitRes, err := client.SubmitTransaction(context.Background(), &proto.SubmitTransactionRequest{
		Transaction: protoTx,
	})
	assert.NoError(t, err)
	assert.True(t, submitRes.Success)
	assert.Equal(t, hex.EncodeToString(tx.ID), submitRes.TxId)

	// Verify transaction is in server's mempool
	snapshot := p2pServer.MempoolSnapshot()
	assert.Len(t, snapshot, 1)
	assert.Equal(t, tx.ID, snapshot[0].ID)
}
