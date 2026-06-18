package network

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/pouyasadri/go-blockchain/internal/core"
)

const protocol = "tcp"
const nodeVersion = 1
const commandLength = 12

type addr struct {
	AddrList []string
}

type block struct {
	AddrFrom string
	Block    []byte
}

type getblocks struct {
	AddrFrom string
}

type getdata struct {
	AddrFrom string
	Type     string
	ID       []byte
}

type inv struct {
	AddrFrom string
	Type     string
	Items    [][]byte
}

type tx struct {
	AddFrom     string
	Transaction []byte
}

type verzion struct {
	Version    int
	BestHeight int
	AddrFrom   string
}

// Server represents a P2P node in the network
type Server struct {
	mu              sync.RWMutex
	nodeAddress     string
	miningAddress   string
	knownNodes      []string
	blocksInTransit [][]byte
	mempool         map[string]core.Transaction
	bc              *core.Blockchain
	logger          *slog.Logger
}

// NewServer initializes a new Server
func NewServer(nodeID, minerAddress string, bc *core.Blockchain, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	return &Server{
		nodeAddress:   fmt.Sprintf("localhost:%s", nodeID),
		miningAddress: minerAddress,
		knownNodes:    []string{"localhost:3000"},
		mempool:       make(map[string]core.Transaction),
		bc:            bc,
		logger:        logger,
	}
}

func commandToBytes(command string) []byte {
	var b [commandLength]byte
	for i, c := range command {
		b[i] = byte(c)
	}
	return b[:]
}

func bytesToCommand(b []byte) string {
	var command []byte
	for _, bt := range b {
		if bt != 0x0 {
			command = append(command, bt)
		}
	}
	return string(command)
}

func (s *Server) requestBlocks() {
	for _, node := range s.knownNodes {
		s.sendGetBlocks(node)
	}
}

func (s *Server) sendBlock(addr string, b *core.Block) {
	data := block{AddrFrom: s.nodeAddress, Block: b.Serialize()}
	payload := gobEncode(data)
	request := append(commandToBytes("block"), payload...)

	s.sendData(addr, request)
}

func (s *Server) sendData(addr string, data []byte) {
	conn, err := net.Dial(protocol, addr)
	if err != nil {
		s.logger.Warn("node is not available", "addr", addr)
		var updatedNodes []string
		for _, node := range s.knownNodes {
			if node != addr {
				updatedNodes = append(updatedNodes, node)
			}
		}
		s.knownNodes = updatedNodes
		return
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			s.logger.Error("failed to close connection", "error", cerr)
		}
	}()

	_, err = io.Copy(conn, bytes.NewReader(data))
	if err != nil {
		s.logger.Error("failed to send data", "addr", addr, "error", err)
	}
}

func (s *Server) sendInv(address, kind string, items [][]byte) {
	inventory := inv{AddrFrom: s.nodeAddress, Type: kind, Items: items}
	payload := gobEncode(inventory)
	request := append(commandToBytes("inv"), payload...)

	s.sendData(address, request)
}

func (s *Server) sendGetBlocks(address string) {
	payload := gobEncode(getblocks{AddrFrom: s.nodeAddress})
	request := append(commandToBytes("getblocks"), payload...)

	s.sendData(address, request)
}

func (s *Server) sendGetData(address, kind string, id []byte) {
	payload := gobEncode(getdata{AddrFrom: s.nodeAddress, Type: kind, ID: id})
	request := append(commandToBytes("getdata"), payload...)

	s.sendData(address, request)
}

func (s *Server) sendTx(addr string, tnx *core.Transaction) {
	data := tx{AddFrom: s.nodeAddress, Transaction: tnx.Serialize()}
	payload := gobEncode(data)
	request := append(commandToBytes("tx"), payload...)

	s.sendData(addr, request)
}

func (s *Server) sendVersion(addr string) {
	bestHeight, err := s.bc.GetBestHeight()
	if err != nil {
		s.logger.Error("failed to get best height", "error", err)
		return
	}
	payload := gobEncode(verzion{Version: nodeVersion, BestHeight: bestHeight, AddrFrom: s.nodeAddress})
	request := append(commandToBytes("version"), payload...)

	s.sendData(addr, request)
}

func (s *Server) handleAddr(request []byte) {
	var buff bytes.Buffer
	var payload addr

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		s.logger.Error("failed to decode addr payload", "error", err)
		return
	}

	s.mu.Lock()
	// Deduplicate before appending to guard against unbounded peer list growth.
	for _, newAddr := range payload.AddrList {
		if !s.nodeIsKnownLocked(newAddr) {
			s.knownNodes = append(s.knownNodes, newAddr)
		}
	}
	count := len(s.knownNodes)
	s.mu.Unlock()

	s.logger.Info("known nodes updated", "count", count)
	s.requestBlocks()
}

func (s *Server) handleBlock(request []byte) {
	var buff bytes.Buffer
	var payload block

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		s.logger.Error("failed to decode block payload", "error", err)
		return
	}

	blockData := payload.Block
	b, err := core.DeserializeBlock(blockData)
	if err != nil {
		s.logger.Error("failed to deserialize block", "error", err)
		return
	}

	s.logger.Info("received a new block", "hash", fmt.Sprintf("%x", b.Hash))
	err = s.bc.AddBlock(b)
	if err != nil {
		s.logger.Error("failed to add block", "error", err)
		return
	}

	s.mu.Lock()
	if len(s.blocksInTransit) > 0 {
		blockHash := s.blocksInTransit[0]
		s.blocksInTransit = s.blocksInTransit[1:]
		s.mu.Unlock()
		s.sendGetData(payload.AddrFrom, "block", blockHash)
	} else {
		s.mu.Unlock()
		UTXOSet := core.UTXOSet{Blockchain: s.bc}
		if rerr := UTXOSet.Reindex(); rerr != nil {
			s.logger.Error("failed to reindex utxo set", "error", rerr)
		}
	}
}

func (s *Server) handleInv(request []byte) {
	var buff bytes.Buffer
	var payload inv

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		s.logger.Error("failed to decode inv payload", "error", err)
		return
	}

	s.logger.Info("received inventory", "count", len(payload.Items), "type", payload.Type)

	if payload.Type == "block" && len(payload.Items) > 0 {
		s.mu.Lock()
		s.blocksInTransit = payload.Items
		blockHash := payload.Items[0]
		var newInTransit [][]byte
		for _, b := range s.blocksInTransit {
			if !bytes.Equal(b, blockHash) {
				newInTransit = append(newInTransit, b)
			}
		}
		s.blocksInTransit = newInTransit
		s.mu.Unlock()
		s.sendGetData(payload.AddrFrom, "block", blockHash)
	}

	if payload.Type == "tx" && len(payload.Items) > 0 {
		txID := payload.Items[0]
		s.mu.RLock()
		_, inPool := s.mempool[hex.EncodeToString(txID)]
		s.mu.RUnlock()
		if !inPool {
			s.sendGetData(payload.AddrFrom, "tx", txID)
		}
	}
}

func (s *Server) handleGetBlocks(request []byte) {
	var buff bytes.Buffer
	var payload getblocks

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		s.logger.Error("failed to decode getblocks payload", "error", err)
		return
	}

	blocks := s.bc.GetBlockHashes()
	s.sendInv(payload.AddrFrom, "block", blocks)
}

func (s *Server) handleGetData(request []byte) {
	var buff bytes.Buffer
	var payload getdata

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		s.logger.Error("failed to decode getdata payload", "error", err)
		return
	}

	if payload.Type == "block" {
		block, err := s.bc.GetBlock(payload.ID)
		if err != nil {
			s.logger.Error("failed to get block", "error", err)
			return
		}

		s.sendBlock(payload.AddrFrom, block)
	}

	if payload.Type == "tx" {
		txID := hex.EncodeToString(payload.ID)
		tx, ok := s.mempool[txID]
		if ok {
			s.sendTx(payload.AddrFrom, &tx)
		}
	}
}

func (s *Server) handleTx(request []byte) {
	var buff bytes.Buffer
	var payload tx

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		s.logger.Error("failed to decode tx payload", "error", err)
		return
	}

	txData := payload.Transaction
	newTx, err := core.DeserializeTransaction(txData)
	if err != nil {
		s.logger.Error("failed to deserialize tx", "error", err)
		return
	}

	s.mu.Lock()
	s.mempool[hex.EncodeToString(newTx.ID)] = newTx
	s.mu.Unlock()

	s.mu.RLock()
	isFirstNode := len(s.knownNodes) > 0 && s.nodeAddress == s.knownNodes[0]
	s.mu.RUnlock()

	if isFirstNode {
		s.mu.RLock()
		nodes := make([]string, len(s.knownNodes))
		copy(nodes, s.knownNodes)
		s.mu.RUnlock()
		for _, node := range nodes {
			if node != s.nodeAddress && node != payload.AddFrom {
				s.sendInv(node, "tx", [][]byte{newTx.ID})
			}
		}
		return
	}

	// Mining node: attempt to mine when mempool reaches threshold.
	for {
		s.mu.RLock()
		mempoolSize := len(s.mempool)
		miningAddr := s.miningAddress
		nodesLen := len(s.knownNodes)
		s.mu.RUnlock()

		if mempoolSize < 2 || len(miningAddr) == 0 || nodesLen == 0 {
			break
		}

		var txs []*core.Transaction
		s.mu.RLock()
		for id := range s.mempool {
			txn := s.mempool[id]
			valid, _ := s.bc.VerifyTransaction(&txn)
			if valid {
				txs = append(txs, &txn)
			}
		}
		s.mu.RUnlock()

		if len(txs) == 0 {
			s.logger.Info("all transactions are invalid! waiting for new ones...")
			break
		}

		cbTx, err := core.NewCoinbaseTX(miningAddr, "")
		if err != nil {
			s.logger.Error("failed to create coinbase tx", "error", err)
			break
		}
		txs = append(txs, cbTx)

		newBlock, err := s.bc.MineBlock(txs)
		if err != nil {
			s.logger.Error("failed to mine block", "error", err)
			break
		}
		UTXOSet := core.UTXOSet{Blockchain: s.bc}
		if rerr := UTXOSet.Reindex(); rerr != nil {
			s.logger.Error("failed to reindex utxoset after mining", "error", rerr)
		}

		s.logger.Info("new block is mined!", "hash", fmt.Sprintf("%x", newBlock.Hash))

		s.mu.Lock()
		for _, t := range txs {
			delete(s.mempool, hex.EncodeToString(t.ID))
		}
		s.mu.Unlock()

		s.mu.RLock()
		nodes := make([]string, len(s.knownNodes))
		copy(nodes, s.knownNodes)
		s.mu.RUnlock()
		for _, node := range nodes {
			if node != s.nodeAddress {
				s.sendInv(node, "block", [][]byte{newBlock.Hash})
			}
		}
	}
}

func (s *Server) handleVersion(request []byte) {
	var buff bytes.Buffer
	var payload verzion

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		s.logger.Error("failed to decode version payload", "error", err)
		return
	}

	myBestHeight, err := s.bc.GetBestHeight()
	if err != nil {
		s.logger.Error("failed to get best height", "error", err)
		return
	}
	foreignerBestHeight := payload.BestHeight

	if myBestHeight < foreignerBestHeight {
		s.sendGetBlocks(payload.AddrFrom)
	} else if myBestHeight > foreignerBestHeight {
		s.sendVersion(payload.AddrFrom)
	}

	s.mu.Lock()
	if !s.nodeIsKnownLocked(payload.AddrFrom) {
		s.knownNodes = append(s.knownNodes, payload.AddrFrom)
	}
	s.mu.Unlock()
}

func (s *Server) handleConnection(conn net.Conn) {
	// Protect against memory exhaustion: cap incoming message size at 32 MB.
	const maxMsgSize = 32 << 20
	request, err := io.ReadAll(io.LimitReader(conn, maxMsgSize))
	if err != nil {
		s.logger.Error("failed to read from connection", "error", err)
		return
	}
	if len(request) < commandLength {
		s.logger.Warn("received message too short to contain a command")
		return
	}
	command := bytesToCommand(request[:commandLength])
	s.logger.Debug("received command", "command", command)

	switch command {
	case "addr":
		s.handleAddr(request)
	case "block":
		s.handleBlock(request)
	case "inv":
		s.handleInv(request)
	case "getblocks":
		s.handleGetBlocks(request)
	case "getdata":
		s.handleGetData(request)
	case "tx":
		s.handleTx(request)
	case "version":
		s.handleVersion(request)
	default:
		s.logger.Warn("unknown command received", "command", command)
	}

	if cerr := conn.Close(); cerr != nil {
		s.logger.Error("failed to close connection", "error", cerr)
	}
}

// Start starts the P2P server and blocks until context is cancelled
func (s *Server) Start(ctx context.Context) {
	ln, err := net.Listen(protocol, s.nodeAddress)
	if err != nil {
		s.logger.Error("failed to start server", "error", err)
		return
	}
	defer func() {
		if cerr := ln.Close(); cerr != nil {
			s.logger.Error("failed to close listener", "error", cerr)
		}
	}()

	if len(s.knownNodes) > 0 && s.nodeAddress != s.knownNodes[0] {
		s.sendVersion(s.knownNodes[0])
	}

	s.logger.Info("server started", "address", s.nodeAddress)

	go func() {
		<-ctx.Done()
		s.logger.Info("shutting down server...")
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.logger.Error("failed to accept connection", "error", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func gobEncode(data any) []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		// Log and return empty to prevent panic
		slog.Error("gob encoding failed", "error", err)
		return nil
	}

	return buff.Bytes()
}

// nodeIsKnownLocked checks if addr is in knownNodes. Must be called with s.mu held.
func (s *Server) nodeIsKnownLocked(addr string) bool {
	for _, node := range s.knownNodes {
		if node == addr {
			return true
		}
	}
	return false
}

// nodeIsKnown checks if addr is in knownNodes (thread-safe).
func (s *Server) nodeIsKnown(addr string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nodeIsKnownLocked(addr)
}

// SendTxToKnownNodes sends a transaction to all known nodes
func (s *Server) SendTxToKnownNodes(tx *core.Transaction) {
	s.mu.RLock()
	if len(s.knownNodes) == 0 {
		s.mu.RUnlock()
		return
	}
	isFirstNode := s.nodeAddress == s.knownNodes[0]
	nodes := make([]string, len(s.knownNodes))
	copy(nodes, s.knownNodes)
	s.mu.RUnlock()

	if isFirstNode {
		for _, node := range nodes {
			if node != s.nodeAddress {
				s.sendInv(node, "tx", [][]byte{tx.ID})
			}
		}
	} else {
		s.sendTx(nodes[0], tx)
	}
}
