package network

import (
	"github.com/pouyasadri/go-blockchain/api/proto"
	"github.com/pouyasadri/go-blockchain/internal/core"
)

// CoreTxToProto converts a core transaction to its protobuf counterpart
func CoreTxToProto(tx *core.Transaction) *proto.ProtoTransaction {
	if tx == nil {
		return nil
	}
	vin := make([]*proto.ProtoTXInput, len(tx.Vin))
	for i, in := range tx.Vin {
		vin[i] = &proto.ProtoTXInput{
			Txid:          in.Txid,
			Vout:          int32(in.Vout),
			Signature:     in.Signature,
			PubKey:        in.PubKey,
			EscrowWitness: in.EscrowWitness,
			IsRefund:      in.IsRefund,
		}
	}
	vout := make([]*proto.ProtoTXOutput, len(tx.Vout))
	for i, out := range tx.Vout {
		vout[i] = &proto.ProtoTXOutput{
			Value:           out.Value,
			PubKeyHash:      out.PubKeyHash,
			ScriptType:      uint32(out.ScriptType),
			DataHashLock:    out.DataHashLock,
			TimeoutBlock:    out.TimeoutBlock,
			BuyerPubKeyHash: out.BuyerPubKeyHash,
		}
	}
	return &proto.ProtoTransaction{
		Id:   tx.ID,
		Vin:  vin,
		Vout: vout,
	}
}

// ProtoTxToCore converts a protobuf transaction to its core counterpart
func ProtoTxToCore(ptx *proto.ProtoTransaction) (*core.Transaction, error) {
	if ptx == nil {
		return nil, nil
	}
	vin := make([]core.TXInput, len(ptx.Vin))
	for i, in := range ptx.Vin {
		vin[i] = core.TXInput{
			Txid:          in.Txid,
			Vout:          int(in.Vout),
			Signature:     in.Signature,
			PubKey:        in.PubKey,
			EscrowWitness: in.EscrowWitness,
			IsRefund:      in.IsRefund,
		}
	}
	vout := make([]core.TXOutput, len(ptx.Vout))
	for i, out := range ptx.Vout {
		vout[i] = core.TXOutput{
			Value:           out.Value,
			PubKeyHash:      out.PubKeyHash,
			ScriptType:      core.ScriptType(out.ScriptType),
			DataHashLock:    out.DataHashLock,
			TimeoutBlock:    out.TimeoutBlock,
			BuyerPubKeyHash: out.BuyerPubKeyHash,
		}
	}
	return &core.Transaction{
		ID:   ptx.Id,
		Vin:  vin,
		Vout: vout,
	}, nil
}

// CoreBlockToProto converts a core block to its protobuf counterpart
func CoreBlockToProto(b *core.Block) *proto.ProtoBlock {
	if b == nil {
		return nil
	}
	txs := make([]*proto.ProtoTransaction, len(b.Transactions))
	for i, tx := range b.Transactions {
		txs[i] = CoreTxToProto(tx)
	}
	return &proto.ProtoBlock{
		Timestamp:          b.Timestamp,
		Transactions:       txs,
		PrevBlockHash:      b.PrevBlockHash,
		Hash:               b.Hash,
		Nonce:              int32(b.Nonce),
		Height:             int32(b.Height),
		Bits:               int32(b.Bits),
		AuthoritySignature: b.AuthoritySignature,
		AuthorityPubKey:    b.AuthorityPubKey,
	}
}

// ProtoBlockToCore converts a protobuf block to its core counterpart
func ProtoBlockToCore(pb *proto.ProtoBlock) (*core.Block, error) {
	if pb == nil {
		return nil, nil
	}
	txs := make([]*core.Transaction, len(pb.Transactions))
	for i, ptx := range pb.Transactions {
		tx, err := ProtoTxToCore(ptx)
		if err != nil {
			return nil, err
		}
		txs[i] = tx
	}
	return &core.Block{
		Timestamp:          pb.Timestamp,
		Transactions:       txs,
		PrevBlockHash:      pb.PrevBlockHash,
		Hash:               pb.Hash,
		Nonce:              int(pb.Nonce),
		Height:             int(pb.Height),
		Bits:               int(pb.Bits),
		AuthoritySignature: pb.AuthoritySignature,
		AuthorityPubKey:    pb.AuthorityPubKey,
	}, nil
}
