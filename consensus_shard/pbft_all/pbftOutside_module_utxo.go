package pbft_all

import (
	"blockEmulator/core"
	"blockEmulator/message"
	"encoding/json"
	"log"
)

// This module used in the blockChain using transaction relaying mechanism.
// "Raw" means that the pbft only make block consensus.
type RawUTXORelayOutsideModule struct {
	pbftNode *PbftConsensusNode
}

// msgType canbe defined in message
func (rrom *RawUTXORelayOutsideModule) HandleMessageOutsidePBFT(msgType message.MessageType, content []byte) bool {
	switch msgType {
	case message.CRelay:
		rrom.handleRelay(content)
	case message.CInject:
		rrom.handleInjectTx(content)
	default:
	}
	return true
}

// receive relay transaction, which is for cross shard txs
func (rrom *RawUTXORelayOutsideModule) handleRelay(content []byte) {
	relay := new(message.RelayUTXO)
	err := json.Unmarshal(content, relay)
	if err != nil {
		log.Panic(err)
	}
	rrom.pbftNode.pl.Plog.Printf("S%dN%d : has received relay txs from shard %d, the senderSeq is %d\n", rrom.pbftNode.ShardID, rrom.pbftNode.NodeID, relay.SenderShardID, relay.SenderSeq)
	newTxs := []*core.UTXOTransaction{}
	for _, tx := range relay.Txs {
		for _, out := range tx.Vout {
			coinbaseTx := core.NewCoinbaseTX(out.PubKeyHash, out.Value)
			newTxs = append(newTxs, coinbaseTx)
		}
	}
	rrom.pbftNode.CurChain.UTXOTxpool.AddTxs2Pool(newTxs)
	rrom.pbftNode.seqMapLock.Lock()
	rrom.pbftNode.seqIDMap[relay.SenderShardID] = relay.SenderSeq
	rrom.pbftNode.seqMapLock.Unlock()
	rrom.pbftNode.pl.Plog.Printf("S%dN%d : has handled relay txs msg\n", rrom.pbftNode.ShardID, rrom.pbftNode.NodeID)
}

func (rrom *RawUTXORelayOutsideModule) handleInjectTx(content []byte) {
	it := new(message.InjectTxs)
	err := json.Unmarshal(content, it)
	if err != nil {
		log.Panic(err)
	}
	txs := []*core.UTXOTransaction{}
	for _, InjectTx := range it.Txs {
		tx, coinbase := rrom.pbftNode.CurChain.NewUTXOTransaction(InjectTx.Sender, InjectTx.Recipient, InjectTx.Value)
		if coinbase != nil { // coinbase
			txs = append(txs, coinbase)
			txs = append(txs, tx)
			rrom.pbftNode.CurChain.AddTx2UTXOSet(tx)
		} else if tx == nil { // insufficient
			continue
		} else { // normal
			txs = append(txs, tx)
			rrom.pbftNode.CurChain.AddTx2UTXOSet(tx)
		}
	}

	rrom.pbftNode.CurChain.UTXOTxpool.AddTxs2Pool(txs)
	rrom.pbftNode.pl.Plog.Printf("S%dN%d : has handled injected txs msg, txs: %d \n", rrom.pbftNode.ShardID, rrom.pbftNode.NodeID, len(txs))
}
