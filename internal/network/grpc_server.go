package network

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/pouyasadri/go-blockchain/api/proto"
	"github.com/pouyasadri/go-blockchain/internal/core"
	"google.golang.org/grpc"
)

// GRPCServer wraps the gRPC NodeService server
type GRPCServer struct {
	proto.UnimplementedNodeServiceServer
	p2pServer    *Server
	authorityKey *ecdsa.PrivateKey
	grpcServer   *grpc.Server
	listener     net.Listener
}

// NewGRPCServer creates a new GRPCServer instance
func NewGRPCServer(p2pServer *Server, authorityKey *ecdsa.PrivateKey) *GRPCServer {
	return &GRPCServer{
		p2pServer:    p2pServer,
		authorityKey: authorityKey,
	}
}

// Start starts the gRPC server on the specified port (blocking)
func (s *GRPCServer) Start(port string) error {
	var err error
	s.listener, err = net.Listen("tcp", port)
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	s.grpcServer = grpc.NewServer(
		grpc.MaxRecvMsgSize(core.MaxTransactionSizeLimitBytes),
	)
	proto.RegisterNodeServiceServer(s.grpcServer, s)

	s.p2pServer.logger.Info("gRPC server starting", "port", port)
	return s.grpcServer.Serve(s.listener)
}

// Stop gracefully stops the gRPC server
func (s *GRPCServer) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// SubmitTransaction receives a transaction from clients, validates it, and adds to mempool
func (s *GRPCServer) SubmitTransaction(ctx context.Context, req *proto.SubmitTransactionRequest) (*proto.SubmitTransactionResponse, error) {
	if req.Transaction == nil {
		return &proto.SubmitTransactionResponse{
			Success: false,
			Error:   "empty transaction in request",
		}, nil
	}

	tx, err := ProtoTxToCore(req.Transaction)
	if err != nil {
		return &proto.SubmitTransactionResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to parse transaction: %v", err),
		}, nil
	}

	// Double check payload size constraint
	serialized := tx.Serialize()
	if len(serialized) > core.MaxTransactionSizeLimitBytes {
		return &proto.SubmitTransactionResponse{
			Success: false,
			Error:   fmt.Sprintf("transaction size %d bytes exceeds maximum limit of %d bytes", len(serialized), core.MaxTransactionSizeLimitBytes),
		}, nil
	}

	// Validate value constraint
	for _, out := range tx.Vout {
		if out.Value > core.MaxTransactionValueLimit {
			return &proto.SubmitTransactionResponse{
				Success: false,
				Error:   fmt.Sprintf("transaction output value %d exceeds maximum limit of %d", out.Value, core.MaxTransactionValueLimit),
			}, nil
		}
	}

	// Submit to mempool (VerifyTransaction + Mempool add + P2P broadcast)
	err = s.p2pServer.SubmitTransaction(tx)
	if err != nil {
		return &proto.SubmitTransactionResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &proto.SubmitTransactionResponse{
		Success: true,
		TxId:    hex.EncodeToString(tx.ID),
	}, nil
}

// StreamNewBlocks streams real-time blocks to subscribers
func (s *GRPCServer) StreamNewBlocks(req *proto.StreamBlocksRequest, stream proto.NodeService_StreamNewBlocksServer) error {
	// 1. Deliver historical blocks if start_height is specified
	if req.StartHeight > 0 {
		var historical []*core.Block
		for b := range s.p2pServer.bc.Blocks() {
			if int64(b.Height) >= req.StartHeight {
				historical = append(historical, b)
			}
		}
		// Send them in chronological order
		for i := len(historical) - 1; i >= 0; i-- {
			pb := CoreBlockToProto(historical[i])
			if err := stream.Send(&proto.BlockEvent{Block: pb}); err != nil {
				return err
			}
		}
	}

	// 2. Stream new blocks in real-time
	ch := s.p2pServer.SubscribeBlocks()
	defer s.p2pServer.UnsubscribeBlocks(ch)

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case b, ok := <-ch:
			if !ok {
				return nil
			}
			if int64(b.Height) < req.StartHeight {
				continue
			}
			pb := CoreBlockToProto(b)
			if err := stream.Send(&proto.BlockEvent{Block: pb}); err != nil {
				return err
			}
		}
	}
}

// MintStableBalance creates a PoA block containing a coinbase-like mint transaction
func (s *GRPCServer) MintStableBalance(ctx context.Context, req *proto.MintRequest) (*proto.MintResponse, error) {
	if s.authorityKey == nil {
		return &proto.MintResponse{
			Success: false,
			Error:   "node is not configured with an authority key for PoA mining",
		}, nil
	}

	if req.Amount <= 0 {
		return &proto.MintResponse{
			Success: false,
			Error:   "mint amount must be positive",
		}, nil
	}

	if req.Amount > core.MaxTransactionValueLimit {
		return &proto.MintResponse{
			Success: false,
			Error:   "mint amount exceeds maximum allowed limit",
		}, nil
	}

	if !core.ValidateAddress(req.Address) {
		return &proto.MintResponse{
			Success: false,
			Error:   "invalid target address",
		}, nil
	}

	// Build a coinbase-like minting transaction
	txin := core.TXInput{
		Txid:          []byte{},
		Vout:          -1,
		Signature:     nil,
		PubKey:        []byte(fmt.Sprintf("Mint %d coins to %s", req.Amount, req.Address)),
		EscrowWitness: nil,
		IsRefund:      false,
	}
	txout := core.NewTXOutput(req.Amount, req.Address)
	tx := core.Transaction{
		ID:   nil,
		Vin:  []core.TXInput{txin},
		Vout: []core.TXOutput{*txout},
	}
	tx.ID = tx.Hash()

	// Mine a PoA block containing only the mint transaction
	bc := s.p2pServer.bc
	lastHash, err := bc.DB().GetTip()
	if err != nil {
		return &proto.MintResponse{
			Success: false,
			Error:   fmt.Errorf("failed to retrieve blockchain tip: %w", err).Error(),
		}, nil
	}

	lastBlockData, err := bc.DB().GetBlock(lastHash)
	if err != nil {
		return &proto.MintResponse{
			Success: false,
			Error:   fmt.Errorf("failed to retrieve last block: %w", err).Error(),
		}, nil
	}

	lastBlock, err := core.DeserializeBlock(lastBlockData)
	if err != nil {
		return &proto.MintResponse{
			Success: false,
			Error:   fmt.Errorf("failed to deserialize last block: %w", err).Error(),
		}, nil
	}

	// We rely on the authority key being pre-registered in core.AuthorizedKeys
	// We no longer dynamically add it here to prevent privilege escalation.

	newBlock, err := core.NewBlockPoA(
		[]*core.Transaction{&tx},
		lastHash,
		lastBlock.Height+1,
		lastBlock.Bits,
		s.authorityKey,
	)
	if err != nil {
		return &proto.MintResponse{
			Success: false,
			Error:   fmt.Errorf("failed to produce PoA block: %w", err).Error(),
		}, nil
	}

	// Save block to blockchain
	err = bc.AddBlock(newBlock)
	if err != nil {
		return &proto.MintResponse{
			Success: false,
			Error:   fmt.Errorf("failed to add PoA block: %w", err).Error(),
		}, nil
	}

	// Notify real-time block subscribers
	s.p2pServer.notifyBlock(newBlock)

	// Rebuild UTXO index
	utxoSet := core.UTXOSet{Blockchain: bc}
	err = utxoSet.Reindex()
	if err != nil {
		return &proto.MintResponse{
			Success: false,
			Error:   fmt.Errorf("failed to reindex UTXO set: %w", err).Error(),
		}, nil
	}

	// Broadcast block to P2P peers
	for _, node := range s.p2pServer.GetKnownNodes() {
		s.p2pServer.sendInv(node, "block", [][]byte{newBlock.Hash})
	}

	return &proto.MintResponse{
		Success: true,
		TxId:    hex.EncodeToString(tx.ID),
	}, nil
}

// GetUTXOs returns unspent transaction outputs for a given address
func (s *GRPCServer) GetUTXOs(ctx context.Context, req *proto.UTXORequest) (*proto.UTXOResponse, error) {
	if req.Address == "" || !core.ValidateAddress(req.Address) {
		return &proto.UTXOResponse{}, fmt.Errorf("invalid address")
	}

	pubKeyHash := core.Base58Decode([]byte(req.Address))
	if len(pubKeyHash) > 4 {
		pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-core.AddressChecksumLen]
	}

	utxoSet := core.UTXOSet{Blockchain: s.p2pServer.bc}
	items, err := utxoSet.FindUTXOItems(pubKeyHash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch utxo items: %w", err)
	}

	var protoItems []*proto.UTXOOutputItem
	for _, item := range items {
		protoItems = append(protoItems, &proto.UTXOOutputItem{
			Txid:            item.TxID,
			Vout:            int32(item.Vout),
			Value:           item.Output.Value,
			PubKeyHash:      item.Output.PubKeyHash,
			ScriptType:      uint32(item.Output.ScriptType),
			DataHashLock:    item.Output.DataHashLock,
			TimeoutBlock:    item.Output.TimeoutBlock,
			BuyerPubKeyHash: item.Output.BuyerPubKeyHash,
		})
	}

	return &proto.UTXOResponse{Utxos: protoItems}, nil
}

// GetBlockByHash retrieves a block by its hash
func (s *GRPCServer) GetBlockByHash(ctx context.Context, req *proto.BlockByHashRequest) (*proto.BlockResponse, error) {
	if len(req.Hash) == 0 {
		return &proto.BlockResponse{Found: false}, nil
	}

	blockData, err := s.p2pServer.bc.DB().GetBlock(req.Hash)
	if err != nil {
		return &proto.BlockResponse{Found: false}, nil
	}

	block, err := core.DeserializeBlock(blockData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize block: %w", err)
	}

	return &proto.BlockResponse{
		Block: CoreBlockToProto(block),
		Found: true,
	}, nil
}

// GetBestHeight returns the current blockchain height
func (s *GRPCServer) GetBestHeight(ctx context.Context, req *proto.HeightRequest) (*proto.HeightResponse, error) {
	height, err := s.p2pServer.bc.GetBestHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get best height: %w", err)
	}
	return &proto.HeightResponse{Height: int32(height)}, nil
}

// GetBalance returns the spendable balance for an address
func (s *GRPCServer) GetBalance(ctx context.Context, req *proto.BalanceRequest) (*proto.BalanceResponse, error) {
	if req.Address == "" || !core.ValidateAddress(req.Address) {
		return &proto.BalanceResponse{Balance: 0}, nil
	}

	pubKeyHash := core.Base58Decode([]byte(req.Address))
	if len(pubKeyHash) > 4 {
		pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-core.AddressChecksumLen]
	}

	utxoSet := core.UTXOSet{Blockchain: s.p2pServer.bc}
	utxos, err := utxoSet.FindUTXO(pubKeyHash)
	if err != nil {
		return nil, fmt.Errorf("failed to query balance: %w", err)
	}

	var balance int64 = 0
	for _, out := range utxos {
		if out.ScriptType != core.ScriptTypeEscrow {
			balance += out.Value
		}
	}

	return &proto.BalanceResponse{Balance: balance}, nil
}
