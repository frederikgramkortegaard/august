package libnet

import (
	"august/blockchain"
	"august/libnet/protobuf"
	"august/types"
)

func InstructionToProtoInstruction(instr blockchain.Instruction) *protobuf.InstructionData {
	protoInstr := &protobuf.InstructionData{
		Opcode: []byte{byte(instr.Opcode)},
	}
	if instr.Value != nil {
		protoInstr.Value = []byte(*instr.Value)
	}
	return protoInstr
}

func ProtoInstructionToInstruction(protoInstr *protobuf.InstructionData) blockchain.Instruction {
	instr := blockchain.Instruction{
		Opcode: types.OPCODE(protoInstr.Opcode[0]),
	}
	if len(protoInstr.Value) > 0 {
		valueStr := string(protoInstr.Value)
		instr.Value = &valueStr
	}
	return instr
}

func TransactionToProtoTransaction(tx blockchain.Transaction) *protobuf.TransactionData {
	callInstructions := make([]*protobuf.InstructionData, len(tx.Instructions))
	for i, instr := range tx.Instructions {
		callInstructions[i] = InstructionToProtoInstruction(instr)
	}

	initInstructions := make([]*protobuf.InstructionData, len(tx.InitInstructions))
	for i, instr := range tx.InitInstructions {
		initInstructions[i] = InstructionToProtoInstruction(instr)
	}

	return &protobuf.TransactionData{
		Frompk:           tx.From[:],
		Topk:             tx.To[:],
		Amount:           tx.Amount,
		Signature:        tx.Signature[:],
		Nonce:            tx.Nonce,
		Timestamp:        tx.Timestamp,
		Chainid:          tx.ChainID,
		CallInstructions: callInstructions,
		InitInstructions: initInstructions,
		GasLimit:         tx.GasLimit,
		GasPrice:         tx.GasPrice,
		Data:             tx.Data,
	}
}

func ProtoTransactionToTransaction(protoTx *protobuf.TransactionData) blockchain.Transaction {
	var from, to blockchain.PublicKey
	var sig blockchain.Signature
	copy(from[:], protoTx.Frompk)
	copy(to[:], protoTx.Topk)
	copy(sig[:], protoTx.Signature)

	callInstructions := make([]blockchain.Instruction, len(protoTx.CallInstructions))
	for i, protoInstr := range protoTx.CallInstructions {
		callInstructions[i] = ProtoInstructionToInstruction(protoInstr)
	}

	initInstructions := make([]blockchain.Instruction, len(protoTx.InitInstructions))
	for i, protoInstr := range protoTx.InitInstructions {
		initInstructions[i] = ProtoInstructionToInstruction(protoInstr)
	}

	return blockchain.Transaction{
		From:             from,
		To:               to,
		Amount:           protoTx.Amount,
		Signature:        sig,
		Nonce:            protoTx.Nonce,
		Timestamp:        protoTx.Timestamp,
		ChainID:          protoTx.Chainid,
		Instructions:     callInstructions,
		InitInstructions: initInstructions,
		GasLimit:         protoTx.GasLimit,
		GasPrice:         protoTx.GasPrice,
		Data:             protoTx.Data,
	}
}

func BlockHeaderToProtoHeader(h *blockchain.BlockHeader) *protobuf.HeaderData {
	return &protobuf.HeaderData{
		Version:      h.Version,
		PreviousHash: h.PreviousHash[:],
		Height:       h.Height,
		Timestamp:    h.Timestamp,
		Nonce:        h.Nonce,
		MerkleRoot:   h.MerkleRoot[:],
		StateRoot:    h.StateRoot[:],
		Bits:         h.Bits,
		TotalWork:    h.TotalWork,
		GasLimit:     h.GasLimit,
		GasUsed:      h.GasUsed,
		Beneficiary:  h.Beneficiary[:],
	}
}

func ProtoHeaderToBlockHeader(h *protobuf.HeaderData) *blockchain.BlockHeader {
	var previousHash, merkleRoot, stateRoot, beneficiary [32]byte
	copy(previousHash[:], h.PreviousHash)
	copy(merkleRoot[:], h.MerkleRoot)
	copy(stateRoot[:], h.StateRoot)
	copy(beneficiary[:], h.Beneficiary)

	return &blockchain.BlockHeader{
		Version:      h.Version,
		PreviousHash: previousHash,
		Height:       h.Height,
		Timestamp:    h.Timestamp,
		Nonce:        h.Nonce,
		MerkleRoot:   merkleRoot,
		StateRoot:    stateRoot,
		Bits:         h.Bits,
		TotalWork:    h.TotalWork,
		GasLimit:     h.GasLimit,
		GasUsed:      h.GasUsed,
		Beneficiary:  beneficiary,
	}
}

func BlockToProtoBlock(block *blockchain.Block) *protobuf.BlockData {
	transactions := make([]*protobuf.TransactionData, len(block.Transactions))
	for i, tx := range block.Transactions {
		transactions[i] = TransactionToProtoTransaction(tx)
	}

	return &protobuf.BlockData{
		Header:       BlockHeaderToProtoHeader(&block.Header),
		Transactions: transactions,
	}
}

func ProtoBlockToBlock(protoBlock *protobuf.BlockData) *blockchain.Block {
	transactions := make([]blockchain.Transaction, len(protoBlock.Transactions))
	for i, protoTx := range protoBlock.Transactions {
		transactions[i] = ProtoTransactionToTransaction(protoTx)
	}

	return &blockchain.Block{
		Header:       *ProtoHeaderToBlockHeader(protoBlock.Header),
		Transactions: transactions,
	}
}
