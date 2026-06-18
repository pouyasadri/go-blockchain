package network

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"

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

func (s *Server) sendAddr(address string) {
	nodes := addr{AddrList: s.knownNodes}
	nodes.AddrList = append(nodes.AddrList, s.nodeAddress)
	payload := gobEncode(nodes)
	request := append(commandToBytes("addr"), payload...)

	s.sendData(address, request)
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
	defer conn.Close()

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

	s.knownNodes = append(s.knownNodes, payload.AddrList...)
	s.logger.Info("known nodes updated", "count", len(s.knownNodes))
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
	block, err := core.DeserializeBlock(blockData)
	if err != nil {
		s.logger.Error("failed to deserialize block", "error", err)
		return
	}

	s.logger.Info("received a new block", "hash", fmt.Sprintf("%x", block.Hash))
	err = s.bc.AddBlock(block)
	if err != nil {
		s.logger.Error("failed to add block", "error", err)
		return
	}

	if len(s.blocksInTransit) > 0 {
		blockHash := s.blocksInTransit[0]
		s.sendGetData(payload.AddrFrom, "block", blockHash)
		s.blocksInTransit = s.blocksInTransit[1:]
	} else {
		UTXOSet := core.UTXOSet{Blockchain: s.bc}
		err := UTXOSet.Reindex()
		if err != nil {
			s.logger.Error("failed to reindex utxo set", "error", err)
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

	if payload.Type == "block" {
		s.blocksInTransit = payload.Items

		blockHash := payload.Items[0]
		s.sendGetData(payload.AddrFrom, "block", blockHash)

		var newInTransit [][]byte
		for _, b := range s.blocksInTransit {
			if !bytes.Equal(b, blockHash) {
				newInTransit = append(newInTransit, b)
			}
		}
		s.blocksInTransit = newInTransit
	}

	if payload.Type == "tx" {
		txID := payload.Items[0]
		if _, ok := s.mempool[hex.EncodeToString(txID)]; !ok {
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
	tx, err := core.DeserializeTransaction(txData)
	if err != nil {
		s.logger.Error("failed to deserialize tx", "error", err)
		return
	}
	s.mempool[hex.EncodeToString(tx.ID)] = tx

	if len(s.knownNodes) > 0 && s.nodeAddress == s.knownNodes[0] {
		for _, node := range s.knownNodes {
			if node != s.nodeAddress && node != payload.AddFrom {
				s.sendInv(node, "tx", [][]byte{tx.ID})
			}
		}
	} else {
		if len(s.mempool) >= 2 && len(s.miningAddress) > 0 && len(s.knownNodes) > 0 {
		MineTransactions:
			var txs []*core.Transaction

			for id := range s.mempool {
				txn := s.mempool[id]
				valid, _ := s.bc.VerifyTransaction(&txn)
				if valid {
					txs = append(txs, &txn)
				}
			}

			if len(txs) == 0 {
				s.logger.Info("all transactions are invalid! waiting for new ones...")
				return
			}

			cbTx, err := core.NewCoinbaseTX(s.miningAddress, "")
			if err != nil {
				s.logger.Error("failed to create coinbase tx", "error", err)
				return
			}
			txs = append(txs, cbTx)

			newBlock, err := s.bc.MineBlock(txs)
			if err != nil {
				s.logger.Error("failed to mine block", "error", err)
				return
			}
			UTXOSet := core.UTXOSet{Blockchain: s.bc}
			err = UTXOSet.Reindex()
			if err != nil {
				s.logger.Error("failed to reindex utxoset after mining", "error", err)
			}

			s.logger.Info("new block is mined!", "hash", fmt.Sprintf("%x", newBlock.Hash))

			for _, t := range txs {
				txID := hex.EncodeToString(t.ID)
				delete(s.mempool, txID)
			}

			for _, node := range s.knownNodes {
				if node != s.nodeAddress {
					s.sendInv(node, "block", [][]byte{newBlock.Hash})
				}
			}

			if len(s.mempool) > 0 {
				goto MineTransactions
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

	if !s.nodeIsKnown(payload.AddrFrom) {
		s.knownNodes = append(s.knownNodes, payload.AddrFrom)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	request, err := io.ReadAll(conn)
	if err != nil {
		s.logger.Error("failed to read from connection", "error", err)
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

	conn.Close()
}

// Start starts the P2P server
func (s *Server) Start() {
	ln, err := net.Listen(protocol, s.nodeAddress)
	if err != nil {
		s.logger.Error("failed to start server", "error", err)
		return
	}
	defer ln.Close()

	if len(s.knownNodes) > 0 && s.nodeAddress != s.knownNodes[0] {
		s.sendVersion(s.knownNodes[0])
	}

	s.logger.Info("server started", "address", s.nodeAddress)

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

func (s *Server) nodeIsKnown(addr string) bool {
	for _, node := range s.knownNodes {
		if node == addr {
			return true
		}
	}
	return false
}

func (s *Server) SendTxToKnownNodes(tx *core.Transaction) {
	if len(s.knownNodes) == 0 {
		return
	}
	if s.nodeAddress == s.knownNodes[0] {
		for _, node := range s.knownNodes {
			if node != s.nodeAddress {
				s.sendInv(node, "tx", [][]byte{tx.ID})
			}
		}
	} else {
		s.sendTx(s.knownNodes[0], tx)
	}
}
