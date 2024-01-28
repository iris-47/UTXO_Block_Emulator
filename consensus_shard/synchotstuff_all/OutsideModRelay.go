package synchotstuff_all

import (
	"blockEmulator/core"
	"blockEmulator/message"
	"encoding/json"
	"log"
)

// This module used in the blockChain using transaction relaying mechanism.
// "Raw" means that the pbft only make block consensus.
type RawUTXORelayOutsideModule struct {
	node *SyncHotstuffConsensusNode
}

// msgType canbe defined in message
func (rrom *RawUTXORelayOutsideModule) HandleMessageOutsideConsensus(msgType message.MessageType, content []byte) bool {
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
	rrom.node.pl.Nlog.Printf("S%dN%d : has received relay txs from shard %d, the senderSeq is %d\n", rrom.node.ShardID, rrom.node.NodeID, relay.SenderShardID, relay.SenderSeq)
	newTxs := []*core.UTXOTransaction{}
	for _, tx := range relay.Txs {
		for _, out := range tx.Vout {
			coinbaseTx := core.NewCoinbaseTX(out.PubKeyHash, out.Value, tx.TxHash)
			newTxs = append(newTxs, coinbaseTx)
		}
	}
	rrom.node.CurChain.UTXOTxpool.AddTxs2Pool(newTxs)
	rrom.node.seqMapLock.Lock()
	rrom.node.seqIDMap[relay.SenderShardID] = relay.SenderSeq
	rrom.node.seqMapLock.Unlock()
	rrom.node.pl.Nlog.Printf("S%dN%d : has handled %d relay txs msg\n", rrom.node.ShardID, rrom.node.NodeID, len(relay.Txs))
}

func (rrom *RawUTXORelayOutsideModule) handleInjectTx(content []byte) {
	it := new(message.InjectTxs)
	err := json.Unmarshal(content, it)
	if err != nil {
		log.Panic(err)
	}
	rrom.node.CurChain.Txpool.AddTxs2Pool(it.Txs)
	rrom.node.pl.Nlog.Printf("S%dN%d : has been injected %d txs\n", rrom.node.ShardID, rrom.node.NodeID, len(it.Txs))
}
