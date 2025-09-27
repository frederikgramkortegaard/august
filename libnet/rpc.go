package libnet

import (
	"august/blockchain"
	"august/types"
	"august/config"
	"august/libnet/protobuf"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"log"
)

const RPCHeadersRequest = "/august/rpc/headersreq/1.0.0"
const RPCHeadersResponse = "/august/rpc/headersresp/1.0.0"
const RPCBlocksRequest = "/august/rpc/blocksreq/1.0.0"
const RPCBlocksResponse = "/august/rpc/blocksresp/1.0.0"

type RPCProtocol struct {
	Host *P2PHost
}

func NewRPCProtocol(h *P2PHost) *RPCProtocol {
	rpc := RPCProtocol{h}
	h.Host.SetStreamHandler(RPCHeadersRequest, rpc.onHeadersRequest)
	h.Host.SetStreamHandler(RPCHeadersResponse, rpc.onHeadersResponse)

	h.Host.SetStreamHandler(RPCBlocksRequest, rpc.onBlocksRequest)
	h.Host.SetStreamHandler(RPCBlocksResponse, rpc.onBlocksResponse)
	return &rpc
}

/* HEADERS */

func (rpc *RPCProtocol) onHeadersRequest(s network.Stream) {
	defer s.Close()

	data := &protobuf.HeadersRequest{}
	if err := ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	log.Printf("Received Headers Request from %s",
		s.Conn().RemotePeer())

	// Attempt to find the headers
	var hash32Headers []blockchain.Hash32

	for _, h := range data.Hashes {
		var hash [32]byte
		copy(hash[:], h)
		hash32Headers = append(hash32Headers, hash)
	}

	headers := rpc.Host.Node.GetHeaders(hash32Headers)
	log.Print("test", hash32Headers)
	log.Print("test", headers)

	var headersData []*protobuf.HeaderData

	for _, h := range headers {
		headersData = append(headersData, BlockHeaderToProtoHeader(h))
	}

	headerResponseData := &protobuf.HeadersResponse{
		MessageData: rpc.Host.NewMessageData(data.MessageData.MessageID),
		HeaderData:  headersData,
	}

	id, _ := peer.Decode(data.MessageData.SenderID)
	log.Println("Sending RPCHeadersResponse to Peer", data.MessageData.SenderID)
	rpc.Host.sendProtoMessage(id, RPCHeadersResponse, headerResponseData)

}

func (rpc *RPCProtocol) RequestHeaders(messageID string, hashes []blockchain.Hash32) {

	peers := rpc.Host.Host.Network().Peers()

	// Convert to [][]byte
	hashBytes := make([][]byte, len(hashes))
	for i, h := range hashes {
		hashBytes[i] = h[:] // convert [32]byte to []byte
	}

	headerRequestData := &protobuf.HeadersRequest{
		MessageData: rpc.Host.NewMessageData(messageID),
		Hashes:      hashBytes,
	}

	// Randomly subsample up to 'config.maxPeersForRequest' peers
	// and request headers for them all.
	maxPeers := config.MaxPeersForRequest
	subsample := samplePeers(peers, maxPeers)

	for _, peer := range subsample {
		log.Println("Sending RPCHeadersRequest to Peer", peer)
		rpc.Host.sendProtoMessage(peer, RPCHeadersRequest, headerRequestData)
	}

}

func (rpc *RPCProtocol) onHeadersResponse(s network.Stream) {
	defer s.Close()

	data := &protobuf.HeadersResponse{}
	if err := ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	rpc.Host.PendingRequestsMu.Lock()
	log.Printf("Received Headers Response with message id %s", data.MessageData.MessageID)
	rpc.Host.PendingRequests[data.MessageData.MessageID] = append(rpc.Host.PendingRequests[data.MessageData.MessageID], data)
	rpc.Host.PendingRequestsMu.Unlock()
	rpc.Host.NotifyMessageWaiters(data.MessageData.MessageID)

}

/* BLOCKS */

func (rpc *RPCProtocol) RequestBlocks(messageID string, hashes []blockchain.Hash32) {

	peers := rpc.Host.Host.Network().Peers()

	// Convert to [][]byte
	hashBytes := make([][]byte, len(hashes))
	for i, h := range hashes {
		hashBytes[i] = h[:] // convert [32]byte to []byte
	}

	blockRequestData := &protobuf.BlocksRequest{
		MessageData: rpc.Host.NewMessageData(messageID),
		Hashes:      hashBytes,
	}

	// Randomly subsample up to 'config.maxPeersForRequest' peers
	// and request headers for them all.
	maxPeers := config.MaxPeersForRequest
	subsample := samplePeers(peers, maxPeers)

	for _, peer := range subsample {
		log.Println("Sending RPCBlocksRequest to Peer", peer)
		rpc.Host.sendProtoMessage(peer, RPCBlocksRequest, blockRequestData)
	}
}
func (rpc *RPCProtocol) onBlocksRequest(s network.Stream){

	defer s.Close()

	data := &protobuf.BlocksRequest{}
	if err := ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	log.Printf("Received Blocks Request from %s",
		s.Conn().RemotePeer())

	// Attempt to find the headers
	var hash32Blocks []blockchain.Hash32

	for _, h := range data.Hashes {
		var hash [32]byte
		copy(hash[:], h)
		hash32Blocks = append(hash32Blocks, hash)
	}

	blocks := rpc.Host.Node.GetBlocks(hash32Blocks)
	log.Print("testblocks", hash32Blocks)
	log.Print("testblocks", blocks)

	var blocksData []*protobuf.BlockData

	for _, h := range blocks {
		blocksData = append(blocksData, BlockToProtoBlock(h))
	}

	blockResponseData := &protobuf.BlocksResponse{
		MessageData: rpc.Host.NewMessageData(data.MessageData.MessageID),
		BlockData:  blocksData,
	}

	id, _ := peer.Decode(data.MessageData.SenderID)
	log.Println("Sending RPCBlockssResponse to Peer", data.MessageData.SenderID)
	rpc.Host.sendProtoMessage(id, RPCBlocksResponse, blockResponseData)
}
func (rpc *RPCProtocol) onBlocksResponse(s network.Stream){
	defer s.Close()

	data := &protobuf.BlocksResponse{}
	if err := ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	rpc.Host.PendingRequestsMu.Lock()
	log.Printf("Received Blocks Response with message id %s", data.MessageData.MessageID)
	rpc.Host.PendingRequests[data.MessageData.MessageID] = append(rpc.Host.PendingRequests[data.MessageData.MessageID], data)
	rpc.Host.PendingRequestsMu.Unlock()
	rpc.Host.NotifyMessageWaiters(data.MessageData.MessageID)
}

/* CONVERSION FUNCTIONS */

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
	var callInstructions []*protobuf.InstructionData
	for _, instr := range tx.Instructions {
		callInstructions = append(callInstructions, InstructionToProtoInstruction(instr))
	}

	var initInstructions []*protobuf.InstructionData
	for _, instr := range tx.InitInstructions {
		initInstructions = append(initInstructions, InstructionToProtoInstruction(instr))
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

	var callInstructions []blockchain.Instruction
	for _, protoInstr := range protoTx.CallInstructions {
		callInstructions = append(callInstructions, ProtoInstructionToInstruction(protoInstr))
	}

	var initInstructions []blockchain.Instruction
	for _, protoInstr := range protoTx.InitInstructions {
		initInstructions = append(initInstructions, ProtoInstructionToInstruction(protoInstr))
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
	var transactions []*protobuf.TransactionData
	for _, tx := range block.Transactions {
		transactions = append(transactions, TransactionToProtoTransaction(tx))
	}

	return &protobuf.BlockData{
		Header:       BlockHeaderToProtoHeader(&block.Header),
		Transactions: transactions,
	}
}

func ProtoBlockToBlock(protoBlock *protobuf.BlockData) *blockchain.Block {
	var transactions []blockchain.Transaction
	for _, protoTx := range protoBlock.Transactions {
		transactions = append(transactions, ProtoTransactionToTransaction(protoTx))
	}

	return &blockchain.Block{
		Header:       *ProtoHeaderToBlockHeader(protoBlock.Header),
		Transactions: transactions,
	}
}
